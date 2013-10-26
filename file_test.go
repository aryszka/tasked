package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type fileInfoT struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (fi *fileInfoT) Name() string       { return fi.name }
func (fi *fileInfoT) Size() int64        { return fi.size }
func (fi *fileInfoT) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfoT) ModTime() time.Time { return fi.modTime }
func (fi *fileInfoT) IsDir() bool        { return fi.isDir }
func (fi *fileInfoT) Sys() interface{}   { return fi.sys }

type testHandler struct {
	sh func(w http.ResponseWriter, r *http.Request)
}

func (th *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if th.sh == nil {
		panic("Test handler not initialized.")
	}
	th.sh(w, r)
}

var (
	thnd = new(testHandler)
	s    *httptest.Server
	mx   = new(sync.Mutex)
)

func init() {
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	if err != nil {
		panic(err)
	}
	c, err := tls.X509KeyPair([]byte(defaultTlsCert), []byte(defaultTlsKey))
	if err != nil {
		panic(err)
	}
	s = httptest.NewUnstartedServer(thnd)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{c}}
	s.StartTLS()
}

func mkclient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}
}

func htreq(t *testing.T, method, url string, body io.Reader, clb func(rsp *http.Response)) {
	r, err := http.NewRequest(method, url, body)
	errFatal(t, err)
	client := mkclient()
	rsp, err := client.Do(r)
	errFatal(t, err)
	defer rsp.Body.Close()
	clb(rsp)
}

func verifyHeader(expect, have map[string][]string) bool {
	for k, vs := range expect {
		vvs, ok := have[k]
		if !ok || len(vs) != len(vvs) {
			return false
		}
		for _, v := range vs {
			found := false
			for _, vv := range vvs {
				if v == vv {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func getFunc(c *http.Client, url string) (*http.Response, error) { return c.Get(url) }

func convert64(m map[string]interface{}, n string) (ok bool) {
	var (
		v interface{}
		f float64
	)
	if v, ok = m[n]; !ok || v == nil {
		return !ok
	}
	if f, ok = v.(float64); !ok {
		return false
	}
	m[n] = int64(f)
	return true
}

func convertFm(pr map[string]interface{}) (ok bool) {
	fv, ok := pr["mode"].(float64)
	if !ok {
		return false
	}
	pr["mode"] = os.FileMode(fv)
	return true
}

func compareProperties(left, right map[string]interface{}) bool {
	// TODO: use reflection
	compareString := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalString, ok := lval.(string)
			if !ok {
				return false
			}
			rvalString, ok := right[key].(string)
			if !ok || rvalString != lvalString {
				return false
			}
		}
		return true
	}
	compareInt64 := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalInt, ok := lval.(int64)
			if !ok {
				return false
			}
			rvalInt, ok := right[key].(int64)
			if !ok || rvalInt != lvalInt {
				return false
			}
		}
		return true
	}
	compareFileMode := func(key string) bool { // make it int32
		lval, ok := left[key]
		if ok {
			lvalInt, ok := lval.(os.FileMode)
			if !ok {
				return false
			}
			rvalInt, ok := right[key].(os.FileMode)
			if !ok || rvalInt != lvalInt {
				return false
			}
		}
		return true
	}
	compareBool := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalBool, ok := lval.(bool)
			if !ok {
				return false
			}
			rvalBool, ok := right[key].(bool)
			if !ok || rvalBool != lvalBool {
				return false
			}
		}
		return true
	}
	if len(left) != len(right) ||
		!compareString("name") ||
		!compareInt64("size") ||
		!compareInt64("modTime") ||
		!compareBool("isDir") ||
		!compareString("modeString") ||
		!compareFileMode("mode") ||
		!compareString("user") ||
		!compareString("group") ||
		!compareInt64("accessTime") ||
		!compareInt64("changeTime") {
		return false
	}
	return true
}

func isPermission(err error) bool {
	if err == nil {
		return false
	}
	if perr, ok := err.(*os.PathError); ok {
		if errno, ok := perr.Err.(syscall.Errno); ok && (errno == syscall.EPERM || errno == syscall.EACCES) {
			return true
		}
	}
	if serr, ok := err.(*os.SyscallError); ok {
		if errno, ok := serr.Err.(syscall.Errno); ok && errno == syscall.EPERM {
			return true
		}
	}
	return false
}

func TestMaxReader(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr := &maxReader{reader: buf, count: 0}
	b := make([]byte, 32)
	n, err := mr.Read(b)
	if n != 0 || err == nil {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &maxReader{reader: buf, count: 6}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 6 || err != nil || mr.count != 0 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err == nil || mr.count != 0 {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &maxReader{reader: buf, count: 12}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 12 || err != nil || mr.count != 0 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err == nil || mr.count != 0 {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &maxReader{reader: buf, count: 64}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 12 || err != nil || mr.count != 52 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err != io.EOF || mr.count != 52 {
		t.Fail()
	}
}

func TestToPropertyMap(t *testing.T) {
	var (
		defaultTime time.Time
		defaultMode os.FileMode
	)
	p := toPropertyMap(&fileInfoT{}, false)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false}) {
		t.Fail()
	}
	now := time.Now()
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, false)
	if !compareProperties(p, map[string]interface{}{
		"name":    "some",
		"size":    int64(42),
		"modTime": now.Unix(),
		"isDir":   true}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "",
		"size":       int64(0),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       defaultMode,
		"modeString": fmt.Sprint(defaultMode)}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "some",
		"size":       int64(42),
		"modTime":    now.Unix(),
		"isDir":      true,
		"mode":       os.ModePerm,
		"modeString": fmt.Sprint(os.ModePerm)}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
		mode: os.ModePerm + 1024}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "",
		"size":       int64(0),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       os.ModePerm,
		"modeString": fmt.Sprint(os.ModePerm)}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfo{
		sys: &fileInfoT{mode: os.ModePerm + 1024}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "",
		"size":       int64(0),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       os.ModePerm,
		"modeString": fmt.Sprint(os.ModePerm),
		"dirname":    "/"}) {
		t.Fail()
	}
	u, err := user.Current()
	errFatal(t, err)
	uid, err := strconv.Atoi(u.Uid)
	gid, err := strconv.Atoi(u.Gid)
	errFatal(t, err)
	g, err := lookupGroupName(uint32(gid))
	errFatal(t, err)
	p = toPropertyMap(&fileInfoT{
		name: "some",
		size: 42,
		modTime: defaultTime,
		isDir: false,
		mode: os.ModePerm,
		sys: &syscall.Stat_t{
			Uid: uint32(uid),
			Gid: uint32(gid),
			Atim: syscall.Timespec{Sec: defaultTime.Unix() + 42},
			Ctim: syscall.Timespec{Sec: defaultTime.Unix() + 42<<1}}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name": "some",
		"size": int64(42),
		"modTime": defaultTime.Unix(),
		"isDir": false,
		"mode": os.ModePerm,
		"modeString": fmt.Sprint(os.ModePerm),
		"user": u.Username,
		"group": g,
		"accessTime": defaultTime.Unix() + 42,
		"changeTime": defaultTime.Unix() + 42<<1}) {
		t.Fail()
	}
}

