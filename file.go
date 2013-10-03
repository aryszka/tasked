package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strconv"
	"syscall"
)

const (
	cmdKey                = "cmd"    // querystring key for method replacement commands
	cmdProps              = "props"  // command replacing the PROPS method
	authTasked            = "tasked" // www-authenticate value used on no permission
	jsonContentType       = "application/json"
	defaultMaxRequestBody = 1 << 30               // todo: make this configurable
	modeMask              = os.FileMode(1)<<9 - 1 // the least significant 9 bits
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

func replaceMode(n, m os.FileMode) os.FileMode {
	return n&^modeMask | m&modeMask
}

func toPropertyMap(fi os.FileInfo, ext bool) map[string]interface{} {
	m := map[string]interface{}{
		"name":    fi.Name(),
		"size":    fi.Size(),
		"modTime": fi.ModTime().Unix(),
		"isDir":   fi.IsDir()}
	if ext {
		mm := replaceMode(0, fi.Mode())
		m["modeString"] = fmt.Sprint(mm)
		m["mode"] = mm
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
// (Only useful when the error cause is directly rooted in the request.)
func checkOsError(w http.ResponseWriter, err error) bool {
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
		perr, ok := err.(*os.PathError)
		if perr, ok = err.(*os.PathError); ok && perr.Err.Error() == os.ErrInvalid.Error() {
			errorResponse(w, http.StatusBadRequest)
		} else {
			errorResponse(w, http.StatusInternalServerError)
		}
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

func isOwner(fi os.FileInfo) (bool, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return false, err
	}
	if user.Uid == strconv.Itoa(0) {
		return true, nil
	}
	sstat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	return strconv.Itoa(int(sstat.Uid)) == user.Uid, nil
}

func fileProps(w http.ResponseWriter, r *http.Request) {
	p := path.Join(dn, r.URL.Path)
	fi, err := os.Stat(p)
	if !checkOsError(w, err) {
		return
	}
	ext, err := isOwner(fi)
	if !checkHandle(w, err == nil, http.StatusInternalServerError) {
		return
	}
	pr := toPropertyMap(fi, ext)
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
	w.Write(js)
}

func fileModprops(w http.ResponseWriter, r *http.Request) {
	br := http.MaxBytesReader(w, r.Body, maxRequestBody)
	defer doretlog42(br.Close)
	b, err := ioutil.ReadAll(br)
	if !checkHandle(w, err == nil, http.StatusRequestEntityTooLarge) {
		return
	}
	var m map[string]interface{}
	if len(b) > 0 {
		err = json.Unmarshal(b, &m)
		if !checkBadReq(w, err == nil) {
			return
		}
	}

	p := path.Join(dn, r.URL.Path)
	fi, err := os.Stat(p)
	if !checkOsError(w, err) {
		return
	}

	for k, v := range m {
		switch k {
		case "mode":
			fv, ok := v.(float64)
			if !checkBadReq(w, ok) {
				return
			}
			err = os.Chmod(p, replaceMode(fi.Mode(), os.FileMode(fv)))
			if !checkOsError(w, err) {
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
	fileModprops(w, r)
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
	if checkOsError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	fi, err := f.Stat()
	if checkOsError(w, err) {
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
	if checkOsError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	_, err = io.Copy(f, r.Body)
	checkOsError(w, err)
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
	checkOsError(w, err)
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
