package util

import (
	"net/http"
	"syscall"
	"os"
	"net/url"
)

const CmdKey                  = "cmd"

type HttpFilter interface {
	http.Handler
	Filter(http.ResponseWriter, *http.Request, interface{}) (interface{}, bool)
}

func ErrorResponse(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

func HandleErrno(w http.ResponseWriter, errno syscall.Errno) {
	switch errno {
	case syscall.ENOENT, syscall.EPERM, syscall.EACCES, syscall.EISDIR:
		ErrorResponse(w, http.StatusNotFound)
	case syscall.EINVAL:
		ErrorResponse(w, http.StatusBadRequest)
	default:
		ErrorResponse(w, http.StatusInternalServerError)
	}
}

func CheckOsError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case os.IsNotExist(err), os.IsPermission(err):
		ErrorResponse(w, http.StatusNotFound)
	default:
		if perr, ok := err.(*os.PathError); ok {
			if errno, ok := perr.Err.(syscall.Errno); ok {
				HandleErrno(w, errno)
				return false
			}
		}
		if serr, ok := err.(*os.SyscallError); ok {
			if errno, ok := serr.Err.(syscall.Errno); ok {
				HandleErrno(w, errno)
				return false
			}
		}
		if lerr, ok := err.(*os.LinkError); ok {
			if errno, ok := lerr.Err.(syscall.Errno); ok {
				HandleErrno(w, errno)
				return false
			}
		}
		ErrorResponse(w, http.StatusInternalServerError)
	}
	return false
}

func CheckHandle(w http.ResponseWriter, exp bool, status int) bool {
	if exp {
		return true
	}
	ErrorResponse(w, status)
	return false
}

func CheckBadReq(w http.ResponseWriter, exp bool) bool {
	return CheckHandle(w, exp, http.StatusBadRequest)
}

func CheckServerError(w http.ResponseWriter, exp bool) bool {
	return CheckHandle(w, exp, http.StatusInternalServerError)
}

func CheckQryValuesCmd(w http.ResponseWriter, qry url.Values, allowed ...string) (string, bool) {
	cmds := qry[CmdKey]
	for _, cmd := range cmds {
		found := false
		for _, ac := range allowed {
			if cmd == ac {
				found = true
				break
			}
		}
		if !CheckBadReq(w, found) {
			return "", false
		}
	}
	if len(cmds) == 0 {
		return "", true
	}
	if !CheckBadReq(w, len(cmds) == 1) {
		return "", false
	}
	return cmds[0], true
}

func CheckQryCmd(w http.ResponseWriter, r *http.Request, allowed ...string) (string, bool) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !CheckBadReq(w, err == nil) {
		return "", false
	}
	return CheckQryValuesCmd(w, qry, allowed...)
}
