package main

import (
	"code.google.com/p/tasked/auth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/share"
	"code.google.com/p/tasked/htauth"
	"code.google.com/p/tasked/htproc"
	"code.google.com/p/tasked/htsocket"
	"log"
	"net"
	"net/http"
)

func main() {
	var (
		err error
		s   *settings
		auth   *auth.It
		f   http.Handler
		ha share.HttpFilter
		hf share.HttpFilter
		hp share.HttpFilter
		hs share.HttpFilter
		ps share.HttpFilter
		l   net.Listener
	)
	if s, err = getSettings(); err != nil {
		log.Panicln(err)
	}
	if f, err = htfile.New(s); err != nil {
		log.Panicln(err)
	}
	hf = share.EndFilter(f)
	var hnd http.Handler
	if share.IsRoot {
		if auth, err = newAuth(s); err != nil {
			log.Panicln(err)
		}
		if ha, err = htauth.New(auth, s); err != nil {
			log.Panicln(err)
		}
		hp = htproc.New(s)
		hs = htsocket.New(s)
		ps = share.CascadeFilters(hp, hs)
		hnd = share.CascadeFilters(ha, share.SelectFilter(func(d interface{}) share.HttpFilter {
			if u, ok := d.(string); ok && len(u) > 0 {
				return ps
			}
			return hf
		}))
	} else {
		hnd = hf
	}
	if l, err = listen(s); err != nil {
		log.Panicln(err)
	}
	defer share.Dolog(l.Close)
	if err = http.Serve(l, hnd); err != nil {
		log.Panicln(err)
	}

	// ps := share.CascadeFilters(p, s)
	// ca := share.FilterFunc(func(w http.ResponseWriter, r *http.Request, d interface{}) (interface{}, bool) {
	// 	if user, ok := d.(string); ok && len(user) > 0 {
	// 		return ps.Filter(w, r, d)
	// 	}
	// 	return f.Filter(w, r, d)
// }
	// hnd := share.CascadeFilters(a, ca)

}

// todo:
// - document that character encoding of request data is always assumed to be utf-8, explicit declarations are
// ignored
// - make it exit nice, trap term and int, cleanup listener, return
