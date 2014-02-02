package htproc

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	// "syscall"
	. "code.google.com/p/tasked/testing"
	"fmt"
	"net/http"
	"testing"
	"time"
)

type testReader struct {
	lines     [][]byte
	readIndex int
	err       error
	errIndex  int
}

type testWriter struct {
	lines      [][]byte
	writeIndex int
	err        error
	errIndex   int
}

type testServer struct {
	access       time.Time
	closed       bool
	cleanupFail  bool
	waitForClose bool
	exit         chan int
}

var (
	testuser   = "testuser"
	eto        = 30 * time.Millisecond
	testError  = errors.New("test error")
	testStatus = status{errors: []error{testError}}
)

func (r *testReader) Read(p []byte) (int, error) {
	if len(r.lines) == 0 {
		return 0, io.EOF
	}
	for i := 0; i < len(p); i++ {
		if r.err != nil && r.readIndex == r.errIndex {
			return i, r.err
		}
		r.readIndex++
		if len(r.lines[0]) == 0 {
			p[i] = '\n'
			r.lines = r.lines[1:]
			if len(r.lines) == 0 {
				return i, io.EOF
			}
			continue
		}
		p[i] = r.lines[0][0]
		r.lines[0] = r.lines[0][1:]
	}
	return len(p), nil
}

func (w *testWriter) Write(p []byte) (int, error) {
	if len(w.lines) == 0 {
		w.lines = append(w.lines, nil)
	}
	for i, b := range p {
		if w.err != nil && i == w.errIndex {
			return i, w.err
		}
		if b == '\n' {
			w.lines = append(w.lines, nil)
			continue
		}
		w.lines[len(w.lines)-1] = append(w.lines[len(w.lines)-1], b)
	}
	return len(p), nil
}

func (s *testServer) accessed() time.Time { return s.access }

func (s *testServer) close() {
	if s.exit != nil {
		close(s.exit)
	}
	s.closed = true
}

func (s *testServer) run() status {
	if s.waitForClose {
		s.exit = make(chan int)
		<-s.exit
	}
	ts := testStatus
	if s.cleanupFail {
		ts.cleanupFailed = true
	}
	return ts
}

func (s *testServer) serve(w http.ResponseWriter, r *http.Request) error {
	return testError
}

func linesEqual(l0, l1 [][]byte) bool {
	if len(l0) != len(l1) {
		if len(l0) > len(l1) {
			l0, l1 = l1, l0
		}
		if len(l1)-len(l0) > 1 {
			return false
		}
		if len(l1[len(l1)-1]) > 0 {
			return false
		}
		l1 = l1[:len(l1)-1]
	}
	for i, l := range l0 {
		if !bytes.Equal(l, l1[i]) {
			return false
		}
	}
	return true
}

func lineFeed(l []byte) []byte {
	return append(l, '\n')
}

func TestNewProc(t *testing.T) {
	address := "address"
	to := time.Duration(42)
	p := newProc(address, to)
	if p.cmd == nil || p.proxy == nil {
		t.Fail()
	}
	proxy, ok := p.proxy.(*proxy)
	if !ok || proxy.address != address || proxy.timeout != to {
		t.Fail()
	}
	if p.failure == nil || p.ready == nil || p.exit == nil {
		t.Fail()
	}
}

