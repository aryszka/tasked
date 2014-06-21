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
	cmd     *exec.Cmd
	proxy   server
	stdout  chan lineRead
	stderr  chan lineRead
	ready   chan int
	failure chan int
	exit    chan int
}

const (
	startupTimeoutMs = 3000
	startupTimeout   = startupTimeoutMs * time.Millisecond
	exitTimeout      = 3 * time.Second
)

var (
	startupMessage   = []byte("ready")
	command          = os.Args[0]
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
	/*
	"serve -runas <user> -address <socket>\
		-tls-key "" -tls-cert "" -tls-key-file "" -tls-cert-file ""
		-cachedir "" -proxy "" -proxy-file ""
		-authenticate false -public-user "" -token-validity 0
		-aes-key "" -aes-iv "" -aes-key-file "" -aes-iv-file ""
		-max-request-processes 0 -process-idle-time 0
		-root <current> -allow-cookies <current>
		-maxSearchResults <current> -maxRequestBody <current> -maxRequestHeader <current>
	*/
	args := [os.Args[0], 
	p.proxy = &proxy{address: address, timeout: dialTimeout}
	p.failure = make(chan int)
	p.ready = make(chan int)
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
	close(p.ready)
	return status{errors: []error{err}}
}

func waitOutput(output chan lineRead) error {
	for {
		var (
			l  lineRead
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

func (p *proc) signalWait(signal bool) status {
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

func (p *proc) waitExit(started, signal bool, err ...error) status {
	s := p.signalWait(signal)
	s.errors = append(s.errors, err...)
	if !started {
		close(p.ready)
	}
	return s
}

func (p *proc) outputError(started bool, err error) status {
	if err == io.EOF {
		err = unexpectedExit
	}
	return p.waitExit(started, err != unexpectedExit, err)
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
	for {
		select {
		case <-to:
			if started {
				break
			}
			return p.waitExit(false, true, startupTimeouted)
		case l := <-p.stdout:
			if l.err != nil {
				return p.outputError(started, l.err)
			}
			if !started && bytes.Equal(l.line, startupMessage) {
				started = true
				close(p.ready)
			}
		case l := <-p.stderr:
			if l.err == nil {
				break
			}
			return p.outputError(started, l.err)
		case <-p.failure:
			return p.waitExit(started, true, socketFailure)
		case <-p.exit:
			return p.waitExit(started, true)
		}
	}
}

func (p *proc) serve(w http.ResponseWriter, r *http.Request) error {
	<-p.ready
	select {
	case <-p.exit:
		return procClosed
	default:
		err := p.proxy.serve(w, r)
		if err != nil {
			close(p.failure)
		}
		return err
	}
}

func (p *proc) close() { close(p.exit) }