func TestPathIntersect(t *testing.T) {
	// equal length, not equal
	s0, s1 := "some/one", "some/two"
	if pathIntersect(s0, s1) != 0 || pathIntersect(s1, s0) != 0 {
		t.Fail()
	}

	// equal length, equal
	s0, s1 = "some/path", "some/path"
	if pathIntersect(s0, s1) != 3 {
		t.Fail()
	}

	// not equal length, not intersect
	s0, s1 = "some/path", "some/pathbutdifferent"
	if pathIntersect(s0, s1) != 0 || pathIntersect(s1, s0) != 0 {
		t.Fail()
	}

	// not equal length, intersect
	s0, s1 = "some/path", "some/path/inside"
	if pathIntersect(s0, s1) != 2 || pathIntersect(s1, s0) != 1 {
		t.Fail()
	}
}

func TestErrorResponse(t *testing.T) {
	testStatus := func(status int) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			errorResponse(w, status)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != status || rsp.Status != fmt.Sprintf("%d %s", status, http.StatusText(status)) {
				t.Fail()
			}
		})
	}
	testStatus(http.StatusBadRequest)
	testStatus(http.StatusNotFound)
	testStatus(http.StatusMethodNotAllowed)
	testStatus(http.StatusExpectationFailed)
	testStatus(http.StatusInternalServerError)
	testStatus(http.StatusRequestEntityTooLarge)
}

func TestHandleErrno(t *testing.T) {
	var errno syscall.Errno
	thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
		handleErrno(w, errno)
	}

	// enoent
	errno = syscall.ENOENT
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EPERM
	errno = syscall.EPERM
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EACCES
	errno = syscall.EACCES
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EISDIR
	errno = syscall.EISDIR
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EINVAL
	errno = syscall.EINVAL
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// other
	errno = syscall.EIO
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusInternalServerError {
			t.Fail()
		}
	})
}

func TestCheckOsError(t *testing.T) {
	test := func(testErr error, status int, clb func(rsp *http.Response)) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			checkOsError(w, testErr)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != status {
				t.Fail()
			}
			if clb != nil {
				clb(rsp)
			}
		})
	}

	// 404
	if checkOsError(httptest.NewRecorder(), os.ErrNotExist) {
		t.Fail()
	}
	test(os.ErrNotExist, http.StatusNotFound, nil)

	// 404 - no permission
	if checkOsError(httptest.NewRecorder(), os.ErrPermission) {
		t.Fail()
	}
	test(os.ErrPermission, http.StatusNotFound, nil)

	// 400
	perr := &os.PathError{Err: syscall.EINVAL}
	if checkOsError(httptest.NewRecorder(), perr) {
		t.Fail()
	}
	test(perr, http.StatusBadRequest, nil)

	// 500
	if checkOsError(httptest.NewRecorder(), errors.New("error")) {
		t.Fail()
	}
	test(errors.New("error"), http.StatusInternalServerError, nil)

	// no error
	if !checkOsError(httptest.NewRecorder(), nil) {
		t.Fail()
	}
	test(nil, http.StatusOK, nil)

	// is dir
	perr = &os.PathError{Err: syscall.EISDIR}
	if checkOsError(httptest.NewRecorder(), perr) {
		t.Fail()
	}
	test(perr, http.StatusNotFound, nil)
	serr := &os.SyscallError{Err: syscall.EISDIR}
	if checkOsError(httptest.NewRecorder(), serr) {
		t.Fail()
	}
	test(serr, http.StatusNotFound, nil)
	lerr := &os.LinkError{Err: syscall.EISDIR}
	if checkOsError(httptest.NewRecorder(), lerr) {
		t.Fail()
	}
	test(lerr, http.StatusNotFound, nil)
}

func TestCheckHandle(t *testing.T) {
	test := func(shouldFail bool, status int) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			checkHandle(w, !shouldFail, status)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != status ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if checkHandle(httptest.NewRecorder(), false, http.StatusNotFound) {
		t.Fail()
	}
	test(true, http.StatusNotFound)
	if checkHandle(httptest.NewRecorder(), false, http.StatusMethodNotAllowed) {
		t.Fail()
	}
	test(true, http.StatusMethodNotAllowed)
	if !checkHandle(httptest.NewRecorder(), true, http.StatusMethodNotAllowed) {
		t.Fail()
	}
	test(false, 0)
}

func TestCheckBadReq(t *testing.T) {
	test := func(shouldFail bool) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			checkBadReq(w, !shouldFail)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if !checkBadReq(httptest.NewRecorder(), true) {
		t.Fail()
	}
	test(true)
	if checkBadReq(httptest.NewRecorder(), false) {
		t.Fail()
	}
	test(false)
}

func TestCheckServerError(t *testing.T) {
	test := func(shouldFail bool) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			checkServerError(w, !shouldFail)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusInternalServerError ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if !checkBadReq(httptest.NewRecorder(), true) {
		t.Fail()
	}
	test(true)
	if checkBadReq(httptest.NewRecorder(), false) {
		t.Fail()
	}
	test(false)
}

func TestChecValueskQryCmd(t *testing.T) {
	mkqry := func(u string) url.Values {
		vs, err := url.ParseQuery(u)
		errFatal(t, err)
		return vs
	}
	test := func(qry url.Values, shouldFail bool, allowed ...string) {
		thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
			checkQryValuesCmd(w, qry, allowed...)
		}
		htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}

	test(mkqry("cmd=1"), true)
	test(mkqry("cmd=1"), false, "1")
	test(mkqry("cmd=1"), false, "1", "2")
	test(mkqry("cmd=1&cmd=2"), true, "1", "2")
	test(mkqry("cmd=1&cmd=1"), true, "1", "2")
	test(mkqry("cmd=3"), true, "1", "2")

	if cmd, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("")); !ok || len(cmd) > 0 {
		t.Fail()
	}
	if _, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1")); ok {
		t.Fail()
	}
	if cmd, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1"), "1"); !ok || cmd != "1" {
		t.Fail()
	}
	if cmd, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1"), "1", "2"); !ok || cmd != "1" {
		t.Fail()
	}
	if _, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1&cmd=2"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1&cmd=1"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := checkQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=3"), "1", "2"); ok {
		t.Fail()
	}
}

func TestCheckQryCmd(t *testing.T) {
	if _, ok := checkQryCmd(httptest.NewRecorder(), &http.Request{URL: &url.URL{RawQuery: "%%"}}); ok {
		t.Fail()
	}
}

