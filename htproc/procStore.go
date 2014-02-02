package htproc

import (
	"errors"
	"fmt"
	"time"
	"path"
)

const (
	maxSocketFailures          = 6
	failureRecoveryTime        = 6 * time.Second
	procStoreCloseTimeout      = 6 * time.Second
	defaultProcIdleCheckPeriod = time.Minute
	defaultProcIdleTimeout     = 12 * time.Minute
	defaultDialTimeout         = 1 * time.Second
)

type ProcError struct {
	User string
	Err  error
}

type exitStatus struct {
	user   string
	proc   runner
	status status
}

type getCreateResult struct {
	proc runner
	err  error
}

type getCreateProc struct {
	user string
	res  chan getCreateResult
}

type runner interface {
	server
	run() status
	close()
}

type procStore struct {
	maxProcs int
	dialTimeout time.Duration
	socketsDir string

	// todo: make struct for the proc related fields
	procs    map[string]runner
	accessed map[string]time.Time
	failures map[string][]time.Time
	banned   map[string]time.Time

	gc   chan getCreateProc
	px   chan exitStatus
	exit chan int
}

var (
	procIdleCheckPeriod     = defaultProcIdleCheckPeriod
	procIdleTimeout         = defaultProcIdleTimeout
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
	if sdto := s.DialTimeout(); sdto > 0 {
		ps.dialTimeout = sdto
	} else {
		ps.dialTimeout = defaultDialTimeout
	}
	ps.socketsDir = path.Join(s.Workdir(), "sockets")
	ps.procs = make(map[string]runner)
	ps.accessed = make(map[string]time.Time)
	ps.failures = make(map[string][]time.Time)
	ps.banned = make(map[string]time.Time)
	ps.gc = make(chan getCreateProc)
	ps.px = make(chan exitStatus)
	ps.exit = make(chan int)
	return ps
}

func (ps *procStore) removeProc(u string) {
	p, ok := ps.procs[u]
	if !ok {
		return
	}
	delete(ps.procs, u)
	delete(ps.accessed, u)
	p.close()
}

func (ps *procStore) removeIdle(now time.Time) {
	for u, _ := range ps.procs {
		if now.Sub(ps.accessed[u]) < procIdleTimeout {
			continue
		}
		ps.removeProc(u)
	}
}

func (ps *procStore) getCreateProc(user string) (runner, error) {
	now := time.Now()
	if p, ok := ps.procs[user]; ok {
		ps.accessed[user] = now
		return p, nil
	}
	if bt, ok := ps.banned[user]; ok && now.Sub(bt) < failureRecoveryTime {
		return nil, temporarilyBanned
	} else if ok {
		delete(ps.banned, user)
	}
	ps.removeIdle(now)
	if ps.maxProcs > 0 && len(ps.procs) >= ps.maxProcs {
		var (
			ou string
			oa = now
		)
		for u, _ := range ps.procs {
			a := ps.accessed[u]
			if a.After(oa) {
				continue
			}
			ou = u
			oa = a
		}
		ps.removeProc(ou)
	}
	p := newProc(path.Join(ps.socketsDir, user), ps.dialTimeout)
	ps.procs[user] = p
	ps.accessed[user] = now
	go func() { ps.px <- exitStatus{user: user, proc: p, status: p.run()} }()
	return p, nil
}

func discardFailures(failures []time.Time, now time.Time) []time.Time {
	for len(failures) > 0 {
		if now.Sub(failures[0]) < failureRecoveryTime {
			break
		}
		failures = failures[1:]
	}
	return failures
}

func (ps *procStore) procErrors(user string, errs []error, notify chan error) {
	if len(errs) == 0 {
		return
	}
	now := time.Now()
	fs := discardFailures(ps.failures[user], now)
	if len(fs) < maxSocketFailures {
		ps.failures[user] = append(fs, now)
	} else {
		delete(ps.failures, user)
		ps.banned[user] = now
	}
	if notify == nil {
		return
	}
	for _, err := range errs {
		notify <- &ProcError{User: user, Err: err}
	}
}

func (ps *procStore) closeAll(procErrors chan error) error {
	pu := make(map[runner]string)
	for u, p := range ps.procs {
		pu[p] = u
		p.close()
	}
	c := len(ps.procs)
	waitAll := make(chan error)
	go func() {
		for c > 0 {
			s := <-ps.px
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

func (ps *procStore) cleanupFailures(now time.Time) {
	for u, fs := range ps.failures {
		ps.failures[u] = discardFailures(fs, now)
	}
	nb := make(map[string]time.Time)
	for u, b := range ps.banned {
		if now.Sub(b) > failureRecoveryTime {
			continue
		}
		nb[u] = b
	}
	ps.banned = nb
}

func (ps *procStore) run(procErrors chan error) error {
	idleCheck := time.After(procIdleCheckPeriod)
	for {
		select {
		case gc := <-ps.gc:
			p, err := ps.getCreateProc(gc.user)
			gc.res <- getCreateResult{proc: p, err: err}
		case s := <-ps.px:
			ps.removeProc(s.user)
			ps.procErrors(s.user, s.status.errors, procErrors)
			if s.status.cleanupFailed {
				return &ProcError{User: s.user, Err: procCleanupFailed}
			}
		case now := <-idleCheck:
			ps.removeIdle(now)
			ps.cleanupFailures(now)
			idleCheck = time.After(procIdleCheckPeriod)
		case <-ps.exit:
			return ps.closeAll(procErrors)
		}
	}
}

func (ps *procStore) getCreate(user string) (runner, error) {
	rc := make(chan getCreateResult)
	select {
	case <-ps.exit:
		return nil, procStoreClosed
	case ps.gc <- getCreateProc{user: user, res: rc}:
		res := <-rc
		return res.proc, res.err
	}
}

func (ps *procStore) close() {
	close(ps.exit)
}
