package main

import (
	"bufio"
	"encoding/json"
	"errors"
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
	jsonContentType         = "application/json; charset=utf-8"
	defaultMaxRequestBody   = 1 << 30 // todo: make this configurable
	modeMask                = os.FileMode(1)<<9 - 1
	defaultMaxSearchResults = 30
	searchQueryMax          = "max"
	searchQueryName         = "name"
	searchQueryContent      = "content"
	copyRenameToKey         = "to"
	cmdKey                  = "cmd"
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

type maxReader struct {
	reader io.Reader
	count  int64
}

func (mr *maxReader) Read(b []byte) (n int, err error) {
	if mr.count <= 0 {
		return 0, errors.New("Maximum read count exceeded.")
	}
	if int64(len(b)) > mr.count {
		b = b[:mr.count]
	}
	n, err = mr.reader.Read(b)
	mr.count = mr.count - int64(n)
	return
}

type queryHandler func(http.ResponseWriter, *http.Request, url.Values)

var (
	dn                  string
	headerContentType         = http.CanonicalHeaderKey("content-type")
	headerContentLength       = http.CanonicalHeaderKey("content-length")
	maxRequestBody      int64 = defaultMaxRequestBody
	marshalError        error = errors.New("Marshaling error.")

	textMimeTypes = map[string]string{
		"css":    "text/css; charset=utf-8",
		"html":   "text/html; charset=utf-8",
		"js":     "application/x-javascript",
		"xml":    "text/xml; charset=utf-8",
		"txt":    "text/plain; charset=utf-8",
		"txt16l": "text/plain; charset=utf-16le",
		"txt16b": "text/plain; charset=utf-16be"}
)

func init() {
	syscall.Umask(0077)
}

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
		if sstat, ok := fi.Sys().(*syscall.Stat_t); ok {
			if owner, err := user.LookupId(strconv.Itoa(int(sstat.Uid))); err == nil {
				m["user"] = owner.Username
			}
			if gn, err := lookupGroupName(sstat.Gid); err == nil {
				m["group"] = gn
			}
			m["accessTime"] = sstat.Atim.Sec
			m["changeTime"] = sstat.Ctim.Sec
		}
	}
	if fii, ok := fi.(*fileInfo); ok {
		m["dirname"] = fii.dirname
	}
	return m
}

func pathIntersect(p0, p1 string) int {
	if len(p0) == len(p1) {
		if p0 == p1 {
			return 3
		}
		return 0
	}
	res := 1
	if len(p0) < len(p1) {
		p0, p1 = p1, p0
		res = 2
	}
	if p0[:len(p1)] != p1 || p0[len(p1)] != '/' {
		res = 0
	}
	return res
}