func TestIsOwner(t *testing.T) {
	cu, err := user.Current()
	errFatal(t, err)
	cui, err := strconv.Atoi(cu.Uid)
	errFatal(t, err)
	cuui := uint32(cui)

	fi := &fileInfoT{sys: &syscall.Stat_t{Uid: cuui}}
	is, err := isOwner(cu, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestWriteJsonResponse(t *testing.T) {
	d := map[string]interface{}{"some": "data"}
	js, err := json.Marshal(d)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		writeJsonResponse(w, r, d)
	}
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(b, js) {
			t.Fail()
		}
		var jsBack map[string]interface{}
		err = json.Unmarshal(b, &jsBack)
		if err != nil || len(jsBack) != 1 || jsBack["some"] != "data" {
			t.Fail()
		}
	})
	htreq(t, "HEAD", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(b, nil) {
			t.Fail()
		}
	})
}

func TestIsOwnerNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	cu, err := user.Current()
	errFatal(t, err)
	fi := &fileInfoT{}
	is, err := isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(cu.Uid)
	errFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerRoot(t *testing.T) {
	if !isRoot {
		t.Skip()
	}

	fi := &fileInfoT{}
	u, err := user.Current()
	errFatal(t, err)
	is, err := isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(u.Uid)
	errFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestDetectContentType(t *testing.T) {
	ct, err := detectContentType("some.html", nil)
	if err != nil || ct != textMimeTypes["html"] {
		t.Fail()
	}
	p := path.Join(testdir, "some-file")
	err = withNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("This suppose to be some human readable text."))
		return err
	})
	errFatal(t, err)
	f, err := os.Open(p)
	errFatal(t, err)
	ct, err = detectContentType("some-file", f)
	if err != nil || ct != textMimeTypes["txt"] {
		t.Fail()
	}
}

func TestSearchFiles(t *testing.T) {
	p := path.Join(testdir, "search")
	removeIfExistsF(t, p)
	ensureDirF(t, p)
	di, err := os.Lstat(p)
	errFatal(t, err)
	dii := &fileInfo{sys: di, dirname: testdir}
	withNewFileF(t, path.Join(p, "file0"), nil)
	withNewFileF(t, path.Join(p, "file1"), nil)

	// 0 max
	if len(searchFiles([]*fileInfo{dii}, 0, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// negative max
	if len(searchFiles([]*fileInfo{dii}, -1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// no root dirs
	if len(searchFiles(nil, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// not existing root dirs
	removeIfExistsF(t, p)
	if len(searchFiles([]*fileInfo{dii}, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// predicate always false
	ensureDirF(t, p)
	di, err = os.Lstat(p)
	errFatal(t, err)
	dii = &fileInfo{sys: di, dirname: testdir}
	withNewFileF(t, path.Join(p, "file0"), nil)
	withNewFileF(t, path.Join(p, "file1"), nil)
	if len(searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return false })) != 0 {
		t.Fail()
	}

	// predicate always true
	result := searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return true })
	if len(result) != 2 {
		t.Fail()
	}

	// root is a file
	fi, err := os.Lstat(path.Join(p, "file0"))
	errFatal(t, err)
	if len(searchFiles([]*fileInfo{&fileInfo{sys: fi, dirname: p}}, 42,
		func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// find all in a recursively
	ensureDirF(t, p)
	di, err = os.Lstat(p)
	errFatal(t, err)
	dii = &fileInfo{sys: di, dirname: testdir}
	withNewFileF(t, path.Join(p, "file0"), nil)
	withNewFileF(t, path.Join(p, "file1"), nil)
	p0 := path.Join(p, "dir0")
	ensureDirF(t, p0)
	withNewFileF(t, path.Join(p0, "file0"), nil)
	withNewFileF(t, path.Join(p0, "file1"), nil)
	result = searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return true })
	if len(result) != 5 {
		t.Fail()
	}
	m := make(map[string]int)
	for _, fii := range result {
		m[path.Join(fii.dirname, fii.Name())] = 1
	}
	if len(m) != len(result) {
		t.Fail()
	}

	// find max, breadth first
	result = searchFiles([]*fileInfo{dii}, 4, func(_ *fileInfo) bool { return true })
	if len(result) != 4 {
		t.Fail()
	}
	var oneOfThem bool
	for _, fii := range result {
		if path.Join(fii.dirname, fii.Name()) == path.Join(p0, "file0") ||
			path.Join(fii.dirname, fii.Name()) == path.Join(p0, "file1") {
			if oneOfThem {
				t.Fail()
			}
			oneOfThem = true
		}
	}

	// find all, filtered
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return fi.Name() == "file0"
	})) != 2 {
		t.Fail()
	}
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "file") == 0
	})) != 4 {
		t.Fail()
	}

	// find max, filtered, step levels
	// don't count max for stepping directories
	// (failure was: Readdir(max))
	p1 := path.Join(p, "dir1")
	ensureDirF(t, p1)
	if len(searchFiles([]*fileInfo{dii}, 2, func(fi *fileInfo) bool {
		if fi.Name() == "dir0" {
			err = os.Rename(path.Join(p0, "file0"), path.Join(p1, "file0"))
			errFatal(t, err)
			return false
		}
		return fi.Name() == "file0"
	})) != 2 {
		t.Fail()
	}
	err = os.Rename(path.Join(p1, "file0"), path.Join(p1, "file0"))
	errFatal(t, err)
	err = os.RemoveAll(p1)
	errFatal(t, err)

	// find all dirs, too
	p1 = path.Join(p0, "dir1")
	ensureDirF(t, p1)
	p2 := path.Join(p0, "dir2")
	ensureDirF(t, p2)
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "dir") == 0
	})) != 3 {
		t.Fail()
	}

	// verify data
	withNewFileF(t, path.Join(p, "file1"), func(f *os.File) error {
		_, err := f.Write([]byte("012345678"))
		return err
	})
	withNewFileF(t, path.Join(p0, "file0"), func(f *os.File) error {
		_, err := f.Write([]byte("012345"))
		return err
	})
	sizeOfADir := dii.Size()
	vm := map[string]map[string]interface{}{
		path.Join(p, "dir0"): map[string]interface{}{
			"name":    "dir0",
			"dirname": p,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p0, "dir1"): map[string]interface{}{
			"name":    "dir1",
			"dirname": p0,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p0, "dir2"): map[string]interface{}{
			"name":    "dir2",
			"dirname": p0,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p, "file0"): map[string]interface{}{
			"name":    "file0",
			"dirname": p,
			"isDir":   false,
			"size":    int64(0)},
		path.Join(p, "file1"): map[string]interface{}{
			"name":    "file1",
			"dirname": p,
			"isDir":   false,
			"size":    int64(9)},
		path.Join(p0, "file0"): map[string]interface{}{
			"name":    "file0",
			"dirname": p0,
			"isDir":   false,
			"size":    int64(6)},
		path.Join(p0, "file1"): map[string]interface{}{
			"name":    "file1",
			"dirname": p0,
			"isDir":   false,
			"size":    int64(0)}}
	result = searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool { return true })
	for _, fi := range result {
		fp := path.Join(fi.dirname, fi.Name())
		if fi.Name() != vm[fp]["name"].(string) ||
			fi.dirname != vm[fp]["dirname"].(string) ||
			fi.IsDir() != vm[fp]["isDir"].(bool) ||
			fi.Size() != vm[fp]["size"].(int64) {
			t.Fail()
		}
	}

	// verify breadth first, order
	level0 := result[:3]
	for _, fi := range level0 {
		if fi.dirname != p {
			t.Fail()
		}
	}
	result = searchFiles([]*fileInfo{dii}, 5, func(fi *fileInfo) bool { return true })
	level0 = result[:3]
	for _, fi := range level0 {
		if fi.dirname != p {
			t.Fail()
		}
	}
	result = searchFiles([]*fileInfo{dii}, 2, func(fi *fileInfo) bool { return true })
	for _, fi := range result {
		if fi.dirname != p {
			t.Fail()
		}
	}
}

func TestSearchFilesNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	p := path.Join(testdir, "search")
	removeIfExistsF(t, p)
	ensureDirF(t, p)
	p0 := path.Join(p, "dir0")
	ensureDirF(t, p0)
	withNewFileF(t, path.Join(p, "file0"), nil)
	withNewFileF(t, path.Join(p, "file1"), nil)
	di, err := os.Lstat(p)
	errFatal(t, err)
	dii := &fileInfo{sys: di, dirname: testdir}

	// not finding when no rights for dir
	err = os.Chmod(p0, 0)
	errFatal(t, err)
	defer func() {
		err = os.Chmod(p0, os.ModePerm)
		errFatal(t, err)
	}()
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool { return true })) != 3 {
		t.Fail()
	}
}

func TestCopyTree(t *testing.T) {
	dir := path.Join(dn, "copy-tree")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)

	// no copy of the same path
	fi0, err := os.Lstat(dir)
	errFatal(t, err)
	err = copyTree(dir, dir)
	if err != nil {
		t.Fail()
	}
	fi1, err := os.Lstat(dir)
	errFatal(t, err)
	if !fi1.ModTime().Equal(fi0.ModTime()) {
		t.Fail()
	}

	// not found
	dir0 := path.Join(dir, "dir0")
	err = removeIfExists(dir0)
	dir1 := path.Join(dir, "dir1")
	removeIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if err == nil || !os.IsNotExist(err) {
		t.Fail()
	}

	// tree
	ensureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	withNewFileF(t, fp, nil)
	removeIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if err != nil {
		t.Fail()
	}
	_, err = os.Lstat(path.Join(dir1, "some"))
	if err != nil {
		t.Fail()
	}

	// copy file
	f0, f1 := path.Join(dir, "file0"), path.Join(dir, "file1")
	withNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	removeIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	cf, err := os.Open(f1)
	errFatal(t, err)
	defer cf.Close()
	ccf, err := ioutil.ReadAll(cf)
	errFatal(t, err)
	if !bytes.Equal(ccf, []byte("some content")) {
		t.Fail()
	}
}

func TestCopyTreeNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	dir := path.Join(dn, "copy")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)

	// no access
	dir0 := path.Join(dir, "dir0")
	ensureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	removeIfExistsF(t, dir1)
	err := os.Chmod(dir, 0600)
	errFatal(t, err)
	err = copyTree(dir0, dir1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, os.ModePerm)
	errFatal(t, err)

	// copy abort
	ensureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	withNewFileF(t, fp, nil)
	err = os.Chmod(fp, 0000)
	removeIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if !isPermission(err) {
		t.Fail()
	}
	_, err = os.Lstat(dir1)
	if err != nil {
		t.Fail()
	}
	_, err = os.Lstat(path.Join(dir1, "some"))
	if !os.IsNotExist(err) {
		t.Fail()
	}
	err = os.Chmod(fp, 0666)
	errFatal(t, err)

	// copy file, no write permission
	f0, f1 := path.Join(dir, "file0"), path.Join(dir, "file1")
	withNewFileF(t, f0, nil)
	removeIfExistsF(t, f1)
	err = os.Chmod(dir, 0600)
	errFatal(t, err)
	err = copyTree(f0, f1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, 0777)
	errFatal(t, err)

	// preserve mode
	withNewFileF(t, f0, nil)
	err = os.Chmod(f0, 0400)
	errFatal(t, err)
	removeIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	fi, err := os.Lstat(f1)
	errFatal(t, err)
	if fi.Mode() != os.FileMode(0400) {
		t.Fail()
	}

	// copy to not existing
	p0, p1 := path.Join(dir, "dir0"), path.Join(dir, "dir1")
	ensureDirF(t, p0)
	removeIfExistsF(t, p1)
	err = copyTree(p0, p1)
	if err != nil {
		t.Fail()
	}

	// copy to existing file
	f0, f1 = path.Join(dir, "file0"), path.Join(dir, "file1")
	withNewFileF(t, f0, nil)
	withNewFileF(t, f1, nil)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
}

func TestGetPath(t *testing.T) {
	p, err := getPath("..")
	if err == nil {
		t.Fail()
	}
	p, err = getPath("some")
	if err != nil || p != path.Join(dn, "some") {
		t.Fail()
	}
}

func TestQryNum(t *testing.T) {
	qry := make(url.Values)
	n, err := getQryNum(qry, "some")
	if n != 0 || err != nil {
		t.Fail()
	}

	qry.Add("some", "0")
	qry.Add("some", "1")
	n, err = getQryNum(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "val")
	n, err = getQryNum(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "42")
	n, err = getQryNum(qry, "some")
	if n != 42 || err != nil {
		t.Fail()
	}
}

func TestGetQryExpression(t *testing.T) {
	qry := make(url.Values)
	qry.Add("some", "val0")
	qry.Add("some", "val1")
	x, err := getQryExpression(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	x, err = getQryExpression(qry, "some")
	if x != nil || err != nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "")
	x, err = getQryExpression(qry, "some")
	if x != nil || err != nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "(")
	x, err = getQryExpression(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "val")
	x, err = getQryExpression(qry, "some")
	if x == nil || err != nil {
		t.Fail()
	}
}

