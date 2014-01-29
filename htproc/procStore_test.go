package htproc

import (
	"errors"
	"strings"
	"testing"
	"time"
	. "code.google.com/p/tasked/testing"
)

func TestProcError(t *testing.T) {
	err := &ProcError{User: "testuser", Err: errors.New("testerror")}
	errstr := err.Error()
	if strings.Index(errstr, "testuser") < 0 || strings.Index(errstr, "testerror") < 0 {
		t.Fail()
	}
	if err.Fatal() {
		t.Fail()
	}

	err = &ProcError{Err: procCleanupFailed}
	if !err.Fatal() {
		t.Fail()
	}
}

func TestNewPorcStore(t *testing.T) {
	ps := newProcStore(&testSettings{maxProcesses: 12})
	if ps.maxProcs != 12 {
		t.Fail()
	}
	if ps.m == nil || ps.ad == nil || ps.rm == nil || ps.procExit == nil || ps.exit == nil {
		t.Fail()
	}
}

func TestRunProc(t *testing.T) {
	ps := new(procStore)
	ps.procExit = make(chan exitStatus)
	p := new(testServer)
	go ps.runProc(p)
	s := <-ps.procExit
	if s.proc != p || len(s.status.errors) != 1 || s.status.errors[0] != testError {
		t.Fail()
	}
}

func TestAddProc(t *testing.T) {
	// try add existing
	ps := new(procStore)
	m := make(procMap)
	p0 := new(testServer)
	m["user0"] = p0
	m["user1"] = new(testServer)
	p0i := new(testServer)
	m = ps.addProc(m, "user0", p0i)
	if len(m) != 2 || m["user0"] != p0 {
		t.Fail()
	}

	// add new
	ps = new(procStore)
	ps.maxProcs = 2
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	m["user0"] = new(testServer)
	p1 := new(testServer)
	m = ps.addProc(m, "user1", p1)
	if len(m) != 2 || m["user1"] != p1 {
		t.Fail()
	}
	WithTimeout(t, eto, func() {
		s := <-ps.procExit
		if s.proc != m["user1"] {
			t.Fail()
		}
	})

	// add new, remove oldest
	ps = new(procStore)
	ps.maxProcs = 2
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	now := time.Now()
	p0 = new(testServer)
	p0.exit = make(chan int)
	m["user0"] = p0
	p0.access = now.Add(-1 * time.Second)
	p1 = new(testServer)
	p1.exit = make(chan int)
	m["user1"] = p1
	p1.access = now
	p2 := new(testServer)
	p2.exit = make(chan int)
	m = ps.addProc(m, "user2", p2)
	if len(m) != 2 || m["user1"] != p1 || m["user2"] != p2 {
		t.Fail()
	}
	WithTimeout(t, eto, func() {
		<-ps.procExit
		if !p0.closed {
			t.Fail()
		}
	})
}

func TestRemoveProcs(t *testing.T) {
	// no proc in map
	m := make(procMap)
	m = removeProcs(m, new(testServer))
	if len(m) != 0 {
		t.Fail()
	}

	// proc not found in map
	m = make(procMap)
	m["user0"] = &testServer{exit: make(chan int)}
	m["user1"] = &testServer{exit: make(chan int)}
	m = removeProcs(m, &testServer{exit: make(chan int)})
	if len(m) != 2 {
		t.Fail()
	}
	for _, p := range m {
		ts, ok := p.(*testServer)
		if !ok {
			t.Fatal()
		}
		if ts.closed {
			t.Fail()
		}
	}

	// remove proc
	m = make(procMap)
	p0 := &testServer{exit: make(chan int)}
	m["user0"] = p0
	m["user1"] = &testServer{exit: make(chan int)}
	m = removeProcs(m, p0)
	if len(m) != 1 || m["user0"] == p0 || !p0.closed {
		t.Fail()
	}

	// remove multiple
	m = make(procMap)
	p0 = &testServer{exit: make(chan int)}
	p1 := &testServer{exit: make(chan int)}
	p2 := &testServer{exit: make(chan int)}
	m["user0"] = p0
	m["user1"] = p1
	m["user2"] = p2
	p3 := &testServer{exit: make(chan int)}
	m = removeProcs(m, p2, p3, p0)
	if len(m) != 1 || m["user1"] != p1 ||
		!p0.closed || p1.closed || !p2.closed || p3.closed {
		t.Fail()
	}
}

