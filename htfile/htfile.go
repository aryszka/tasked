package htfile

import (
	"bufio"
	"code.google.com/p/tasked/share"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	defaultMaxRequestBody   = 1 << 30
	modeMask                = os.FileMode(1)<<9 - 1
	defaultMaxSearchResults = 30
	searchQueryMax          = "max"
	searchQueryName         = "name"
	searchQueryContent      = "content"
	copyRenameToKey         = "to"
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

type queryHandler func(http.ResponseWriter, *http.Request, url.Values)

type handler struct {
	dn               string
	maxRequestBody   int64
	maxSearchResults int
}

type Settings interface {
	Root() string
	MaxRequestBody() int64
	MaxSearchResults() int
}

var (
	textMimeTypes = map[string]string{
		"css":    "text/css; charset=utf-8",
		"html":   "text/html; charset=utf-8",
		"js":     "application/x-javascript",
		"xml":    "text/xml; charset=utf-8",
		"txt":    "text/plain; charset=utf-8",
		"txt16l": "text/plain; charset=utf-16le",
		"txt16b": "text/plain; charset=utf-16be"}
	invalidQueryString = errors.New("Invalid querystring.")
	invalidPath        = errors.New("Invalid path.")
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
		m["mode"] = mm
		if sstat, ok := fi.Sys().(*syscall.Stat_t); ok {
			if owner, err := user.LookupId(strconv.FormatUint(uint64(sstat.Uid), 10)); err == nil {
				m["user"] = owner.Username
			}
			if grp, err := share.LookupGroupById(sstat.Gid); err == nil {
				m["group"] = grp.Name
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

func isOwner(u *user.User, fi os.FileInfo) (bool, error) {
	if u.Uid == strconv.Itoa(0) {
		return true, nil
	}
	sstat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	return strconv.FormatUint(uint64(sstat.Uid), 10) == u.Uid, nil
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
		defer share.Doretlog42(d.Close)
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
	defer share.Doretlog42(ff.Close)
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
		defer share.Doretlog42(tf.Close)
		_, err = io.Copy(tf, ff)
		if err != nil {
			return err
		}
	}
	return os.Chmod(to, fi.Mode())
}

func getQryNum(qry url.Values, key string) (int, error) {
	ns, ok := qry[key]
	if !ok {
		return 0, nil
	}
	if len(ns) != 1 {
		return 0, invalidQueryString
	}
	return strconv.Atoi(ns[0])
}

func getQryExpression(qry url.Values, key string) (*regexp.Regexp, error) {
	exprs := qry[key]
	if len(exprs) > 1 {
		return nil, invalidQueryString
	}
	if len(exprs) == 0 {
		return nil, nil
	}
	expr := exprs[0]
	if expr == "" {
		return nil, nil
	}
	return regexp.Compile(expr)
}

func (h *handler) getPath(sp string) (string, error) {
	p := path.Join(h.dn, sp)
	i := pathIntersect(h.dn, p)
	if i < 2 {
		return "", invalidPath
	}
	return p, nil
}

func (h *handler) searchf(w http.ResponseWriter, r *http.Request, qry url.Values) {
	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	max, err := getQryNum(qry, searchQueryMax)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	if max <= 0 || max > h.maxSearchResults {
		max = h.maxSearchResults
	}
	rxn, err := getQryExpression(qry, searchQueryName)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	rxc, err := getQryExpression(qry, searchQueryContent)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	di, err := os.Lstat(p)
	if !share.CheckOsError(w, err) {
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
			defer share.Doretlog42(f.Close)
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
	_, err = share.WriteJsonResponse(w, r, pmaps)
	share.CheckServerError(w, err != share.MarshalError)
}

func (h *handler) propsf(w http.ResponseWriter, r *http.Request) {
	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	fi, err := os.Lstat(p)
	if !share.CheckOsError(w, err) {
		return
	}
	u, err := user.Current()
	if !share.CheckServerError(w, u != nil && err == nil) {
		return
	}
	own, err := isOwner(u, fi)
	if !share.CheckServerError(w, err == nil) {
		return
	}
	pr := toPropertyMap(fi, own)
	_, err = share.WriteJsonResponse(w, r, pr)
	share.CheckServerError(w, err != share.MarshalError)
}

func (h *handler) modpropsf(w http.ResponseWriter, r *http.Request) {
	var m map[string]interface{}
	err := share.ReadJsonRequest(r, &m, h.maxRequestBody)
	if !share.CheckHandle(w, err != share.RequestBodyTooLarge, http.StatusRequestEntityTooLarge) ||
		!share.CheckBadReq(w, err == nil) {
		return
	}

	mode := int64(-1)
	owner := ""
	group := ""
	for k, v := range m {
		switch k {
		case "mode":
			n, ok := v.(json.Number)
			if !share.CheckBadReq(w, ok) {
				return
			}
			mode, err = n.Int64()
			if !share.CheckBadReq(w, err == nil && mode >= 0) {
				return
			}
		case "owner":
			var ok bool
			owner, ok = v.(string)
			if !share.CheckBadReq(w, ok) {
				return
			}
		case "group":
			var ok bool
			group, ok = v.(string)
			if !share.CheckBadReq(w, ok) {
				return
			}
		default:
			share.ErrorResponse(w, http.StatusBadRequest)
			return
		}
	}

	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	fi, err := os.Lstat(p)
	if !share.CheckOsError(w, err) {
		return
	}

	u, err := user.Current()
	if !share.CheckServerError(w, u != nil && err == nil) {
		return
	}
	own, err := isOwner(u, fi)
	if !share.CheckServerError(w, err == nil) || !share.CheckHandle(w, own, http.StatusNotFound) {
		return
	}

	var (
		uid *uint32
		gid *uint32
	)
	if len(owner) > 0 {
		usr, err := user.Lookup(owner)
		if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
			return
		}
		puid, err := strconv.ParseUint(usr.Uid, 10, 32)
		if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
			return
		}
		upuid := uint32(puid)
		uid = &upuid
	}
	if len(group) > 0 {
		grp, err := share.LookupGroupByName(group)
		if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
			return
		}
		gid = &grp.Id
	}

	if mode >= 0 {
		rmode := replaceMode(fi.Mode(), os.FileMode(mode))
		if rmode != fi.Mode() {
			err := os.Chmod(p, rmode)
			if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
				return
			}
		}
	}
	if uid != nil || gid != nil {
		sstat, ok := fi.Sys().(*syscall.Stat_t)
		if !share.CheckHandle(w, ok, http.StatusNotFound) {
			return
		}
		if uid == nil {
			uid = &sstat.Uid
		}
		if gid == nil {
			gid = &sstat.Gid
		}
		if *uid != sstat.Uid || *gid != sstat.Gid {
			err = os.Chown(p, int(*uid), int(*gid))
			if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
				return
			}
		}
	}
}