func TestSearchf(t *testing.T) {
	var queryString url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		searchf(w, r, queryString)
	}

	p := path.Join(dn, "search")
	removeIfExistsF(t, p)
	ensureDirF(t, p)

	checkError := func(err int) {
		htreq(t, "SEARCH", s.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != err {
				t.Fail()
			}
		})
	}
	checkBadReq := func() { checkError(http.StatusBadRequest) }
	checkLen := func(l int) {
		htreq(t, "SEARCH", s.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			js, err := ioutil.ReadAll(rsp.Body)
			errFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			errFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// max results set
	for i := 0; i < 42; i++ {
		pi := path.Join(p, fmt.Sprintf("file%d", i))
		withNewFileF(t, pi, nil)
	}
	queryString = make(url.Values)
	checkLen(defaultMaxSearchResults)
	queryString.Set("max", "42")
	checkLen(defaultMaxSearchResults)
	queryString.Set("max", "3")
	checkLen(3)

	// only one name
	queryString = make(url.Values)
	queryString.Add("name", "some")
	queryString.Add("name", "someOther")
	checkBadReq()

	// only one content
	queryString = make(url.Values)
	queryString.Add("content", "some")
	queryString.Add("content", "someOther")
	checkBadReq()

	// invalid regexp
	queryString = make(url.Values)
	queryString.Set("name", "(")
	checkBadReq()

	// not found
	queryString = make(url.Values)
	err := os.RemoveAll(p)
	errFatal(t, err)
	checkError(http.StatusNotFound)

	removeIfExistsF(t, p)
	ensureDirF(t, p)
	withNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	withNewFileF(t, path.Join(p, "fileB"), func(f *os.File) error {
		_, err := f.Write([]byte("b"))
		return err
	})
	ensureDirF(t, path.Join(p, "dirA"))

	// filtering by name
	queryString = make(url.Values)
	queryString.Set("name", "A")
	checkLen(2)

	// filtering by content
	queryString = make(url.Values)
	queryString.Set("content", "b")
	checkLen(1)

	// filtering by name and content
	queryString = make(url.Values)
	queryString.Set("name", "A")
	queryString.Set("content", "a")
	checkLen(1)
	queryString = make(url.Values)
	queryString.Set("name", "A")
	queryString.Set("content", "b")
	checkLen(0)

	// check data
	fia, err := os.Lstat(path.Join(p, "fileA"))
	errFatal(t, err)
	fib, err := os.Lstat(path.Join(p, "fileB"))
	errFatal(t, err)
	fid, err := os.Lstat(path.Join(p, "dirA"))
	errFatal(t, err)
	mts := make(map[string]int64)
	mts["fileA"] = fia.ModTime().Unix()
	mts["fileB"] = fib.ModTime().Unix()
	mts["dirA"] = fid.ModTime().Unix()
	sizeOfADir := fid.Size()
	queryString = make(url.Values)
	htreq(t, "SEARCH", s.URL+"/search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		errFatal(t, err)
		if len(m) != 3 {
			t.Fail()
		}
		for _, pr := range m {
			convert64(pr, "size")
			convert64(pr, "modTime")
			n, ok := pr["name"].(string)
			if !ok {
				t.Fail()
			}
			if !compareProperties(pr, map[string]interface{}{
				"dirname": p,
				"name":    n,
				"isDir":   n == "dirA",
				"size": func() int64 {
					if n == "dirA" {
						return sizeOfADir
					} else {
						return int64(1)
					}
				}(),
				"modTime": mts[n]}) {
				t.Fail()
			}
		}
	})
}

func TestSearchfNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	var queryString url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		searchf(w, r, queryString)
	}

	p := path.Join(dn, "search")
	removeIfExistsF(t, p)
	ensureDirF(t, p)

	checkLen := func(l int) {
		htreq(t, "SEARCH", s.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			js, err := ioutil.ReadAll(rsp.Body)
			errFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			errFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// no permissions
	ensureDirF(t, p)
	err := os.Chmod(p, 0)
	errFatal(t, err)
	defer func() {
		err = os.Chmod(p, os.ModePerm)
		errFatal(t, err)
	}()
	checkLen(0)

	// filtering by content, no rights
	err = os.Chmod(p, 0777)
	errFatal(t, err)
	withNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	err = os.Chmod(path.Join(p, "fileA"), 0)
	errFatal(t, err)
	defer func() {
		err = os.Chmod(path.Join(p, "fileA"), os.ModePerm)
		errFatal(t, err)
	}()
	queryString = make(url.Values)
	queryString.Set("content", "a")
	checkLen(0)
}

func TestPropsf(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		propsf(w, r)
	}

	fn := "some-file"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn

	removeIfExistsF(t, p)
	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	htreq(t, "PROPS", s.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	withNewFileF(t, p, nil)
	fiVerify, err := os.Stat(p)
	errFatal(t, err)
	prVerify := toPropertyMap(fiVerify, true)
	jsVerify, err := json.Marshal(prVerify)
	errFatal(t, err)

	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		errFatal(t, err)
		if !convert64(pr, "modTime") || !convert64(pr, "size") ||
			!convert64(pr, "accessTime") || !convert64(pr, "changeTime") ||
			!convertFm(pr) {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Log(pr)
			t.Log(prVerify)
			t.Fail()
		}
	})

	withNewFileF(t, p, nil)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if len(js) != 0 {
			t.Fail()
		}
	})
}

func TestPropsfRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !isRoot || !propsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	err := os.Chmod(testdir, 0777)
	errFatal(t, err)
	err = os.Chmod(dn, 0777)
	errFatal(t, err)
	fn := "some-file-uid"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	withNewFileF(t, p, nil)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		propsf(w, r)
	}

	// uid := syscall.Getuid()
	usr, err := user.Lookup(testuser)
	errFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	errFatal(t, err)
	err = syscall.Setuid(tuid)
	errFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	errFatal(t, err)
	// }()

	fiVerify, err := os.Stat(p)
	errFatal(t, err)
	prVerify := toPropertyMap(fiVerify, false)
	jsVerify, err := json.Marshal(prVerify)
	errFatal(t, err)
	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		errFatal(t, err)
		if !convert64(pr, "modTime") || !convert64(pr, "size") {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Fail()
		}
		_, ok := pr["mode"]
		if ok {
			t.Fail()
		}
		_, ok = pr["modeString"]
		if ok {
			t.Fail()
		}
		_, ok = pr["owner"]
		if ok {
			t.Fail()
		}
		_, ok = pr["group"]
		if ok {
			t.Fail()
		}
	})
}

