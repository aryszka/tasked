package htproc

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func (s *testSettings) Hostname() string           { return s.hostname }
func (s *testSettings) PortRange() (int, int)      { return s.portRangeFrom, s.portRangeTo }
func (s *testSettings) MaxProcesses() int          { return s.maxProcesses }
func (s *testSettings) IdleTimeout() time.Duration { return s.idleTimeout }

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
	if ps.m == nil || ps.cr == nil || ps.rm == nil || ps.procExit == nil || ps.exit == nil {
		t.Fail()
	}
}

func TestRunProc(t *testing.T) {
	ps := new(procStore)
	ps.procExit = make(chan exitStatus)
	p := new(proc)
	p.ready = make(chan int)
	p.cmd = exec.Command("")
	go func() {
		ps.runProc(p)
	}()
	s := <-ps.procExit
	if s.proc != p {
		t.Fail()
	}
}

func TestAddProc(t *testing.T) {
	// try add existing
	ps := new(procStore)
	ps.maxProcs = 2
	m := make(procMap)
	p0 := new(proc)
	m["user0"] = p0
	m["user1"] = new(proc)
	m = ps.addProc(m, "user0")
	if len(m) != 2 || m["user0"] != p0 {
		t.Fail()
	}

	// add new
	ps = new(procStore)
	ps.maxProcs = 2
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	m["user0"] = new(proc)
	m = ps.addProc(m, "user1")
	if len(m) != 2 || m["user1"] == nil {
		t.Fail()
	}
	s := <-ps.procExit
	if s.proc != m["user1"] {
		t.Fail()
	}

	// add new, remove oldest
	ps = new(procStore)
	ps.maxProcs = 2
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	now := time.Now()
	p0 = new(proc)
	p0.exit = make(chan int)
	p0.accessed = now.Add(-1 * time.Second)
	m["user0"] = p0
	p1 := new(proc)
	p1.accessed = now
	m["user1"] = p1
	m = ps.addProc(m, "user2")
	if len(m) != 2 || m["user1"] == nil || m["user2"] == nil {
		t.Fail()
	}
	s = <-ps.procExit
	<-p0.exit
}

func TestRemoveProc(t *testing.T) {
	// no proc in map
	m := make(procMap)
	m = removeProc(m, new(proc))
	if len(m) != 0 {
		t.Fail()
	}

	// proc not found in map
	m = make(procMap)
	m["user0"] = new(proc)
	m["user1"] = new(proc)
	m = removeProc(m, new(proc))
	if len(m) != 2 {
		t.Fail()
	}

	// remove proc
	m = make(procMap)
	p0 := new(proc)
	p0.exit = make(chan int)
	m["user0"] = p0
	m["user1"] = new(proc)
	m = removeProc(m, p0)
	if len(m) != 1 || m["user1"] == nil {
		t.Fail()
	}
}

func TestCloseAll(t *testing.T) {
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
	p0 := new(proc)
	p0.exit = make(chan int)
	go func() {
		<-p0.exit
		ps.procExit <- exitStatus{proc: p0}
	}()
	m["user0"] = p0
	p1 := new(proc)
	p1.exit = make(chan int)
	go func() {
		<-p1.exit
		ps.procExit <- exitStatus{proc: p1}
	}()
	m["user1"] = p1
	err = ps.closeAll(m, nil)
	if err != nil {
		t.Fail()
	}

	// cleanup failed
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = new(proc)
	p0.exit = make(chan int)
	go func() {
		<-p0.exit
		ps.procExit <- exitStatus{proc: p0}
	}()
	m["user0"] = p0
	p1 = new(proc)
	p1.exit = make(chan int)
	go func() {
		<-p1.exit
		ps.procExit <- exitStatus{proc: p1, status: status{cleanupFailed: true}}
	}()
	m["user1"] = p1
	err = ps.closeAll(m, nil)
	if err != procCleanupFailed {
		t.Fail()
	}

	// timeout
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = new(proc)
	p0.exit = make(chan int)
	go func() {
		<-p0.exit
		ps.procExit <- exitStatus{proc: p0}
	}()
	m["user0"] = p0
	p1 = new(proc)
	p1.exit = make(chan int)
	go func() {
		<-p1.exit
	}()
	m["user1"] = p1
	err = ps.closeAll(m, nil)
	if err != procStoreCloseTimeouted {
		t.Fail()
	}

	// proc error
	ps = new(procStore)
	ps.procExit = make(chan exitStatus)
	m = make(procMap)
	p0 = new(proc)
	p0.exit = make(chan int)
	go func() {
		<-p0.exit
		ps.procExit <- exitStatus{proc: p0}
	}()
	m["user0"] = p0
	p1 = new(proc)
	p1.exit = make(chan int)
	testError := errors.New("test error")
	go func() {
		<-p1.exit
		ps.procExit <- exitStatus{proc: p1, status: status{errors: []error{testError}}}
	}()
	m["user1"] = p1
	procErrors := make(chan error)
	err = ps.closeAll(m, procErrors)
	if err != procStoreCloseTimeouted {
		t.Fail()
	}
	err = <-procErrors
	if perr, ok := err.(*ProcError); !ok || perr.Err != testError || perr.User != "user1" {
		t.Fail()
	}
}

