package main

import (
	"code.google.com/p/tasked/auth"
	"errors"
	"net/http"
	"code.google.com/p/tasked/util"
)

type mux struct {
	auth *auth.Type
	file http.Handler
}

func newMux(a *auth.Type, file http.Handler) (http.Handler, error) {
	if a == nil {
		return nil, errors.New("Auth must be specified.")
	}
	if file == nil {
		return nil, errors.New("File handler must be specified.")
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
	if !util.IsRoot {
		m.file.ServeHTTP(w, r)
		return
	}
}
