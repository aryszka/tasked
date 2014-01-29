package htproc

import (
	"errors"
	"fmt"
	"time"
)

const (
	maxSocketFailures = 6
	failureRecoveryTime = 6 * time.Second
	procStoreCloseTimeout = 6 * time.Second
	defaultProcIdleCheckPeriod = time.Minute
	defaultProcIdleTimeout = 12 * time.Minute
)

type ProcError struct {
	User string
	Err  error
}

type exitStatus struct {
	proc   runner
	status status
}

type addProc struct {
	user string
	proc runner
	err chan error
}

type runner interface {
	server
	run() status
	accessed() time.Time
	close()
}

type procMap map[string]runner

type procStore struct {
	maxProcs int
	failures map[string]int
	banned   map[string]time.Time
	m        chan procMap
	ad       chan addProc
	rm       chan runner
	procExit chan exitStatus
	exit     chan int
}

var (
	procIdleCheckPeriod = defaultProcIdleCheckPeriod
	procIdleTimeout = defaultProcIdleTimeout
	procStoreClosed         = errors.New("Proc store closed.")
	procCleanupFailed       = errors.New("Proc cleanup failed.")
	procStoreCloseTimeouted = errors.New("Proc store close timeouted.")
	temporarilyBanned       = errors.New("Processes for this user temporarily banned.")
)

func (pe *ProcError) Error() string {
	return fmt.Sprintf("[%s]: %v\n", pe.User, pe.Err)
}

func (pe *ProcError) Fatal() bool {
	return pe.Err == procCleanupFailed
}

func newProcStore(s Settings) *procStore {
	ps := new(procStore)
	ps.maxProcs = s.MaxProcesses()
	ps.m = make(chan procMap)
	ps.ad = make(chan addProc)
	ps.rm = make(chan runner)
	ps.procExit = make(chan exitStatus)
	ps.exit = make(chan int)
	return ps
}

func (ps *procStore) runProc(p runner) {
	s := p.run()
	ps.procExit <- exitStatus{proc: p, status: s}
}

func (ps *procStore) addProc(m procMap, user string, p runner) procMap {
	if _, ok := m[user]; ok {
		return m
	}
	var (
		oldestUser string
		oldestProc runner
		oldestAccess time.Time
	)
	nm := make(procMap)
	for u, p := range m {
		nm[u] = p
		accessed := p.accessed()
		if oldestProc == nil || accessed.Before(oldestAccess) {
			oldestProc, oldestUser, oldestAccess = p, u, accessed
		}
	}
	if ps.maxProcs > 0 && len(nm) >= ps.maxProcs {
		oldestProc.close()
		delete(nm, oldestUser)
	}
	go ps.runProc(p)
	nm[user] = p
	return nm
}

func removeProcs(m procMap, p ...runner) procMap {
	nm := make(procMap)
	for u, pi := range m {
		var (
			remove bool
			idx int
		)
		for i, pii := range p {
			if pi != pii {
				continue
			}
			remove = true
			idx = i
			break
		}
		if remove {
			pi.close()
			p = append(p[:idx], p[idx + 1:]...)
			continue
		}
		nm[u] = pi
	}
	return nm
}

func findUser(m procMap, p runner) string {
	for u, pi := range m {
		if pi != p {
			continue
		}
		return u
	}
	return ""
}

func (ps *procStore) closeAll(m procMap, procErrors chan error) error {
	pu := make(map[runner]string)
	for u, p := range m {
		pu[p] = u
		p.close()
	}
	c := len(m)
	waitAll := make(chan error)
	go func() {
		for c > 0 {
			s := <-ps.procExit
			c--
			if procErrors != nil {
				user, _ := pu[s.proc]
				for _, err := range s.status.errors {
					procErrors <- &ProcError{User: user, Err: err}
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
	f := make(map[string]int)
	b := make(map[string]time.Time)
	idleCheck := time.After(procIdleCheckPeriod)
	for {
		select {
		case ps.m <- m:
		case ad := <-ps.ad:
			if bt, ok := b[ad.user]; ok && time.Now().Sub(bt) < failureRecoveryTime {
				ad.err <- temporarilyBanned
				break
			} else if ok {
				delete(b, ad.user)
			}
			m = ps.addProc(m, ad.user, ad.proc)
			ad.err <- nil
		case p := <-ps.rm:
			m = removeProcs(m, p)
		case s := <-ps.procExit:
			user := findUser(m, s.proc)
			m = removeProcs(m, s.proc)
			for _, err := range s.status.errors {
				if err == socketFailure {
					failures := f[user]
					if failures < maxSocketFailures {
						f[user] = failures + 1
					} else {
						delete(f, user)
						b[user] = time.Now()
					}
					if procErrors == nil {
						break
					}
				}
				if procErrors != nil {
					procErrors <- &ProcError{User: user, Err: err}
				}
			}
			if s.status.cleanupFailed {
				return &ProcError{User: user, Err: procCleanupFailed}
			}
		case now := <-idleCheck:
			var remove []runner
			for _, p := range m {
				if now.Sub(p.accessed()) < procIdleTimeout {
					continue
				}
				remove = append(remove, p)
			}
			m = removeProcs(m, remove...)
			idleCheck = time.After(procIdleCheckPeriod)
		case <-ps.exit:
			return ps.closeAll(m, procErrors)
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

func (ps *procStore) create(user string) (runner, error) {
	p := newProc(user, 1)
	err := make(chan error)
	select {
	case <-ps.exit:
		return nil, procStoreClosed
	case ps.ad <- addProc{user: user, proc: p, err: err}:
		return p, <-err
	}
}

func (ps *procStore) get(user string) (runner, error) {
	for {
		m, err := ps.getMap()
		if err != nil {
			return nil, err
		}
		if p, ok := m[user]; ok {
			return p, nil
		}
		return ps.create(user)
	}
}

func (ps *procStore) remove(p runner) error {
	select {
	case <-ps.exit:
		return procStoreClosed
	case ps.rm <- p:
		return nil
	}
}

func (ps *procStore) close() {
	close(ps.exit)
}
