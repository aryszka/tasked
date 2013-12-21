package htproc

import (
	"errors"
	"fmt"
)

type ProcError struct {
	User string
	Err  error
}

func (pe *ProcError) Error() string {
	return fmt.Sprintf("[%s]: %v\n", pe.User, pe.Err)
}

type procMap map[string]*proc

type procStore struct {
	maxProcs  int
	m         chan procMap
	cr        chan string
	rm        chan *proc
	closed    chan int
	ProcError chan *ProcError
}

var (
	procStoreClosed   = errors.New("ProcStore closed.")
	procCleanupFailed = errors.New("Proc cleanup failed.")
)

func newProcStore(s Settings) *procStore {
	ps := new(procStore)
	ps.maxProcs = s.MaxProcesses()
	ps.m = make(chan procMap)
	ps.cr = make(chan string)
	ps.rm = make(chan *proc)
	ps.closed = make(chan int)
	ps.ProcError = make(chan *ProcError)
	go ps.mapFeed()
	return ps
}

func (ps *procStore) sendProcError(user string, err error) {
	select {
	case <-ps.closed:
	case ps.ProcError <- &ProcError{User: user, Err: err}:
	default:
	}
}

func (ps *procStore) remove(p *proc) {
	select {
	case <-ps.closed:
	case ps.rm <- p:
	}
}

func (ps *procStore) procExited(p *proc) {
	err := <-p.exit
	ps.remove(p)
	if err != nil {
		ps.sendProcError(p.user, err)
	}
}

func (ps *procStore) procCleanup(p *proc) {
	s := <-p.cleanup
	for _, err := range s.errors {
		ps.sendProcError(p.user, err)
	}
	if !s.complete {
		panic(procCleanupFailed)
	}
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
	go ps.procExited(p)
	go ps.procCleanup(p)
	nm[user] = p
	return nm
}

func (ps *procStore) removeProc(m procMap, remove *proc) procMap {
	if p, ok := m[remove.user]; !ok || p != remove {
		return m
	}
	remove.close()
	nm := make(procMap)
	for u, p := range m {
		if p != remove {
			nm[u] = p
		}
	}
	return nm
}

func (ps *procStore) mapFeed() {
	m := make(procMap)
	for {
		select {
		case user := <-ps.cr:
			m = ps.addProc(m, user)
		case p := <-ps.rm:
			m = ps.removeProc(m, p)
		case <-ps.closed:
			for _, p := range m {
				p.close()
			}
			return
		default:
			ps.m <- m
		}
	}
}

func (ps *procStore) getMap() (procMap, error) {
	select {
	case <-ps.closed:
		return nil, procStoreClosed
	case m := <-ps.m:
		return m, nil
	}
}

func (ps *procStore) create(user string) error {
	select {
	case <-ps.closed:
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
	close(ps.closed)
	close(ps.ProcError)
}
