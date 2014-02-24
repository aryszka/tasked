package main

import (
	"code.google.com/p/tasked/htauth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/htproc"
	. "code.google.com/p/tasked/share"
	"log"
	"net/http"
	"os/user"
	"strconv"
	"syscall"
)

func serve(o *options) error {
	runas := o.Runas()
	if runas != "" {
		u, err := user.Lookup(runas)
		if err != nil {
			return err
		}
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return err
		}
		err = syscall.Setuid(uid)
		if err != nil {
			return err
		}
	}

	var root http.Handler
	if o.Root() == "" {
		// root = htio.New(o)
	} else {
		root = htfile.New(o)
	}
	var hp *htproc.ProcFilter
	var h http.Handler
	if o.Authenticate() {
		a, err := mkauth(o)
		if err != nil {
			return err
		}
		ha := htauth.New(a, o)
		hp = htproc.New(o)
		hf := EndFilter(root)
		h = CascadeFilters(ha, hp, hf)
	} else {
		h = root
	}

	l, err := listen(o)
	if err != nil {
		return err
	}

	c := make(chan error)
	if hp != nil {
		go func() { c <- hp.Run(nil) }()
	}
	go func() { c <- http.Serve(l, h) }() // max header bytes needs to be set
	return <-c
}

func main() {
	o, err := readOptions()
	if err != nil {
		log.Panicln(err)
	}
	cmd := o.Command()
	switch cmd {
	case cmdServe:
		err := serve(o)
		if err != nil {
			log.Panicln(err)
		}
	default:
		log.Panicln("not there yet, not implemented")
	}
}
