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

// func checkFile(content []byte) (bool, error) {
// 	f, err := os.Open(fn)
// 	if os.IsNotExist(err) {
// 		return false, nil
// 	}
// 	if err != nil {
// 		return false, err
// 	}
// 	defer f.Close()
// 	contentDsk, err := ioutil.ReadAll(f)
// 	if err != nil {
// 		return false, err
// 	}
// 	return bytes.Equal(contentDsk, content), nil
// }

// func testReq(body []byte, code int, header http.Header,
// 	req func(*http.Client, string) (*http.Response, error), t *testing.T) {
// 	s := serveTest(t, nil)
// 	c := mkclient()
// 	rsp, err := req(c, s.URL)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer rsp.Body.Close()
// 	if rsp.StatusCode != code {
// 		t.Fail()
// 	}
// 	verifyHeader
// 	rspBody, err := ioutil.ReadAll(rsp.Body)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if strings.TrimRight(string(rspBody), "\n\r") != string(body) {
// 		t.Fail()
// 	}
// }

// func testGet(body []byte, code int, header http.Header, t *testing.T) {
// 	testReq(body, code, header, func(c *http.Client, url string) (*http.Response, error) {
// 		return c.Get(url)
// 	}, t)
// }

// func testPut(content, body []byte, code int, t *testing.T) {
// 	testReq(body, code, nil,
// 		func(c *http.Client, url string) (*http.Response, error) {
// 			req, err := http.NewRequest("PUT", url, bytes.NewBuffer(content))
// 			if err != nil {
// 				return nil, err
// 			}
// 			return c.Do(req)
// 		}, t)
// }

// func testDelete(body []byte, code int, t *testing.T) {
// 	testReq(body, code, nil,
// 		func(c *http.Client, url string) (*http.Response, error) {
// 			req, err := http.NewRequest("DELETE", url, nil)
// 			if err != nil {
// 				return nil, err
// 			}
// 			return c.Do(req)
// 		}, t)
// }

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
		!compareFileMode("mode") {
		return false
	}
	return true
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
}

