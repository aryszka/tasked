package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

const (
	cmdKey                  = "cmd"   // querystring key for method replacement commands
	cmdProps                = "props" // command replacing the PROPS method
	cmdSearch               = "search"
	jsonContentType         = "application/json; charset=utf-8"
	defaultMaxRequestBody   = 1 << 30               // todo: make this configurable
	modeMask                = os.FileMode(1)<<9 - 1 // the least significant 9 bits
	defaultMaxSearchResults = 30
	searchQueryMax          = "max"
	searchQueryName         = "name"
	searchQueryContent      = "content"

	// not for files, because of privacy
	authTasked = "tasked" // www-authenticate value used on no permission
)

type fileInfo struct {
	sys     os.FileInfo
	dirname string
}

func (fi *fileInfo) Name() string       { return fi.sys.Name() }
func (fi *fileInfo) Size() int64        { return fi.sys.Size() }
func (fi *fileInfo) Mode() os.FileMode  { return fi.sys.Mode() }
func (fi *fileInfo) ModTime() time.Time { return fi.sys.ModTime() }
func (fi *fileInfo) IsDir() bool        { return fi.sys.IsDir() }
func (fi *fileInfo) Sys() interface{}   { return fi.sys.Sys() }

var (
	dn                  string // directory opened for HTTP
	headerContentType          = http.CanonicalHeaderKey("content-type")
	headerContentLength        = http.CanonicalHeaderKey("content-length")
	maxRequestBody      int64  = defaultMaxRequestBody

	// not for files, because of privacy
	headerWwwAuth = http.CanonicalHeaderKey("www-authenticate")

	// mapping from HTTP methods to functions
	reqmap = map[string]func(http.ResponseWriter, *http.Request){
		"OPTIONS":  options,
		"HEAD":     get,
		"SEARCH":   search,
		"GET":      get,
		"PROPS":    props,
		"MODPROPS": modprops,
		"PUT":      put,
		"POST":     post,
		"COPY":     copy,
		"RENAME":   rename,
		"DELETE":   delete,
		"MKDIR":    mkdir}
	textMimeTypes = map[string]string{
		"css":    "text/css; charset=utf-8",
		"html":   "text/html; charset=utf-8",
		"js":     "application/x-javascript",
		"xml":    "text/xml; charset=utf-8",
		"txt":    "text/plain; charset=utf-8",
		"txt16l": "text/plain; charset=utf-16le",
		"txt16b": "text/plain; charset=utf-16be"}
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
	if fii, ok := fi.(*fileInfo); ok {
		m["dirname"] = fii.dirname
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
	case os.IsNotExist(err), os.IsPermission(err):
		errorResponse(w, http.StatusNotFound)
	default:
		if perr, ok := err.(*os.PathError); ok && perr.Err.Error() == os.ErrInvalid.Error() {
			errorResponse(w, http.StatusBadRequest)
		} else if serr, ok := err.(*os.SyscallError); ok {
			if nerr, ok := serr.Err.(syscall.Errno); ok &&
				(nerr == syscall.ENOENT || nerr == syscall.EPERM || nerr == syscall.EACCES) {
				errorResponse(w, http.StatusNotFound)
			}
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

func checkServerError(w http.ResponseWriter, exp bool) bool {
	return checkHandle(w, exp, http.StatusInternalServerError)
}

func checkQryValuesCmd(w http.ResponseWriter, qry url.Values, allowed ...string) (string, bool) {
	cmds, ok := getValues(qry, cmdKey, allowed...)
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

func checkQryCmd(w http.ResponseWriter, r *http.Request, allowed ...string) (string, bool) {
	p, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return "", false
	}
	return checkQryValuesCmd(w, p, allowed...)
}

func isOwner(u *user.User, fi os.FileInfo) (bool, error) {
	if u.Uid == strconv.Itoa(0) {
		return true, nil
	}
	sstat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	return strconv.Itoa(int(sstat.Uid)) == u.Uid, nil
}

func writeJsonResponse(w http.ResponseWriter, r *http.Request, c []byte) (int, error) {
	h := w.Header()
	h.Set(headerContentType, jsonContentType)
	h.Set(headerContentLength, fmt.Sprintf("%d", len(c)))
	if r.Method == "HEAD" {
		return 0, nil
	}
	return w.Write(c)
}

func detectContentType(name string, f *os.File) (ct string, err error) {
	ct = mime.TypeByExtension(filepath.Ext(name))
	if len(ct) > 0 {
		return
	}
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return
	}
	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		return
	}
	ct = http.DetectContentType(buf[:n])
	return
}

// Executes breadth first file search with a limit on the number of the results.
func searchFiles(dirs []*fileInfo, max int, qry func(fi *fileInfo) bool) []*fileInfo {
	if max <= 0 || len(dirs) == 0 {
		return nil
	}
	var res []*fileInfo
	di := dirs[0]
	dirs = dirs[1:]
	p := path.Join(di.dirname, di.Name())
	if d, err := os.Open(p); err == nil {
		defer doretlog42(d.Close)
		if fis, err := d.Readdir(0); err == nil {
			for _, fi := range fis {
				fii := &fileInfo{sys: fi, dirname: p}
				if qry(fii) {
					res = append(res, fii)
					max = max - 1
					if max == 0 {
						return res
					}
				}
				if fii.IsDir() {
					dirs = append(dirs, fii)
				}
			}
		}
	}
	return append(res, searchFiles(dirs, max, qry)...)
}

func fileSearch(w http.ResponseWriter, r *http.Request, qry url.Values) {
	var err error
	max := defaultMaxSearchResults
	if qryMax, ok := qry[searchQueryMax]; ok {
		if !checkBadReq(w, len(qryMax) == 1) {
			return
		}
		max, err = strconv.Atoi(qryMax[0])
		if !checkBadReq(w, err == nil) {
			return
		}
		if max > defaultMaxSearchResults {
			max = defaultMaxSearchResults
		}
	}
	names := qry[searchQueryName]
	if !checkBadReq(w, len(names) < 2) {
		return
	}
	contents := qry[searchQueryContent]
	if !checkBadReq(w, len(contents) < 2) {
		return
	}
	var name, content string
	if len(names) > 0 {
		name = names[0]
	}
	if len(contents) > 0 {
		content = contents[0]
	}
	var rxn, rxc *regexp.Regexp
	if len(name) > 0 {
		rxn, err = regexp.Compile(name)
		if !checkBadReq(w, err == nil) {
			return
		}
	}
	if len(content) > 0 {
		rxc, err = regexp.Compile(content)
		if !checkBadReq(w, err == nil) {
			return
		}
	}
	p := path.Join(dn, r.URL.Path)
	di, err := os.Lstat(p)
	if !checkOsError(w, err) {
		return
	}
	result := searchFiles([]*fileInfo{&fileInfo{sys: di, dirname: path.Dir(p)}}, max, func(fi *fileInfo) bool {
		if len(name) > 0 && !rxn.MatchString(fi.Name()) {
			return false
		}
		if len(content) > 0 {
			if fi.IsDir() {
				return false
			}
			f, err := os.Open(path.Join(fi.dirname, fi.Name()))
			if err != nil {
				return false
			}
			defer doretlog42(f.Close)
			ct, err := detectContentType(fi.Name(), f)
			if err != nil {
				return false
			}
			textType := false
			for _, tct := range textMimeTypes {
				if tct == ct {
					textType = true
					break
				}
			}
			if !textType || !rxc.MatchReader(bufio.NewReader(f)) {
				return false
			}
		}
		return true
	})
	pmaps := make([]map[string]interface{}, len(result))
	for i, fi := range result {
		pmaps[i] = toPropertyMap(fi, false)
	}
	js, err := json.Marshal(pmaps)
	if !checkServerError(w, err == nil) {
		return
	}
	writeJsonResponse(w, r, js)
}

func fileProps(w http.ResponseWriter, r *http.Request) {
	p := path.Join(dn, r.URL.Path)
	fi, err := os.Stat(p)
	if !checkOsError(w, err) {
		return
	}
	u, err := user.Current()
	if !checkServerError(w, u != nil && err == nil) {
		return
	}
	own, err := isOwner(u, fi)
	if !checkServerError(w, err == nil) {
		return
	}
	pr := toPropertyMap(fi, own)
	js, err := json.Marshal(pr)
	if !checkServerError(w, err == nil) {
		return
	}
	writeJsonResponse(w, r, js)
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

	u, err := user.Current()
	if !checkServerError(w, u != nil && err == nil) {
		return
	}
	own, err := isOwner(u, fi)
	if !checkServerError(w, err == nil) || !checkHandle(w, own, http.StatusNotFound) {
		return
	}

	for k, v := range m {
		switch k {
		case "mode":
			fv, ok := v.(float64)
			if !checkBadReq(w, ok) {
				return
			}
			err := os.Chmod(p, replaceMode(fi.Mode(), os.FileMode(fv)))
			if !checkOsError(w, err) {
				return
			}
			// case "owner":
			// case "group"
		}
	}
}

func getDir(w http.ResponseWriter, r *http.Request, d *os.File) {
	dfis, err := d.Readdir(0)
	if !checkOsError(w, err) {
		return
	}
	u, err := user.Current()
	if !checkServerError(w, u != nil && err == nil) {
		return
	}
	prs := make([]map[string]interface{}, len(dfis))
	for i, dfi := range dfis {
		own, err := isOwner(u, dfi)
		if !checkServerError(w, err == nil) {
			return
		}
		prs[i] = toPropertyMap(dfi, own)
	}
	js, err := json.Marshal(prs)
	if !checkServerError(w, err == nil) {
		return
	}
	writeJsonResponse(w, r, js)
}

func getFile(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	// here a couple of seek/read errors may appear
	ct, err := detectContentType(fi.Name(), f)
	if !checkServerError(w, err == nil) {
		return
	}
	h := w.Header()
	h.Set(headerContentType, ct)
	h.Set(headerContentLength, fmt.Sprintf("%d", fi.Size()))
	if r.Method == "HEAD" {
		return
	}
	w.WriteHeader(http.StatusOK)
	io.Copy(w, f)
}

func options(w http.ResponseWriter, r *http.Request) {
	// no-op
}

func search(w http.ResponseWriter, r *http.Request) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return
	}
	if _, ok := checkQryValuesCmd(w, qry); !ok {
		return
	}
	fileSearch(w, r, qry)
}

func get(w http.ResponseWriter, r *http.Request) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return
	}
	cmd, ok := checkQryValuesCmd(w, qry, cmdProps, cmdSearch)
	if !ok {
		return
	}
	switch cmd {
	case cmdProps:
		fileProps(w, r)
		return
	case cmdSearch:
		fileSearch(w, r, qry)
		return
	}
	p := path.Join(dn, r.URL.Path)
	f, err := os.Open(p)
	if !checkOsError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	fi, err := f.Stat()
	if !checkOsError(w, err) {
		return
	}
	if fi.IsDir() {
		getDir(w, r, f)
		return

	}
	getFile(w, r, f, fi)
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
