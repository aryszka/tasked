package main

import (
	"code.google.com/p/tasked/auth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/share"
	"log"
	"net"
	"net/http"
)

func main() {
	var (
		err error
		s   *settings
		a   *auth.Type
		f   http.Handler
		m   http.Handler
		l   net.Listener
	)
	if s, err = getSettings(); err != nil {
		log.Panicln(err)
	}
	if a, err = newAuth(s); err != nil {
		log.Panicln(err)
	}
	if f, err = htfile.New(s); err != nil {
		log.Panicln(err)
	}
	if m, err = newMux(a, f); err != nil {
		log.Panicln(err)
	}
	if l, err = listen(s); err != nil {
		log.Panicln(err)
	}
	defer share.Doretlog42(l.Close)
	if err = http.Serve(l, m); err != nil {
		log.Panicln(err)
	}
}

// todo:
// - document that character encoding of request data is always assumed to be utf-8, explicit declarations are
// ignored
// - make it exit nice, trap term and int, cleanup listener, return
