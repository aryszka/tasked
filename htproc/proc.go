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

type proc struct {
	cmd      *exec.Cmd
	accessed time.Time
	stdout   chan lineRead
	stderr   chan lineRead
	ready    chan int
	access   chan time.Time
	exit     chan int
}

const (
	startupTimeoutMs = 3000
	startupTimeout   = startupTimeoutMs * time.Millisecond
	exitTimeout      = 3 * time.Second
)

var (
	startupMessage   = []byte("ready")
	procClosed       = errors.New("Process closed.")
	unexpectedExit   = errors.New("Process exited unexpectedly.")
	startupTimeouted = errors.New("Process startup timeouted.")
	exitTimeouted    = errors.New("Process exit timeouted.")
)

func newProc(cmd *exec.Cmd) *proc {
	p := new(proc)
	p.cmd = cmd
	p.accessed = time.Now()
	p.ready = make(chan int)
	p.access = make(chan time.Time)
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
			eof := err == io.EOF
			if err != nil && !eof {
				c <- lineRead{err: err}
				return
			}
			found := false
			for i, li := range ls {
				if eof {
					li = li[0 : len(li)-1]
				}
				if !bytes.Equal(li, lr) {
					continue
				}
				found = true
				c <- lineRead{line: l[i]}
				break
			}
			if !found {
				_, err = w.Write([]byte(lr))
				if err != nil {
					c <- lineRead{err: err}
					return
				}
			}
			if eof {
				c <- lineRead{err: io.EOF}
				return
			}
		}
	}()
	return c
}

func waitOutput(output chan lineRead) error {
	for {
		var l lineRead
		if l = <-output; l.err == nil {
			continue
		}
		if l.err == io.EOF {
			return nil
		}
		return l.err
	}
}

func (p *proc) waitExit() status {
	err := p.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return status{cleanupFailed: true, errors: []error{err}}
	}
	w := make(chan status)
	go func() {
		var (
			err error
			s   status
		)
		if err = p.cmd.Wait(); err != nil {
			s.errors = append(s.errors, err)
			xerr, ok := err.(*exec.ExitError)
			switch {
			case !ok:
				s.cleanupFailed = true
			case !xerr.Exited():
				ws, ok := xerr.Sys().(syscall.WaitStatus)
				s.cleanupFailed = !ok || !ws.Signaled()
			}
		}
		if err = waitOutput(p.stdout); err != nil {
			s.errors = append(s.errors, err)
		}
		if err = waitOutput(p.stderr); err != nil {
			s.errors = append(s.errors, err)
		}
		w <- s
	}()
	var kill bool
	for {
		select {
		case s := <-w:
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
	s := p.waitExit()
	s.errors = append(s.errors, err)
	return s
}

func (p *proc) run() status {
	var (
		err    error
		so, se io.Reader
	)
	if so, err = p.cmd.StdoutPipe(); err != nil {
		close(p.ready)
		return status{errors: []error{err}}
	}
	if se, err = p.cmd.StderrPipe(); err != nil {
		close(p.ready)
		return status{errors: []error{err}}
	}
	if err = p.cmd.Start(); err != nil {
		close(p.ready)
		return status{errors: []error{err}}
	}
	p.stdout = filterLines(os.Stdout, so, startupMessage)
	p.stderr = filterLines(os.Stderr, se)
	to := time.After(startupTimeout)
	started := false
	for {
		select {
		case <-to:
			s := p.waitExit()
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
			if l.err != nil {
				return p.outputError(l.err)
			}
		case t := <-p.access:
			p.accessed = t
		case <-p.exit:
			if !started {
				close(p.ready)
			}
			return p.waitExit()
		}
	}
}

func (p *proc) close() {
	close(p.exit)
}

func (p *proc) serve(w http.ResponseWriter, r *http.Request) error {
	<-p.ready
	select {
	case <-p.exit:
		return procClosed
	default:
		go func() {
			select {
			case <-p.exit:
				return
			default:
				p.access <- time.Now()
			}
		}()
		return nil
	}
}
