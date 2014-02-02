package main

import (
	"code.google.com/p/tasked/htauth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/htproc"
	"code.google.com/p/tasked/share"
	"log"
	"net/http"
)

func main() {
	s, err := getSettings()
	if err != nil {
		log.Panicln(err)
	}
	f, err := htfile.New(s)
	if err != nil {
		log.Panicln(err)
	}
	hf := share.EndFilter(f)
	var hnd http.Handler
	if share.IsRoot {
		auth, err := newAuth(s)
		if err != nil {
			log.Panicln(err)
		}
		ha, err := htauth.New(auth, s)
		if err != nil {
			log.Panicln(err)
		}
		hp := htproc.New(s)
		hp.Run(nil)
		hnd = share.CascadeFilters(ha, hp, hf)
	} else {
		hnd = hf
	}
	l, err := listenTcp(s)
	if err != nil {
		log.Panicln(err)
	}
	defer share.Dolog(l.Close)
	err = http.Serve(l, hnd)
	if err != nil {
		log.Panicln(err)
	}
}

// todo:
// - document that character encoding of request data is always assumed to be utf-8, explicit declarations are
// ignored
// - make it exit nice, trap term and int, cleanup listener, return