func (h *handler) getDir(w http.ResponseWriter, r *http.Request, d *os.File) {
	dfis, err := d.Readdir(0)
	if !share.CheckOsError(w, err) {
		return
	}
	u, err := user.Current()
	if !share.CheckServerError(w, u != nil && err == nil) {
		return
	}
	prs := make([]map[string]interface{}, len(dfis))
	for i, dfi := range dfis {
		own, err := isOwner(u, dfi)
		if !share.CheckServerError(w, err == nil) {
			return
		}
		prs[i] = toPropertyMap(dfi, own)
	}
	_, err = share.WriteJsonResponse(w, r, prs)
	share.CheckServerError(w, err != share.MarshalError)
}

func (h *handler) getFile(w http.ResponseWriter, r *http.Request, f *os.File, fi os.FileInfo) {
	ct, err := detectContentType(fi.Name(), f)
	if !share.CheckServerError(w, err == nil) {
		return
	}
	header := w.Header()
	header.Set(share.HeaderContentType, ct)
	header.Set(share.HeaderContentLength, fmt.Sprintf("%d", fi.Size()))
	if r.Method == "HEAD" {
		return
	}
	io.Copy(w, f) // todo: diag log
}

func (h *handler) putf(w http.ResponseWriter, r *http.Request) {
	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.MkdirAll(path.Dir(p), os.ModePerm)
	if !share.CheckOsError(w, err) {
		return
	}
	f, err := os.Create(p)
	if !share.CheckOsError(w, err) {
		return
	}
	defer share.Doretlog42(f.Close)
	if !share.CheckOsError(w, err) {
		return
	}
	mr := &share.MaxReader{Reader: r.Body, Count: h.maxRequestBody}
	_, err = io.Copy(f, mr)
	if share.CheckHandle(w, err == io.EOF || mr.Count > 0, http.StatusRequestEntityTooLarge) {
		share.CheckOsError(w, err)
	}
}

func (h *handler) copyRename(w http.ResponseWriter, r *http.Request,
	qry url.Values, multi bool, f func(string, string) error) {
	tos, ok := qry[copyRenameToKey]
	if !share.CheckBadReq(w, ok && (multi || len(tos) == 1)) {
		return
	}
	from, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	for _, to := range tos {
		to, err := h.getPath(to)
		if !share.CheckHandle(w, err == nil, http.StatusNotFound) ||
			!share.CheckBadReq(w, pathIntersect(from, to) == 0) {
			return
		}
		err = f(from, to)
		if !share.CheckOsError(w, err) {
			return
		}
	}
}

func (h *handler) copyf(w http.ResponseWriter, r *http.Request, qry url.Values) {
	h.copyRename(w, r, qry, true, copyTree)
}

func (h *handler) renamef(w http.ResponseWriter, r *http.Request, qry url.Values) {
	h.copyRename(w, r, qry, false, os.Rename)
}