func TestFilterLines(t *testing.T) {
	one, two, three := []byte("one"), []byte("two"), []byte("three")

	// empty stream, waiting for matching line
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{make([]byte, 0)}}, []byte("some"))
		li := <-lr
		if li.err != io.EOF || !linesEqual(w.lines, make([][]byte, 0)) {
			t.Fail()
		}
		<-lr
	})

	// copy one line
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		l := one
		lr := filterLines(w, &testReader{lines: [][]byte{l}})
		li := <-lr
		if li.err != io.EOF || !linesEqual(w.lines, [][]byte{l}) {
			t.Fail()
		}
		<-lr
	})

	// copy three lines
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}})
		li := <-lr
		if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, two, three}) {
			t.Fail()
		}
		<-lr
	})

	// find a line
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}}, two)
		found := false
		for {
			li := <-lr
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, three}) {
					t.Fail()
				}
				return
			} else {
				if li.err != nil || !bytes.Equal(li.line, two) {
					t.Fail()
				}
				found = true
			}
		}
		<-lr
	})

	// find last line
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}}, three)
		found := false
		for {
			li := <-lr
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, two}) {
					t.Fail()
				}
				return
			} else {
				if !bytes.Equal(li.line, three) {
					t.Fail()
				}
				found = true
			}
		}
		<-lr
	})

	// find a line, with line-break
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}}, lineFeed(two))
		found := false
		for {
			li := <-lr
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, three}) {
					t.Fail()
				}
				return
			} else {
				if !bytes.Equal(li.line, lineFeed(two)) {
					t.Fail()
				}
				found = true
			}
		}
		<-lr
	})

	// find multiple lines
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}}, lineFeed(two), three)
		var rl [][]byte
		for {
			li := <-lr
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != io.EOF {
					t.Fail()
				}
				if !linesEqual(w.lines, [][]byte{one}) || !linesEqual(rl, [][]byte{lineFeed(two), three}) {
					t.Fail()
				}
				return
			}
		}
		<-lr
	})

	// detect non-eof read error, immediately
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError})
		for {
			li := <-lr
			if li.err != testError ||
				!linesEqual(w.lines, nil) {
				t.Fail()
			}
			return
		}
		<-lr
	})

	// detect non-eof read error
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 4})
		for {
			li := <-lr
			if li.err != testError ||
				!linesEqual(w.lines, [][]byte{one}) {
				t.Fail()
			}
			return
		}
		<-lr
	})

	// detect non-eof read error, last
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 7})
		for {
			li := <-lr
			if li.err != testError ||
				!linesEqual(w.lines, [][]byte{one}) && !linesEqual(w.lines, [][]byte{one, two}) {
				t.Fail()
			}
			return
		}
		<-lr
	})

	// detect non-eof read error, before found
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 1}, two)
		var rl [][]byte
		for {
			li := <-lr
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if !linesEqual(rl, nil) {
					t.Fail()
				}
				return
			}
		}
		<-lr
	})

	// detect non-eof read error, after found
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 6}, two)
		var rl [][]byte
		for {
			li := <-lr
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if !linesEqual(w.lines, [][]byte{one}) ||
					!linesEqual(rl, nil) && !linesEqual(rl, [][]byte{two}) {
					t.Fail()
				}
				return
			}
		}
		<-lr
	})

	// detect write error
	WithTimeout(t, eto, func() {
		w := new(testWriter)
		w.err = testError
		w.errIndex = 4
		lr := filterLines(w, &testReader{lines: [][]byte{one, two, three}}, two)
		var rl [][]byte
		for {
			li := <-lr
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if len(w.lines) == 0 || !bytes.Equal(w.lines[0], one) ||
					!linesEqual(rl, nil) && !linesEqual(rl, [][]byte{two}) {
					t.Fail()
				}
				return
			}
		}
		<-lr
	})
}

func TestStartError(t *testing.T) {
	p := &proc{ready: make(chan int)}
	e := errors.New("testerror")
	s := p.startError(e)
	WithTimeout(t, eto, func() { <-p.ready })
	if len(s.errors) != 1 || s.errors[0] != e {
		t.Fail()
	}
}

func TestWaitOutput(t *testing.T) {
	var err error
	c := make(chan lineRead)

	// error immediately
	go func() {
		c <- lineRead{err: testError}
	}()
	err = waitOutput(c)
	if err != testError {
		t.Fail()
	}

	// eof immediately
	go func() {
		c <- lineRead{err: io.EOF}
	}()
	err = waitOutput(c)
	if err != nil {
		t.Fail()
	}

	// error after read
	go func() {
		c <- lineRead{}
		c <- lineRead{err: testError}
	}()
	err = waitOutput(c)
	if err != testError {
		t.Fail()
	}

	// eof after read
	go func() {
		c <- lineRead{}
		c <- lineRead{err: io.EOF}
	}()
	err = waitOutput(c)
	if err != nil {
		t.Fail()
	}

	// waiting on closed channel
	WithTimeout(t, eto, func() {
		close(c)
		err = waitOutput(c)
		if err != nil {
			t.Fail()
		}
	})
}

func TestSignalWait(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	var (
		p *proc
	)

	// no signal
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	ErrFatal(t, p.cmd.Start())
	WithTimeout(t, eto, func() {
		s := p.signalWait(false)
		if s.cleanupFailed || len(s.errors) > 0 {
			t.Fail()
		}
	})

	// send signal
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait")
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	ErrFatal(t, p.cmd.Start())
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if s.cleanupFailed || len(s.errors) > 0 {
			t.Fail()
		}
	})

	// sigterm fail
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	ErrFatal(t, p.cmd.Run())
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if !s.cleanupFailed || len(s.errors) == 0 {
			t.Fail()
		}
	})

	// sigterm
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if s.cleanupFailed || len(s.errors) != 0 {
			t.Fail()
		}
	})

	// sigterm to done
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	ErrFatal(t, p.cmd.Run())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if !s.cleanupFailed || len(s.errors) != 1 {
			t.Fail()
		}
	})

	// sigkill, no sigterm
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, 2*exitTimeout, func() {
		s := p.signalWait(false)
		if s.cleanupFailed || len(s.errors) != 1 || s.errors[0] != killSignaled {
			t.Fail()
		}
	})

	// sigkill, after sigterm
	p = new(proc)
	p.cmd = exec.Command("testproc", "gulpterm")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, 2*exitTimeout, func() {
		time.Sleep(120 * time.Millisecond)
		s := p.signalWait(true)
		if s.cleanupFailed || len(s.errors) != 1 || s.errors[0] != killSignaled {
			t.Fail()
		}
	})

	// stdout err
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: testError} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if s.cleanupFailed {
			t.Fail()
		}
		if len(s.errors) != 1 || s.errors[0] != testError {
			t.Fail()
		}
	})

	// stderr err
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: testError} }()
	WithTimeout(t, eto, func() {
		s := p.signalWait(true)
		if s.cleanupFailed {
			t.Fail()
		}
		if len(s.errors) != 1 || s.errors[0] != testError {
			t.Fail()
		}
	})
}