func TestCloseAll(t *testing.T) {
	if !testLong {
		t.Skip()
	}
	// nothing to close
	ps := new(procStore)
	m := make(procMap)
	err := ps.closeAll(m, nil)
	if err != nil {
		t.Fail()
	}

	// close all
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 := &testServer{exit: make(chan int)}
	m["user0"] = p0
	go ps.runProc(p0)
	p1 := &testServer{exit: make(chan int)}
	m["user1"] = p1
	go ps.runProc(p1)
	WithTimeout(t, eto, func() {
		err = ps.closeAll(m, nil)
		if err != nil || !p0.closed || !p1.closed ||
			!p0.closed || !p1.closed {
			t.Fail()
		}
	})

	// cleanup failed
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = &testServer{exit: make(chan int)}
	m["user0"] = p0
	go ps.runProc(p0)
	p1 = &testServer{exit: make(chan int)}
	m["user1"] = p1
	p1.cleanupFail = true
	go ps.runProc(p1)
	WithTimeout(t, eto, func() {
		err = ps.closeAll(m, nil)
		if err != procCleanupFailed {
			t.Fail()
		}
	})

	// timeout
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = &testServer{exit: make(chan int)}
	m["user0"] = p0
	go ps.runProc(p0)
	p1 = &testServer{exit: make(chan int)}
	m["user1"] = p1
	WithTimeout(t, 2 * procStoreCloseTimeout, func() {
		err = ps.closeAll(m, nil)
		if err != procStoreCloseTimeouted {
			t.Fail()
		}
	})

	// proc error
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = &testServer{exit: make(chan int)}
	m["user0"] = p0
	go ps.runProc(p0)
	p1 = &testServer{exit: make(chan int)}
	m["user1"] = p1
	go ps.runProc(p1)
	WithTimeout(t, 2 * procStoreCloseTimeout, func() {
		procErrors := make(chan error)
		go func() {
			err = ps.closeAll(m, procErrors)
			if err != nil || !p0.closed || !p1.closed {
				t.Fail()
			}
		}()
		errs := make(map[string]bool)
		for {
			err = <-procErrors
			perr, ok := err.(*ProcError)
			if !ok {
				t.Fail()
			}
			if perr.User != "user0" && perr.User != "user1" || perr.Err != testError || errs[perr.User] {
				t.Fail()
			}
			errs[perr.User] = true
			if len(errs) == 2 {
				break
			}
		}
	})
}

func TestFindUser(t *testing.T) {
	// empty map
	m := make(procMap)
	u := findUser(m, new(testServer))
	if u != "" {
		t.Fail()
	}

	// not found
	m = make(procMap)
	m["user0"] = new(testServer)
	m["user1"] = new(testServer)
	u = findUser(m, new(testServer))
	if u != "" {
		t.Fail()
	}

	// found
	m = make(procMap)
	p0 := new(testServer)
	m["user0"] = p0
	m["user1"] = new(testServer)
	u = findUser(m, p0)
	if u != "user0" {
		t.Fail()
	}
}

