package share

import (
	"bytes"
	tst "code.google.com/p/tasked/testing"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"testing"
)

func TestCascadeFilter(t *testing.T) {
	f := CascadeFilters()
	res, handled := f.Filter(nil, nil, nil)
	if res != nil || handled {
		t.Fail()
	}

	f = CascadeFilters(
		FilterFunc(func(_ http.ResponseWriter, _ *http.Request, d interface{}) (interface{}, bool) {
			di, ok := d.(int)
			if !ok || di != 1 {
				t.Fail()
			}
			return 2, false
		}), FilterFunc(func(_ http.ResponseWriter, _ *http.Request, d interface{}) (interface{}, bool) {
			di, ok := d.(int)
			if !ok || di != 2 {
				t.Fail()
			}
			return 3, true
		}), FilterFunc(func(_ http.ResponseWriter, _ *http.Request, d interface{}) (interface{}, bool) {
			t.Fail()
			return nil, false
		}))
	res, handled = f.Filter(nil, nil, 1)
	resi, ok := res.(int)
	if !ok || resi != 3 || !handled {
		t.Fail()
	}
}

func TestMaxReader(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr := &MaxReader{Reader: buf, Count: 0}
	b := make([]byte, 32)
	n, err := mr.Read(b)
	if n != 0 || err == nil {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &MaxReader{Reader: buf, Count: 6}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 6 || err != nil || mr.Count != 0 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err == nil || mr.Count != 0 {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &MaxReader{Reader: buf, Count: 12}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 12 || err != nil || mr.Count != 0 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err == nil || mr.Count != 0 {
		t.Fail()
	}

	buf = bytes.NewBuffer([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1})
	mr = &MaxReader{Reader: buf, Count: 64}
	b = make([]byte, 32)
	n, err = mr.Read(b)
	if n != 12 || err != nil || mr.Count != 52 {
		t.Fail()
	}
	n, err = mr.Read(b)
	if n != 0 || err != io.EOF || mr.Count != 52 {
		t.Fail()
	}
}

func TestGetQryValuesCmd(t *testing.T) {
	var (
		qry url.Values
		cmd string
		err error
	)
	qry = url.Values{}
	cmd, err = GetQryValuesCmd(qry, "all_")
	if cmd != "" || err != nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{CmdProps}}
	cmd, err = GetQryValuesCmd(qry, "all_")
	if cmd != CmdProps || err != nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{CmdProps, CmdModprops}}
	_, err = GetQryValuesCmd(qry, "all_")
	if err == nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{CmdProps, CmdModprops}}
	_, err = GetQryValuesCmd(qry, "all_", CmdModprops)
	if err == nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{CmdModprops}}
	_, err = GetQryValuesCmd(qry, "all_", CmdModprops)
	if err == nil {
		t.Fail()
	}

	qry = url.Values{}
	cmd, err = GetQryValuesCmd(qry)
	if cmd != "" || err != nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{CmdProps}}
	_, err = GetQryValuesCmd(qry)
	if err == nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{"custom0"}}
	cmd, err = GetQryValuesCmd(qry, "custom0", "custom1")
	if cmd != "custom0" || err != nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{"custom0", "custom1"}}
	_, err = GetQryValuesCmd(qry, "custom0", "custom1")
	if err == nil {
		t.Fail()
	}

	qry = url.Values{CmdKey: []string{"custom2"}}
	_, err = GetQryValuesCmd(qry, "custom0", "custom1")
	if err == nil {
		t.Fail()
	}
}

func TestGetQryCmd(t *testing.T) {
	if _, err := GetQryCmd(&http.Request{URL: &url.URL{RawQuery: "%%"}}); err == nil {
		t.Fail()
	}
}

func TestHandleErrno(t *testing.T) {
	var errno syscall.Errno
	tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
		HandleErrno(w, errno)
	}

	// enoent
	errno = syscall.ENOENT
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EPERM
	errno = syscall.EPERM
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EACCES
	errno = syscall.EACCES
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EISDIR
	errno = syscall.EISDIR
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// EINVAL
	errno = syscall.EINVAL
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// other
	errno = syscall.EIO
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusInternalServerError {
			t.Fail()
		}
	})
}

func TestErrorResponse(t *testing.T) {
	testStatus := func(status int) {
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			ErrorResponse(w, status)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
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
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			CheckOsError(w, testErr)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != status {
				t.Fail()
			}
			if clb != nil {
				clb(rsp)
			}
		})
	}

	// 404
	if CheckOsError(httptest.NewRecorder(), os.ErrNotExist) {
		t.Fail()
	}
	test(os.ErrNotExist, http.StatusNotFound, nil)

	// 404 - no permission
	if CheckOsError(httptest.NewRecorder(), os.ErrPermission) {
		t.Fail()
	}
	test(os.ErrPermission, http.StatusNotFound, nil)

	// 400
	perr := &os.PathError{Err: syscall.EINVAL}
	if CheckOsError(httptest.NewRecorder(), perr) {
		t.Fail()
	}
	test(perr, http.StatusBadRequest, nil)

	// 500
	if CheckOsError(httptest.NewRecorder(), errors.New("error")) {
		t.Fail()
	}
	test(errors.New("error"), http.StatusInternalServerError, nil)

	// no error
	if !CheckOsError(httptest.NewRecorder(), nil) {
		t.Fail()
	}
	test(nil, http.StatusOK, nil)

	// is dir
	perr = &os.PathError{Err: syscall.EISDIR}
	if CheckOsError(httptest.NewRecorder(), perr) {
		t.Fail()
	}
	test(perr, http.StatusNotFound, nil)
	serr := &os.SyscallError{Err: syscall.EISDIR}
	if CheckOsError(httptest.NewRecorder(), serr) {
		t.Fail()
	}
	test(serr, http.StatusNotFound, nil)
	lerr := &os.LinkError{Err: syscall.EISDIR}
	if CheckOsError(httptest.NewRecorder(), lerr) {
		t.Fail()
	}
	test(lerr, http.StatusNotFound, nil)
}

