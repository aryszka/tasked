package htproc

import (
	"net/http"
	"code.google.com/p/tasked/share"
	"net"
	"net/http/httputil"
	"time"
	"io"
)

type socketError struct {
	err error
	address string
	handled bool
}

type proxy struct {
	address string
	timeout time.Duration
}

func (s *socketError) Error() string {
	return s.err.Error()
}

func (s *proxy) serve(w http.ResponseWriter, r *http.Request) error {
	rr, err := http.NewRequest(r.Method, r.URL.Path+"?"+r.URL.RawQuery, r.Body)
	if err != nil {
		return err
	}
	rr.Header = r.Header
	nc, err := net.DialTimeout("unixpacket", s.address, s.timeout)
	if err != nil {
		return &socketError{err: err, address: s.address}
	}
	hc := httputil.NewClientConn(nc, nil)
	defer share.Dolog(hc.Close)
	rsp, err := hc.Do(rr)
	if err != nil {
		return &socketError{err: err, address: s.address}
	}
	defer share.Dolog(rsp.Body.Close)
	h := w.Header()
	for key, value := range rsp.Header {
		h[key] = value
	}
	w.WriteHeader(rsp.StatusCode)
	_, err = io.Copy(w, rsp.Body)
	if err != nil {
		return &socketError{err: err, address: s.address, handled: true}
	}
	return nil
}