func TestProcStoreRun(t *testing.T) {
	if (!testLong) {
		t.Skip()
	}

	// run, exit
	ps := new(procStore)
	ps.exit = make(chan int)
	close(ps.exit)
	err := ps.run(nil)
	if err != nil {
		t.Fail()
	}

	// run, get empty map, exit
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.m = make(chan procMap)
	WithTimeout(t, eto, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		m := <-ps.m
		if len(m) != 0 {
			t.Fail()
		}
		close(ps.exit)
	})

	// run, add proc, get proc, exit
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	WithTimeout(t, eto, func() {
		p0 := new(testServer)
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
			if !p0.closed {
				t.Fail()
			}
		}()
		p0.waitForClose = true
		ps.ad <- addProc{user: "user0", proc: p0}
		m := <-ps.m
		if len(m) != 1 || m["user0"] == nil {
			t.Fail()
		}
		close(ps.exit)
	})

	// run, add proc, get proc, remove proc, exit
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	ps.rm = make(chan runner)
	WithTimeout(t, 240 * time.Millisecond + exitTimeout, func() {
		p0 := new(testServer)
		p0.waitForClose = true
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
			if !p0.closed {
				t.Fail()
			}
		}()
		ps.ad <- addProc{user: "user0", proc: p0}
		time.Sleep(120 * time.Millisecond)
		m := <-ps.m
		if m["user0"] != p0 {
			t.Fail()
		}
		ps.rm <- p0
		time.Sleep(120 * time.Millisecond)
		m = <-ps.m
		if len(m) != 0 {
			t.Fail()
		}
		close(ps.exit)
	})

	// run, add proc, let proc exit, exit
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	WithTimeout(t, exitTimeout, func() {
		p0 := new(testServer)
		p0.exit = make(chan int)
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
			if !p0.closed {
				t.Fail()
			}
		}()
		ps.ad <- addProc{user: "user0", proc: p0}
		time.Sleep(120 * time.Millisecond)
		m := <-ps.m
		if len(m) != 0 || !p0.closed {
			t.Fail()
		}
		close(ps.exit)
	})

	// run, add proc, let proc exit, collect errors
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	WithTimeout(t, exitTimeout, func() {
		procErrors := make(chan error)
		go func() {
			err := ps.run(procErrors)
			if err != nil {
				t.Fail()
			}
		}()
		p0 := new(testServer)
		p0.exit = make(chan int)
		ps.ad <- addProc{user: "user0", proc: p0}
		time.Sleep(120 * time.Millisecond)
		err := <-procErrors
		if err == nil {
			t.Fail()
		}
		if perr, ok := err.(*ProcError); !ok || perr.User != "user0" || perr.Err != testError {
			t.Fail()
		}
		close(ps.exit)
	})

	// idle check
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	defer func(period, timeout time.Duration) {
		procIdleCheckPeriod = period
		procIdleTimeout = timeout
	}(procIdleCheckPeriod, procIdleTimeout)
	procIdleCheckPeriod = 15 * time.Millisecond
	procIdleTimeout = 30 * time.Millisecond
	WithTimeout(t, exitTimeout, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		now := time.Now()
		p0 := new(testServer)
		p0.access = now.Add(-2 * procIdleCheckPeriod)
		p0.exit = make(chan int)
		ps.ad <- addProc{user: "user0", proc: p0}
		p1 := new(testServer)
		p1.access = now.Add(-2 * procIdleCheckPeriod)
		p1.exit = make(chan int)
		ps.ad <- addProc{user: "user1", proc: p1}
		p2 := new(testServer)
		p2.access = now
		p2.exit = make(chan int)
		ps.ad <- addProc{user: "user2", proc: p2}
		p3 := new(testServer)
		p3.access = now.Add(6 * procIdleCheckPeriod)
		p3.exit = make(chan int)
		ps.ad <- addProc{user: "user3", proc: p3}
		time.Sleep(15 * time.Millisecond)
		m := <-ps.m
		if len(m) != 2 || m["user2"] != p2 || m["user3"] != p3 {
			t.Fail()
		}
		time.Sleep(30 * time.Millisecond)
		m = <-ps.m
		if len(m) != 1 || m["user3"] != p3 {
			t.Fail()
		}
		close(ps.exit)
	})
}

func TestGetMap(t *testing.T) {
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.m = make(chan procMap)
	WithTimeout(t, eto, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		m, err := ps.getMap()
		if m == nil || err != nil {
			t.Fail()
		}
		close(ps.exit)
		m, err = ps.getMap()
		if err != procStoreClosed {
			t.Fail()
		}
	})
}

func TestCreate(t *testing.T) {
	commandOrig := command
	argsOrig := args
	defer func() {
		command = commandOrig
		args = argsOrig
	}()
	command = "testproc"
	args = []string{"wait", "12000"}
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	WithTimeout(t, exitTimeout, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		time.Sleep(120 * time.Millisecond)
		err := ps.create("user0")
		if err != nil {
			t.Fail()
		}
		time.Sleep(120 * time.Millisecond)
		m := <-ps.m
		if len(m) != 1 || m["user0"] == nil {
			t.Fail()
		}
		close(ps.exit)
		err = ps.create("user1")
		if err != procStoreClosed {
			t.Fail()
		}
	})
}

func TestGet(t *testing.T) {
	commandOrig := command
	argsOrig := args
	defer func() {
		command = commandOrig
		args = argsOrig
	}()
	command = "testproc"
	args = []string{"wait", "12000"}
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	WithTimeout(t, exitTimeout, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		p, err := ps.get("user0")
		if p == nil || err != nil {
			t.Fail()
		}
		close(ps.exit)
		time.Sleep(120 * time.Millisecond)
		_, err = ps.get("user0")
		if err != procStoreClosed {
			t.Fail()
		}
	})
}

func TestRemove(t *testing.T) {
	commandOrig := command
	argsOrig := args
	defer func() {
		command = commandOrig
		args = argsOrig
	}()
	command = "testproc"
	args = []string{"wait", "12000"}
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	ps.rm = make(chan runner)
	WithTimeout(t, exitTimeout, func() {
		go func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		}()
		p, err := ps.get("user0")
		if p == nil || err != nil {
			t.Fail()
		}
		err = ps.remove(p)
		if err != nil {
			t.Fail()
		}
		time.Sleep(120 * time.Millisecond)
		pi, err := ps.get("user0")
		if pi == nil || pi == p || err != nil {
			t.Fail()
		}
		close(ps.exit)
		time.Sleep(120 * time.Millisecond)
		err = ps.remove(pi)
		if err != procStoreClosed {
			t.Fail()
		}
	})
}
