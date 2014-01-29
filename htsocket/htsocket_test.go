package htsocket

import (
	tst "code.google.com/p/tasked/testing"
	"testing"
	"net/http"
	"net/http/httptest"
	"path"
	"bytes"
	"io/ioutil"
)

type settings struct {
	dirname string
}

func (s *settings) Dirname() string { return s.dirname }

func TestNew(t *testing.T) {
	f := New(nil)
	p, ok := f.(*proxy)
	if !ok || p == nil || len(p.dirname) > 0 {
		t.Fail()
	}
	f = New(&settings{})
	p, ok = f.(*proxy)
	if !ok || p == nil || len(p.dirname) > 0 {
		t.Fail()
	}
	f = New(&settings{dirname: "dirname"})
	p, ok = f.(*proxy)
	if !ok || p == nil || p.dirname != "dirname" {
		t.Fail()
	}
}

func TestServe(t *testing.T) {
	var (
		p = &proxy{}
		u = "testuser"
		d interface{} = u
		hello = []byte("hello")
		hello2 = []byte("hellohello")
	)

	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		p.serveHTTP(w, r, d)
	}
	p.dirname = path.Join(tst.Testdir, "sockets")
	s, err := tst.StartSocketServer(path.Join(p.dirname, u))
	tst.ErrFatal(t, err)
	defer s.Close()

	// no user
	d = nil
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	d = u

	// no connection
	dn := p.dirname
	p.dirname = "invalid"
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	p.dirname = dn

	// ok, empty
	tst.Thndx.Sh = func(w http.ResponseWriter, r *http.Request){}
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// ok, send/recieve
	tst.Thndx.Sh = func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(body, hello) {
			t.Fail()
		}
		w.Write(hello2)
	}
	tst.Htreq(t, "GET", tst.S.URL, bytes.NewBuffer(hello), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		body, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(body, hello2) {
			t.Fail()
		}
	})
}

func TestFilter(t *testing.T) {
	p := &proxy{}
	d, h := p.Filter(httptest.NewRecorder(), nil, 1)
	if n, _ := d.(int); n != 1 || !h {
		t.Fail()
	}
}
