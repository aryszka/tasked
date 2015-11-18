package main

// todo: create setuid tests externally

import (
	"testing"
	"path"
	. "github.com/aryszka/tasked/testing"
	"time"
	"github.com/aryszka/tasked/auth"
	"net/http"
	"github.com/aryszka/tasked/htproc"
	"net"
)

func TestNewServer(t *testing.T) {
	s := newServer()
	if s.q == nil {
		t.Fail()
	}
}

func TestRunasUser(t *testing.T) {
	err := runasUser("")
	if err != nil {
		t.Fail()
	}
	err = runasUser("invalid, not existing user")
	if err == nil {
		t.Fail()
	}
}

func TestRunasUserNotRoot(t *testing.T) {
	if IsRoot {
		t.Skip()
	}

	err := runasUser("root")
	if err == nil {
		t.Fail()
	}
}

func TestCreateHandler(t *testing.T) {
	// no auth
	o := new(options)
	o.root = path.Join(Testdir, "root")
	h, p := createHandler(o, nil)
	if h == nil || p != nil {
		t.Fail()
	}

	// auth
	o = new(options)
	o.root = path.Join(Testdir, "root")
	a := auth.New(
		auth.PasswordCheckerFunc(authPam),
		new(authOptions))
	h, p = createHandler(o, a)
	if h == nil || p == nil {
		t.Fail()
	}
}

func TestRun(t *testing.T) {
	// no proc filter
	s := newServer()
	o := new(options)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){})
	l, err := net.Listen("tcp", ":9099")
	ErrFatal(t, err)
	s.l = l
	WithTimeout(t, 240 * time.Millisecond, func() {
		done := make(chan int)
		go func() {
			err := s.run(o, h)
			if err != nil {
				t.Fail()
			}
			done <- 0
		}()
		<-time.After(120 * time.Millisecond)
		close(s.q)
		<-done
	})

	// proc filter
	s = newServer()
	o = new(options)
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){})
	l, err = net.Listen("tcp", ":9099")
	ErrFatal(t, err)
	s.l = l
	s.p = htproc.New(o)
	WithTimeout(t, 240 * time.Millisecond, func() {
		done := make(chan int)
		go func() {
			err := s.run(o, h)
			if err != nil {
				t.Fail()
			}
			done <- 0
		}()
		<-time.After(120 * time.Millisecond)
		close(s.q)
		<-done
	})
}

func TestServe(t *testing.T) {
	// runas user fails
	s := newServer()
	o := new(options)
	o.runas = "invalid, not existing user"
	err := s.serve(o)
	if err == nil {
		t.Fail()
	}

	// mkauth fails
	s = newServer()
	o = new(options)
	o.authenticate = true
	f := path.Join(Testdir, "not existing file")
	RemoveIfExistsF(t, f)
	o.aesKeyFile = f
	err = s.serve(o)
	if err == nil {
		t.Fail()
	}

	// listen fails
	s = newServer()
	o = new(options)
	o.address = "http::"
	err = s.serve(o)
	if err == nil {
		t.Fail()
	}

	// listen
	s = newServer()
	o = new(options)
	o.tlsKey = testTlsKey
	o.tlsCert = testTlsCert
	o.root = path.Join(Testdir, "root")
	o.address = "https:9099"
	EnsureDirF(t, o.root)
	WithTimeout(t, 240 * time.Millisecond, func() {
		done := make(chan int)
		go func() {
			err := s.serve(o)
			if err != nil {
				t.Fail()
			}
			done <- 0
		}()
		<-time.After(120 * time.Millisecond)
		s.close()
		<-done
	})
}

func TestServeAuthenticate(t *testing.T) {
	if !IsRoot {
		t.Skip()
	}

	s := newServer()
	o := new(options)
	o.authenticate = true
	o.tlsKey = testTlsKey
	o.tlsCert = testTlsCert
	o.root = path.Join(Testdir, "root")
	o.address = "https:9099"
	EnsureDirF(t, o.root)
	WithTimeout(t, 240 * time.Millisecond, func() {
		done := make(chan int)
		go func() {
			err := s.serve(o)
			if err != nil {
				t.Fail()
			}
			done <- 0
		}()
		<-time.After(120 * time.Millisecond)
		s.close()
		<-done
	})
}