func TestGetValues(t *testing.T) {
	v, ok := getValues(nil, "", "")
	if len(v) != 0 || !ok {
		t.Fail()
	}
	v, ok = getValues(map[string][]string{"k": []string{"v"}}, "k", "v")
	if len(v) != 1 || v[0] != "v" || !ok {
		t.Fail()
	}
	_, ok = getValues(map[string][]string{"k": []string{"v"}}, "k", "")
	if ok {
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

func TestCheckQryCmd(t *testing.T) {
	mkreq := func(u string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: u}}
	}
	test := func(qry string, shouldFail bool, allowed ...string) {
		thnd.sh = func(w http.ResponseWriter, r *http.Request) {
			checkQryCmd(w, r, allowed...)
		}
		htreq(t, "GET", s.URL+"/?"+qry, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}

	test("cmd=1", true)
	test("cmd=1", false, "1")
	test("cmd=1", false, "1", "2")
	test("cmd=1&cmd=2", true, "1", "2")
	test("cmd=1&cmd=1", true, "1", "2")
	test("cmd=3", true, "1", "2")

	if _, ok := checkQryCmd(httptest.NewRecorder(), mkreq("%%")); ok {
		t.Fail()
	}
	if cmd, ok := checkQryCmd(httptest.NewRecorder(), mkreq("")); !ok || len(cmd) > 0 {
		t.Fail()
	}
	if _, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=1")); ok {
		t.Fail()
	}
	if cmd, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=1"), "1"); !ok || cmd != "1" {
		t.Fail()
	}
	if cmd, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=1"), "1", "2"); !ok || cmd != "1" {
		t.Fail()
	}
	if _, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=1&cmd=2"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=1&cmd=1"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := checkQryCmd(httptest.NewRecorder(), mkreq("cmd=3"), "1", "2"); ok {
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
	js := []byte("{\"some\": \"data\"}")
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		writeJsonResponse(w, r, js)
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
	err := removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)
	di, err := os.Lstat(p)
	errFatal(t, err)
	dii := &fileInfo{sys: di, dirname: testdir}
	err = withNewFile(path.Join(p, "file0"), nil)
	errFatal(t, err)
	err = withNewFile(path.Join(p, "file1"), nil)
	errFatal(t, err)

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
	err = removeIfExists(p)
	errFatal(t, err)
	if len(searchFiles([]*fileInfo{dii}, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// predicate always false
	err = ensureDir(p)
	errFatal(t, err)
	di, err = os.Lstat(p)
	errFatal(t, err)
	dii = &fileInfo{sys: di, dirname: testdir}
	err = withNewFile(path.Join(p, "file0"), nil)
	errFatal(t, err)
	err = withNewFile(path.Join(p, "file1"), nil)
	errFatal(t, err)
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
	err = ensureDir(p)
	errFatal(t, err)
	di, err = os.Lstat(p)
	dii = &fileInfo{sys: di, dirname: testdir}
	errFatal(t, err)
	err = withNewFile(path.Join(p, "file0"), nil)
	errFatal(t, err)
	err = withNewFile(path.Join(p, "file1"), nil)
	errFatal(t, err)
	p0 := path.Join(p, "dir0")
	err = ensureDir(p0)
	errFatal(t, err)
	err = withNewFile(path.Join(p0, "file0"), nil)
	errFatal(t, err)
	err = withNewFile(path.Join(p0, "file1"), nil)
	errFatal(t, err)
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
	err = ensureDir(p1)
	errFatal(t, err)
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
	ensureDir(p1)
	errFatal(t, err)
	p2 := path.Join(p0, "dir2")
	ensureDir(p2)
	errFatal(t, err)
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "dir") == 0
	})) != 3 {
		t.Fail()
	}

	// verify data
	err = withNewFile(path.Join(p, "file1"), func(f *os.File) error {
		_, err := f.Write([]byte("012345678"))
		return err
	})
	errFatal(t, err)
	err = withNewFile(path.Join(p0, "file0"), func(f *os.File) error {
		_, err := f.Write([]byte("012345"))
		return err
	})
	errFatal(t, err)
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

