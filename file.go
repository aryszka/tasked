package main

import (
	"io"
	"net/http"
	"os"
)

// File name used for the single-file http server.
var fn string

// Writes an error response with a specific status code.
func errorResponse(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

// Writes an error response according to the given error.
func handleError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case os.IsNotExist(err):
		errorResponse(w, http.StatusNotFound)
	case os.IsPermission(err):
		errorResponse(w, http.StatusUnauthorized)
	default:
		errorResponse(w, http.StatusInternalServerError)
	}
	return true
}

// Serves a single file for reading and writing.
// On GET, returns the content of the served file or 404 Not found.
// On PUT, saves the request body as the content of the served file.
// On DELETE, deletes the served file. If the file doesn't exist, it
// doesn't do anything, and returns 200 OK. Delete is not allowed, if
// the process has no write permission on the served file.
// If the running process doesn't have permissions for the requested
// operation, it returns 401 Unauthorized.
// POST can be used instead of PUT.
// If the HTTP method is not GET, PUT or POST, then it returns 405
// Method Not Allowed.
// For any unexpected, returns 500 Internal Server Error.
func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	// todo: OPTIONS, HEAD
	case "GET":
		f, err := os.Open(fn)
		if handleError(w, r, err) {
			return
		}
		defer doretlog42(f.Close)
		s, err := f.Stat()
		if handleError(w, r, err) {
			return
		}
		http.ServeContent(w, r, fn, s.ModTime(), f)
	case "PUT", "POST": // accept post for bad clients
		f, err := os.Create(fn)
		if handleError(w, r, err) {
			return
		}
		defer doretlog42(f.Close)
		_, err = io.Copy(f, r.Body)
		handleError(w, r, err)
	case "DELETE":
		fi, err := os.Stat(fn)
		if os.IsNotExist(err) {
			return
		}
		if handleError(w, r, err) {
			return
		}
		if fi.Mode()&(1<<7) == 0 {
			errorResponse(w, http.StatusUnauthorized)
			return
		}
		err = os.Remove(fn)
		if os.IsNotExist(err) {
			return
		}
		handleError(w, r, err)
	default:
		errorResponse(w, http.StatusMethodNotAllowed)
	}
}
