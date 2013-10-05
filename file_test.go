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

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() interface{}   { return fi.sys }

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

func errFatal(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

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
	p := toPropertyMap(&fileInfo{}, false)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false}) {
		t.Fail()
	}
	now := time.Now()
	p = toPropertyMap(&fileInfo{
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
	p = toPropertyMap(&fileInfo{}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "",
		"size":       int64(0),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       defaultMode,
		"modeString": fmt.Sprint(defaultMode)}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfo{
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
	p = toPropertyMap(&fileInfo{
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
	perr := &os.PathError{Err: os.ErrInvalid}
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

	fi := &fileInfo{sys: &syscall.Stat_t{Uid: cuui}}
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
	fi := &fileInfo{}
	is, err := isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(cu.Uid)
	errFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfo{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerRoot(t *testing.T) {
	if !isRoot {
		t.Skip()
	}

	fi := &fileInfo{}
	u, err := user.Current()
	errFatal(t, err)
	is, err := isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(u.Uid)
	errFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfo{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}
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

func TestChmod(t *testing.T) {
	rc := httptest.NewRecorder()
	var v interface{}
	fn := "some-file"
	dn = path.Join(testdir, "http")
	err := ensureDir(dn)
	errFatal(t, err)
	p := path.Join(dn, fn)
	fi, err := os.Stat(p)
	errFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, _ *http.Request) {
		chmod(w, p, fi, v)
	}

	if chmod(rc, "", nil, "not float") {
		t.Fail()
	}

	err = removeIfExists(p)
	errFatal(t, err)
	if chmod(rc, p, fi, float64(os.ModePerm)) {
		t.Fail()
	}

	err = withNewFile(p, nil)
	errFatal(t, err)
	fi, err = os.Stat(p)
	errFatal(t, err)
	if !chmod(rc, p, fi, float64(fi.Mode()^os.FileMode(0111))) {
		t.Fail()
	}

	v = "not float"
	htreq(t, "MODPROPS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	err = withNewFile(p, nil)
	errFatal(t, err)
	fi, err = os.Stat(p)
	errFatal(t, err)
	err = removeIfExists(p)
	errFatal(t, err)
	v = float64(fi.Mode() ^ os.FileMode(0111))
	htreq(t, "MODPROPS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	err = withNewFile(p, nil)
	errFatal(t, err)
	fi, err = os.Stat(p)
	errFatal(t, err)
	v = float64(fi.Mode() ^ os.FileMode(0111))
	htreq(t, "MODPROPS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
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

func TestOptions(t *testing.T) {
	thnd.sh = handler
	htreq(t, "OPTIONS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			!verifyHeader(map[string][]string{"Content-Length": []string{"0"}}, rsp.Header) {
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

// func TestGet(t *testing.T) {
// 	// until custom rights checking, skip these tests, if root
// 	if isRoot {
// 		t.Skip()
// 	}
//
// 	// not found
// 	err := os.Remove(fn)
// 	if err != nil && !os.IsNotExist(err) {
// 		t.Fatal()
// 	}
// 	testStatusCode(http.StatusNotFound, http.HandlerFunc(handler), getFunc, t)
//
// 	// no perm
// 	err = withNewFile(fn, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	err = os.Chmod(fn, os.FileMode(os.ModePerm&^(1<<8)))
// 	defer os.Chmod(fn, os.ModePerm)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	testStatusCode(http.StatusUnauthorized, http.HandlerFunc(handler), getFunc, t)
//
// 	// empty
// 	err = withNewFile(fn, nil)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	fs, err := os.Stat(fn)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	header := make(map[string][]string)
// 	header[http.CanonicalHeaderKey("Last-Modified")] = []string{fs.ModTime().UTC().Format(http.TimeFormat)}
// 	testGet(nil, http.StatusOK, header, t)
//
// 	// has content
// 	hello := []byte("hello")
// 	err = withNewFile(fn, func(f *os.File) error {
// 		_, err := f.Write(hello)
// 		return err
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	fs, err = os.Stat(fn)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	header = make(map[string][]string)
// 	header[http.CanonicalHeaderKey("Last-Modified")] = []string{fs.ModTime().UTC().Format(http.TimeFormat)}
// 	testGet(hello, http.StatusOK, header, t)
// }

// func TestPut(t *testing.T) {
// 	// until custom rights checking, skip these tests, if root
// 	if isRoot {
// 		t.Skip()
// 	}
//
// 	hello := []byte("hello")
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
// 		testPut(hello, []byte(http.StatusText(http.StatusUnauthorized)), http.StatusUnauthorized, t)
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
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	testPut(hello, nil, http.StatusOK, t)
// 	ok, err := checkFile(hello)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if !ok {
// 		t.Fail()
// 	}
//
// 	// exists, empty
// 	err = withNewFile(fn, nil)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	testPut(hello, nil, http.StatusOK, t)
// 	ok, err = checkFile(hello)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if !ok {
// 		t.Fail()
// 	}
//
// 	// exists, not empty
// 	err = withNewFile(fn, func(f *os.File) error {
// 		_, err := f.Write([]byte("olleh"))
// 		return err
// 	})
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	testPut(hello, nil, http.StatusOK, t)
// 	ok, err = checkFile(hello)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if !ok {
// 		t.Fail()
// 	}
//
// 	// POST
// 	testReq(nil, http.StatusOK, nil, func(c *http.Client, url string) (*http.Response, error) {
// 		req, err := http.NewRequest("POST", url, bytes.NewBuffer(hello))
// 		if err != nil {
// 			return nil, err
// 		}
// 		return c.Do(req)
// 	}, t)
// 	ok, err = checkFile(hello)
// 	if err != nil {
// 		t.Fatal()
// 	}
// 	if !ok {
// 		t.Fail()
// 	}
// }

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