func TestWaitExit(t *testing.T) {
	// nothing
	p := new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.ready = make(chan int)
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.waitExit(true, false)
		if len(s.errors) != 0 {
			t.Fail()
		}
		close(p.ready)
	})

	// append error
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.ready = make(chan int)
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.waitExit(true, false, testError)
		if len(s.errors) != 1 || s.errors[0] != testError {
			t.Fail()
		}
		close(p.ready)
	})

	// started
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.ready = make(chan int)
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.waitExit(false, false)
		if len(s.errors) != 0 {
			t.Fail()
		}
		_, open := <-p.ready
		if open {
			t.Fail()
		}
	})
}

func TestOutputError(t *testing.T) {
	// eof replaced
	p := new(proc)
	p.cmd = exec.Command("testproc", "noop")
	ErrFatal(t, p.cmd.Start())
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	WithTimeout(t, eto, func() {
		s := p.outputError(true, io.EOF)
		if len(s.errors) != 1 || s.errors[0] != unexpectedExit {
			t.Fail()
		}
	})
}

func TestProcRun(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	// start fail
	p := new(proc)
	p.ready = make(chan int)
	p.cmd = exec.Command("")
	s := p.run()
	if s.cleanupFailed || len(s.errors) != 1 {
		t.Fail()
	}

	// start timeout
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", fmt.Sprint(startupTimeoutMs*2))
	p.ready = make(chan int)
	WithTimeout(t, 2*startupTimeout, func() {
		s = p.run()
		if s.cleanupFailed || len(s.errors) != 1 || s.errors[0] != startupTimeouted {
			t.Fail()
		}
	})

	// exit before timeout
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", fmt.Sprint(startupTimeoutMs/2))
	p.ready = make(chan int)
	WithTimeout(t, 2*startupTimeout, func() {
		s = p.run()
		if s.cleanupFailed || len(s.errors) != 1 || s.errors[0] != unexpectedExit {
			t.Fail()
		}
	})

	// run, exit
	p = new(proc)
	p.cmd = exec.Command("testproc", "printwait", "4500", "some message", string(startupMessage), "")
	p.ready = make(chan int)
	p.exit = make(chan int)
	WithTimeout(t, startupTimeout+exitTimeout, func() {
		w := Wait(func() {
			s = p.run()
			if s.cleanupFailed || len(s.errors) != 0 {
				t.Fail()
			}
		})
		<-p.ready
		p.close()
		<-w
	})

	// start message twice
	p = new(proc)
	p.cmd = exec.Command("testproc", "printwait", "4500", string(startupMessage), string(startupMessage))
	p.ready = make(chan int)
	p.exit = make(chan int)
	WithTimeout(t, startupTimeout, func() {
		w := Wait(func() {
			s = p.run()
			if s.cleanupFailed || len(s.errors) != 0 {
				t.Fail()
			}
		})
		<-p.ready
		p.close()
		<-w
	})

	// close before started
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	p.ready = make(chan int)
	p.exit = make(chan int)
	WithTimeout(t, startupTimeout, func() {
		w := Wait(func() {
			s = p.run()
			if s.cleanupFailed || len(s.errors) != 0 {
				t.Fail()
			}
		})
		p.close()
		<-p.ready
		<-w
	})

	// failure
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	p.ready = make(chan int)
	p.failure = make(chan int)
	WithTimeout(t, startupTimeout, func() {
		w := Wait(func() {
			s = p.run()
			if s.cleanupFailed || len(s.errors) != 1 || s.errors[0] != socketFailure {
				t.Fail()
			}
		})
		close(p.failure)
		<-p.ready
		<-w
	})
}

func TestServe(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	// serve
	p := new(proc)
	p.ready = make(chan int)
	p.failure = make(chan int)
	p.proxy = new(testServer)
	WithTimeout(t, startupTimeout, func() {
		w := Wait(func() {
			err := p.serve(nil, nil)
			if err != testError {
				t.Fail()
			}
		})
		close(p.ready)
		<-p.failure
		<-w
	})

	// exit
	p = new(proc)
	p.ready = make(chan int)
	p.failure = make(chan int)
	p.exit = make(chan int)
	WithTimeout(t, startupTimeout, func() {
		w := Wait(func() {
			err := p.serve(nil, nil)
			if err != procClosed {
				t.Fail()
			}
		})
		close(p.exit)
		close(p.ready)
		<-w
	})
}
