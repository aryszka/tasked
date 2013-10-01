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
	"path"
	"strconv"
	"testing"
	"time"
)

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() interface{}   { return nil }

type testHandler struct {
	hnd func(w http.ResponseWriter, r *http.Request)
}

func (th *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	th.hnd(w, r)
}

var (
	servers          = make(map[http.Handler]*httptest.Server)
	defaultHnd       = &testHandler{hnd: handler}
	errorResponseHnd = new(testHandler)
	checkOsErrorHnd  = new(testHandler)
	checkHandleHnd   = new(testHandler)
	checkBadReqHnd   = new(testHandler)
	checkQryCmdHnd   = new(testHandler)
	filePropsHnd     = new(testHandler)
	modFilePropsHnd  = new(testHandler)
	propsHnd         = new(testHandler)
)

func errFatal(t *testing.T, err error) {
	if err != nil {
		t.Fatal()
	}
}

func serveTest(t *testing.T, h http.Handler) *httptest.Server {
	if h == nil {
		h = defaultHnd
	}
	s := servers[h]
	if s != nil {
		return s
	}
	c, err := tls.X509KeyPair([]byte(defaultTlsCert), []byte(defaultTlsKey))
	errFatal(t, err)
	s = httptest.NewUnstartedServer(h)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{c}}
	s.StartTLS()
	servers[h] = s
	return s
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
		!compareBool("isDir") {
		return false
	}
	if lext, ok := left["ext"]; ok {
		lextMap, ok := lext.(map[string]interface{})
		if !ok {
			return false
		}
		rextMap, ok := right["ext"].(map[string]interface{})
		if !ok {
			return false
		}
		left, right = lextMap, rextMap
		if len(left) != len(right) ||
			!compareString("modeString") ||
			!compareFileMode("mode") {
			return false
		}
	}
	return true
}

