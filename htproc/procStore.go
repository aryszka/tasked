package htproc

import (
	"errors"
	"fmt"
	"time"
)

type ProcError struct {
	User string
	Err  error
}

type exitStatus struct {
	proc   *proc
	status status
}

func (pe *ProcError) Error() string {
	return fmt.Sprintf("[%s]: %v\n", pe.User, pe.Err)
}

func (pe *ProcError) Fatal() bool {
	return pe.Err == procCleanupFailed
}

type procMap map[string]*proc

type procStore struct {
	maxProcs int
	m        chan procMap
	cr       chan string
	rm       chan *proc
	procExit chan exitStatus
	exit     chan int
}

const (
	procStoreCloseTimeout = 6 * time.Second
)

var (
	procStoreClosed         = errors.New("Proc store closed.")
	procCleanupFailed       = errors.New("Proc cleanup failed.")
	procStoreCloseTimeouted = errors.New("Proc store close timeouted.")
)

func newProcStore(s Settings) *procStore {
	ps := new(procStore)
	ps.maxProcs = s.MaxProcesses()
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	ps.rm = make(chan *proc)
	ps.procExit = make(chan exitStatus)
	ps.exit = make(chan int)
	return ps
}

func (ps *procStore) runProc(p *proc) {
	s := p.run()
	ps.procExit <- exitStatus{proc: p, status: s}
}

func (ps *procStore) addProc(m procMap, user string) procMap {
	if _, ok := m[user]; ok {
		return m
	}
	var oldest *proc
	nm := make(procMap)
	for u, p := range m {
		nm[u] = p
		if oldest == nil || p.access.Before(oldest.access) {
			oldest = p
		}
	}
	if ps.maxProcs > 0 && len(nm) >= ps.maxProcs {
		oldest.close()
		delete(nm, oldest.user)
	}
	p := newProc(user)
	go ps.runProc(p)
	nm[user] = p
	return nm
}

func removeProc(m procMap, p *proc) procMap {
	if pu, ok := m[p.user]; !ok || pu != p {
		return m
	}
	p.close()
	nm := make(procMap)
	for u, pu := range m {
		if pu != p {
			nm[u] = pu
		}
	}
	return nm
}

func (ps *procStore) closeAll(m procMap, procErrors chan error) error {
	for _, p := range m {
		p.close()
	}
	waitAll := make(chan error)
	go func() {
		for len(m) > 0 {
			s := <-ps.procExit
			delete(m, s.proc.user)
			if procErrors != nil {
				for _, err := range s.status.errors {
					procErrors <- &ProcError{User: s.proc.user, Err: err}
				}
			}
			if s.status.cleanupFailed {
				waitAll <- procCleanupFailed
				return
			}
		}
		waitAll <- nil
	}()
	select {
	case err := <-waitAll:
		return err
	case <-time.After(procStoreCloseTimeout):
		return procStoreCloseTimeouted
	}
}

func (ps *procStore) run(procErrors chan error) error {
	m := make(procMap)
	for {
		select {
		case user := <-ps.cr:
			m = ps.addProc(m, user)
		case p := <-ps.rm:
			m = removeProc(m, p)
		case s := <-ps.procExit:
			m = removeProc(m, s.proc)
			if procErrors != nil {
				for _, err := range s.status.errors {
					procErrors <- &ProcError{User: s.proc.user, Err: err}
				}
			}
			if s.status.cleanupFailed {
				return &ProcError{User: s.proc.user, Err: procCleanupFailed}
			}
		case <-ps.exit:
			return ps.closeAll(m, procErrors)
		default:
			ps.m <- m
		}
	}
}

func (ps *procStore) getMap() (procMap, error) {
	select {
	case <-ps.exit:
		return nil, procStoreClosed
	case m := <-ps.m:
		return m, nil
	}
}

func (ps *procStore) create(user string) error {
	select {
	case <-ps.exit:
		return procStoreClosed
	case ps.cr <- user:
		return nil
	}
}

func (ps *procStore) get(user string) (*proc, error) {
	for {
		m, err := ps.getMap()
		if err != nil {
			return nil, err
		}
		if p, ok := m[user]; ok {
			return p, nil
		}
		err = ps.create(user)
		if err != nil {
			return nil, err
		}
	}
}

func (ps *procStore) close() {
	close(ps.exit)
}