func TestCheckHandle(t *testing.T) {
	test := func(shouldFail bool, status int) {
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			CheckHandle(w, !shouldFail, status)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != status ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if CheckHandle(httptest.NewRecorder(), false, http.StatusNotFound) {
		t.Fail()
	}
	test(true, http.StatusNotFound)
	if CheckHandle(httptest.NewRecorder(), false, http.StatusMethodNotAllowed) {
		t.Fail()
	}
	test(true, http.StatusMethodNotAllowed)
	if !CheckHandle(httptest.NewRecorder(), true, http.StatusMethodNotAllowed) {
		t.Fail()
	}
	test(false, 0)
}

func TestCheckBadReq(t *testing.T) {
	test := func(shouldFail bool) {
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			CheckBadReq(w, !shouldFail)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if !CheckBadReq(httptest.NewRecorder(), true) {
		t.Fail()
	}
	test(true)
	if CheckBadReq(httptest.NewRecorder(), false) {
		t.Fail()
	}
	test(false)
}

func TestCheckServerError(t *testing.T) {
	test := func(shouldFail bool) {
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			CheckServerError(w, !shouldFail)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusInternalServerError ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	if !CheckBadReq(httptest.NewRecorder(), true) {
		t.Fail()
	}
	test(true)
	if CheckBadReq(httptest.NewRecorder(), false) {
		t.Fail()
	}
	test(false)
}

func TestCheckQryValuesCmd(t *testing.T) {
	mkqry := func(u string) url.Values {
		vs, err := url.ParseQuery(u)
		tst.ErrFatal(t, err)
		return vs
	}
	test := func(qry url.Values, shouldFail bool, allowed ...string) {
		tst.Thnd.Sh = func(w http.ResponseWriter, _ *http.Request) {
			CheckQryValuesCmd(w, qry, allowed...)
		}
		tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
			if shouldFail && rsp.StatusCode != http.StatusBadRequest ||
				!shouldFail && rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}

	test(mkqry("cmd=1"), true)
	test(mkqry("cmd=1"), false, "1")
}

func TestCheckQryCmd(t *testing.T) {
	if _, ok := CheckQryCmd(httptest.NewRecorder(), &http.Request{URL: &url.URL{RawQuery: "%%"}}); ok {
		t.Fail()
	}
}

func TestReadJsonRequest(t *testing.T) {
	var (
		maxreq = int64(1 << 10)
		ip     interface{}
		err    error
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		err = ReadJsonRequest(r, &ip, maxreq)
	}

	// not json
	tst.Htreq(t, "MODPROPS", tst.S.URL, bytes.NewBufferString("not json"), func(rsp *http.Response) {
		if err != InvalidJson {
			t.Fail()
		}
	})

	// to big
	maxreq = int64(4)
	tst.Htreq(t, "MODPROPS", tst.S.URL, bytes.NewBufferString("{\"some\": \"long data here\"}"),
		func(rsp *http.Response) {
			if err != RequestBodyTooLarge {
				t.Fail()
			}
		})
	maxreq = int64(1 << 10)

	// mixed
	tst.Htreq(t, "MODPROPS", tst.S.URL, bytes.NewBufferString("{\"some\": \"some data\"} - outside of json"),
		func(rsp *http.Response) {
			if err != InvalidJson {
				t.Fail()
			}
		})

	// valid
	ip = nil
	tst.Htreq(t, "MODPROPS", tst.S.URL, bytes.NewBufferString("{\"some\": \"some data\"}"),
		func(rsp *http.Response) {
			if err != nil {
				t.Fail()
			}
			m, ok := ip.(map[string]interface{})
			if !ok {
				t.Fail()
			}
			v, ok := m["some"]
			if !ok || v != "some data" {
				t.Fail()
			}
		})

	// empty
	ip = nil
	tst.Htreq(t, "MODPROPS", tst.S.URL, nil, func(rsp *http.Response) {
		if err != nil || ip != nil {
			t.Fail()
		}
	})
}

func TestWriteJsonResponse(t *testing.T) {
	d := map[string]interface{}{"some": "data"}
	js, err := json.Marshal(d)
	tst.ErrFatal(t, err)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		WriteJsonResponse(w, r, d)
	}
	tst.Htreq(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(HeaderContentType) != JsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(HeaderContentLength))
		tst.ErrFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(b, js) {
			t.Fail()
		}
		var jsBack map[string]interface{}
		err = json.Unmarshal(b, &jsBack)
		if err != nil || len(jsBack) != 1 || jsBack["some"] != "data" {
			t.Fail()
		}
	})
	tst.Htreq(t, "HEAD", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(HeaderContentType) != JsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(HeaderContentLength))
		tst.ErrFatal(t, err)
		if clen != len(js) {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(b, nil) {
			t.Fail()
		}
	})
}
