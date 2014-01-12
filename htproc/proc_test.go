package htproc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"syscall"
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

var (
	testuser  = "testuser"
	eto       = 30 * time.Millisecond
	testError = errors.New("test error")
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
	t0 := time.Now()
	p := newProc(testuser)
	t1 := time.Now()
	if p.user != testuser {
		t.Fail()
	}
	if p.accessed.Before(t0) || p.accessed.After(t1) {
		t.Fail()
	}
}

func TestFilterLines(t *testing.T) {
	one, two, three := []byte("one"), []byte("two"), []byte("three")
	// copy one line
	w := new(testWriter)
	l := one
	lr := filterLines(w, &testReader{lines: [][]byte{l}})
	select {
	case li := <-lr:
		if li.err != io.EOF || !linesEqual(w.lines, [][]byte{l}) {
			t.Fail()
		}
	case <-time.After(eto):
		t.Fail()
	}

	// copy three lines
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}})
	select {
	case li := <-lr:
		if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, two, three}) {
			t.Fail()
		}
	case <-time.After(eto):
		t.Fail()
	}

	// find a line
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}}, two)
	found := false
loop0:
	for {
		select {
		case li := <-lr:
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, three}) {
					t.Fail()
				}
				break loop0
			} else {
				if !bytes.Equal(li.line, two) {
					t.Fail()
				}
				found = true
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// find last line
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}}, three)
	found = false
loop00:
	for {
		select {
		case li := <-lr:
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, two}) {
					t.Fail()
				}
				break loop00
			} else {
				if !bytes.Equal(li.line, three) {
					t.Fail()
				}
				found = true
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// find a line, with line-break
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}}, lineFeed(two))
	found = false
loop1:
	for {
		select {
		case li := <-lr:
			if found {
				if li.err != io.EOF || !linesEqual(w.lines, [][]byte{one, three}) {
					t.Fail()
				}
				break loop1
			} else {
				if !bytes.Equal(li.line, lineFeed(two)) {
					t.Fail()
				}
				found = true
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// find multiple lines
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}}, lineFeed(two), three)
	var rl [][]byte
loop2:
	for {
		select {
		case li := <-lr:
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != io.EOF {
					t.Fail()
				}
				break loop2
			}
		case <-time.After(eto):
			t.Fail()
		}
	}
	if !linesEqual(w.lines, [][]byte{one}) || !linesEqual(rl, [][]byte{lineFeed(two), three}) {
		t.Fail()
	}

	// detect non-eof read error, immediately
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError})
loop3:
	for {
		select {
		case li := <-lr:
			if li.err != testError ||
				!linesEqual(w.lines, nil) {
				t.Fail()
			}
			break loop3
		case <-time.After(eto):
			t.Fail()
		}
	}

	// detect non-eof read error
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 4})
loop4:
	for {
		select {
		case li := <-lr:
			if li.err != testError ||
				!linesEqual(w.lines, [][]byte{one}) {
				t.Fail()
			}
			break loop4
		case <-time.After(eto):
			t.Fail()
		}
	}

	// detect non-eof read error, last
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 7})
loop5:
	for {
		select {
		case li := <-lr:
			if li.err != testError ||
				!linesEqual(w.lines, [][]byte{one}) && !linesEqual(w.lines, [][]byte{one, two}) {
				t.Fail()
			}
			break loop5
		case <-time.After(eto):
			t.Fail()
		}
	}

	// detect non-eof read error, before found
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 1}, two)
	rl = nil
loop6:
	for {
		select {
		case li := <-lr:
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if !linesEqual(rl, nil) {
					t.Fail()
				}
				break loop6
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// detect non-eof read error, after found
	w = new(testWriter)
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}, err: testError, errIndex: 6}, two)
	rl = nil
