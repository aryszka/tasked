package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"syscall"
)

const (
	CmdKey      = "cmd"
	CmdProps    = "props"
	CmdSearch   = "search"
	CmdModprops = "modprops"
	CmdDelete   = "delete"
	CmdMkdir    = "mkdir"
	CmdCopy     = "copy"
	CmdRename   = "rename"
	CmdAuth     = "auth"
	CmdAll      = "all_"
)

var (
	allCmds                   = []string{CmdProps, CmdSearch, CmdModprops, CmdDelete, CmdMkdir, CmdCopy, CmdRename, CmdAuth}
	HeaderContentType         = http.CanonicalHeaderKey("content-type")
	HeaderContentLength       = http.CanonicalHeaderKey("content-length")
	JsonContentType           = "application/json; charset=utf-8"
	maxReadExceeded           = errors.New("Maximum read count exceeded.")
	MarshalError        error = errors.New("Marshaling error.")
	InvalidQryCmd             = errors.New("Invalid query command.")
	RequestBodyTooLarge       = errors.New("Request body too large.")
	InvalidJson               = errors.New("Invalid JSON.")
)

type HttpFilter interface {
	Filter(http.ResponseWriter, *http.Request, interface{}) (interface{}, bool)
}

type MaxReader struct {
	Reader io.Reader
	Count  int64
}

func (mr *MaxReader) Read(b []byte) (n int, err error) {
	if mr.Count <= 0 {
		return 0, maxReadExceeded
	}
	if int64(len(b)) > mr.Count {
		b = b[:mr.Count]
	}
	n, err = mr.Reader.Read(b)
	mr.Count = mr.Count - int64(n)
	return
}

func GetQryValuesCmd(qry url.Values, allowed ...string) (string, error) {
	if len(allowed) > 0 && allowed[0] == CmdAll {
		if len(allowed) == 1 {
			allowed = allCmds
		} else {
			filtered := make([]string, 0, 8)
			for _, cmd := range allCmds {
				found := false
				for _, filter := range allowed[1:] {
					if cmd == filter {
						found = true
						break
					}
				}
				if !found {
					filtered = append(filtered, cmd)
				}
			}
			allowed = filtered
		}
	}
	cmds := qry[CmdKey]
	for _, cmd := range cmds {
		found := false
		for _, ac := range allowed {
			if cmd == ac {
				found = true
				break
			}
		}
		if !found {
			return "", InvalidQryCmd
		}
	}
	if len(cmds) == 0 {
		return "", nil
	}
	if len(cmds) != 1 {
		return "", InvalidQryCmd
	}
	return cmds[0], nil
}

func GetQryCmd(r *http.Request, allowed ...string) (string, error) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", err
	}
	return GetQryValuesCmd(qry, allowed...)
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
	cmd, err := GetQryValuesCmd(qry, allowed...)
	return cmd, CheckBadReq(w, err == nil)
}

func CheckQryCmd(w http.ResponseWriter, r *http.Request, allowed ...string) (string, bool) {
	qry, err := url.ParseQuery(r.URL.RawQuery)
	if !CheckBadReq(w, err == nil) {
		return "", false
	}
	return CheckQryValuesCmd(w, qry, allowed...)
}

func ReadJsonRequest(r *http.Request, ip interface{}, maxBody int64) error {
	mr := &MaxReader{Reader: r.Body, Count: maxBody}
	dec := json.NewDecoder(mr)
	dec.UseNumber()
	err := dec.Decode(ip)
	if err != io.EOF && mr.Count <= 0 {
		return RequestBodyTooLarge
	}
	if err != io.EOF && err != nil {
		return InvalidJson
	}
	buf := dec.Buffered()
	n, _ := buf.Read(make([]byte, 1))
	if n != 0 {
		return InvalidJson
	}
	return nil
}

func WriteJsonResponse(w http.ResponseWriter, r *http.Request, d interface{}) (int, error) {
	js, err := json.Marshal(d)
	if err != nil {
		return 0, MarshalError
	}
	h := w.Header()
	h.Set(HeaderContentType, JsonContentType)
	h.Set(HeaderContentLength, fmt.Sprintf("%d", len(js)))
	if r.Method == "HEAD" {
		return 0, nil
	}
	return w.Write(js)
}