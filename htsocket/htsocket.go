package htsocket

import (
	"code.google.com/p/tasked/share"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"path"
)

type Settings interface {
	Dirname() string
}

type proxy struct {
	dirname string
}

func New(s Settings) share.HttpFilter {
	var p proxy
	if s != nil {
		p.dirname = s.Dirname()
	}
	return &p
}

func (p *proxy) Filter(w http.ResponseWriter, r *http.Request, d interface{}) (interface{}, bool) {
	p.serveHTTP(w, r, d)
	return d, true
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.serveHTTP(w, r, nil)
}