loop7:
	for {
		select {
		case li := <-lr:
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if !linesEqual(w.lines, [][]byte{one}) ||
					!linesEqual(rl, nil) && !linesEqual(rl, [][]byte{two}) {
					t.Fail()
				}
				break loop7
			}
		case <-time.After(eto):
			t.Fail()
		}
	}

	// detect write error
	w = new(testWriter)
	w.err = testError
	w.errIndex = 4
	lr = filterLines(w, &testReader{lines: [][]byte{one, two, three}}, two)
	rl = nil
loop8:
	for {
		select {
		case li := <-lr:
			rl = append(rl, li.line)
			if li.err != nil {
				if li.err != testError {
					t.Fail()
				}
				if len(w.lines) == 0 || !bytes.Equal(w.lines[0], one) ||
					!linesEqual(rl, nil) && !linesEqual(rl, [][]byte{two}) {
					t.Fail()
				}
				break loop8
			}
		case <-time.After(eto):
			t.Fail()
		}
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
}

func TestWaitExit(t *testing.T) {
	var (
		p     *proc
		err   error
		s     status
		found bool
		ok    bool
		xerr  *exec.ExitError
		ws    syscall.WaitStatus
	)

	// sigterm fail
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	err = p.cmd.Run()
	if err != nil {
		t.Fatal()
	}
	time.Sleep(120 * time.Millisecond)
	s = p.waitExit()
	if !s.cleanupFailed || len(s.errors) == 0 {
		t.Fail()
	}

	// sigterm
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	err = p.cmd.Start()
	if err != nil {
		t.Fatal()
	}
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	time.Sleep(120 * time.Millisecond)
	s = p.waitExit()
	if s.cleanupFailed {
		t.Fail()
	}
	xerr = nil
	for _, err = range s.errors {
		if xerr, ok = err.(*exec.ExitError); ok {
			break
		}
		continue
	}
	if xerr == nil {
		t.Fail()
	}
	if ws, ok = xerr.Sys().(syscall.WaitStatus); !ok {
		t.Fail()
	}
	if ws.Signaled() && ws.Signal() != syscall.SIGTERM {
		t.Fail()
	}

	// sigkill
	p = new(proc)
	p.cmd = exec.Command("testproc", "gulpterm", "12000")
	err = p.cmd.Start()
	if err != nil {
		t.Fatal()
	}
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	time.Sleep(120 * time.Millisecond)
	s = p.waitExit()
	if s.cleanupFailed {
		t.Fail()
	}
	xerr = nil
	for _, err = range s.errors {
		if xerr, ok = err.(*exec.ExitError); ok {
			break
		}
		continue
	}
	if xerr == nil {
		t.Fail()
	}
	if ws, ok = xerr.Sys().(syscall.WaitStatus); !ok {
		t.Fail()
	}
	if ws.Signal() != syscall.SIGKILL {
		t.Fail()
	}

	// stdout err
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	err = p.cmd.Start()
	if err != nil {
		t.Fatal()
	}
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: testError} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	time.Sleep(120 * time.Millisecond)
	s = p.waitExit()
	if s.cleanupFailed {
		t.Fail()
	}
	found = false
	for _, err := range s.errors {
		if err != testError {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fail()
	}

	// stderr err
	p = new(proc)
	p.cmd = exec.Command("testproc", "wait", "4500")
	err = p.cmd.Start()
	if err != nil {
		t.Fatal()
	}
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: testError} }()
	time.Sleep(120 * time.Millisecond)
	s = p.waitExit()
	if s.cleanupFailed {
		t.Fail()
	}
	found = false
	for _, err := range s.errors {
		if err != testError {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fail()
	}
}

