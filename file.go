package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
)

const (
	cmdKey                = "cmd"    // querystring key for method replacement commands
	cmdProps              = "props"  // command replacing the PROPS method
	authTasked            = "tasked" // www-authenticate value used on no permission
	jsonContentType       = "application/json"
	defaultMaxRequestBody = 1 << 30 // todo: make this configurable
)

var (
	dn                  string // directory opened for HTTP
	headerWwwAuth              = http.CanonicalHeaderKey("www-authenticate")
	headerContentType          = http.CanonicalHeaderKey("content-type")
	headerContentLength        = http.CanonicalHeaderKey("content-length")
	maxRequestBody      int64  = defaultMaxRequestBody

	// mapping from HTTP methods to functions
	reqmap = map[string]func(http.ResponseWriter, *http.Request){
		"OPTIONS":  options,
		"HEAD":     get,
		"GET":      get,
		"PROPS":    props,
		"MODPROPS": modprops,
		"PUT":      put,
		"POST":     post,
		"COPY":     copy,
		"RENAME":   rename,
		"DELETE":   delete,
		"MKDIR":    mkdir}
)

func toPropertyMap(fi os.FileInfo, ext bool) map[string]interface{} {
	m := map[string]interface{}{
		"name":    fi.Name(),
		"size":    fi.Size(),
		"modTime": fi.ModTime().Unix(),
		"isDir":   fi.IsDir()}
	if ext {
		m["ext"] = map[string]interface{}{
			"modeString": fmt.Sprint(fi.Mode()),
			"mode":       fi.Mode()}
		// missing:
		// - owner, group, accessTime, changeTime
	}
	return m
}

