package htproc

import (
	"code.google.com/p/tasked/share"
	. "code.google.com/p/tasked/share"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"time"
)

type proxy struct {
	address string
	timeout time.Duration
}

func (p *proxy) serve(w http.ResponseWriter, r *http.Request) error {
	rr, err := http.NewRequest(r.Method, r.URL.Path+"?"+r.URL.RawQuery, r.Body)
	if !CheckHandle(w, err == nil, http.StatusNotFound) {
		return err
	}
	rr.Header = r.Header
	nc, err := net.DialTimeout("unixpacket", p.address, p.timeout)
	if !CheckHandle(w, err == nil, http.StatusNotFound) {
		return err
	}
	hc := httputil.NewClientConn(nc, nil)
	defer share.Dolog(hc.Close)
	rsp, err := hc.Do(rr)
	if !CheckHandle(w, err == nil, http.StatusNotFound) {
		return err
	}
	defer share.Dolog(rsp.Body.Close)
	h := w.Header()
	for key, value := range rsp.Header {
		h[key] = value
	}
	w.WriteHeader(rsp.StatusCode)
	io.Copy(w, rsp.Body) // todo: what to do with this error, probably only diag log
	return nil
}
