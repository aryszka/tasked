package main

import (
	"os/user"
	"strconv"
	"syscall"
	"net/http"
	"github.com/aryszka/tasked/auth"
	"github.com/aryszka/tasked/htfile"
	"github.com/aryszka/tasked/htproc"
	"github.com/aryszka/tasked/htauth"
	. "github.com/aryszka/tasked/share"
	"net"
)

type server struct {
	l net.Listener
	p *htproc.ProcFilter
	q chan int
}

func newServer() *server {
	s := new(server)
	s.q = make(chan int)
	return s
}

func runasUser(un string) error {
	if un == "" {
		return nil
	}
	u, err := user.Lookup(un)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	if err = syscall.Setgid(gid); err != nil {
		return err
	} else {
		return syscall.Setuid(uid)
	}
}

func createHandler(o *options, a *auth.It) (http.Handler, *htproc.ProcFilter) {
	var root http.Handler
	if o.Root() == "" {
		// root = htio.New(o)
	} else {
		root = htfile.New(o)
	}
	if a == nil {
		return root, nil
	}
	ha := htauth.New(a, o)
	p := htproc.New(o)
	hf := EndFilter(root)
	return CascadeFilters(ha, p, hf), p
}

func (s *server) run(o *options, h http.Handler) error {
	ep := make(chan error)
	if s.p != nil {
		go func() { ep <- s.p.Run(nil) }()
	}
	es := make(chan error)
	go func() {
		hs := new(http.Server)
		hs.Handler = h
		hs.MaxHeaderBytes = o.MaxRequestHeader()
		es <- hs.Serve(s.l)
	}()
	select {
	case err := <-ep:
		Doretlog42(s.l.Close)
		return err
	case err := <-es:
		if s.p != nil {
			s.p.Close()
		}
		return err
	case <-s.q:
		Doretlog42(s.l.Close)
		if s.p != nil {
			s.p.Close()
		}
		return nil
	}
}

func (s *server) serve(o *options) error {
	var (
		a *auth.It
		h http.Handler
		p *htproc.ProcFilter
		l net.Listener
		err error
	)
	if err = runasUser(o.Runas()); err != nil {
		return err
	}
	if o.Authenticate() {
		if a, err = mkauth(o); err != nil {
			return err
		}
	}
	h, p = createHandler(o, a)
	if l, err = listen(o); err != nil {
		return err
	}
	s.p = p
	s.l = l
	return s.run(o, h)
}

func (s *server) close() {
	close(s.q)
}