func errorResponse(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

func handleErrno(w http.ResponseWriter, errno syscall.Errno) {
	switch errno {
	case syscall.ENOENT, syscall.EPERM, syscall.EACCES, syscall.EISDIR:
		errorResponse(w, http.StatusNotFound)
	case syscall.EINVAL:
		errorResponse(w, http.StatusBadRequest)
	default:
		errorResponse(w, http.StatusInternalServerError)
	}
}

func checkOsError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case os.IsNotExist(err), os.IsPermission(err):
		errorResponse(w, http.StatusNotFound)
	default:
		if perr, ok := err.(*os.PathError); ok {
			if errno, ok := perr.Err.(syscall.Errno); ok {
				handleErrno(w, errno)
				return false
			}
		}
		if serr, ok := err.(*os.SyscallError); ok {
			if errno, ok := serr.Err.(syscall.Errno); ok {
				handleErrno(w, errno)
				return false
			}
		}
		if lerr, ok := err.(*os.LinkError); ok {
			if errno, ok := lerr.Err.(syscall.Errno); ok {
				handleErrno(w, errno)
				return false
			}
		}
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

func checkServerError(w http.ResponseWriter, exp bool) bool {
	return checkHandle(w, exp, http.StatusInternalServerError)
}

func checkQryValuesCmd(w http.ResponseWriter, qry url.Values, allowed ...string) (string, bool) {
	cmds := qry[cmdKey]
	for _, cmd := range cmds {
		found := false
		for _, ac := range allowed {
			if cmd == ac {
				found = true
				break
			}
		}
		if !checkBadReq(w, found) {
			return "", false
		}
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
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return "", false
	}
	return checkQryValuesCmd(w, qry, allowed...)
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

func writeJsonResponse(w http.ResponseWriter, r *http.Request, d interface{}) (int, error) {
	js, err := json.Marshal(d)
	if err != nil {
		return 0, marshalError
	}
	h := w.Header()
	h.Set(headerContentType, jsonContentType)
	h.Set(headerContentLength, fmt.Sprintf("%d", len(js)))
	if r.Method == "HEAD" {
		return 0, nil
	}
	return w.Write(js)
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

func searchFiles(dirs []*fileInfo, max int, qry func(fi *fileInfo) bool) []*fileInfo {
	if max <= 0 || len(dirs) == 0 {
		return nil
	}
	return append(func() (res []*fileInfo) {
		di := dirs[0]
		dirs = dirs[1:]
		p := path.Join(di.dirname, di.Name())
		d, err := os.Open(p)
		if err != nil {
			return
		}
		defer doretlog42(d.Close)
		fis, err := d.Readdir(0)
		if err != nil {
			return
		}
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
		return
	}(), searchFiles(dirs, max, qry)...)
}

func copyTree(from, to string) error {
	if from == to {
		return nil
	}
	fi, err := os.Lstat(from)
	if err != nil {
		return err
	}
	ff, err := os.Open(from)
	if err != nil {
		return err
	}
	defer doretlog42(ff.Close)
	if fi.IsDir() {
		err = os.Mkdir(to, os.ModePerm)
		if err != nil {
			return err
		}
		fis, err := ff.Readdir(0)
		if err != nil {
			return err
		}
		for _, fii := range fis {
			err = copyTree(path.Join(from, fii.Name()), path.Join(to, fii.Name()))
			if err != nil {
				return err
			}
		}
	} else {
		tf, err := os.Create(to)
		if err != nil {
			return err
		}
		defer doretlog42(tf.Close)
		_, err = io.Copy(tf, ff)
		if err != nil {
			return err
		}
	}
	return os.Chmod(to, fi.Mode())
}

func getPath(sp string) (string, error) {
	p := path.Join(dn, sp)
	i := pathIntersect(dn, p)
	if i < 2 {
		return "", errors.New("Invalid path.")
	}
	return p, nil
}

func getQryNum(qry url.Values, key string) (int, error) {
	ns, ok := qry[key]
	if !ok {
		return 0, nil
	}
	if len(ns) != 1 {
		return 0, errors.New("Invalid querystring.")
	}
	return strconv.Atoi(ns[0])
}

func getQryExpression(qry url.Values, key string) (*regexp.Regexp, error) {
	exprs := qry[key]
	if len(exprs) > 1 {
		return nil, errors.New("Invalid querystring.")
	}
	if len(exprs) == 0 {
		return nil, nil
	}
	expr := exprs[0]
	if len(expr) == 0 {
		return nil, nil
	}
	return regexp.Compile(expr)
}

func searchf(w http.ResponseWriter, r *http.Request, qry url.Values) {
	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	max, err := getQryNum(qry, searchQueryMax)
	if !checkBadReq(w, err == nil) {
		return
	}
	if max <= 0 || max > defaultMaxSearchResults {
		max = defaultMaxSearchResults
	}
	rxn, err := getQryExpression(qry, searchQueryName)
	if !checkBadReq(w, err == nil) {
		return
	}
	rxc, err := getQryExpression(qry, searchQueryContent)
	if !checkBadReq(w, err == nil) {
		return
	}
	di, err := os.Lstat(p)
	if !checkOsError(w, err) {
		return
	}
	result := searchFiles([]*fileInfo{&fileInfo{sys: di, dirname: path.Dir(p)}}, max, func(fi *fileInfo) bool {
		if rxn != nil && !rxn.MatchString(fi.Name()) {
			return false
		}
		if rxc != nil {
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
	_, err = writeJsonResponse(w, r, pmaps)
	checkServerError(w, err != marshalError)
}

func propsf(w http.ResponseWriter, r *http.Request) {
	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	fi, err := os.Lstat(p)
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
	_, err = writeJsonResponse(w, r, pr)
	checkServerError(w, err != marshalError)
}

func modpropsf(w http.ResponseWriter, r *http.Request) {
	mr := &maxReader{reader: r.Body, count: maxRequestBody}
	b, err := ioutil.ReadAll(mr)
	if !checkHandle(w, err == io.EOF || mr.count > 0, http.StatusRequestEntityTooLarge) ||
		!checkServerError(w, err == nil) {
		return
	}
	var m map[string]interface{}
	if len(b) > 0 {
		err = json.Unmarshal(b, &m)
		if !checkBadReq(w, err == nil) {
			return
		}
	}

	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	fi, err := os.Lstat(p)
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
	_, err = writeJsonResponse(w, r, prs)
	checkServerError(w, err != marshalError)
}

func getFile(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
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

func putf(w http.ResponseWriter, r *http.Request) {
	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.MkdirAll(path.Dir(p), os.ModePerm)
	if !checkOsError(w, err) {
		return
	}
	f, err := os.Create(p)
	if !checkOsError(w, err) {
		return
	}
	defer doretlog42(f.Close)
	if !checkOsError(w, err) {
		return
	}
	mr := &maxReader{reader: r.Body, count: maxRequestBody}
	_, err = io.Copy(f, mr)
	if checkHandle(w, err == io.EOF || mr.count > 0, http.StatusRequestEntityTooLarge) {
		checkOsError(w, err)
	}
}

func copyRename(multi bool, f func(string, string) error) queryHandler {
	return func(w http.ResponseWriter, r *http.Request, qry url.Values) {
		tos, ok := qry[copyRenameToKey]
		if !checkBadReq(w, ok && (multi || len(tos) == 1)) {
			return
		}
		from, err := getPath(r.URL.Path)
		if !checkHandle(w, err == nil, http.StatusNotFound) {
			return
		}
		for _, to := range tos {
			to, err := getPath(to)
			if !checkHandle(w, err == nil, http.StatusNotFound) ||
				!checkBadReq(w, pathIntersect(from, to) == 0) {
				return
			}
			err = f(from, to)
			if !checkOsError(w, err) {
				return
			}
		}
	}
}

var (
	copyf   = copyRename(true, copyTree)
	renamef = copyRename(false, os.Rename)
)

func deletef(w http.ResponseWriter, r *http.Request) {
	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.RemoveAll(p)
	if os.IsNotExist(err) {
		return
	}
	checkOsError(w, err)
}

func mkdirf(w http.ResponseWriter, r *http.Request) {
	p, err := getPath(r.URL.Path)
	if !checkHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.MkdirAll(p, os.ModePerm)
	checkOsError(w, err)
}

func noCmd(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := checkQryCmd(w, r)
		if !ok {
			return
		}
		f(w, r)
	}
}

func queryNoCmd(f queryHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qry, err := url.ParseQuery(r.URL.RawQuery)
		if !checkBadReq(w, err == nil) {
			return
		}
		if _, ok := checkQryValuesCmd(w, qry); !ok {
			return
		}
		f(w, r, qry)
	}
}

var (
	options  = noCmd(func(_ http.ResponseWriter, _ *http.Request) {})
	props    = noCmd(propsf)
	modprops = noCmd(modpropsf)
	put      = noCmd(putf)
	delete   = noCmd(deletef)
	mkdir    = noCmd(mkdirf)
	search   = queryNoCmd(searchf)
	copy     = queryNoCmd(copyf)
	rename   = queryNoCmd(renamef)
)

const (
	cmdProps    = "props"
	cmdSearch   = "search"
	cmdModprops = "modprops"
	cmdDelete   = "delete"
	cmdMkdir    = "mkdir"
	cmdCopy     = "copy"
	cmdRename   = "rename"
)

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
		propsf(w, r)
	case cmdSearch:
		searchf(w, r, qry)
	default:
		p, err := getPath(r.URL.Path)
		if !checkHandle(w, err == nil, http.StatusNotFound) {
			return
		}
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
}

func post(w http.ResponseWriter, r *http.Request) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !checkBadReq(w, err == nil) {
		return
	}
	cmd, ok := checkQryValuesCmd(w, qry, cmdModprops, cmdDelete, cmdMkdir, cmdCopy, cmdRename)
	if !ok {
		return
	}
	switch cmd {
	case cmdModprops:
		modpropsf(w, r)
	case cmdDelete:
		deletef(w, r)
	case cmdMkdir:
		mkdirf(w, r)
	case cmdCopy:
		copyf(w, r, qry)
	case cmdRename:
		renamef(w, r, qry)
	default:
		putf(w, r)
	}
}

var reqmap = map[string]func(http.ResponseWriter, *http.Request){
	"OPTIONS":  options,
	"HEAD":     get,
	"SEARCH":   search,
	"GET":      get,
	"PROPS":    props,
	"MODPROPS": modprops,
	"PUT":      put,
	"COPY":     copy,
	"RENAME":   rename,
	"DELETE":   delete,
	"MKDIR":    mkdir,
	"POST":     post}

func handler(w http.ResponseWriter, r *http.Request) {
	// todo: document that the accept headers are simply ignored
	h := reqmap[r.Method]
	if h == nil {
		errorResponse(w, http.StatusMethodNotAllowed)
		return
	}
	h(w, r)
}
