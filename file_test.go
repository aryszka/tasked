package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"path"
	"encoding/json"
	"strconv"
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

func serveTest(h http.Handler) (*httptest.Server, error) {
	c, err := tls.X509KeyPair([]byte(defaultTlsCert), []byte(defaultTlsKey))
	if err != nil {
		return nil, err
	}
	if h == nil {
		h = http.HandlerFunc(handler)
	}
	s := httptest.NewUnstartedServer(h)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{c}}
	s.StartTLS()
	return s, nil
}

func withTestServer(h http.Handler, do func(string) error) error {
	server, err := serveTest(h)
	if err != nil {
		return err
	}
	defer server.Close()
	return do(server.URL)
}

func mkclient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}
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

func testStatusCode(code int, h http.Handler,
	req func(*http.Client, string) (*http.Response, error), t *testing.T) *http.Response {
	server, err := serveTest(h)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := mkclient()
	rsp, err := req(client, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != code {
		t.Fail()
	}
	return rsp
}

func testMethodNotAllowed(method string, t *testing.T) {
	testStatusCode(http.StatusMethodNotAllowed, http.HandlerFunc(handler),
		func(c *http.Client, url string) (*http.Response, error) {
			req, err := http.NewRequest(method, url, nil)
			if err != nil {
				return nil, err
			}
			return c.Do(req)
		}, t)
}

func testCheckHandleError(err error, code int, t *testing.T) *http.Response {
	return testStatusCode(code, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		checkHandleError(w, err)
	}), getFunc, t)
}

func testReq(body []byte, code int, header http.Header,
	req func(*http.Client, string) (*http.Response, error), t *testing.T) {
	s, err := serveTest(http.HandlerFunc(handler))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := mkclient()
	rsp, err := req(c, s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != code {
		t.Fail()
	}
	for k, vs := range header {
		vvs, ok := rsp.Header[k]
		if !ok || len(vs) != len(vvs) {
			t.Fail()
			break
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
				t.Fail()
				break
			}
		}
	}
	rspBody, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(string(rspBody), "\n\r") != string(body) {
		t.Fail()
	}
}

func testGet(body []byte, code int, header http.Header, t *testing.T) {
	testReq(body, code, header, func(c *http.Client, url string) (*http.Response, error) {
		return c.Get(url)
	}, t)
}

func testPut(content, body []byte, code int, t *testing.T) {
	testReq(body, code, nil,
		func(c *http.Client, url string) (*http.Response, error) {
			req, err := http.NewRequest("PUT", url, bytes.NewBuffer(content))
			if err != nil {
				return nil, err
			}
			return c.Do(req)
		}, t)
}

func testDelete(body []byte, code int, t *testing.T) {
	testReq(body, code, nil,
		func(c *http.Client, url string) (*http.Response, error) {
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				return nil, err
			}
			return c.Do(req)
		}, t)
}

func TestToProps(t *testing.T) {
	var (
		defaultTime time.Time
		defaultMode os.FileMode
	)
	p := toProps(&fileInfo{}, false)
	if len(p.Name) != 0 ||
		p.Size != 0 ||
		p.ModTime != defaultTime.Unix() ||
		p.IsDir ||
		p.Ext != nil {
		t.Fail()
	}
	now := time.Now()
	p = toProps(&fileInfo{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, false)
	if p.Name != "some" ||
		p.Size != 42 ||
		p.ModTime != now.Unix() ||
		!p.IsDir ||
		p.Ext != nil {
		t.Fail()
	}
	p = toProps(&fileInfo{}, true)
	if len(p.Name) != 0 ||
		p.Size != 0 ||
		p.ModTime != defaultTime.Unix() ||
		p.IsDir ||
		p.Ext.Mode != defaultMode ||
		p.Ext.ModeString != fmt.Sprint(defaultMode) {
		t.Fail()
	}
	p = toProps(&fileInfo{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, true)
	if p.Name != "some" ||
		p.Size != 42 ||
		p.ModTime != now.Unix() ||
		!p.IsDir ||
		p.Ext.Mode != os.ModePerm ||
		p.Ext.ModeString != fmt.Sprint(os.ModePerm) {
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
	var status int
	testStatus := func(url string) error {
		resp, err := mkclient().Get(url)
		if err != nil {
			return err
		}
		if resp.StatusCode != status || resp.Status != fmt.Sprintf("%d %s", status, http.StatusText(status)) {
			return errors.New("error")
		}
		return nil
	}
	err := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errorResponse(w, status)
	}), func(url string) error {
		status = http.StatusOK
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusCreated
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusNoContent
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusNotModified
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusBadRequest
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusNotFound
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusMethodNotAllowed
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusExpectationFailed
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusInternalServerError
		if err := testStatus(url); err != nil {
			return err
		}
		status = http.StatusNotImplemented
		if err := testStatus(url); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
}

func TestCheckHandleError(t *testing.T) {
	// 404
	testCheckHandleError(os.ErrNotExist, http.StatusNotFound, t)
	if checkHandleError(httptest.NewRecorder(), os.ErrNotExist) {
		t.Fail()
	}

	// 404 - no permission
	r := testCheckHandleError(os.ErrPermission, http.StatusNotFound, t)
	wah := r.Header[headerWwwAuth]
	if len(wah) != 1 || wah[0] != authTasked {
		t.Fail()
	}
	if checkHandleError(httptest.NewRecorder(), os.ErrPermission) {
		t.Fail()
	}

	// 500
	testCheckHandleError(errors.New("error"), http.StatusInternalServerError, t)
	if checkHandleError(httptest.NewRecorder(), errors.New("error")) {
		t.Fail()
	}

	// no error
	if !checkHandleError(httptest.NewRecorder(), nil) {
		t.Fail()
	}
}

func TestCheckHandle(t *testing.T) {
	var (
		should bool
		status int
	)
	testCheckHandle := func(url string) error {
		resp, err := mkclient().Get(url)
		if err != nil {
			return err
		}
		if should && resp.StatusCode != status ||
			!should && resp.StatusCode != http.StatusOK {
			return errors.New("error")
		}
		return nil
	}
	err := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		checkHandle(w, !should, status)
	}), func(url string) error {
		should = true
		status = http.StatusNotFound
		err := testCheckHandle(url)
		if err != nil {
			return err
		}
		status = http.StatusMethodNotAllowed
		err = testCheckHandle(url)
		if err != nil {
			return err
		}
		should = false
		return testCheckHandle(url)
	})
	if err != nil {
		t.Fail()
	}
	if checkHandle(httptest.NewRecorder(), false, http.StatusNotFound) {
		t.Fail()
	}
	if checkHandle(httptest.NewRecorder(), false, http.StatusMethodNotAllowed) {
		t.Fail()
	}
	if !checkHandle(httptest.NewRecorder(), true, http.StatusMethodNotAllowed) {
		t.Fail()
	}
}

