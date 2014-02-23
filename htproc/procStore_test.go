package htproc

import (
	. "code.google.com/p/tasked/testing"
	"errors"
	"strings"
	"testing"
	"time"
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

func TestNewProcStore(t *testing.T) {
	ps := newProcStore(&testOptions{maxProcesses: 12})
	if ps.maxProcs != 12 {
		t.Fail()
	}
	if ps.procs == nil || ps.accessed == nil || ps.failures == nil || ps.banned == nil ||
		ps.gc == nil || ps.px == nil || ps.exit == nil {
		t.Fail()
	}
}

func TestRemoveProc(t *testing.T) {
	// no proc in map
	ps := new(procStore)
	ps.procs = make(map[string]runner)
	ps.removeProc("user0")
	if len(ps.procs) != 0 {
		t.Fail()
	}

	// proc not found in map
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.procs["user0"] = &testServer{}
	ps.procs["user1"] = &testServer{}
	ps.removeProc("user2")
	if len(ps.procs) != 2 {
		t.Fail()
	}
	for _, p := range ps.procs {
		ts, ok := p.(*testServer)
		if !ok {
			t.Fatal()
		}
		if ts.closed {
			t.Fail()
		}
	}

	// remove proc
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	p0 := &testServer{}
	ps.procs["user0"] = p0
	ps.procs["user1"] = &testServer{}
	ps.removeProc("user0")
	if len(ps.procs) != 1 || ps.procs["user0"] == p0 || !p0.closed {
		t.Fail()
	}
}

func TestRemoveIdle(t *testing.T) {
	// no procs
	ps := new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.removeIdle(time.Now())
	if len(ps.procs) != 0 || len(ps.accessed) != 0 {
		t.Fail()
	}

	// no idle procs
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	p0 := new(testServer)
	ps.procs["user0"] = p0
	ps.accessed["user0"] = time.Now().Add(-procIdleTimeout / 2)
	ps.removeIdle(time.Now())
	if len(ps.procs) != 1 || len(ps.accessed) != 1 || ps.procs["user0"] != p0 {
		t.Fail()
	}

	// remove idle procs
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	p0 = new(testServer)
	ps.procs["user0"] = p0
	ps.accessed["user0"] = time.Now().Add(-2 * procIdleTimeout)
	p1 := new(testServer)
	ps.procs["user1"] = p1
	ps.accessed["user1"] = time.Now().Add(-procIdleTimeout / 2)
	p2 := new(testServer)
	ps.procs["user2"] = p2
	ps.accessed["user2"] = time.Now().Add(-2 * procIdleTimeout)
	ps.removeIdle(time.Now())
	if len(ps.procs) != 1 || len(ps.accessed) != 1 || ps.procs["user1"] != p1 {
		t.Fail()
	}
}

func TestGetCreateProc(t *testing.T) {
	defer func(c string, a []string) { command, args = c, a }(command, args)
	command, args = "testproc", []string{"wait", "4500"}

	// get existing
	ps := new(procStore)
	now := time.Now()
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	p0 := new(testServer)
	ps.procs["user0"] = p0
	p0i, err := ps.getCreateProc("user0")
	if p0i != p0 || err != nil ||
		ps.accessed["user0"].Before(now) || ps.accessed["user0"].After(time.Now()) {
		t.Fail()
	}

	// create new
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	ps.banned = make(map[string]time.Time)
	ps.accessed = make(map[string]time.Time)
	p0i, err = ps.getCreateProc("user0")
	if err != nil || len(ps.procs) != 1 || ps.procs["user0"] == nil {
		t.Fail()
	}
	p0i.close()
	WithTimeout(t, eto, func() {
		s := <-ps.px
		if s.user != "user0" || s.proc != p0i {
			t.Fail()
		}
	})

	// banned
	ps = new(procStore)
	ps.banned = make(map[string]time.Time)
	ps.banned["user0"] = time.Now().Add(-failureRecoveryTime / 2)
	_, err = ps.getCreateProc("user0")
	if err != temporarilyBanned {
		t.Fail()
	}

	// banned, expired
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	ps.accessed = make(map[string]time.Time)
	ps.banned = make(map[string]time.Time)
	ps.banned["user0"] = time.Now().Add(-2 * failureRecoveryTime)
	p0i, err = ps.getCreateProc("user0")
	if err != nil || len(ps.banned) != 0 || len(ps.procs) == 0 || ps.procs["user0"] == p0 {
		t.Fail()
	}
	p0i.close()
	WithTimeout(t, eto, func() {
		s := <-ps.px
		if s.user != "user0" || s.proc != p0i {
			t.Fail()
		}
	})

	// has idle
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	ps.banned = make(map[string]time.Time)
	ps.accessed = make(map[string]time.Time)
	p0 = new(testServer)
	ps.procs["user0"] = p0
	ps.accessed["user0"] = time.Now().Add(-2 * procIdleTimeout)
	p1 := new(testServer)
	ps.procs["user1"] = p1
	ps.accessed["user1"] = time.Now()
	p2i, err := ps.getCreateProc("user2")
	if err != nil || len(ps.procs) != 2 || ps.procs["user1"] != p1 || ps.procs["user2"] == nil {
		t.Fail()
	}
	p2i.close()
	WithTimeout(t, eto, func() {
		s := <-ps.px
		if s.user != "user2" || s.proc != p2i {
			t.Fail()
		}
	})

	// max procs exceede
	ps = new(procStore)
	ps.maxProcs = 1
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	ps.banned = make(map[string]time.Time)
	ps.accessed = make(map[string]time.Time)
	p0 = new(testServer)
	ps.procs["user0"] = p0
	ps.accessed["user0"] = time.Now()
	p1i, err := ps.getCreateProc("user1")
	if err != nil || len(ps.procs) != 1 || ps.procs["user1"] == nil {
		t.Fail()
	}
	p1i.close()
	WithTimeout(t, eto, func() {
		s := <-ps.px
		if s.user != "user1" || s.proc != p1i {
			t.Fail()
		}
	})
}

func TestDiscardFailures(t *testing.T) {
	// empty
	f := discardFailures(nil, time.Now())
	if len(f) != 0 {
		t.Fail()
	}

	// nothing to discard
	f = []time.Time{time.Now().Add(-failureRecoveryTime / 2), time.Now()}
	f = discardFailures(f, time.Now())
	if len(f) != 2 {
		t.Fail()
	}

	// discard
	f = []time.Time{time.Now().Add(-2 * failureRecoveryTime), time.Now()}
	f = discardFailures(f, time.Now())
	if len(f) != 1 {
		t.Fail()
	}
}

func TestProcErrors(t *testing.T) {
	// no error
	ps := new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.procErrors("user0", nil, nil)
	if len(ps.failures["user0"]) != 0 {
		t.Fail()
	}

	// failure added
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	now := time.Now()
	ps.procErrors("user0", []error{testError}, nil)
	if len(ps.failures["user0"]) != 1 || ps.failures["user0"][0].Before(now) {
		t.Fail()
	}

	// discard if old
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.failures["user0"] = []time.Time{time.Now().Add(-2 * failureRecoveryTime)}
	now = time.Now()
	ps.procErrors("user0", []error{testError}, nil)
	if len(ps.failures["user0"]) != 1 || ps.failures["user0"][0].Before(now) {
		t.Fail()
	}

	// max exceeded
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.banned = make(map[string]time.Time)
	now = time.Now()
	for i := 0; i < maxSocketFailures; i++ {
		ps.failures["user0"] = append(ps.failures["user0"], now)
	}
	ps.procErrors("user0", []error{testError}, nil)
	if len(ps.failures["user0"]) != 0 || ps.banned["user0"].Before(now) {
		t.Fail()
	}

	// notify
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	notify := make(chan error)
	WithTimeout(t, eto, func() {
		go func() { ps.procErrors("user0", []error{testError}, notify) }()
		err := <-notify
		if perr, ok := err.(*ProcError); !ok ||
			perr.User != "user0" || perr.Err != testError {
			t.Fail()
		}
	})
}

func TestCloseAll(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	// nothing to close
	ps := new(procStore)
	ps.procs = make(map[string]runner)
	WithTimeout(t, eto, func() {
		err := ps.closeAll(nil)
		if err != nil {
			t.Fail()
		}
	})

	// close all
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	p0 := &testServer{exit: make(chan int)}
	ps.procs["user0"] = p0
	p1 := &testServer{exit: make(chan int)}
	ps.procs["user1"] = p1
	WithTimeout(t, eto, func() {
		go func() { ps.px <- exitStatus{proc: p0, status: p0.run()} }()
		go func() { ps.px <- exitStatus{proc: p1, status: p1.run()} }()
		err := ps.closeAll(nil)
		if err != nil || !p0.closed || !p1.closed {
			t.Fail()
		}
	})

	// cleanup failed
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	p0 = &testServer{exit: make(chan int)}
	ps.procs["user0"] = p0
	p1 = &testServer{exit: make(chan int)}
	p1.cleanupFail = true
	ps.procs["user1"] = p1
	WithTimeout(t, eto, func() {
		go func() { ps.px <- exitStatus{proc: p0, status: p0.run()} }()
		go func() { ps.px <- exitStatus{proc: p1, status: p1.run()} }()
		err := ps.closeAll(nil)
		if err != procCleanupFailed || !p0.closed || !p1.closed {
			t.Fail()
		}
	})

	// timeout
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	p0 = &testServer{exit: make(chan int)}
	ps.procs["user0"] = p0
	p1 = &testServer{exit: make(chan int)}
	p1.cleanupFail = true
	ps.procs["user1"] = p1
	WithTimeout(t, 2*procStoreCloseTimeout, func() {
		go func() { ps.px <- exitStatus{proc: p0, status: p0.run()} }()
		err := ps.closeAll(nil)
		if err != procStoreCloseTimeouted || !p0.closed || !p1.closed {
			t.Fail()
		}
	})

	// proc error
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.px = make(chan exitStatus)
	p0 = &testServer{exit: make(chan int)}
	ps.procs["user0"] = p0
	p1 = &testServer{exit: make(chan int)}
	ps.procs["user1"] = p1
	WithTimeout(t, eto, func() {
		go func() { ps.px <- exitStatus{proc: p0, status: p0.run()} }()
		go func() { ps.px <- exitStatus{proc: p1, status: p1.run()} }()
		procErrors := make(chan error)
		w := Wait(func() {
			err := ps.closeAll(procErrors)
			if err != nil || !p0.closed || !p1.closed {
				t.Fail()
			}
		})
		errs := make(map[string]bool)
		for {
			err := <-procErrors
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
		<-w
	})
}

func TestCleanupFailures(t *testing.T) {
	// nothing to cleanup
	ps := new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.banned = make(map[string]time.Time)
	ps.cleanupFailures(time.Now())
	if len(ps.failures) != 0 || len(ps.banned) != 0 {
		t.Fail()
	}

	// nothing to recover
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.banned = make(map[string]time.Time)
	ps.failures["user0"] = []time.Time{time.Now()}
	ps.banned["user1"] = time.Now()
	ps.cleanupFailures(time.Now())
	if len(ps.failures["user0"]) != 1 || len(ps.banned) != 1 {
		t.Fail()
	}

	// cleanup
	ps = new(procStore)
	ps.failures = make(map[string][]time.Time)
	ps.banned = make(map[string]time.Time)
	ps.failures["user0"] = []time.Time{time.Now().Add(-2 * failureRecoveryTime), time.Now()}
	ps.banned["user1"] = time.Now().Add(-2 * failureRecoveryTime)
	ps.banned["user2"] = time.Now().Add(-failureRecoveryTime / 2)
	ps.cleanupFailures(time.Now())
	if len(ps.failures["user0"]) != 1 || len(ps.banned) != 1 {
		t.Fail()
	}
}

func TestProcStoreRun(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	defer func(c string, a []string) { command, args = c, a }(command, args)
	command, args = "testproc", []string{"wait", "4500"}

	// run, exit
	ps := new(procStore)
	ps.exit = make(chan int)
	close(ps.exit)
	err := ps.run(nil)
	if err != nil {
		t.Fail()
	}

	// run, add proc, get proc, exit
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.exit = make(chan int)
	ps.px = make(chan exitStatus)
	ps.gc = make(chan getCreateProc)
	WithTimeout(t, eto, func() {
		var p0 runner
		w := Wait(func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
			<-p0.(*proc).exit
		})
		rc := make(chan getCreateResult)
		ps.gc <- getCreateProc{user: "user0", res: rc}
		res := <-rc
		if res.proc == nil || res.err != nil {
			t.Fail()
		}
		if len(ps.procs) != 1 || ps.procs["user0"] == nil {
			t.Fail()
		}
		p0 = res.proc
		close(ps.exit)
		<-w
	})

	// run, add proc, let proc exit, exit
	args = []string{"wait", "240"}
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.failures = make(map[string][]time.Time)
	ps.exit = make(chan int)
	ps.px = make(chan exitStatus)
	ps.gc = make(chan getCreateProc)
	WithTimeout(t, exitTimeout, func() {
		w := Wait(func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		})
		rc := make(chan getCreateResult)
		ps.gc <- getCreateProc{user: "user0", res: rc}
		res := <-rc
		<-res.proc.(*proc).exit
		time.Sleep(120 * time.Millisecond)
		if len(ps.procs) > 0 {
			t.Fail()
		}
		close(ps.exit)
		<-w
	})

	// run, add proc, let proc exit, collect errors
	args = []string{"wait", "240"}
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.failures = make(map[string][]time.Time)
	ps.exit = make(chan int)
	ps.px = make(chan exitStatus)
	ps.gc = make(chan getCreateProc)
	WithTimeout(t, exitTimeout, func() {
		pe := make(chan error)
		w := Wait(func() {
			err := ps.run(pe)
			if err != nil {
				t.Fail()
			}
		})
		rc := make(chan getCreateResult)
		ps.gc <- getCreateProc{user: "user0", res: rc}
		res := <-rc
		<-res.proc.(*proc).exit
		time.Sleep(120 * time.Millisecond)
		if len(ps.procs) > 0 {
			t.Fail()
		}
		err := <-pe
		if err == nil {
			t.Fail()
		}
		if perr, ok := err.(*ProcError); !ok || perr.User != "user0" || perr.Err != unexpectedExit {
			t.Fail()
		}
		close(ps.exit)
		<-w
	})

	// idle check
	args = []string{"wait", "12000"}
	defer func(period, timeout time.Duration) {
		procIdleCheckPeriod = period
		procIdleTimeout = timeout
	}(procIdleCheckPeriod, procIdleTimeout)
	ps = new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.failures = make(map[string][]time.Time)
	ps.exit = make(chan int)
	ps.px = make(chan exitStatus)
	ps.gc = make(chan getCreateProc)
	procIdleCheckPeriod = 15 * time.Millisecond
	procIdleTimeout = 30 * time.Millisecond
	WithTimeout(t, exitTimeout, func() {
		w := Wait(func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		})
		rc := make(chan getCreateResult)
		ps.gc <- getCreateProc{user: "user0", res: rc}
		res := <-rc
		<-res.proc.(*proc).exit
		close(ps.exit)
		<-w
	})
}

func TestGetCreate(t *testing.T) {
	defer func(c string, a []string) { command, args = c, a }(command, args)
	command, args = "testproc", []string{"wait", "4500"}

	ps := new(procStore)
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.exit = make(chan int)
	ps.px = make(chan exitStatus)
	ps.gc = make(chan getCreateProc)
	WithTimeout(t, exitTimeout, func() {
		w := Wait(func() {
			err := ps.run(nil)
			if err != nil {
				t.Fail()
			}
		})
		p, err := ps.getCreate("user0")
		if p == nil || err != nil {
			t.Fail()
		}
		close(ps.exit)
		time.Sleep(120 * time.Millisecond)
		p, err = ps.getCreate("user0")
		if p != nil || err != procStoreClosed {
			t.Fail()
		}
		<-w
	})
}
