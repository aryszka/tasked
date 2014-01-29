package htproc

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type lineRead struct {
	line []byte
	err  error
}

type status struct {
	errors        []error
	cleanupFailed bool // false even when killed
}

type server interface {
	serve(http.ResponseWriter, *http.Request) error
}

type proc struct {
	cmd      *exec.Cmd
	proxy   server
	stdout   chan lineRead
	stderr   chan lineRead
	ready    chan int
	access   chan time.Time
	laccess  chan time.Time
	failure  chan int
	exit     chan int
}

const (
	startupTimeoutMs = 3000
	startupTimeout   = startupTimeoutMs * time.Millisecond
	exitTimeout      = 3 * time.Second
)

var (
	startupMessage   = []byte("ready")
	command          = os.Args[0]
	args             []string
	procClosed       = errors.New("Process closed.")
	unexpectedExit   = errors.New("Process exited unexpectedly.")
	startupTimeouted = errors.New("Process startup timeouted.")
	exitTimeouted    = errors.New("Process exit timeouted.")
	killSignaled     = errors.New("Process kill signaled.")
	socketFailure    = errors.New("Socket failure.")
)

func newProc(address string, dialTimeout time.Duration) *proc {
	p := new(proc)
	p.cmd = exec.Command(command, append(args, "-sock", address)...)
	p.proxy = &proxy{address: address, timeout: dialTimeout}
	p.ready = make(chan int)
	p.access = make(chan time.Time)
	p.laccess = make(chan time.Time)
	p.exit = make(chan int)
	return p
}

func filterLines(w io.Writer, r io.Reader, l ...[]byte) chan lineRead {
	c := make(chan lineRead)
	go func() {
		ls := make([][]byte, len(l))
		for i, li := range l {
			if li[len(li)-1] != '\n' {
				li = append(li, '\n')
			}
			ls[i] = li
		}
		br := bufio.NewReader(r)
		for {
			lr, err := br.ReadSlice('\n')
			if err != nil && err != io.EOF {
				c <- lineRead{err: err}
				close(c)
				return
			}
			found := false
			for i, li := range ls {
				last := len(lr) - 1
				if last >= 0 && lr[last] != '\n' {
					li = li[0 : len(li)-1]
				}
				if bytes.Equal(li, lr) {
					found = true
					c <- lineRead{line: l[i], err: err}
					break
				}
			}
			if !found {
				if _, werr := w.Write([]byte(lr)); werr != nil {
					err = werr
				}
			}
			if err != nil {
				c <- lineRead{err: err}
				close(c)
				return
			}
		}
	}()
	return c
}

func (p *proc) startError(err error) status {
	close(p.laccess)
	close(p.ready)
	return status{errors: []error{err}}
}

func waitOutput(output chan lineRead) error {
	for {
		var (
			l lineRead
			ok bool
		)
		if l, ok = <-output; ok && l.err == nil {
			continue
		}
		if l.err == io.EOF {
			return nil
		}
		return l.err
	}
}

func (p *proc) waitExit(signal bool) status {
	if signal {
		if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return status{cleanupFailed: true, errors: []error{err}}
		}
	}
	w := make(chan status)
	go func() {
		var s status
		if err := p.cmd.Wait(); err != nil {
			xerr, ok := err.(*exec.ExitError)
			switch {
			case !ok:
				s.errors = append(s.errors, err)
				s.cleanupFailed = true
			case !xerr.Exited():
				ws, ok := xerr.Sys().(syscall.WaitStatus)
				s.cleanupFailed = !ok || !ws.Signaled()
				if s.cleanupFailed {
					s.errors = append(s.errors, err)
				}
			}
		}
		if err := waitOutput(p.stdout); err != nil {
			s.errors = append(s.errors, err)
		}
		if err := waitOutput(p.stderr); err != nil {
			s.errors = append(s.errors, err)
		}
		w <- s
	}()
	var kill bool
	for {
		select {
		case s := <-w:
			if kill {
				s.errors = append(s.errors, killSignaled)
			}
			return s
		case <-time.After(exitTimeout):
			if kill {
				return status{cleanupFailed: true, errors: []error{exitTimeouted}}
			}
			kill = true
			if err := p.cmd.Process.Signal(syscall.SIGKILL); err != nil {
				return status{cleanupFailed: true, errors: []error{err}}
			}
		}
	}
}

func (p *proc) outputError(err error) status {
	if err == io.EOF {
		err = unexpectedExit
	}
	close(p.laccess)
	close(p.ready)
	s := p.waitExit(err != unexpectedExit)
	s.errors = append(s.errors, err)
	return s
}

func (p *proc) run() status {
	var (
		err    error
		so, se io.Reader
	)
	if so, err = p.cmd.StdoutPipe(); err != nil {
		return p.startError(err)
	}
	if se, err = p.cmd.StderrPipe(); err != nil {
		return p.startError(err)
	}
	if err = p.cmd.Start(); err != nil {
		return p.startError(err)
	}
	p.stdout = filterLines(os.Stdout, so, startupMessage)
	p.stderr = filterLines(os.Stderr, se)
	to := time.After(startupTimeout)
	started := false
	access := time.Now()
	for {
		select {
		case <-to:
			if started {
				break
			}
			close(p.laccess)
			close(p.ready)
			s := p.waitExit(true)
			s.errors = append(s.errors, startupTimeouted)
			return s
		case l := <-p.stdout:
			if l.err != nil {
				return p.outputError(l.err)
			}
			if !started && bytes.Equal(l.line, startupMessage) {
				started = true
				close(p.ready)
			}
		case l := <-p.stderr:
			if l.err == nil {
				break
			}
			return p.outputError(l.err)
		case access = <-p.access:
		case p.laccess <- access:
		case <-p.failure:
			s := p.waitExit(true)
			s.errors = append(s.errors, socketFailure)
			return s
		case <-p.exit:
			close(p.laccess)
			if !started {
				close(p.ready)
			}
			return p.waitExit(true)
		}
	}
}

func (p *proc) serve(w http.ResponseWriter, r *http.Request) error {
	<-p.ready
	select {
	case <-p.exit:
		return procClosed
	case p.access <- time.Now():
		err := p.proxy.serve(w, r)
		if _, ok := err.(*socketError); ok {
			p.failure <- 0
		}
		return err
	}
}

func (p *proc) accessed() time.Time { return <-p.laccess }
func (p *proc) close() { close(p.exit) }
