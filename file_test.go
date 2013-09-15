package main

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"strings"
	"bytes"
)

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

func mkclient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}
}

func get(c *http.Client, url string) (*http.Response, error) { return c.Get(url) }

func checkFile(content []byte) (bool, error) {
	f, err := os.Open(fn)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	contentDsk, err := ioutil.ReadAll(f)
	if err != nil {
		return false, err
	}
	return bytes.Equal(contentDsk, content), nil
}

func testStatusCode(code int, h http.Handler,
	req func(*http.Client, string) (*http.Response, error), t *testing.T) {
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
}

func testGetError(err error, code int, t *testing.T) {
	testStatusCode(code, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replyError(w, r, err)
	}), get, t)
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

func TestReplyError(t *testing.T) {
	// 404
	testGetError(os.ErrNotExist, http.StatusNotFound, t)

	// 401
	testGetError(os.ErrPermission, http.StatusUnauthorized, t)

	// 500
	testGetError(errors.New("error"), http.StatusInternalServerError, t)
}

func TestGet(t *testing.T) {
	// not found
	err := os.Remove(fn)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal()
	}
	testStatusCode(http.StatusNotFound, http.HandlerFunc(handler), get, t)

	// no perm
	err = create(fn)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(fn, os.FileMode(os.ModePerm&^(1<<8)))
	defer os.Chmod(fn, os.ModePerm)
	if err != nil {
		t.Fatal()
	}
	testStatusCode(http.StatusUnauthorized, http.HandlerFunc(handler), get, t)

	// empty
	err = create(fn)
	if err != nil {
		t.Fatal(err)
	}
	fs, err := os.Stat(fn)
	if err != nil {
		t.Fatal(err)
	}
	header := make(map[string][]string)
	header[http.CanonicalHeaderKey("Last-Modified")] = []string{fs.ModTime().UTC().Format(http.TimeFormat)}
	testGet(nil, http.StatusOK, header, t)

	// has content
	hello := []byte("hello")
	err = withNewFile(fn, func(f *os.File) error {
		_, err := f.Write(hello)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	fs, err = os.Stat(fn)
	if err != nil {
		t.Fatal(err)
	}
	header = make(map[string][]string)
	header[http.CanonicalHeaderKey("Last-Modified")] = []string{fs.ModTime().UTC().Format(http.TimeFormat)}
	testGet(hello, http.StatusOK, header, t)
}

func TestPut(t *testing.T) {
	hello := []byte("hello")

	// exists no permission to write
	err := create(fn)
	if err != nil {
		t.Fatal()
	}
	func() {
		err = os.Chmod(fn, os.FileMode(os.ModePerm&^(1<<7)))
		defer os.Chmod(fn, os.ModePerm)
		if err != nil {
			t.Fatal()
		}
		testPut(hello, []byte(http.StatusText(http.StatusUnauthorized)), http.StatusUnauthorized, t)
		ok, err := checkFile(nil)
		if err != nil {
			t.Fatal()
		} else if !ok {
			t.Fail()
		}
	}()

	// doesn't exist
	err = os.Remove(fn)
	if err != nil {
		t.Fatal()
	}
	testPut(hello, nil, http.StatusOK, t)
	ok, err := checkFile(hello)
	if err != nil {
		t.Fatal()
	} else if !ok {
		t.Fail()
	}

	// exists, empty
	err = create(fn)
	if err != nil {
		t.Fatal()
	}
	testPut(hello, nil, http.StatusOK, t)
	ok, err = checkFile(hello)
	if err != nil {
		t.Fatal()
	} else if !ok {
		t.Fail()
	}

	// exists, not empty
	err = withNewFile(fn, func(f *os.File) error {
		_, err := f.Write([]byte("olleh"))
		return err
	})
	if err != nil {
		t.Fatal()
	}
	testPut(hello, nil, http.StatusOK, t)
	ok, err = checkFile(hello)
	if err != nil {
		t.Fatal()
	} else if !ok {
		t.Fail()
	}

	// POST
	testReq(nil, http.StatusOK, nil, func(c *http.Client, url string) (*http.Response, error) {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(hello))
		if err != nil {
			return nil, err
		}
		return c.Do(req)
	}, t)
	ok, err = checkFile(hello)
	if err != nil {
		t.Fatal()
	} else if !ok {
		t.Fail()
	}
}

func TestDelete(t *testing.T) {
	// exists no permission to write
	// doesn't exist
	// exists
}

func TestNotSupported(t *testing.T) {
	// TRACE
	// CONNECT
	// TINAH
}

func TestMultipleRequests(t *testing.T) {
	// start server
	// get file notfound
	// put file
	// get file
}