func TestFileSearch(t *testing.T) {
	var queryString url.Values
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		fileSearch(w, r, queryString)
	}

	p := path.Join(testdir, "search")
	err := removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)

	checkError := func(err int) {
		htreq(t, "SEARCH", s.URL+p, nil, func(rsp *http.Response) {
			if rsp.StatusCode != err {
				t.Fail()
			}
		})
	}
	checkBadReq := func() { checkError(http.StatusBadRequest) }

	// max results set
	for i := 0; i < 42; i++ {
		pi := path.Join(p, fmt.Sprintf("file%d", i))
		err := withNewFile(pi, nil)
		errFatal(t, err)
	}
	checkLen := func(l int) {
		htreq(t, "SEARCH", s.URL+p, nil, func(rsp *http.Response) {
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
	err = os.RemoveAll(p)
	errFatal(t, err)
	checkError(http.StatusNotFound)

	// no permissions
	err = ensureDir(p)
	errFatal(t, err)
	err = os.Chmod(p, 0)
	errFatal(t, err)
	defer func() {
		err = os.Chmod(p, os.ModePerm)
		errFatal(t, err)
	}()
	checkLen(0)

	err = removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)
	err = withNewFile(path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	errFatal(t, err)
	err = withNewFile(path.Join(p, "fileB"), func(f *os.File) error {
		_, err := f.Write([]byte("b"))
		return err
	})
	errFatal(t, err)
	err = ensureDir(path.Join(p, "dirA"))
	errFatal(t, err)

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

	// filtering by content, no rights
	err = os.Chmod(path.Join(p, "fileA"), 0)
	errFatal(t, err)
	defer func() {
		err = os.Chmod(path.Join(p, "fileA"), os.ModePerm)
		errFatal(t, err)
	}()
	queryString = make(url.Values)
	queryString.Set("content", "a")
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
	htreq(t, "SEARCH", s.URL+p, nil, func(rsp *http.Response) {
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

func TestFileProps(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		fileProps(w, r)
	}

	fn := "some-file"
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn

	err = removeIfExists(p)
	errFatal(t, err)
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

	err = withNewFile(p, nil)
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
		if !convert64(pr, "modTime") || !convert64(pr, "size") || !convertFm(pr) {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Fail()
		}
	})

	err = withNewFile(p, nil)
	errFatal(t, err)
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

func TestFilePropsRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !isRoot || !propsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	fn := "some-file-uid"
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	err = withNewFile(p, nil)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		fileProps(w, r)
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

func TestFileModprops(t *testing.T) {
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	fn := "some-file"
	p := path.Join(dn, fn)
	err = withNewFile(p, nil)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		fileModprops(w, r)
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
	err = removeIfExists(p)
	errFatal(t, err)
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
	err = withNewFile(p, nil)
	errFatal(t, err)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString("{\"mode\": \"not a number\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})

	// mod, success
	err = os.Chmod(p, os.ModePerm)
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

func TestFileModpropsRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !isRoot || !modpropsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	fn := "some-file-uid-mod"
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	err = withNewFile(p, nil)
	errFatal(t, err)
	err = os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		fileModprops(w, r)
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
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	fn := "some-dir"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	var d *os.File
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		err := withNewFile(path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
		errFatal(t, err)
	}

	// not found
	err = ensureDir(p)
	errFatal(t, err)
	err = os.Chmod(p, os.ModePerm)
	errFatal(t, err)
	d, err = os.Open(p)
	errFatal(t, err)
	err = removeIfExists(p)
	errFatal(t, err)
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
	err = removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)
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
	err = removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)
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
				if !convert64(m, "modTime") || !convert64(m, "size") || !convertFm(m) {
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
	err = removeIfExists(p)
	errFatal(t, err)
	err = ensureDir(p)
	errFatal(t, err)
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

	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	fn := "some-dir"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	var d *os.File
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		err := withNewFile(path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
		errFatal(t, err)
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
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
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
	err = withNewFile(p, nil)
	errFatal(t, err)
	f, err = os.Open(p)
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
	err = withNewFile(p, func(f *os.File) error {
		_, err = f.Write(html)
		return err
	})
	errFatal(t, err)
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

func TestFilePut(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		filePut(w, r)
	}
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)

	// invalid path
	htreq(t, "PUT", s.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// existing dir
	err = os.MkdirAll(path.Join(dn, "dir"), os.ModePerm)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// no permission to write dir
	dp := path.Join(dn, "dir")
	err = ensureDir(dp)
	errFatal(t, err)
	p := path.Join(dp, "file")
	err = removeIfExists(p)
	errFatal(t, err)
	err = os.Chmod(dp, 0555)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to execute dir
	dp = path.Join(dn, "dir")
	err = ensureDir(dp)
	errFatal(t, err)
	p = path.Join(dp, "file")
	err = removeIfExists(p)
	errFatal(t, err)
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
	err = withNewFile(p, nil)
	errFatal(t, err)
	err = os.Chmod(p, 0444)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// existing file
	p = path.Join(dn, "file")
	err = removeIfExists(p)
	errFatal(t, err)
	err = withNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	errFatal(t, err)
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
	err = removeIfExists(p)
	errFatal(t, err)
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
	})

	// clear file
	p = path.Join(dn, "file")
	err = removeIfExists(p)
	errFatal(t, err)
	err = withNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	errFatal(t, err)
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
	err = removeIfExists(d0)
	errFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir0/dir1/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		_, err := os.Lstat(path.Join(dn, "dir0/dir1/file"))
		if err != nil {
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
	dn = testdir
	for i := 0; i < 3; i++ {
		err := withNewFile(path.Join(dn, fmt.Sprintf("file%d", i)), nil)
		errFatal(t, err)
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
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)

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
	err = removeIfExists(p)
	errFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// listing if directory
	err = os.RemoveAll(dn)
	errFatal(t, err)
	err = ensureDir(dn)
	errFatal(t, err)
	c := []byte("some-content")
	err = withNewFile(p, func(f *os.File) error {
		_, err = f.Write(c)
		return err
	})
	errFatal(t, err)
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
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
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
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
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	fn := "some-file"
	p := path.Join(dn, fn)
	err = withNewFile(p, nil)
	if err != nil {
		t.Fatal()
	}
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
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	fn := "some-file"
	p := path.Join(dn, fn)
	err = withNewFile(p, nil)
	if err != nil {
		t.Fatal()
	}
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
	dn = path.Join(testdir, "http")

	// invalid command
	htreq(t, "PUT", s.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// put
	p := path.Join(dn, "some-file")
	err := removeIfExists(p)
	errFatal(t, err)
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

// func TestDelete(t *testing.T) {
// 	// until custom rights checking, skip these tests, if root
// 	if isRoot {
// 		t.Skip()
// 	}
//
// 	// exists no permission to write
// 	err := withNewFile(fn, nil)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	func() {
// 		err = os.Chmod(fn, os.FileMode(os.ModePerm&^(1<<7)))
// 		if err != nil {
// 			t.Fatal()
// 		}
// 		defer os.Chmod(fn, os.ModePerm)
// 		testDelete([]byte(http.StatusText(http.StatusUnauthorized)), http.StatusUnauthorized, t)
// 		ok, err := checkFile(nil)
// 		if err != nil {
// 			t.Fatal()
// 		}
// 		if !ok {
// 			t.Fail()
// 		}
// 	}()
//
// 	// doesn't exist
// 	err = os.Remove(fn)
// 	if err != nil && !os.IsNotExist(err) {
// 		t.Fatal(err)
// 	}
// 	testDelete(nil, http.StatusOK, t)
// 	ok, err := checkFile(nil)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if ok {
// 		t.Fail()
// 	}
//
// 	// exists
// 	err = withNewFile(fn, func(f *os.File) error {
// 		_, err := f.Write([]byte("hello"))
// 		return err
// 	})
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	testDelete(nil, http.StatusOK, t)
// 	ok, err = checkFile(nil)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if ok {
// 		t.Fail()
// 	}
// }

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

// func TestMultipleRequests(t *testing.T) {
// 	// until custom rights checking, skip these tests, if root
// 	if isRoot {
// 		t.Skip()
// 	}
//
// 	hello := []byte("hello")
// 	client := mkclient()
//
// 	// start server
// 	server, err := serveTest(http.HandlerFunc(handler))
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	defer server.Close()
//
// 	// get file notfound
// 	err = os.Remove(fn)
// 	if err != nil && !os.IsNotExist(err) {
// 		t.Fatal()
// 	}
// 	rsp, err := client.Get(server.URL)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	defer rsp.Body.Close()
// 	if rsp.StatusCode != http.StatusNotFound {
// 		t.Fail()
// 	}
//
// 	// put file
// 	req, err := http.NewRequest("PUT", server.URL, bytes.NewBuffer(hello))
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	rsp, err = client.Do(req)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	defer rsp.Body.Close()
// 	if rsp.StatusCode != http.StatusOK {
// 		t.Fail()
// 	}
//
// 	// get file
// 	rsp, err = client.Get(server.URL)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	defer rsp.Body.Close()
// 	if rsp.StatusCode != http.StatusOK {
// 		t.Fail()
// 	}
// 	body, err := ioutil.ReadAll(rsp.Body)
// 	if !bytes.Equal(body, hello) {
// 		t.Fail()
// 	}
// }
