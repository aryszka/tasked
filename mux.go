package main

import (
	"code.google.com/p/tasked/auth"
	"code.google.com/p/tasked/share"
	"errors"
	"net/http"
)

var (
	noAuth        = errors.New("Auth must be specified.")
	noFileHandler = errors.New("File handler must be specified.")
)

type mux struct {
	auth *auth.Type
	file http.Handler
}

func newMux(a *auth.Type, file http.Handler) (http.Handler, error) {
	if a == nil {
		return nil, noAuth
	}
	if file == nil {
		return nil, noFileHandler
	}
	var m mux
	m.auth = a
	m.file = file
	return &m, nil
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if root, htfile and return
	// auth
	// if handled return
	// if authenticated, own process
	// if not authenticated, htfile
	if !share.IsRoot {
		m.file.ServeHTTP(w, r)
		return
	}
}
