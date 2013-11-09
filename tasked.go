package main

import (
	"code.google.com/p/tasked/auth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/util"
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
	defer util.Doretlog42(l.Close)
	if err = http.Serve(l, m); err != nil {
		log.Panicln(err)
	}
}