func TestPropertyMap(t *testing.T) {
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
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"ext": map[string]interface{}{
			"mode":       defaultMode,
			"modeString": fmt.Sprint(defaultMode)}}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfo{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "some",
		"size":    int64(42),
		"modTime": now.Unix(),
		"isDir":   true,
		"ext": map[string]interface{}{
			"mode":       os.ModePerm,
			"modeString": fmt.Sprint(os.ModePerm)}}) {
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
	server := serveTest(t, errorResponseHnd)
	testStatus := func(s int) {
		errorResponseHnd.hnd = func(w http.ResponseWriter, _ *http.Request) {
			errorResponse(w, s)
		}
		htreq(t, "GET", server.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != s || rsp.Status != fmt.Sprintf("%d %s", s, http.StatusText(s)) {
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
	server := serveTest(t, checkOsErrorHnd)
	test := func(testErr error, status int, clb func(rsp *http.Response)) {
		checkOsErrorHnd.hnd = func(w http.ResponseWriter, _ *http.Request) {
			checkOsError(w, testErr, http.StatusBadRequest)
		}
		htreq(t, "GET", server.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != status {
				t.Log(status)
				t.Log("here")
				t.Fail()
			}
			if clb != nil {
				clb(rsp)
			}
		})
	}

	// 404
	if checkOsError(httptest.NewRecorder(), os.ErrNotExist, 0) {
		t.Fail()
	}
	test(os.ErrNotExist, http.StatusNotFound, nil)

	// 404 - no permission
	if checkOsError(httptest.NewRecorder(), os.ErrPermission, 0) {
		t.Fail()
	}
	test(os.ErrPermission, http.StatusNotFound, func(rsp *http.Response) {
		wah := rsp.Header[headerWwwAuth]
		if len(wah) != 1 || wah[0] != authTasked {
			t.Fail()
		}
	})

	// 400
	if checkOsError(httptest.NewRecorder(), errors.New("error"), 0) {
		t.Fail()
	}
	test(errors.New("error"), http.StatusBadRequest, nil)

	// no error
	if !checkOsError(httptest.NewRecorder(), nil, 0) {
		t.Fail()
	}
	test(nil, http.StatusOK, nil)
}

func TestCheckHandle(t *testing.T) {
	server := serveTest(t, checkHandleHnd)
	test := func(shouldFail bool, status int) {
		checkHandleHnd.hnd = func(w http.ResponseWriter, _ *http.Request) {
			checkHandle(w, !shouldFail, status)
		}
		htreq(t, "GET", server.URL, nil, func(rsp *http.Response) {
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
	server := serveTest(t, checkBadReqHnd)
	test := func(shouldFail bool) {
		checkBadReqHnd.hnd = func(w http.ResponseWriter, _ *http.Request) {
			checkBadReq(w, !shouldFail)
		}
		htreq(t, "GET", server.URL, nil, func(rsp *http.Response) {
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

func TestCheckQryCmd(t *testing.T) {
	server := serveTest(t, checkQryCmdHnd)
	var mkreq = func(u string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: u}}
	}
	test := func(qry string, shouldFail bool, allowed ...string) {
		checkQryCmdHnd.hnd = func(w http.ResponseWriter, r *http.Request) {
			checkQryCmd(w, r, allowed...)
		}
		htreq(t, "GET", server.URL+"/?"+qry, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Log(shouldFail)
				t.Log(allowed)
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

func TestFileProps(t *testing.T) {
	convert64 := func(m map[string]interface{}, n string) (ok bool) {
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
	filePropsHnd.hnd = func(w http.ResponseWriter, r *http.Request) {
		fileProps(w, r)
	}
	server := serveTest(t, filePropsHnd)

	fn := "some-file"
	dn = path.Join(testdir, "http")
	ensureDir(dn)
	p := path.Join(dn, fn)
	url := server.URL + "/" + fn

	err := os.Remove(p)
	if !os.IsNotExist(err) {
		errFatal(t, err)
	}
	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	err = withNewFile(p, nil)
	fiVerify, err := os.Stat(p)
	errFatal(t, err)
	prVerify := toPropertyMap(fiVerify, false)
	jsVerify, err := json.Marshal(prVerify)

	errFatal(t, err)
	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		errFatal(t, err)
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
		if !convert64(pr, "modTime") || !convert64(pr, "size") {
			t.Fail()
		}
		errFatal(t, err)
		if !compareProperties(pr, prVerify) {
			t.Log(pr)
			t.Log(prVerify)
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

func TestModFileProps(t *testing.T) {
	modFilePropsHnd.hnd = func(w http.ResponseWriter, r *http.Request) {
		modFileProps(w, r)
	}
	server := serveTest(t, modFilePropsHnd)
	mrb := maxRequestBody
	defer func() { maxRequestBody = mrb }()

	// max req length
	maxRequestBody = 8
	htreq(t, "MODPROPS", server.URL, io.LimitReader(rand.Reader, maxRequestBody<<1), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fail()
		}
		maxRequestBody = mrb
	})

	// json
	htreq(t, "MODPROPS", server.URL, bytes.NewBufferString("not json"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", server.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", server.URL, bytes.NewBufferString("null"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// not found
	dn = path.Join(testdir, "http")
	ensureDir(dn)
	fn := "some-file"
	p := path.Join(dn, fn)
	err := os.Remove(p)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal()
	}
	htreq(t, "MODPROPS", server.URL+"/"+fn, bytes.NewBufferString("{\"t\":0}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestOptions(t *testing.T) {
	server := serveTest(t, nil)
	htreq(t, "OPTIONS", server.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			!verifyHeader(map[string][]string{"Content-Length": []string{"0"}}, rsp.Header) {
			t.Fail()
		}
	})
}

func TestNotSupported(t *testing.T) {
	server := serveTest(t, nil)
	test := func(method string) {
		htreq(t, method, server.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusMethodNotAllowed {
				t.Fail()
			}
		})
	}
	// TRACE
	test("TRACE")

	// CONNECT
	test("CONNECT")

	// TINAM
	test("TINAM")
}

func TestProps(t *testing.T) {
	propsHnd.hnd = func(w http.ResponseWriter, r *http.Request) {
		props(w, r)
	}
	server := serveTest(t, propsHnd)
	dn = path.Join(testdir, "http")
	ensureDir(dn)
	fn := "some-file"
	p := path.Join(dn, fn)
	err := withNewFile(p, nil)
	if err != nil {
		t.Fatal()
	}
	htreq(t, "PROPS", server.URL+"/"+fn, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "PROPS", server.URL+"/"+fn+"?cmd=anything", nil, func(rsp *http.Response) {
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