func TestOutputError(t *testing.T) {
	// eof
	p := new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.cmd.Start()
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	time.Sleep(120 * time.Millisecond)
	s := p.outputError(io.EOF)
	found := false
	for _, e := range s.errors {
		if e != unexpectedExit {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fail()
	}

	// error
	p = new(proc)
	p.cmd = exec.Command("testproc", "noop")
	p.cmd.Start()
	p.stdout = make(chan lineRead)
	p.stderr = make(chan lineRead)
	go func() { p.stdout <- lineRead{err: io.EOF} }()
	go func() { p.stderr <- lineRead{err: io.EOF} }()
	time.Sleep(120 * time.Millisecond)
	s = p.outputError(testError)
	found = false
	for _, e := range s.errors {
		if e != testError {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fail()
	}
}

func TestRun(t *testing.T) {
	// start fail
	p := new(proc)
	s := p.run("")
	if len(s.errors) == 0 {
		t.Fail()
	}

	// start timeout running
	p = new(proc)
	s = p.run("testproc", "wait", fmt.Sprint(startupTimeoutMs*2))
	found := false
	for _, e := range s.errors {
		if e != startupTimeouted {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fail()
	}

	// exit before timeout
	p = new(proc)
	s = p.run("testproc", "wait", fmt.Sprint(startupTimeoutMs/2))
	found = false
	for _, e := range s.errors {
		if e != unexpectedExit {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Log("here")
		t.Fail()
	}

	// run, exit
	p = new(proc)
	p.ready = make(chan int)
	p.exit = make(chan int)
	to := make(chan int)
	go func() {
		s = p.run("testproc", "printwait", "4500", "some message", string(startupMessage), "")
		log.Println("exited")
		if s.cleanupFailed {
			log.Println("cleanup failed")
			t.Fail()
		}
		close(to)
	}()
	select {
	case <-p.ready:
		log.Println("even if")
		p.close()
		<-to
	case <-to:
		log.Println("closing")
		t.Fail()
	}

	// start message twice
	p = new(proc)
	p.ready = make(chan int)
	p.exit = make(chan int)
	to = make(chan int)
	go func() {
		s = p.run("testproc", "printwait", "4500", string(startupMessage), string(startupMessage))
		log.Println("exited")
		if s.cleanupFailed {
			log.Println("cleanup failed")
			t.Fail()
		}
		close(to)
	}()
	select {
	case <-p.ready:
		log.Println("even if")
		p.close()
		<-to
	case <-to:
		log.Println("closing")
		t.Fail()
	}

	// close before started
	p = new(proc)
	p.ready = make(chan int)
	p.exit = make(chan int)
	to = make(chan int)
	go func() {
		s = p.run("testproc", "wait", "4500")
		if s.cleanupFailed {
			t.Fail()
		}
		close(to)
	}()
	time.Sleep(120 * time.Millisecond)
	p.close()
	select {
	case <-p.ready:
		<-to
	case <-time.After(1200 * time.Millisecond):
		t.Fail()
	}

	// update access time
	p = new(proc)
	p.ready = make(chan int)
	p.access = make(chan int)
	p.exit = make(chan int)
	now := time.Now()
	p.accessed = now
	to = make(chan int)
	go func() {
		log.Println("started")
		s = p.run("testproc", "printwait", "4500", string(startupMessage))
		if s.cleanupFailed {
			t.Log("really, here")
			t.Log(s.errors)
			t.Fail()
		}
		close(to)
	}()
	time.Sleep(120)
	p.access <- 0
	p.close()
	<-to
	if !p.accessed.After(now) {
		t.Fail()
	}
}

func TestServe(t *testing.T) {
	// access
	p := new(proc)
	p.ready = make(chan int)
	p.access = make(chan int)
	to := make(chan int)
	go func() {
		err := p.serve(nil, nil)
		if err != nil {
			t.Fail()
		}
		close(to)
	}()
	close(p.ready)
	<-p.access
	<-to

	// exit
	p = new(proc)
	p.ready = make(chan int)
	p.exit = make(chan int)
	to = make(chan int)
	go func() {
		err := p.serve(nil, nil)
		if err != procClosed {
			t.Fail()
		}
		close(to)
	}()
	close(p.exit)
	close(p.ready)
	<-to
}
