package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
)

const (
	cmdKey          = "cmd"    // querystring key for method replacement commands
	cmdProps        = "props"  // command replacing the PROPS method
	authTasked      = "tasked" // www-authenticate value used on no permission
	jsonContentType = "application/json"
)

type propertiesExt struct {
	Mode       os.FileMode
	Owner      string
	Group      string
	ModeString string
	AccessTime int64
	ChangeTime int64
}

type properties struct {
	// owner only
	Ext *propertiesExt `json:"omitempty"`

	// everybody can see with read rights
	Name    string
	Size    int64
	ModTime int64
	IsDir   bool
}

var (
	dn                  string // directory opened for HTTP
	wwwAuthHeader       = http.CanonicalHeaderKey("www-authenticate")
	contentTypeHeader   = http.CanonicalHeaderKey("content-type")
	contentLengthHeader = http.CanonicalHeaderKey("content-length")

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

func toProps(fi os.FileInfo, ext bool) *properties {
	if fi == nil {
		return nil
	}
	p := &properties{
		Name:    fi.Name(),
		Size:    fi.Size(),
		ModTime: fi.ModTime().Unix(),
		IsDir:   fi.IsDir()}
	if ext {
		p.Ext = &propertiesExt{
			ModeString: fmt.Sprint(fi.Mode()),
			Mode:       fi.Mode()}
	}
	return p
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
func checkHandleError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case os.IsNotExist(err):
		errorResponse(w, http.StatusNotFound)
	case os.IsPermission(err):
		w.Header().Set(wwwAuthHeader, authTasked)
		errorResponse(w, http.StatusNotFound)
	default:
		errorResponse(w, http.StatusInternalServerError)
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
	if !checkHandleError(w, err) {
		return
	}
	pr := toProps(fi, false)
	if !checkHandle(w, pr != nil, http.StatusInternalServerError) {
		return
	}
	js, err := json.Marshal(pr)
	if !checkHandleError(w, err) {
		return
	}
	h := w.Header()
	h.Set(contentTypeHeader, jsonContentType)
	h.Set(contentLengthHeader, fmt.Sprintf("%d", len(js)))
	if r.Method == "HEAD" {
		return
	}
	w.Write(js) // TODO: log err and if written count != len
}

func getDir(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	// check accept encoding. possible form or json, default form.
	// value, key/merged val, url encoded form
}

func getFile(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	// if accept encoding doesn't match, then error
}

func options(w http.ResponseWriter, r *http.Request) { /* no-op */
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
	if checkHandleError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	fi, err := f.Stat()
	if checkHandleError(w, err) {
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
	f, err := os.Create(dn)
	if checkHandleError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	_, err = io.Copy(f, r.Body)
	checkHandleError(w, err)
}

func post(w http.ResponseWriter, r *http.Request) {
	// only modprops, copy, rename, delete, or if empty a put
}

func copy(w http.ResponseWriter, r *http.Request) {
}

func rename(w http.ResponseWriter, r *http.Request) {
}

func delete(w http.ResponseWriter, r *http.Request) {
	fi, err := os.Stat(dn)
	if os.IsNotExist(err) {
		return
	}
	if !checkHandleError(w, err) {
		return
	}
	if !checkHandle(w, fi.Mode()&(1<<7) == 0, http.StatusUnauthorized) {
		return
	}
	err = os.Remove(dn)
	if os.IsNotExist(err) {
		return
	}
	checkHandleError(w, err)
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