func TestFindUser(t *testing.T) {
	// empty map
	m := make(procMap)
	u := findUser(m, new(proc))
	if u != "" {
		t.Fail()
	}

	// not found
	m = make(procMap)
	m["user0"] = new(proc)
	m["user1"] = new(proc)
	u = findUser(m, new(proc))
	if u != "" {
		t.Fail()
	}

	// found
	m = make(procMap)
	p0 := new(proc)
	m["user0"] = p0
	m["user1"] = new(proc)
	u = findUser(m, p0)
	if u != "user0" {
		t.Fail()
	}
}

func TestProcStoreRun(t *testing.T) {
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
	to := make(chan error)
	go func() { to <- ps.run(nil) }()
	m := <-ps.m
	if len(m) != 0 {
		t.Fail()
	}
	close(ps.exit)
	err = <-to
	if err != nil {
		t.Fail()
	}

	// run, add proc, get proc
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	to = make(chan error)
	go func() { to <- ps.run(nil) }()
	ps.cr <- "user0"
	m = <-ps.m
	if len(m) != 1 || m["user0"] == nil {
		t.Fail()
	}
	close(ps.exit)
	err = <-to
	if err != nil {
		t.Fail()
	}

	// run, add proc, close proc
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	to = make(chan error)
	go func() { to <- ps.run(nil) }()
	ps.cr <- "user0"
	m = <-ps.m
	m["user0"].close()
	time.Sleep(120 * time.Millisecond)
	m = <-ps.m
	if len(m) != 0 {
		t.Fail()
	}
	close(ps.exit)
	err = <-to
	if err != nil {
		t.Fail()
	}

	// run, add proc, remove proc
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	ps.rm = make(chan *proc)
	to = make(chan error)
	go func() { to <- ps.run(nil) }()
	ps.cr <- "user0"
	m = <-ps.m
	ps.rm <- m["user0"]
	time.Sleep(120 * time.Millisecond)
	m = <-ps.m
	if len(m) != 0 {
		t.Fail()
	}
	close(ps.exit)
	err = <-to
	if err != nil {
		t.Fail()
	}

	// run, add proc, close proc, collect errors
	ps = new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	to = make(chan error)
	procErrors := make(chan error)
	go func() { to <- ps.run(procErrors) }()
	ps.cr <- "user0"
	m = <-ps.m
	m["user0"].close()
	err = <-procErrors
	if err == nil {
		t.Fail()
	}
	close(ps.exit)
	err = <-to
	if err != nil {
		t.Fail()
	}
}

func TestGetMap(t *testing.T) {
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.m = make(chan procMap)
	to := make(chan error)
	go func() { to <- ps.run(nil) }()
	m, err := ps.getMap()
	if m == nil || err != nil {
		t.Fail()
	}
	close(ps.exit)
	m, err = ps.getMap()
	if err != procStoreClosed {
		t.Fail()
	}
	err = <-to
	if err != nil {
		t.Fail()
	}
}

func TestCreate(t *testing.T) {
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	to := make(chan error)
	go func() { to <- ps.run(nil) }()
	err := ps.create("user0")
	if err != nil {
		t.Fail()
	}
	m := <-ps.m
	if len(m) != 1 || m["user0"] == nil {
		t.Fail()
	}
	close(ps.exit)
	err = ps.create("user1")
	if err != procStoreClosed {
		t.Fail()
	}
	err = <-to
	if err != nil {
		t.Fail()
	}
}

func TestGet(t *testing.T) {
	ps := new(procStore)
	ps.exit = make(chan int)
	ps.procExit = make(chan exitStatus)
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	to := make(chan error)
	go func() { to <- ps.run(nil) }()
	p, err := ps.get("user0")
	if p == nil || err != nil {
		t.Fail()
	}
	close(ps.exit)
	_, err = ps.get("user0")
	if err != procStoreClosed {
		t.Fail()
	}
}
