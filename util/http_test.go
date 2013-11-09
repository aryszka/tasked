package util

import (
	"testing"
	tst "code.google.com/p/tasked/testing"
	"syscall"
	"net/http"
	"net/http/httptest"
	"fmt"
	"os"
	"errors"
	"net/url"
)

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

func TestChecValueskQryCmd(t *testing.T) {
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
	test(mkqry("cmd=1"), false, "1", "2")
	test(mkqry("cmd=1&cmd=2"), true, "1", "2")
	test(mkqry("cmd=1&cmd=1"), true, "1", "2")
	test(mkqry("cmd=3"), true, "1", "2")

	if cmd, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("")); !ok || len(cmd) > 0 {
		t.Fail()
	}
	if _, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1")); ok {
		t.Fail()
	}
	if cmd, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1"), "1"); !ok || cmd != "1" {
		t.Fail()
	}
	if cmd, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1"), "1", "2"); !ok || cmd != "1" {
		t.Fail()
	}
	if _, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1&cmd=2"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=1&cmd=1"), "1", "2"); ok {
		t.Fail()
	}
	if _, ok := CheckQryValuesCmd(httptest.NewRecorder(), mkqry("cmd=3"), "1", "2"); ok {
		t.Fail()
	}
}

func TestCheckQryCmd(t *testing.T) {
	if _, ok := CheckQryCmd(httptest.NewRecorder(), &http.Request{URL: &url.URL{RawQuery: "%%"}}); ok {
		t.Fail()
	}
}
