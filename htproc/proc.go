package htproc

// todo: test different process exit scenarios

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type lineRead struct {
	line string
	err  error
}

type cleanupStatus struct {
	complete bool
	errors   []error
}

type proc struct {
	user    string
	access  time.Time
	cmd     *exec.Cmd
	started bool
	stdout  chan lineRead
	stderr  chan lineRead
	ready   chan int
	closed  chan int
	exit    chan error
	cleanup chan cleanupStatus
}

const (
	userFlag       = "-user"
	startupMessage = "ready"
	startupTimeout = 3 * time.Second
	exitTimeout    = 3 * time.Second
)

var (
	procClosed       = errors.New("Process closed.")
	unexpectedExit   = errors.New("Process exited unexpectedly.")
	startupTimeouted = errors.New("Process startup timeouted.")
	exitTimeouted    = errors.New("Process exit timeouted.")
)

func newProc(user string) *proc {
	p := new(proc)
	p.user = user
	p.access = time.Now()
	p.ready = make(chan int)
	p.closed = make(chan int)
	p.exit = make(chan error)
	p.cleanup = make(chan cleanupStatus)
	go p.start()
	return p
}

func (p *proc) exitUnstarted(err error) {
	close(p.ready)
	p.exit <- err
	p.cleanup <- cleanupStatus{complete: true}
}

// sends nil, eof or other error
func copyLines(w io.Writer, r io.Reader, l ...string) chan lineRead {
	c := make(chan lineRead)
	go func() {
		ls := make([]string, len(l))
		for i, li := range l {
			if li[len(li)-1] != '\n' {
				li = li + "\n"
			}
			ls[i] = li
		}
		br := bufio.NewReader(r)
		for {
			lr, err := br.ReadString('\n')
			if err != nil {
				c <- lineRead{err: err}
				return
			}
			found := false
			for i, li := range ls {
				if li != lr {
					continue
				}
				found = true
				c <- lineRead{line: l[i]}
			}
			if !found {
				_, err = w.Write([]byte(lr))
				if err != nil {
					c <- lineRead{err: err}
					return
				}
			}
		}
	}()
	return c
}

func (p *proc) signalExit(kill bool) error {
	var s syscall.Signal
	if kill {
		s = syscall.SIGKILL
	} else {
		s = syscall.SIGTERM
	}
	return p.cmd.Process.Signal(s)
}

func waitOutput(skip, lr chan lineRead) error {
	if skip == lr {
		return nil
	}
	l := <-lr
	if l.err == io.EOF {
		return nil
	}
	return l.err
}

func (p *proc) cleanupSocket() error {
	var err error
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (p *proc) waitExit(skip chan lineRead, kill bool) {
	w := make(chan error)
	go func() { w <- p.cmd.Wait() }()
	for {
		select {
		case err := <-w:
			cs := cleanupStatus{complete: true}
			if err != nil {
				cs.complete = false
				cs.errors = append(cs.errors, err)
			}
			if err = waitOutput(skip, p.stdout); err != nil {
				cs.errors = append(cs.errors, err)
			}
			if err = waitOutput(skip, p.stderr); err != nil {
				cs.errors = append(cs.errors, err)
			}
			if err = p.cleanupSocket(); err != nil {
				cs.errors = append(cs.errors, err)
			}
			p.cleanup <- cs
			return
		case <-time.After(exitTimeout):
			if kill {
				p.cleanup <- cleanupStatus{errors: []error{exitTimeouted}}
				return
			}
			kill = true
			if err := p.signalExit(true); err != nil {
				p.cleanup <- cleanupStatus{errors: []error{err}}
				return
			}
		}
	}
}

func (p *proc) checkOutput(o chan lineRead, err error) bool {
	switch err {
	case nil:
		return true
	case io.EOF:
		if !p.started {
			close(p.ready)
		}
		p.exit <- nil
		p.waitExit(o, false)
	default:
		if !p.started {
			close(p.ready)
		}
		p.exit <- err
		if err = p.signalExit(true); err == nil {
			p.waitExit(o, true)
		} else {
			p.cleanup <- cleanupStatus{errors: []error{err}}
		}
	}
	return false
}

func (p *proc) start() {
	p.cmd = exec.Command(os.Args[0], userFlag, p.user)
	var (
		err    error
		so, se io.Reader
	)
	if so, err = p.cmd.StdoutPipe(); err != nil {
		p.exitUnstarted(err)
		return
	}
	if se, err = p.cmd.StderrPipe(); err != nil {
		p.exitUnstarted(err)
		return
	}
	if err = p.cmd.Start(); err != nil {
		p.exitUnstarted(err)
		return
	}
	sto := time.After(startupTimeout)
	p.stdout = copyLines(os.Stdout, so, startupMessage)
	p.stderr = copyLines(os.Stderr, se)
	for {
		select {
		case <-sto:
			if !p.started {
				close(p.ready)
				p.exit <- startupTimeouted
				if err := p.signalExit(true); err != nil {
					p.cleanup <- cleanupStatus{errors: []error{err}}
					return
				}
				p.waitExit(nil, true)
				return
			}
		case l := <-p.stdout:
			if !p.checkOutput(p.stdout, l.err) {
				return
			}
			if !p.started && l.line == startupMessage {
				close(p.ready)
				p.started = true
			}
		case l := <-p.stderr:
			if !p.checkOutput(p.stderr, l.err) {
				return
			}
		case <-p.closed:
			if !p.started {
				close(p.ready)
			}
			p.exit <- nil
			if err := p.signalExit(false); err != nil {
				p.cleanup <- cleanupStatus{errors: []error{err}}
				return
			}
			p.waitExit(nil, false)
			return
		}
	}
}

func (p *proc) serve(w http.ResponseWriter, r *http.Request) error { return nil }
func (p *proc) close()                                             { close(p.closed) }
