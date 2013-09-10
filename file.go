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

func handleError(w http.ResponseWriter, r *http.Request, err error) bool {
	// todo: check if http.Error writes the status text automatically
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

func empty(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		f, err := os.Open(fn)
		if handleError(w, r, err) {
			return
		}
		defer f.Close() // todo: err is ignored now, retry later then log and forget
		s, err := f.Stat()
		if handleError(w, r, err) {
			return
		}
		http.ServeContent(w, r, fn, s.ModTime(), f)
	case "PUT", "POST": // post only for bad clients
		f, err := os.Create(fn)
		if handleError(w, r, err) {
			return
		}
		_, err = io.Copy(f, r.Body)
		if err != nil {
			handleError(w, r, err)
			return
		}
		empty(w)
	case "DELETE":
		err := os.Remove(fn)
		if handleError(w, r, err) {
			return
		}
		empty(w)
	default:
		errorStatus(w, http.StatusNotFound)
	}
}