func getValues(vs map[string][]string, key string, allowed ...string) ([]string, bool) {
	v := vs[key]
	if len(v) == 0 {
		return nil, true
	}
	for _, vi := range v {
		found := false
		for _, ac := range allowed {
			if vi == ac {
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return v, true
}

// Writes an error response with a specific status code
// and with the default status text for that code.
func errorResponse(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

// Writes an error response according to the given error.
// If the error is permission related, it uses 404 Not Found,
// but the response header will contain: 'Www-Authenticate: tasked.'
func checkOsError(w http.ResponseWriter, err error, defaultStatus int) bool {
	if err == nil {
		return true
	}
	switch {
	case os.IsNotExist(err):
		errorResponse(w, http.StatusNotFound)
	case os.IsPermission(err):
		w.Header().Set(headerWwwAuth, authTasked)
		errorResponse(w, http.StatusNotFound)
	default:
		errorResponse(w, defaultStatus)
	}
	return false
}

func checkHandle(w http.ResponseWriter, exp bool, status int) bool {
	if exp {
		return true
	}
	errorResponse(w, status)
	return false
}

func checkBadReq(w http.ResponseWriter, exp bool) bool {
	return checkHandle(w, exp, http.StatusBadRequest)
}

func checkQryCmd(w http.ResponseWriter, r *http.Request, allowed ...string) (string, bool) {
	p, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return "", false
	}
	cmds, ok := getValues(p, cmdKey, allowed...)
	if !checkBadReq(w, ok) {
		return "", false
	}
	if len(cmds) == 0 {
		return "", true
	}
	if !checkBadReq(w, len(cmds) == 1) {
		return "", false
	}
	return cmds[0], true
}

func fileProps(w http.ResponseWriter, r *http.Request) {
	p := path.Join(dn, r.URL.Path)
	fi, err := os.Stat(p)
	if !checkOsError(w, err, http.StatusBadRequest) {
		return
	}
	pr := toPropertyMap(fi, false)
	js, err := json.Marshal(pr)
	if !checkBadReq(w, err == nil) {
		return
	}
	h := w.Header()
	h.Set(headerContentType, jsonContentType)
	h.Set(headerContentLength, fmt.Sprintf("%d", len(js)))
	if r.Method == "HEAD" {
		return
	}
	w.Write(js) // todo: add optional log err and if written count != len
}

func modFileProps(w http.ResponseWriter, r *http.Request) {
	br := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer doretlog42(br.Close)
	b, err := ioutil.ReadAll(br)
	if !checkHandle(w, err == nil, http.StatusRequestEntityTooLarge) {
		return
	}
	if len(b) == 0 {
		return
	}
	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if !checkBadReq(w, err == nil) {
		return
	}
	if len(m) == 0 {
		return
	}

	p := path.Join(dn, r.URL.Path)
	_, err = os.Stat(p)
	if !checkOsError(w, err, http.StatusBadRequest) {
		return
	}

	for k, v := range m {
		switch k {
		case "mode":
			mode, ok := v.(uint32)
			if !checkBadReq(w, ok) {
				return
			}
			// todo: what all can be in os.FileMode?
			err = os.Chmod(p, os.FileMode(mode))
			if !checkOsError(w, err, http.StatusBadRequest) {
				return
			}
			// case "owner":
			// case "group"
		}
	}
}

func getDir(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	// check accept encoding. possible form or json, default form.
	// value, key/merged val, url encoded form
}

func getFile(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	// if accept encoding doesn't match, then error
}

func options(w http.ResponseWriter, r *http.Request) {
	// no-op
}

func props(w http.ResponseWriter, r *http.Request) {
	_, ok := checkQryCmd(w, r)
	if !ok {
		return
	}
	fileProps(w, r)
}

func modprops(w http.ResponseWriter, r *http.Request) {
	_, ok := checkQryCmd(w, r)
	if !ok {
		return
	}
	modFileProps(w, r)
}

func get(w http.ResponseWriter, r *http.Request) {
	cmd, ok := checkQryCmd(w, r, cmdProps)
	if !ok {
		return
	}
	if cmd == cmdProps {
		fileProps(w, r)
		return
	}
	p := path.Join(dn, r.URL.Path)
	f, err := os.Open(p)
	if checkOsError(w, err, http.StatusBadRequest) {
		return
	}
	defer doretlog42(f.Close)
	fi, err := f.Stat()
	if checkOsError(w, err, http.StatusInternalServerError) {
		return
	}
	if fi.IsDir() {
		getDir(w, r, f, fi)
	} else {
		getFile(w, r, f, fi)
	}
}

func put(w http.ResponseWriter, r *http.Request) {
	_, ok := checkQryCmd(w, r)
	if !ok {
		return
	}
	f, err := os.Create(path.Join(dn, r.URL.Path))
	if checkOsError(w, err, http.StatusBadRequest) {
		return
	}
	defer doretlog42(f.Close)
	_, err = io.Copy(f, r.Body)
	checkOsError(w, err, http.StatusInternalServerError)
}

func post(w http.ResponseWriter, r *http.Request) {
	// only modprops, copy, rename, delete, or if empty a put
}

func copy(w http.ResponseWriter, r *http.Request) {
}

func rename(w http.ResponseWriter, r *http.Request) {
}

func delete(w http.ResponseWriter, r *http.Request) {
	err := os.RemoveAll(path.Join(dn, r.URL.Path))
	if os.IsNotExist(err) {
		return
	}
	checkOsError(w, err, http.StatusBadRequest)
}

func mkdir(w http.ResponseWriter, r *http.Request) {
}

// Serves a directory for manipulating its content.
// HTTP method OPTIONS can be used as no-op ping-pong.
//
// On GET, returns the content of the served file or 404 Not found.
// On PUT, saves the request body as the content of the served file.
// On DELETE, deletes the served file. If the file doesn't exist, it
// doesn't do anything, and returns 200 OK.
// If the running process doesn't have permissions for the requested
// operation, it returns 401 Unauthorized.
// POST can be used instead of PUT.
// If the HTTP method is not GET, PUT or POST, then it returns 405
// Method Not Allowed.
// For any unexpected, returns 500 Internal Server Error.
func handler(w http.ResponseWriter, r *http.Request) {
	h := reqmap[r.Method]
	if h == nil {
		errorResponse(w, http.StatusMethodNotAllowed)
		return
	}
	// handle any expectation header with 417 here
	h(w, r)
}