func TestCheckBadReq(t *testing.T) {
	var should bool
	testBadRequest := func(url string) error {
		resp, err := mkclient().Get(url)
		if err != nil {
			return err
		}
		if should && resp.StatusCode != http.StatusBadRequest ||
			!should && resp.StatusCode != http.StatusOK {
			return errors.New("error")
		}
		return nil
	}
	err := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		checkBadReq(w, !should)
	}), func(url string) error {
		should = true
		err := testBadRequest(url)
		if err != nil {
			return err
		}
		should = false
		return testBadRequest(url)
	})
	if err != nil {
		t.Fail()
	}
	if !checkBadReq(httptest.NewRecorder(), true) {
		t.Fail()
	}
	if checkBadReq(httptest.NewRecorder(), false) {
		t.Fail()
	}
}

func TestCheckQryCmd(t *testing.T) {
	var allowed []string
	testQryCmd := func(url string, shouldFail bool) error {
		c := mkclient()
		rsp, err := c.Get(url)
		if err != nil {
			return err
		}
		if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
			!shouldFail && rsp.StatusCode != http.StatusOK {
			return errors.New("error")
		}
		return nil
	}
	err := withTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkQryCmd(w, r, allowed...)
	}), func(url string) error {
		err := testQryCmd(url+"/?cmd=1", true)
		if err != nil {
			return err
		}
		allowed = []string{"1"}
		err = testQryCmd(url+"/?cmd=1", false)
		if err != nil {
			return err
		}
		allowed = []string{"1", "2"}
		err = testQryCmd(url+"/?cmd=1", false)
		if err != nil {
			return err
		}
		err = testQryCmd(url+"/?cmd=1&cmd=2", true)
		if err != nil {
			return err
		}
		err = testQryCmd(url+"/?cmd=1&cmd=1", true)
		if err != nil {
			return err
		}
		err = testQryCmd(url+"/?cmd=3", true)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}

	var mkreq = func(u string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: u}}
	}
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
	dn = path.Join(testdir, "http")
	ensureDir(dn)
	err := withTestServer(http.HandlerFunc(fileProps), func(url string) error {
		client := mkclient()
		fn := "some-file"
		p := path.Join(dn, fn)
		err := os.Remove(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		req, err := http.NewRequest("PROPS", url + "/" + fn, nil)
		if err != nil {
			return err
		}
		rsp, err := client.Do(req)
		if err != nil {
			return err
		}
		if rsp.StatusCode != http.StatusNotFound {
			return errors.New("fail")
		}
		err = withNewFile(p, nil)
		if err != nil {
			return err
		}
		req, err = http.NewRequest("PROPS", url + "/" + fn, nil)
		if err != nil {
			return err
		}
		rsp, err = client.Do(req)
		if err != nil {
			return err
		}
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			return errors.New("fail")
		}
		fiVerify, err := os.Stat(p)
		if err != nil {
			return err
		}
		jsVerify, err := json.Marshal(toProps(fiVerify, false))
		if err != nil {
			return err
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		if err != nil {
			return err
		}
		if len(jsVerify) != clen {
			return errors.New("fail")
		}
		js, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		if !bytes.Equal(js, jsVerify) {
			return errors.New("fail")
		}
		var pr properties
		err = json.Unmarshal(js, &pr)
		if err != nil {
			return err
		}
		req, err = http.NewRequest("HEAD", url + "/" + fn, nil)
		if err != nil {
			return err
		}
		rsp, err = client.Do(req)
		if err != nil {
			return err
		}
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			return errors.New("fail")
		}
		clen, err = strconv.Atoi(rsp.Header.Get(headerContentLength))
		if err != nil {
			return err
		}
		if len(jsVerify) != clen {
			return errors.New("fail")
		}
		js, err = ioutil.ReadAll(rsp.Body)
		if err != nil {
			return err
		}
		if len(js) != 0 {
			return errors.New("fail")
		}
		return nil
	});
	if err != nil {
		t.Fail()
	}
}

func TestOptions(t *testing.T) {
	testReq(nil, 200, map[string][]string{"Content-Length": []string{"0"}},
		func(c *http.Client, url string) (*http.Response, error) {
			req, err := http.NewRequest("OPTIONS", url, nil)
			if err != nil {
				return nil, err
			}
			return c.Do(req)
		}, t)
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

func TestNotSupported(t *testing.T) {
	// until custom rights checking, skip these tests, if root
	if isRoot {
		t.Skip()
	}

	// TRACE
	testMethodNotAllowed("TRACE", t)

	// CONNECT
	testMethodNotAllowed("CONNECT", t)

	// TINAM
	testMethodNotAllowed("TINAM", t)
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
