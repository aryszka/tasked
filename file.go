package main

import (
	"io"
	"net/http"
	"os"
)

var fn string

func errorStatus(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

func replyError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case os.IsNotExist(err):
		errorStatus(w, http.StatusNotFound)
	case os.IsPermission(err):
		errorStatus(w, http.StatusUnauthorized)
	default:
		errorStatus(w, http.StatusInternalServerError)
	}
	return true
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	// todo: OPTIONS, HEAD
	case "GET":
		f, err := os.Open(fn)
		if replyError(w, r, err) {
			return
		}
		defer f.Close() // todo: err is ignored now, retry later then log and forget
		s, err := f.Stat()
		if replyError(w, r, err) {
			return
		}
		http.ServeContent(w, r, fn, s.ModTime(), f)
	case "PUT", "POST": // post only for bad clients
		f, err := os.Create(fn)
		if replyError(w, r, err) {
			return
		}
		_, err = io.Copy(f, r.Body)
		if err != nil {
			replyError(w, r, err)
			return
		}
	case "DELETE":
		fi, err := os.Stat(fn)
		if os.IsNotExist(err) {
			return
		}
		if replyError(w, r, err) {
			return
		}
		if fi.Mode()&(1<<7) == 0 {
			replyError(w, r, os.ErrPermission)
			return
		}
		err = os.Remove(fn)
		if os.IsNotExist(err) {
			return
		}
		if replyError(w, r, err) {
			return
		}
	default:
		// todo: should be not supported
		errorStatus(w, http.StatusMethodNotAllowed)
	}
}
