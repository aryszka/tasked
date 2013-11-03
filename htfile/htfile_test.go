package htfile

import (
	"bytes"
	"code.google.com/p/tasked/util"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
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

const (
	testTlsKey = `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBANfK7I6VGd4yxNNK
rg1+GdveTB4aAiqC916Yl5vFTlpCg6LIhAYmDvXPM9XJZc/h8N4jh/JNC39wgcEG
/RV2wl9T63+NR6TBLVx6nJbKjCvEuzwpB3BIun4827cU6PCksBc4hke9pTgD9v0y
DOtECKl+HuxRuKJLGoRCQ9rcoJL1AgMBAAECgYBz0R+hbvjRPuJQnNZJu5JZZTfp
OABNnLjzdmZ4Xi8tVmGcLo5dVnPVDf4+EbepGRTTxLIkI6G2JkYduYh/ypuK3TtD
JQ2j2Wb4hSFXc3jGGGmx3SFYrmajM6nW7vnBw7Ld6PaJqo5lZtYcFzpOSrzP5G0p
TPEJ1091aOrhoNexgQJBAP7M2XMw4TJqddT03/y4y46ESq4bNYOIyMd3X9yYM77Q
KH5v1x+95znBkb8hJoPgO2+un4uLr2A8L8umxByTHJECQQDYzw2BxF6D9GSDjQr6
BEX1UxfM96DiSE2N3i+1YJWOdcqg9dvJRByYzvdlqEobY2DB8Cnh1HS94V3vyruw
R1IlAkEA9NTnuTzllukfEiK+O3th9S5/B+8TK7G6o5e8IB6L0jT4RA25W0HBtgie
wFXdSWikE/tqSM9PFByhHIHA/WgKUQJALTMlbrtgtQPbfK2H7026xAV5vcqWaPaH
7J64tYiYRWX7Q4leM9yWVak4XKI0KPeT8Xq/UIx5diio69gJPxvvXQJAM1lr5o49
D0qEjXcpHjsMHcrYgQLGZPCfNn3gkGZ/pxr/3N36SyaqF6/7NRe7BLHbll9lb+8f
8FF/8F+a66TGLw==
-----END PRIVATE KEY-----
`
	testTlsCert = `-----BEGIN CERTIFICATE-----
MIIC7jCCAlegAwIBAgIJAIvCpMZ/RhydMA0GCSqGSIb3DQEBBQUAMIGPMQswCQYD
VQQGEwJERTEPMA0GA1UECAwGQmVybGluMQ8wDQYDVQQHDAZCZXJsaW4xHDAaBgNV
BAoME0JlcmxpbmVyIFJvYm90d2Vya2UxGTAXBgNVBAMMEHRhc2tlZHNlcnZlci5j
b20xJTAjBgkqhkiG9w0BCQEWFmFycGFkLnJ5c3prYUBnbWFpbC5jb20wHhcNMTMw
OTA3MTk1MzU1WhcNMTYwOTA2MTk1MzU1WjCBjzELMAkGA1UEBhMCREUxDzANBgNV
BAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMRwwGgYDVQQKDBNCZXJsaW5lciBS
b2JvdHdlcmtlMRkwFwYDVQQDDBB0YXNrZWRzZXJ2ZXIuY29tMSUwIwYJKoZIhvcN
AQkBFhZhcnBhZC5yeXN6a2FAZ21haWwuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQDXyuyOlRneMsTTSq4Nfhnb3kweGgIqgvdemJebxU5aQoOiyIQGJg71
zzPVyWXP4fDeI4fyTQt/cIHBBv0VdsJfU+t/jUekwS1cepyWyowrxLs8KQdwSLp+
PNu3FOjwpLAXOIZHvaU4A/b9MgzrRAipfh7sUbiiSxqEQkPa3KCS9QIDAQABo1Aw
TjAdBgNVHQ4EFgQUrAUcn4JJ13CSKXdKquzs03OHl0gwHwYDVR0jBBgwFoAUrAUc
n4JJ13CSKXdKquzs03OHl0gwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOB
gQB2VmcD9Hde1Bf9lgk3iWw+ZU8JbdJvhK0MoU4RhCDEl01K2omxoT4B8OVWlFD5
GWX4rnIZtcLahM1eu8h+QxdcTNGwCpIiait2pmpVcV6pjNKv8LUxAcaemq178OfK
h3I2CsHAUTwxT1ca8SGLCsFTm03AyXaU0Q061+RX1Do/Iw==
-----END CERTIFICATE-----
`
)

type testHandler struct {
	sh func(w http.ResponseWriter, r *http.Request)
}

type testSettings struct {
	root             string
	maxRequestBody   int64
	maxSearchResults int
}

func (ts *testSettings) Root() string          { return ts.root }
func (ts *testSettings) MaxRequestBody() int64 { return ts.maxRequestBody }
func (ts *testSettings) MaxSearchResults() int { return ts.maxSearchResults }

func (th *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if th.sh == nil {
		panic("Test handler not initialized.")
	}
	th.sh(w, r)
}

var (
	thnd         = new(testHandler)
	s            *httptest.Server
	mx           = new(sync.Mutex)
	dn           string
	propsRoot    bool
	modpropsRoot bool
	getDirRoot   bool
)

func init() {
	tpr := flag.Bool("test.propsroot", false, "")
	tmpr := flag.Bool("test.modpropsroot", false, "")
	tgdr := flag.Bool("test.getdirroot", false, "")
	flag.Parse()
	propsRoot = *tpr
	modpropsRoot = *tmpr
	getDirRoot = *tgdr

	dn = path.Join(util.Testdir, "http")
	err := util.EnsureDir(dn)
	if err != nil {
		panic(err)
	}
	c, err := tls.X509KeyPair([]byte(testTlsCert), []byte(testTlsKey))
	if err != nil {
		panic(err)
	}
	s = httptest.NewUnstartedServer(thnd)
	s.TLS = &tls.Config{Certificates: []tls.Certificate{c}}
	s.StartTLS()
}

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

func compareProperties(left, right map[string]interface{}) bool {
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
		!compareFileMode("mode") ||
		!compareString("user") ||
		!compareString("group") ||
		!compareInt64("accessTime") ||
		!compareInt64("changeTime") {
		return false
	}
	return true
}

func mkclient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}
}