func TestModpropsf(t *testing.T) {
	fn := "some-file"
	p := path.Join(dn, fn)
	withNewFileF(t, p, nil)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		modpropsf(w, r)
	}
	mrb := maxRequestBody
	defer func() { maxRequestBody = mrb }()

	// max req length
	maxRequestBody = 8
	htreq(t, "MODPROPS", s.URL, io.LimitReader(rand.Reader, maxRequestBody<<1), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fail()
		}
		maxRequestBody = mrb
	})

	// json
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("not json"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("null"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// not found
	removeIfExistsF(t, p)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString("{\"t\":0}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// bad req, path
	htreq(t, "MODPROPS", s.URL+"/"+string([]byte{0}), bytes.NewBufferString("{\"t\":0}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})

	// mod, bad val
	withNewFileF(t, p, nil)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString("{\"mode\": \"not a number\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})

	// mod, success
	err := os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Stat(p)
			errFatal(t, err)
			if fi.Mode() != os.FileMode(0744) {
				t.Fail()
			}
		})

	// mod, success, masked
	err = os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 01744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Stat(p)
			errFatal(t, err)
			if fi.Mode() != os.FileMode(0744) {
				t.Fail()
			}
		})
}

func TestModpropsfRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !isRoot || !modpropsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	fn := "some-file-uid-mod"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	withNewFileF(t, p, nil)
	err := os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		modpropsf(w, r)
	}

	// uid := syscall.Getuid()
	usr, err := user.Lookup(testuser)
	errFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	errFatal(t, err)
	err = syscall.Setuid(tuid)
	errFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	errFatal(t, err)
	// }()

	htreq(t, "MODPROPS", url,
		bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d,\"some-prop\": \"some val\"}", 0744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusNotFound {
				t.Fail()
			}
			fi, err := os.Stat(p)
			errFatal(t, err)
			if fi.Mode() != os.ModePerm {
				t.Fail()
			}
		})
}

func TestGetDir(t *testing.T) {
	fn := "some-dir"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	var d *os.File
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		withNewFileF(t, path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
	}

	// not found
	ensureDirF(t, p)
	err := os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	d, err = os.Open(p)
	errFatal(t, err)
	removeIfExistsF(t, p)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if strings.Trim(string(b), "\n") != http.StatusText(http.StatusNotFound) {
			t.Fail()
		}
	})

	// empty dir
	removeIfExistsF(t, p)
	ensureDirF(t, p)
	d, err = os.Open(p)
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var res []interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 0 {
			t.Fail()
		}
	})

	// dir with files
	removeIfExistsF(t, p)
	ensureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var res []map[string]interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 3 {
			t.Fail()
		}
		for _, m := range res {
			n, ok := m["name"].(string)
			if !ok {
				t.Fail()
			}
			switch n {
			case "some0", "some1", "some2":
				if !convert64(m, "modTime") || !convert64(m, "size") ||
					!convert64(m, "accessTime") || !convert64(m, "changeTime") ||
					!convertFm(m) {
					t.Fail()
				}
				fi, err := os.Stat(path.Join(p, n))
				errFatal(t, err)
				if !compareProperties(m, toPropertyMap(fi, true)) {
					t.Fail()
				}
			default:
				t.Fail()
			}
		}
	})

	// tests the same with HEAD
	removeIfExistsF(t, p)
	ensureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	errFatal(t, err)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if len(b) > 0 {
			t.Fail()
		}
	})
}

func TestGetDirRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !isRoot || !getDirRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	err := os.Chmod(testdir, 0777)
	errFatal(t, err)
	err = os.Chmod(dn, 0777)
	errFatal(t, err)

	fn := "some-dir"
	p := path.Join(dn, fn)
	ensureDirF(t, p)
	err = os.Chmod(p, 0777)
	errFatal(t, err)
	url := s.URL + "/" + fn
	var d *os.File
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		withNewFileF(t, path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
	}

	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})

	// uid := syscall.Getuid()
	usr, err := user.Lookup(testuser)
	errFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	errFatal(t, err)
	tgid, err := strconv.Atoi(usr.Gid)
	errFatal(t, err)
	err = os.Chown(path.Join(p, "some1"), tuid, tgid)
	errFatal(t, err)
	err = syscall.Setuid(tuid)
	errFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	errFatal(t, err)
	// }()

	d, err = os.Open(p)
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var res []map[string]interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 3 {
			t.Fail()
		}
		for _, m := range res {
			n, ok := m["name"].(string)
			if !ok {
				t.Fail()
			}
			switch n {
			case "some0", "some1", "some2":
				if !convert64(m, "modTime") || !convert64(m, "size") {
					t.Fail()
				}
				if n == "some1" && !convertFm(m) {
					t.Fail()
				}
				fi, err := os.Stat(path.Join(p, n))
				errFatal(t, err)
				if !compareProperties(m, toPropertyMap(fi, n == "some1")) {
					t.Fail()
				}
			default:
				t.Fail()
			}
		}
	})
}

func TestGetFile(t *testing.T) {
	var (
		f  *os.File
		fi os.FileInfo
	)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		getFile(w, r, f, fi)
	}

	// extension tested
	fn := "some-file.html"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	withNewFileF(t, p, nil)
	f, err := os.Open(p)
	errFatal(t, err)
	fi, err = f.Stat()
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
	})
	err = f.Close()
	errFatal(t, err)

	// content tested, length set, content sent
	fn = "some-file"
	p = path.Join(dn, fn)
	url = s.URL + "/" + fn
	html := []byte("<html></html>")
	withNewFileF(t, p, func(f *os.File) error {
		_, err = f.Write(html)
		return err
	})
	f, err = os.Open(p)
	errFatal(t, err)
	fi, err = f.Stat()
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(b, html) {
			t.Fail()
		}
	})
	err = f.Close()
	errFatal(t, err)

	// HEAD handled
	f, err = os.Open(p)
	errFatal(t, err)
	fi, err = f.Stat()
	errFatal(t, err)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if len(b) != 0 {
			t.Fail()
		}
	})
	err = f.Close()
	errFatal(t, err)

	// emulate copy failure
	// file handler still open, but file deleted, can't help this without performance penalty
	f, err = os.Open(p)
	errFatal(t, err)
	fi, err = f.Stat()
	errFatal(t, err)
	err = os.Remove(p)
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		errFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
	})
	err = f.Close()
	errFatal(t, err)
}

func TestPutf(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		putf(w, r)
	}

	// invalid path
	htreq(t, "PUT", s.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// existing dir
	err := os.MkdirAll(path.Join(dn, "dir"), os.ModePerm)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// existing file
	p := path.Join(dn, "file")
	removeIfExistsF(t, p)
	withNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	htreq(t, "PUT", s.URL+"/file", bytes.NewBufferString("new content"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		errFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		errFatal(t, err)
		if !bytes.Equal(content, []byte("new content")) {
			t.Fail()
		}
	})

	// new file
	p = path.Join(dn, "file")
	removeIfExistsF(t, p)
	htreq(t, "PUT", s.URL+"/file", bytes.NewBufferString("some content"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		errFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		errFatal(t, err)
		if !bytes.Equal(content, []byte("some content")) {
			t.Fail()
		}
		fi, err := os.Lstat(p)
		errFatal(t, err)
		if fi.Mode() != os.FileMode(0600) {
			t.Fail()
		}
	})

	// clear file
	p = path.Join(dn, "file")
	removeIfExistsF(t, p)
	withNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	htreq(t, "PUT", s.URL+"/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		errFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		errFatal(t, err)
		if len(content) != 0 {
			t.Fail()
		}
	})

	// create full path
	d0 := path.Join(dn, "dir0")
	removeIfExistsF(t, d0)
	htreq(t, "PUT", s.URL+"/dir0/dir1/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		_, err := os.Lstat(path.Join(dn, "dir0/dir1/file"))
		if err != nil {
			t.Fail()
		}
	})

	// max the body
	mrb := maxRequestBody
	defer func() { maxRequestBody = mrb }()
	maxRequestBody = 8
	htreq(t, "PUT", s.URL+"/file", io.LimitReader(rand.Reader, maxRequestBody<<1), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fail()
		}
	})
}

func TestPutfNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		putf(w, r)
	}

	// no permission to write dir
	dp := path.Join(dn, "dir")
	ensureDirF(t, dp)
	p := path.Join(dp, "file")
	removeIfExistsF(t, p)
	err := os.Chmod(dp, 0555)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to execute dir
	dp = path.Join(dn, "dir")
	ensureDirF(t, dp)
	p = path.Join(dp, "file")
	removeIfExistsF(t, p)
	err = os.Chmod(dp, 0666)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to write file
	p = path.Join(dn, "file")
	withNewFileF(t, p, nil)
	err = os.Chmod(p, 0444)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestCopyRename(t *testing.T) {
	var (
		multiple bool
		qry      url.Values
		f        = func(_, _ string) error { return nil }
	)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		copyRename(multiple, f)(w, r, qry)
	}

	// no to
	qry = make(url.Values)
	htreq(t, "COPY", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// multiple to not allowed
	multiple = false
	qry = make(url.Values)
	qry.Add("to", "path0")
	qry.Add("to", "path1")
	htreq(t, "COPY", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// multiple to allowed
	multiple = true
	qry = make(url.Values)
	qry.Add("to", "path0")
	qry.Add("to", "path1")
	htreq(t, "COPY", s.URL+"/from", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// copy tree above self
	qry = make(url.Values)
	qry.Add("to", "/path/path0")
	htreq(t, "COPY", s.URL+"/path/path0/path1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// copy tree below self
	qry = make(url.Values)
	qry.Add("to", "/path/path0/path1")
	htreq(t, "COPY", s.URL+"/path/path0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// func called
	pfrom, pto := "from", "to"
	f = func(from, to string) error {
		if from != path.Join(dn, pfrom) || to != path.Join(dn, pto) {
			t.Fail()
		}
		return os.ErrNotExist
	}
	qry = make(url.Values)
	qry.Set("to", pto)
	htreq(t, "COPY", s.URL+"/"+pfrom, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestCopyf(t *testing.T) {
	var qry url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		copyf(w, r, qry)
	}

	dir := path.Join(dn, "copy")
	ensureDirF(t, dir)
	fn0 := path.Join(dir, "file0")
	withNewFileF(t, fn0, nil)
	fn1 := path.Join(dir, "file1")
	removeIfExistsF(t, fn1)

	// copy over not existing file
	qry = make(url.Values)
	qry.Set("to", "/copy/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// copy over existing file
	withNewFileF(t, fn1, nil)
	qry = make(url.Values)
	qry.Set("to", "/copy/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// copy under not existing dir
	dir0 := path.Join(dir, "dir0")
	removeIfExistsF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over empty directory
	withNewDirF(t, dir0)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over not empty directory
	withNewDirF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	withNewFileF(t, fn1, nil)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRenamef(t *testing.T) {
	var qry url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		renamef(w, r, qry)
	}

	dir := path.Join(dn, "rename")
	ensureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	ensureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	ensureDirF(t, dir1)
	fn0 := path.Join(dir0, "file0")
	withNewFileF(t, fn0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	fn1 := path.Join(dir1, "file1")
	withNewFileF(t, fn1, nil)

	// rename file
	qry = make(url.Values)
	qry.Set("to", "rename/dir1/file1")
	htreq(t, "RENAME", s.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		_, err := os.Lstat(fn0)
		if !os.IsNotExist(err) {
			t.Fail()
		}
		fi1, err := os.Lstat(fn1)
		if err != nil || fi1.Size() != int64(len("some content")) {
			t.Fail()
		}
	})

	// to not existing dir
	dir0 = path.Join(dir, "dir0")
	removeIfExistsF(t, dir0)
	fn0 = path.Join(dir, "file0")
	withNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0/file1")
	htreq(t, "RENAME", s.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// over existing dir
	ensureDirF(t, dir0)
	withNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0")
	htreq(t, "RENAME", s.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRenamefNotRoot(t *testing.T) {
	if isRoot {
		t.Skip()
	}

	var qry url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		renamef(w, r, qry)
	}

	dir := path.Join(dn, "rename")
	ensureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	ensureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	ensureDirF(t, dir1)
	fn0 := path.Join(dir0, "file0")
	withNewFileF(t, fn0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	fn1 := path.Join(dir1, "file1")
	withNewFileF(t, fn1, nil)

	// no write source
	err := os.Chmod(dir0, 0500)
	errFatal(t, err)
	qry = make(url.Values)
	qry.Set("to", "rename/dir1/file1")
	htreq(t, "RENAME", s.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		_, err := os.Lstat(fn0)
		if err != nil {
			t.Fail()
		}
		fi1, err := os.Lstat(fn1)
		if err != nil || fi1.Size() != 0 {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, os.ModePerm)
	errFatal(t, err)

	// no write to target
	err = os.Chmod(dir1, 0500)
	errFatal(t, err)
	htreq(t, "RENAME", s.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		_, err := os.Lstat(fn0)
		if err != nil {
			t.Fail()
		}
		fi1, err := os.Lstat(fn1)
		if err != nil || fi1.Size() != 0 {
			t.Fail()
		}
	})
	err = os.Chmod(dir1, os.ModePerm)
	errFatal(t, err)
}

func TestDeletef(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		deletef(w, r)
	}
	dir := path.Join(dn, "delete")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	ensureDirF(t, dir0)
	file0 := path.Join(dir0, "file0")
	withNewFileF(t, file0, nil)
	err := os.Chmod(dir0, 0500)
	errFatal(t, err)
	htreq(t, "DELETE", s.URL + "/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	errFatal(t, err)

	// doesn't exist
	removeIfExistsF(t, file0)
	htreq(t, "DELETE", s.URL + "/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// exists, deleted
	withNewFileF(t, file0, nil)
	htreq(t, "DELETE", s.URL + "/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); !os.IsNotExist(err) {
			t.Fail()
		}
	})
}

func TestFileMkdir(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		mkdirf(w, r)
	}
	dir := path.Join(dn, "mkdir")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	ensureDirF(t, dir0)
	err := os.Chmod(dir0, 0500)
	errFatal(t, err)
	dir1 := path.Join(dir0, "dir1")
	htreq(t, "MKDIR", s.URL + "/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		if _, err := os.Lstat(dir1); !os.IsNotExist(err) {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	errFatal(t, err)

	// doesn't exist, created
	removeIfExistsF(t, dir1)
	htreq(t, "MKDIR", s.URL + "/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(dir1); err != nil {
			t.Fail()
		}
	})

	// exists, not touched
	ensureDirF(t, dir1)
	file0 := path.Join(dir1, "file0")
	withNewFileF(t, file0, nil)
	htreq(t, "MKDIR", s.URL + "/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); err != nil {
			t.Fail()
		}
	})
}

func TestNoCmd(t *testing.T) {
	thnd.sh = noCmd(func(w http.ResponseWriter, r *http.Request) {})
	htreq(t, "GET", s.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "GET", s.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
}

func TestQueryNoCmd(t *testing.T) {
	var testQuery url.Values
	thnd.sh = queryNoCmd(func(w http.ResponseWriter, r *http.Request, qry url.Values) {
		if len(qry) != len(testQuery) {
			t.Fail()
			return
		}
		for key, testVals := range testQuery {
			vals, ok := qry[key]
			if !ok || len(vals) != len(testVals) {
				t.Fail()
				return
			}
			for _, testVal := range testVals {
				found := false
				for _, val := range vals {
					if val == testVal {
						found = true
						break
					}
				}
				if !found {
					t.Fail()
					return
				}
			}
		}
	})

	// no query
	testQuery = make(url.Values)
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// invalid query format
	testQuery = make(url.Values)
	htreq(t, "GET", s.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// contains query, command
	testQuery = make(url.Values)
	testQuery.Set("param", "some")
	htreq(t, "GET", s.URL+"?param=some&cmd=somecmd", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// contains query
	testQuery = make(url.Values)
	testQuery.Set("param", "some")
	htreq(t, "GET", s.URL+"?param=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
}

func TestOptions(t *testing.T) {
	thnd.sh = handler
	htreq(t, "OPTIONS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			!verifyHeader(map[string][]string{"Content-Length": []string{"0"}}, rsp.Header) {
			t.Fail()
		}
	})
}

func TestSearch(t *testing.T) {
	thnd.sh = handler
	htreq(t, "SEARCH", s.URL+"?cmd=search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	for i := 0; i < 3; i++ {
		withNewFileF(t, path.Join(dn, fmt.Sprintf("file%d", i)), nil)
	}
	htreq(t, "SEARCH", s.URL+"?max=3", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		errFatal(t, err)
		if len(m) != 3 {
			t.Fail()
		}
	})
}

func TestGet(t *testing.T) {
	thnd.sh = handler

	// cmd can be props or search only
	htreq(t, "GET", s.URL+"?cmd=invalid", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "GET", s.URL+"?cmd=search&cmd=props", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "GET", s.URL+"?cmd=search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "GET", s.URL+"?cmd=props", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// not found
	fn := "some-file"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	removeIfExistsF(t, p)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// listing if directory
	dir := path.Join(dn, "search")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)
	c := []byte("some content")
	p = path.Join(dir, "some-file")
	withNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write(c)
		return err
	})
	htreq(t, "GET", s.URL+"/search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		var d []map[string]interface{}
		err = json.Unmarshal(b, &d)
		if err != nil || len(d) != 1 || d[0]["name"] != fn {
			t.Fail()
		}
	})

	// file otherwise
	htreq(t, "GET", s.URL+"/search/some-file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		errFatal(t, err)
		if !bytes.Equal(b, c) {
			t.Fail()
		}
	})
}

func TestProps(t *testing.T) {
	thnd.sh = handler
	fn := "some-file"
	p := path.Join(dn, fn)
	withNewFileF(t, p, nil)
	htreq(t, "PROPS", s.URL+"/"+fn, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "PROPS", s.URL+"/"+fn+"?cmd=anything", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestModprops(t *testing.T) {
	thnd.sh = handler
	fn := "some-file"
	p := path.Join(dn, fn)
	withNewFileF(t, p, nil)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0777)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	htreq(t, "MODPROPS", s.URL+"/"+fn+"?cmd=anything", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestPut(t *testing.T) {
	thnd.sh = handler

	// invalid command
	htreq(t, "PUT", s.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// put
	p := path.Join(dn, "some-file")
	removeIfExistsF(t, p)
	htreq(t, "PUT", s.URL+"/some-file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		_, err := os.Lstat(p)
		if err != nil {
			t.Fail()
		}
	})
}

func TestCopy(t *testing.T) {
	thnd.sh = handler

	// invalid query
	htreq(t, "COPY", s.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// invalid command
	htreq(t, "COPY", s.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestPost(t *testing.T) {
	thnd.sh = handler

	// invalid query
	htreq(t, "POST", s.URL + "?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// invalid command
	htreq(t, "POST", s.URL + "?cmd=invalid", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	dir := path.Join(dn, "post")
	removeIfExistsF(t, dir)
	ensureDirF(t, dir)

	// modprops
	file := path.Join(dir, "file")
	withNewFileF(t, file, nil)
	err := os.Chmod(file, 0600)
	errFatal(t, err)
	fi, err := os.Lstat(file)
	errFatal(t, err)
	props := toPropertyMap(fi, false)
	props["mode"] = 0660
	js, err := json.Marshal(props)
	htreq(t, "POST", s.URL + "/post/file?cmd=modprops", bytes.NewBuffer(js), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		fi, err := os.Lstat(file)
		errFatal(t, err)
		if fi.Mode() != os.FileMode(0660) {
			t.Fail()
		}
	})

	// delete
	withNewFileF(t, file, nil)
	htreq(t, "POST", s.URL + "/post/file?cmd=delete", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		_, err := os.Lstat(file)
		if !os.IsNotExist(err) {
			t.Fail()
		}
	})

	// mkdir
	dir0 := path.Join(dir, "dir0")
	removeIfExistsF(t, dir0)
	htreq(t, "POST", s.URL + "/post/dir0?cmd=mkdir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(dir0); err != nil {
			t.Fail()
		}
	})

	// copy
	withNewFileF(t, file, nil)
	file0 := path.Join(dir, "file0")
	removeIfExistsF(t, file0)
	htreq(t, "POST", s.URL + "/post/file?cmd=copy&to=/post/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); err != nil {
			t.Fail()
		}
	})

	// rename
	withNewFileF(t, file, nil)
	removeIfExistsF(t, file0)
	htreq(t, "POST", s.URL + "/post/file?cmd=rename&to=/post/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file); !os.IsNotExist(err) {
			t.Fail()
		}
		if _, err = os.Lstat(file0); err != nil {
			t.Fail()
		}
	})
}

func TestNotSupported(t *testing.T) {
	thnd.sh = handler
	test := func(method string) {
		htreq(t, method, s.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusMethodNotAllowed {
				t.Fail()
			}
		})
	}
	test("TRACE")
	test("CONNECT")
	test("TINAM")
}
