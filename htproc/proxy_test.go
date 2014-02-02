package htproc

import (
	"bytes"
	tst "code.google.com/p/tasked/testing"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
)

func TestServeProxy(t *testing.T) {
	var (
		s      = &proxy{address: path.Join(tst.Testdir, "sockets/default")}
		hello  = []byte("hello")
		hello2 = []byte("hellohello")
	)

	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) { s.serve(w, r) }
	sx, err := tst.StartSocketServer(s.address)
	tst.ErrFatal(t, err)
	defer sx.Close()

	// ok, empty
	tst.Thndx.Sh = func(w http.ResponseWriter, r *http.Request) {}
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// no connection
	address := s.address
	s.address = path.Join(tst.Testdir, "sockets/invalid")
	err = tst.RemoveIfExists(s.address)
	tst.ErrFatal(t, err)
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	s.address = address

	// send method, receive status code
	tst.Thndx.Sh = func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "SEARCH" {
			t.Fail()
		}
		w.WriteHeader(http.StatusTeapot)
	}
	r, err := http.NewRequest("SEARCH", tst.S.URL, nil)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusTeapot {
			t.Fail()
		}
	})

	// ok, send/recieve, add headers
	tst.Thndx.Sh = func(w http.ResponseWriter, r *http.Request) {
		ch := r.Header["X-Custom-Request-Header"]
		if len(ch) != 1 || ch[0] != "custom-request-header-value" {
			t.Fail()
		}
		body, err := ioutil.ReadAll(r.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(body, hello) {
			t.Fail()
		}
		h := w.Header()
		h["X-Custom-Response-Header"] = []string{"custom-response-header-value"}
		w.Write(hello2)
	}
	r, err = http.NewRequest("GET", tst.S.URL, bytes.NewBuffer(hello))
	r.Header["X-Custom-Request-Header"] = []string{"custom-request-header-value"}
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		ch := rsp.Header["X-Custom-Response-Header"]
		if len(ch) != 1 || ch[0] != "custom-response-header-value" {
			t.Fail()
		}
		body, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(body, hello2) {
			t.Fail()
		}
	})
}