func htreq(t *testing.T, method, url string, body io.Reader, clb func(rsp *http.Response)) {
	r, err := http.NewRequest(method, url, body)
	util.ErrFatal(t, err)
	client := mkclient()
	rsp, err := client.Do(r)
	util.ErrFatal(t, err)
	defer rsp.Body.Close()
	clb(rsp)
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
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    defaultMode}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
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
		"mode":    os.ModePerm}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
		mode: os.ModePerm + 1024}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    os.ModePerm}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfo{
		sys: &fileInfoT{mode: os.ModePerm + 1024}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    os.ModePerm,
		"dirname": "/"}) {
		t.Fail()
	}
	u, err := user.Current()
	util.ErrFatal(t, err)
	uid, err := strconv.Atoi(u.Uid)
	gid, err := strconv.Atoi(u.Gid)
	util.ErrFatal(t, err)
	g, err := util.LookupGroupById(uint32(gid))
	util.ErrFatal(t, err)
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		modTime: defaultTime,
		isDir:   false,
		mode:    os.ModePerm,
		sys: &syscall.Stat_t{
			Uid:  uint32(uid),
			Gid:  uint32(gid),
			Atim: syscall.Timespec{Sec: defaultTime.Unix() + 42},
			Ctim: syscall.Timespec{Sec: defaultTime.Unix() + 42<<1}}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "some",
		"size":       int64(42),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       os.ModePerm,
		"user":       u.Username,
		"group":      g.Name,
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
		util.ErrFatal(t, err)
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
	util.ErrFatal(t, err)
	cui, err := strconv.Atoi(cu.Uid)
	util.ErrFatal(t, err)
	cuui := uint32(cui)

	fi := &fileInfoT{sys: &syscall.Stat_t{Uid: cuui}}
	is, err := isOwner(cu, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	cu, err := user.Current()
	util.ErrFatal(t, err)
	fi := &fileInfoT{}
	is, err := isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(cu.Uid)
	util.ErrFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerRoot(t *testing.T) {
	if !util.IsRoot {
		t.Skip()
	}

	fi := &fileInfoT{}
	u, err := user.Current()
	util.ErrFatal(t, err)
	is, err := isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(u.Uid)
	util.ErrFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestWriteJsonResponse(t *testing.T) {
	d := map[string]interface{}{"some": "data"}
	js, err := json.Marshal(d)
	util.ErrFatal(t, err)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		writeJsonResponse(w, r, d)
	}
	htreq(t, "GET", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
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
		util.ErrFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if !bytes.Equal(b, nil) {
			t.Fail()
		}
	})
}

func TestDetectContentType(t *testing.T) {
	ct, err := detectContentType("some.html", nil)
	if err != nil || ct != textMimeTypes["html"] {
		t.Fail()
	}
	p := path.Join(util.Testdir, "some-file")
	err = util.WithNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("This suppose to be some human readable text."))
		return err
	})
	util.ErrFatal(t, err)
	f, err := os.Open(p)
	util.ErrFatal(t, err)
	ct, err = detectContentType("some-file", f)
	if err != nil || ct != textMimeTypes["txt"] {
		t.Fail()
	}
}