func (h *handler) deletef(w http.ResponseWriter, r *http.Request) {
	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.RemoveAll(p)
	if os.IsNotExist(err) {
		return
	}
	share.CheckOsError(w, err)
}

func (h *handler) mkdirf(w http.ResponseWriter, r *http.Request) {
	p, err := h.getPath(r.URL.Path)
	if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
		return
	}
	err = os.MkdirAll(p, os.ModePerm)
	share.CheckOsError(w, err)
}

func noCmd(w http.ResponseWriter, r *http.Request, f http.HandlerFunc) {
	_, ok := share.CheckQryCmd(w, r)
	if !ok {
		return
	}
	if f != nil {
		f(w, r)
	}
}

func queryNoCmd(w http.ResponseWriter, r *http.Request, f queryHandler) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	if _, ok := share.CheckQryValuesCmd(w, qry); !ok {
		return
	}
	if f != nil {
		f(w, r, qry)
	}
}

func New(s Settings) (http.Handler, error) {
	var h handler
	if s != nil {
		h.dn = s.Root()
		h.maxRequestBody = s.MaxRequestBody()
		h.maxSearchResults = s.MaxSearchResults()
	}
	if len(h.dn) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		h.dn = wd
	}
	if h.maxRequestBody <= 0 {
		h.maxRequestBody = defaultMaxRequestBody
	}
	if h.maxSearchResults <= 0 {
		h.maxSearchResults = defaultMaxSearchResults
	}
	return &h, nil
}

func (h *handler) options(w http.ResponseWriter, r *http.Request)  { noCmd(w, r, nil) }
func (h *handler) props(w http.ResponseWriter, r *http.Request)    { noCmd(w, r, h.propsf) }
func (h *handler) modprops(w http.ResponseWriter, r *http.Request) { noCmd(w, r, h.modpropsf) }
func (h *handler) put(w http.ResponseWriter, r *http.Request)      { noCmd(w, r, h.putf) }
func (h *handler) delete(w http.ResponseWriter, r *http.Request)   { noCmd(w, r, h.deletef) }
func (h *handler) mkdir(w http.ResponseWriter, r *http.Request)    { noCmd(w, r, h.mkdirf) }
func (h *handler) search(w http.ResponseWriter, r *http.Request)   { queryNoCmd(w, r, h.searchf) }
func (h *handler) copy(w http.ResponseWriter, r *http.Request)     { queryNoCmd(w, r, h.copyf) }
func (h *handler) rename(w http.ResponseWriter, r *http.Request)   { queryNoCmd(w, r, h.renamef) }

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	cmd, ok := share.CheckQryValuesCmd(w, qry, share.CmdProps, share.CmdSearch)
	if !ok {
		return
	}
	switch cmd {
	case share.CmdProps:
		h.propsf(w, r)
	case share.CmdSearch:
		h.searchf(w, r, qry)
	default:
		p, err := h.getPath(r.URL.Path)
		if !share.CheckHandle(w, err == nil, http.StatusNotFound) {
			return
		}
		f, err := os.Open(p)
		if !share.CheckOsError(w, err) {
			return
		}
		defer share.Doretlog42(f.Close)
		fi, err := f.Stat()
		if !share.CheckOsError(w, err) {
			return
		}
		if fi.IsDir() {
			h.getDir(w, r, f)
			return

		}
		h.getFile(w, r, f, fi)
	}
}

func (h *handler) post(w http.ResponseWriter, r *http.Request) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !share.CheckBadReq(w, err == nil) {
		return
	}
	cmd, ok := share.CheckQryValuesCmd(w, qry,
		share.CmdModprops, share.CmdDelete, share.CmdMkdir, share.CmdCopy, share.CmdRename)
	if !ok {
		return
	}
	switch cmd {
	case share.CmdModprops:
		h.modpropsf(w, r)
	case share.CmdDelete:
		h.deletef(w, r)
	case share.CmdMkdir:
		h.mkdirf(w, r)
	case share.CmdCopy:
		h.copyf(w, r, qry)
	case share.CmdRename:
		h.renamef(w, r, qry)
	default:
		h.putf(w, r)
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// todo:
	// - document that the accept headers are simply ignored
	// - document that the content-type for json requests is simply ignored
	switch r.Method {
	case "OPTIONS":
		h.options(w, r)
	case "HEAD":
		h.get(w, r)
	case "SEARCH":
		h.search(w, r)
	case "GET":
		h.get(w, r)
	case "PROPS":
		h.props(w, r)
	case "MODPROPS":
		h.modprops(w, r)
	case "PUT":
		h.put(w, r)
	case "COPY":
		h.copy(w, r)
	case "RENAME":
		h.rename(w, r)
	case "DELETE":
		h.delete(w, r)
	case "MKDIR":
		h.mkdir(w, r)
	case "POST":
		h.post(w, r)
	default:
		share.ErrorResponse(w, http.StatusMethodNotAllowed)
	}
}