func TestSearchFiles(t *testing.T) {
	p := path.Join(util.Testdir, "search")
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	di, err := os.Lstat(p)
	util.ErrFatal(t, err)
	dii := &fileInfo{sys: di, dirname: util.Testdir}
	util.WithNewFileF(t, path.Join(p, "file0"), nil)
	util.WithNewFileF(t, path.Join(p, "file1"), nil)

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
	util.RemoveIfExistsF(t, p)
	if len(searchFiles([]*fileInfo{dii}, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// predicate always false
	util.EnsureDirF(t, p)
	di, err = os.Lstat(p)
	util.ErrFatal(t, err)
	dii = &fileInfo{sys: di, dirname: util.Testdir}
	util.WithNewFileF(t, path.Join(p, "file0"), nil)
	util.WithNewFileF(t, path.Join(p, "file1"), nil)
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
	util.ErrFatal(t, err)
	if len(searchFiles([]*fileInfo{&fileInfo{sys: fi, dirname: p}}, 42,
		func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// find all in a recursively
	util.EnsureDirF(t, p)
	di, err = os.Lstat(p)
	util.ErrFatal(t, err)
	dii = &fileInfo{sys: di, dirname: util.Testdir}
	util.WithNewFileF(t, path.Join(p, "file0"), nil)
	util.WithNewFileF(t, path.Join(p, "file1"), nil)
	p0 := path.Join(p, "dir0")
	util.EnsureDirF(t, p0)
	util.WithNewFileF(t, path.Join(p0, "file0"), nil)
	util.WithNewFileF(t, path.Join(p0, "file1"), nil)
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
	util.EnsureDirF(t, p1)
	if len(searchFiles([]*fileInfo{dii}, 2, func(fi *fileInfo) bool {
		if fi.Name() == "dir0" {
			err = os.Rename(path.Join(p0, "file0"), path.Join(p1, "file0"))
			util.ErrFatal(t, err)
			return false
		}
		return fi.Name() == "file0"
	})) != 2 {
		t.Fail()
	}
	err = os.Rename(path.Join(p1, "file0"), path.Join(p1, "file0"))
	util.ErrFatal(t, err)
	err = os.RemoveAll(p1)
	util.ErrFatal(t, err)

	// find all dirs, too
	p1 = path.Join(p0, "dir1")
	util.EnsureDirF(t, p1)
	p2 := path.Join(p0, "dir2")
	util.EnsureDirF(t, p2)
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "dir") == 0
	})) != 3 {
		t.Fail()
	}

	// verify data
	util.WithNewFileF(t, path.Join(p, "file1"), func(f *os.File) error {
		_, err := f.Write([]byte("012345678"))
		return err
	})
	util.WithNewFileF(t, path.Join(p0, "file0"), func(f *os.File) error {
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
	if util.IsRoot {
		t.Skip()
	}

	p := path.Join(util.Testdir, "search")
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	p0 := path.Join(p, "dir0")
	util.EnsureDirF(t, p0)
	util.WithNewFileF(t, path.Join(p, "file0"), nil)
	util.WithNewFileF(t, path.Join(p, "file1"), nil)
	di, err := os.Lstat(p)
	util.ErrFatal(t, err)
	dii := &fileInfo{sys: di, dirname: util.Testdir}

	// not finding when no rights for dir
	err = os.Chmod(p0, 0)
	util.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(p0, os.ModePerm)
		util.ErrFatal(t, err)
	}()
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool { return true })) != 3 {
		t.Fail()
	}
}

func TestCopyTree(t *testing.T) {
	dir := path.Join(dn, "copy-tree")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)

	// no copy of the same path
	fi0, err := os.Lstat(dir)
	util.ErrFatal(t, err)
	err = copyTree(dir, dir)
	if err != nil {
		t.Fail()
	}
	fi1, err := os.Lstat(dir)
	util.ErrFatal(t, err)
	if !fi1.ModTime().Equal(fi0.ModTime()) {
		t.Fail()
	}

	// not found
	dir0 := path.Join(dir, "dir0")
	err = util.RemoveIfExists(dir0)
	dir1 := path.Join(dir, "dir1")
	util.RemoveIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if err == nil || !os.IsNotExist(err) {
		t.Fail()
	}

	// tree
	util.EnsureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	util.WithNewFileF(t, fp, nil)
	util.RemoveIfExistsF(t, dir1)
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
	util.WithNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	util.RemoveIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	cf, err := os.Open(f1)
	util.ErrFatal(t, err)
	defer cf.Close()
	ccf, err := ioutil.ReadAll(cf)
	util.ErrFatal(t, err)
	if !bytes.Equal(ccf, []byte("some content")) {
		t.Fail()
	}
}

func TestCopyTreeNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	dir := path.Join(dn, "copy")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)

	// no access
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	util.RemoveIfExistsF(t, dir1)
	err := os.Chmod(dir, 0600)
	util.ErrFatal(t, err)
	err = copyTree(dir0, dir1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, os.ModePerm)
	util.ErrFatal(t, err)

	// copy abort
	util.EnsureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	util.WithNewFileF(t, fp, nil)
	err = os.Chmod(fp, 0000)
	util.RemoveIfExistsF(t, dir1)
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
	util.ErrFatal(t, err)

	// copy file, no write permission
	f0, f1 := path.Join(dir, "file0"), path.Join(dir, "file1")
	util.WithNewFileF(t, f0, nil)
	util.RemoveIfExistsF(t, f1)
	err = os.Chmod(dir, 0600)
	util.ErrFatal(t, err)
	err = copyTree(f0, f1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, 0777)
	util.ErrFatal(t, err)

	// preserve mode
	util.WithNewFileF(t, f0, nil)
	err = os.Chmod(f0, 0400)
	util.ErrFatal(t, err)
	util.RemoveIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	fi, err := os.Lstat(f1)
	util.ErrFatal(t, err)
	if fi.Mode() != os.FileMode(0400) {
		t.Fail()
	}

	// copy to not existing
	p0, p1 := path.Join(dir, "dir0"), path.Join(dir, "dir1")
	util.EnsureDirF(t, p0)
	util.RemoveIfExistsF(t, p1)
	err = copyTree(p0, p1)
	if err != nil {
		t.Fail()
	}

	// copy to existing file
	f0, f1 = path.Join(dir, "file0"), path.Join(dir, "file1")
	util.WithNewFileF(t, f0, nil)
	util.WithNewFileF(t, f1, nil)
	err = copyTree(f0, f1)
	if err != nil {
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

func TestGetPath(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	p, err := ht.getPath("..")
	if err == nil {
		t.Fail()
	}
	p, err = ht.getPath("some")
	if err != nil || p != path.Join(dn, "some") {
		t.Fail()
	}
}

func TestSearch(t *testing.T) {
	var queryString url.Values
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.search(w, r, queryString)
	}

	p := path.Join(dn, "search")
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)

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
			util.ErrFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			util.ErrFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// max results set
	for i := 0; i < 42; i++ {
		pi := path.Join(p, fmt.Sprintf("file%d", i))
		util.WithNewFileF(t, pi, nil)
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
	util.ErrFatal(t, err)
	checkError(http.StatusNotFound)

	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	util.WithNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	util.WithNewFileF(t, path.Join(p, "fileB"), func(f *os.File) error {
		_, err := f.Write([]byte("b"))
		return err
	})
	util.EnsureDirF(t, path.Join(p, "dirA"))

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
	util.ErrFatal(t, err)
	fib, err := os.Lstat(path.Join(p, "fileB"))
	util.ErrFatal(t, err)
	fid, err := os.Lstat(path.Join(p, "dirA"))
	util.ErrFatal(t, err)
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
		util.ErrFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		util.ErrFatal(t, err)
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

func TestSearchNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	var queryString url.Values
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.search(w, r, queryString)
	}

	p := path.Join(dn, "search")
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)

	checkLen := func(l int) {
		htreq(t, "SEARCH", s.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			js, err := ioutil.ReadAll(rsp.Body)
			util.ErrFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			util.ErrFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// no permissions
	util.EnsureDirF(t, p)
	err := os.Chmod(p, 0)
	util.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(p, os.ModePerm)
		util.ErrFatal(t, err)
	}()
	checkLen(0)

	// filtering by content, no rights
	err = os.Chmod(p, 0777)
	util.ErrFatal(t, err)
	util.WithNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	err = os.Chmod(path.Join(p, "fileA"), 0)
	util.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(path.Join(p, "fileA"), os.ModePerm)
		util.ErrFatal(t, err)
	}()
	queryString = make(url.Values)
	queryString.Set("content", "a")
	checkLen(0)
}

func TestProps(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.props(w, r)
	}

	fn := "some-file"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn

	util.RemoveIfExistsF(t, p)
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

	util.WithNewFileF(t, p, nil)
	fiVerify, err := os.Stat(p)
	util.ErrFatal(t, err)
	prVerify := toPropertyMap(fiVerify, true)
	jsVerify, err := json.Marshal(prVerify)
	util.ErrFatal(t, err)

	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		util.ErrFatal(t, err)
		if !convert64(pr, "modTime") || !convert64(pr, "size") ||
			!convert64(pr, "accessTime") || !convert64(pr, "changeTime") ||
			!convertFm(pr) {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Fail()
		}
	})

	util.WithNewFileF(t, p, nil)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(headerContentType) != jsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if len(js) != 0 {
			t.Fail()
		}
	})
}

func TestPropsRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !util.IsRoot || !propsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	err := os.Chmod(util.Testdir, 0777)
	util.ErrFatal(t, err)
	err = os.Chmod(dn, 0777)
	util.ErrFatal(t, err)
	fn := "some-file-uid"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	util.WithNewFileF(t, p, nil)
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.props(w, r)
	}

	// uid := syscall.Getuid()
	usr, err := user.Lookup(util.Testuser)
	util.ErrFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	util.ErrFatal(t, err)
	err = syscall.Setuid(tuid)
	util.ErrFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	util.ErrFatal(t, err)
	// }()

	fiVerify, err := os.Stat(p)
	util.ErrFatal(t, err)
	prVerify := toPropertyMap(fiVerify, false)
	jsVerify, err := json.Marshal(prVerify)
	util.ErrFatal(t, err)
	htreq(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		util.ErrFatal(t, err)
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

func TestModprops(t *testing.T) {
	fn := "some-file"
	p := path.Join(dn, fn)
	util.WithNewFileF(t, p, nil)
	st := &testSettings{root: dn, maxRequestBody: defaultMaxRequestBody}
	ht := New(dn, st)
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.modprops(w, r)
	}

	// max req length
	st.maxRequestBody = 8
	ht = New(dn, st)
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("{\"something\": \"long enough\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fail()
			}
		})
	st.maxRequestBody = defaultMaxRequestBody
	ht = New(dn, st)

	// json number
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("{\"mode\":  \"not a number\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("{\"mode\": 0.1}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("{\"mode\": -2}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0600)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	err := os.Chmod(p, 0600)
	util.ErrFatal(t, err)

	// valid json
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("not json"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// map only or nil
	htreq(t, "MODPROP", s.URL, bytes.NewBufferString("[]"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("null"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	htreq(t, "MODPROPS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// one map only
	htreq(t, "MODPROP", s.URL, bytes.NewBufferString("{\"mode\": 0}{\"mode\": 0}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// valid fields only
	htreq(t, "MODPROPS", s.URL, bytes.NewBufferString("{\"some\": \"value\"}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// not found
	util.RemoveIfExistsF(t, p)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString("{\"mode\":0}"), func(rsp *http.Response) {
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

	// mod, success
	util.WithNewFileF(t, p, nil)
	err = os.Chmod(p, os.ModePerm)
	util.ErrFatal(t, err)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Stat(p)
			util.ErrFatal(t, err)
			if fi.Mode() != os.FileMode(0744) {
				t.Fail()
			}
		})

	// mod, success, masked
	err = os.Chmod(p, os.ModePerm)
	util.ErrFatal(t, err)
	htreq(t, "MODPROPS", s.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 01744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Stat(p)
			util.ErrFatal(t, err)
			if fi.Mode() != os.FileMode(0744) {
				t.Fail()
			}
		})
}

func TestModpropsRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !util.IsRoot || !modpropsRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	fn := "some-file-uid-mod"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn

	usr, err := user.Lookup(util.Testuser)
	util.ErrFatal(t, err)

	err = os.Chmod(dn, os.ModePerm)
	util.ErrFatal(t, err)
	err = os.Chmod(util.Testdir, os.ModePerm)
	util.ErrFatal(t, err)
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.modprops(w, r)
	}

	// chown
	util.WithNewFileF(t, p, nil)
	htreq(t, "MODPROPS", url, bytes.NewBufferString(fmt.Sprintf("{\"owner\": \"%s\"}", util.Testuser)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Lstat(p)
			util.ErrFatal(t, err)
			sstat, ok := fi.Sys().(*syscall.Stat_t)
			if !ok {
				t.Fatal()
			}
			if strconv.Itoa(int(sstat.Uid)) != usr.Uid {
				t.Fail()
			}
		})

	// uid := syscall.Getuid()
	util.WithNewFileF(t, p, nil)
	err = os.Chmod(p, os.ModePerm)
	util.ErrFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	util.ErrFatal(t, err)
	err = syscall.Setuid(tuid)
	util.ErrFatal(t, err)

	htreq(t, "MODPROPS", url,
		bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0744)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusNotFound {
				t.Fail()
			}
			fi, err := os.Lstat(p)
			util.ErrFatal(t, err)
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
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		util.WithNewFileF(t, path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
	}

	// not found
	util.EnsureDirF(t, p)
	err := os.Chmod(p, os.ModePerm)
	util.ErrFatal(t, err)
	d, err = os.Open(p)
	util.ErrFatal(t, err)
	util.RemoveIfExistsF(t, p)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if strings.Trim(string(b), "\n") != http.StatusText(http.StatusNotFound) {
			t.Fail()
		}
	})

	// empty dir
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	d, err = os.Open(p)
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		var res []interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 0 {
			t.Fail()
		}
	})

	// dir with files
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
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
				util.ErrFatal(t, err)
				if !compareProperties(m, toPropertyMap(fi, true)) {
					t.Fail()
				}
			default:
				t.Fail()
			}
		}
	})

	// tests the same with HEAD
	util.RemoveIfExistsF(t, p)
	util.EnsureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	util.ErrFatal(t, err)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if len(b) > 0 {
			t.Fail()
		}
	})
}

func TestGetDirRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !util.IsRoot || !getDirRoot {
		t.Skip()
	}

	t.Parallel()
	mx.Lock()
	defer mx.Unlock()

	err := os.Chmod(util.Testdir, 0777)
	util.ErrFatal(t, err)
	err = os.Chmod(dn, 0777)
	util.ErrFatal(t, err)

	fn := "some-dir"
	p := path.Join(dn, fn)
	util.EnsureDirF(t, p)
	err = os.Chmod(p, 0777)
	util.ErrFatal(t, err)
	url := s.URL + "/" + fn
	var d *os.File
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		util.WithNewFileF(t, path.Join(p, n), func(f *os.File) error {
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
	usr, err := user.Lookup(util.Testuser)
	util.ErrFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	util.ErrFatal(t, err)
	tgid, err := strconv.Atoi(usr.Gid)
	util.ErrFatal(t, err)
	err = os.Chown(path.Join(p, "some1"), tuid, tgid)
	util.ErrFatal(t, err)
	err = syscall.Setuid(tuid)
	util.ErrFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	util.ErrFatal(t, err)
	// }()

	d, err = os.Open(p)
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
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
					!convert64(m, "accessTime") || !convert64(m, "changeTime") {
					t.Fail()
				}
				if n == "some1" && !convertFm(m) {
					t.Fail()
				}
				fi, err := os.Stat(path.Join(p, n))
				util.ErrFatal(t, err)
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
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getFile(w, r, f, fi)
	}

	// extension tested
	fn := "some-file.html"
	p := path.Join(dn, fn)
	url := s.URL + "/" + fn
	util.WithNewFileF(t, p, nil)
	f, err := os.Open(p)
	util.ErrFatal(t, err)
	fi, err = f.Stat()
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
	})
	err = f.Close()
	util.ErrFatal(t, err)

	// content tested, length set, content sent
	fn = "some-file"
	p = path.Join(dn, fn)
	url = s.URL + "/" + fn
	html := []byte("<html></html>")
	util.WithNewFileF(t, p, func(f *os.File) error {
		_, err = f.Write(html)
		return err
	})
	f, err = os.Open(p)
	util.ErrFatal(t, err)
	fi, err = f.Stat()
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if !bytes.Equal(b, html) {
			t.Fail()
		}
	})
	err = f.Close()
	util.ErrFatal(t, err)

	// HEAD handled
	f, err = os.Open(p)
	util.ErrFatal(t, err)
	fi, err = f.Stat()
	util.ErrFatal(t, err)
	htreq(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		if len(b) != 0 {
			t.Fail()
		}
	})
	err = f.Close()
	util.ErrFatal(t, err)

	// emulate copy failure
	// file handler still open, but file deleted, can't help this without performance penalty
	f, err = os.Open(p)
	util.ErrFatal(t, err)
	fi, err = f.Stat()
	util.ErrFatal(t, err)
	err = os.Remove(p)
	util.ErrFatal(t, err)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if rsp.Header.Get(headerContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(headerContentLength))
		util.ErrFatal(t, err)
		if clen != len(html) {
			t.Fail()
		}
	})
	err = f.Close()
	util.ErrFatal(t, err)
}

func TestPut(t *testing.T) {
	ht := New(dn, &testSettings{root: dn, maxRequestBody: defaultMaxRequestBody})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.put(w, r)
	}

	// invalid path
	htreq(t, "PUT", s.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// existing dir
	err := os.MkdirAll(path.Join(dn, "dir"), os.ModePerm)
	util.ErrFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// existing file
	p := path.Join(dn, "file")
	util.RemoveIfExistsF(t, p)
	util.WithNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	htreq(t, "PUT", s.URL+"/file", bytes.NewBufferString("new content"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		util.ErrFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		util.ErrFatal(t, err)
		if !bytes.Equal(content, []byte("new content")) {
			t.Fail()
		}
	})

	// new file
	p = path.Join(dn, "file")
	util.RemoveIfExistsF(t, p)
	htreq(t, "PUT", s.URL+"/file", bytes.NewBufferString("some content"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		util.ErrFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		util.ErrFatal(t, err)
		if !bytes.Equal(content, []byte("some content")) {
			t.Fail()
		}
		fi, err := os.Lstat(p)
		util.ErrFatal(t, err)
		if fi.Mode() != os.FileMode(0600) {
			t.Fail()
		}
	})

	// clear file
	p = path.Join(dn, "file")
	util.RemoveIfExistsF(t, p)
	util.WithNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write([]byte("old content"))
		return err
	})
	htreq(t, "PUT", s.URL+"/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		f, err := os.Open(p)
		util.ErrFatal(t, err)
		defer f.Close()
		content, err := ioutil.ReadAll(f)
		util.ErrFatal(t, err)
		if len(content) != 0 {
			t.Fail()
		}
	})

	// create full path
	d0 := path.Join(dn, "dir0")
	util.RemoveIfExistsF(t, d0)
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
	ht = New(dn, &testSettings{root: dn, maxRequestBody: 8})
	htreq(t, "PUT", s.URL+"/file", io.LimitReader(rand.Reader, 16), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fail()
		}
	})
}

func TestPutNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	ht := New(dn, &testSettings{root: dn, maxRequestBody: defaultMaxRequestBody})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.put(w, r)
	}

	// no permission to write dir
	dp := path.Join(dn, "dir")
	util.EnsureDirF(t, dp)
	p := path.Join(dp, "file")
	util.RemoveIfExistsF(t, p)
	err := os.Chmod(dp, 0555)
	util.ErrFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to execute dir
	dp = path.Join(dn, "dir")
	util.EnsureDirF(t, dp)
	p = path.Join(dp, "file")
	util.RemoveIfExistsF(t, p)
	err = os.Chmod(dp, 0666)
	util.ErrFatal(t, err)
	htreq(t, "PUT", s.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to write file
	p = path.Join(dn, "file")
	util.WithNewFileF(t, p, nil)
	err = os.Chmod(p, 0444)
	util.ErrFatal(t, err)
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
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.copyRename(w, r, qry, multiple, f)
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

func TestCopy(t *testing.T) {
	var qry url.Values
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.copy(w, r, qry)
	}

	dir := path.Join(dn, "copy")
	util.EnsureDirF(t, dir)
	fn0 := path.Join(dir, "file0")
	util.WithNewFileF(t, fn0, nil)
	fn1 := path.Join(dir, "file1")
	util.RemoveIfExistsF(t, fn1)

	// copy over not existing file
	qry = make(url.Values)
	qry.Set("to", "/copy/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// copy over existing file
	util.WithNewFileF(t, fn1, nil)
	qry = make(url.Values)
	qry.Set("to", "/copy/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// copy under not existing dir
	dir0 := path.Join(dir, "dir0")
	util.RemoveIfExistsF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0/file1")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over empty directory
	util.WithNewDirF(t, dir0)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over not empty directory
	util.WithNewDirF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	util.WithNewFileF(t, fn1, nil)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	htreq(t, "COPY", s.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRename(t *testing.T) {
	var qry url.Values
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.rename(w, r, qry)
	}

	dir := path.Join(dn, "rename")
	util.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	util.EnsureDirF(t, dir1)
	fn0 := path.Join(dir0, "file0")
	util.WithNewFileF(t, fn0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	fn1 := path.Join(dir1, "file1")
	util.WithNewFileF(t, fn1, nil)

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
	util.RemoveIfExistsF(t, dir0)
	fn0 = path.Join(dir, "file0")
	util.WithNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0/file1")
	htreq(t, "RENAME", s.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// over existing dir
	util.EnsureDirF(t, dir0)
	util.WithNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0")
	htreq(t, "RENAME", s.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRenamefNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	var qry url.Values
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.rename(w, r, qry)
	}

	dir := path.Join(dn, "rename")
	util.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	util.EnsureDirF(t, dir1)
	fn0 := path.Join(dir0, "file0")
	util.WithNewFileF(t, fn0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	fn1 := path.Join(dir1, "file1")
	util.WithNewFileF(t, fn1, nil)

	// no write source
	err := os.Chmod(dir0, 0500)
	util.ErrFatal(t, err)
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
	util.ErrFatal(t, err)

	// no write to target
	err = os.Chmod(dir1, 0500)
	util.ErrFatal(t, err)
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
	util.ErrFatal(t, err)
}

func TestDeletef(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.delete(w, r)
	}
	dir := path.Join(dn, "delete")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	file0 := path.Join(dir0, "file0")
	util.WithNewFileF(t, file0, nil)

	// doesn't exist
	util.RemoveIfExistsF(t, file0)
	htreq(t, "DELETE", s.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// exists, deleted
	util.WithNewFileF(t, file0, nil)
	htreq(t, "DELETE", s.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); !os.IsNotExist(err) {
			t.Fail()
		}
	})
}

func TestDeletefNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.delete(w, r)
	}
	dir := path.Join(dn, "delete")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	file0 := path.Join(dir0, "file0")
	util.WithNewFileF(t, file0, nil)
	err := os.Chmod(dir0, 0500)
	util.ErrFatal(t, err)
	htreq(t, "DELETE", s.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	util.ErrFatal(t, err)
}

func TestMkdirf(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.mkdir(w, r)
	}
	dir := path.Join(dn, "mkdir")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)

	// doesn't exist, created
	dir1 := path.Join(dir0, "dir1")
	util.RemoveIfExistsF(t, dir1)
	htreq(t, "MKDIR", s.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(dir1); err != nil {
			t.Fail()
		}
	})

	// exists, not touched
	util.EnsureDirF(t, dir1)
	file0 := path.Join(dir1, "file0")
	util.WithNewFileF(t, file0, nil)
	htreq(t, "MKDIR", s.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); err != nil {
			t.Fail()
		}
	})
}

func TestMkdirfNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	ht := New(dn, &testSettings{root: dn})
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		ht.mkdir(w, r)
	}
	dir := path.Join(dn, "mkdir")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	util.EnsureDirF(t, dir0)
	err := os.Chmod(dir0, 0500)
	util.ErrFatal(t, err)
	dir1 := path.Join(dir0, "dir1")
	htreq(t, "MKDIR", s.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		if _, err := os.Lstat(dir1); !os.IsNotExist(err) {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	util.ErrFatal(t, err)
}

func TestNoCmd(t *testing.T) {
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		noCmd(w, r, func(w http.ResponseWriter, r *http.Request) {})
	}
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
	thnd.sh = func(w http.ResponseWriter, r *http.Request) {
		queryNoCmd(w, r, func(w http.ResponseWriter, r *http.Request, qry url.Values) {
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
	}

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

func TestNew(t *testing.T) {
	ht := New("", nil)
	if ht.dn != "" ||
		ht.maxRequestBody != defaultMaxRequestBody ||
		ht.maxSearchResults != defaultMaxSearchResults {
		t.Fail()
	}
	ht = New(dn, &testSettings{
		maxRequestBody:   42,
		maxSearchResults: 1764})
	if ht.dn != dn ||
		ht.maxRequestBody != 42 ||
		ht.maxSearchResults != 1764 {
		t.Fail()
	}
}

func TestOptionsHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP
	htreq(t, "OPTIONS", s.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			!verifyHeader(map[string][]string{"Content-Length": []string{"0"}}, rsp.Header) {
			t.Fail()
		}
	})
}

func TestPropsHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP
	fn := "some-file"
	p := path.Join(dn, fn)
	util.WithNewFileF(t, p, nil)
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

func TestModpropsHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP
	fn := "some-file"
	p := path.Join(dn, fn)
	util.WithNewFileF(t, p, nil)
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

func TestPutHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP

	// invalid command
	htreq(t, "PUT", s.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// put
	p := path.Join(dn, "some-file")
	util.RemoveIfExistsF(t, p)
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

func TestSearchHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP
	htreq(t, "SEARCH", s.URL+"?cmd=search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	for i := 0; i < 3; i++ {
		util.WithNewFileF(t, path.Join(dn, fmt.Sprintf("file%d", i)), nil)
	}
	htreq(t, "SEARCH", s.URL+"?max=3", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		util.ErrFatal(t, err)
		if len(m) != 3 {
			t.Fail()
		}
	})
}

func TestCopyHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP

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

func TestGet(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP

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
	util.RemoveIfExistsF(t, p)
	htreq(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// listing if directory
	dir := path.Join(dn, "search")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)
	c := []byte("some content")
	p = path.Join(dir, "some-file")
	util.WithNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write(c)
		return err
	})
	htreq(t, "GET", s.URL+"/search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		util.ErrFatal(t, err)
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
		util.ErrFatal(t, err)
		if !bytes.Equal(b, c) {
			t.Fail()
		}
	})
}

func TestPostHandler(t *testing.T) {
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP

	// invalid query
	htreq(t, "POST", s.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// invalid command
	htreq(t, "POST", s.URL+"?cmd=invalid", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	dir := path.Join(dn, "post")
	util.RemoveIfExistsF(t, dir)
	util.EnsureDirF(t, dir)

	// modprops
	file := path.Join(dir, "file")
	util.WithNewFileF(t, file, nil)
	err := os.Chmod(file, 0600)
	util.ErrFatal(t, err)
	props := map[string]interface{}{"mode": 0660}
	js, err := json.Marshal(props)
	htreq(t, "POST", s.URL+"/post/file?cmd=modprops", bytes.NewBuffer(js), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		fi, err := os.Lstat(file)
		util.ErrFatal(t, err)
		if fi.Mode() != os.FileMode(0660) {
			t.Fail()
		}
	})

	// delete
	util.WithNewFileF(t, file, nil)
	htreq(t, "POST", s.URL+"/post/file?cmd=delete", nil, func(rsp *http.Response) {
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
	util.RemoveIfExistsF(t, dir0)
	htreq(t, "POST", s.URL+"/post/dir0?cmd=mkdir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(dir0); err != nil {
			t.Fail()
		}
	})

	// copy
	util.WithNewFileF(t, file, nil)
	file0 := path.Join(dir, "file0")
	util.RemoveIfExistsF(t, file0)
	htreq(t, "POST", s.URL+"/post/file?cmd=copy&to=/post/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		if _, err := os.Lstat(file0); err != nil {
			t.Fail()
		}
	})

	// rename
	util.WithNewFileF(t, file, nil)
	util.RemoveIfExistsF(t, file0)
	htreq(t, "POST", s.URL+"/post/file?cmd=rename&to=/post/file0", nil, func(rsp *http.Response) {
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
	ht := New(dn, &testSettings{root: dn})
	thnd.sh = ht.ServeHTTP
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
