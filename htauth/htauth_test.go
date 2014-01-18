package htauth

import (
	"bytes"
	"code.google.com/p/tasked/share"
	tst "code.google.com/p/tasked/testing"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"io/ioutil"
)

const autoUser = "auto user"

var authError = errors.New("auth error")

type auth int

func (a *auth) AuthPwd(user, pwd string) ([]byte, error) {
	if user == "" {
		return nil, authError
	}
	if user == pwd {
		return []byte("123"), nil
	}
	return nil, authError
}

func (a *auth) AuthToken(t []byte) ([]byte, string, error) {
	if t == nil {
		return nil, "", authError
	}
	if bytes.Equal(t, []byte("123")) {
		return t, autoUser, nil
	}
	if bytes.Equal(t, []byte("456")) {
		return []byte("123"), autoUser, nil
	}
	return nil, "", authError
}

func TestNewTokenString(t *testing.T) {
	tk, err := newTokenString("")
	if err != nil || t == nil || len(tk) != 0 {
		t.Fail()
	}
	tk, err = newTokenString("not base64")
	if err == nil {
		t.Fail()
	}
	b := []byte("some")
	tk, err = newTokenString(base64.StdEncoding.EncodeToString(b))
	if err != nil || !bytes.Equal(tk, b) {
		t.Fail()
	}
}

func TestIsAuthRequest(t *testing.T) {
	var (
		isAuth bool
		err    error
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		isAuth, err = isAuthRequest(r)
	}

	tst.Htreq(t, "AUTH", tst.S.URL+"?cmd=custom", nil, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	tst.Htreq(t, "AUTH", tst.S.URL+"?cmd="+share.CmdProps, nil, func(rsp *http.Response) {
		if err != noAlternativeCmdAllowed {
			t.Fail()
		}
	})
	tst.Htreq(t, "AUTH", tst.S.URL, nil, func(rsp *http.Response) {
		if err != nil || !isAuth {
			t.Fail()
		}
	})
	tst.Htreq(t, "GET", tst.S.URL+"?cmd="+share.CmdProps, nil, func(rsp *http.Response) {
		if err != nil || isAuth {
			t.Fail()
		}
	})
	tst.Htreq(t, "GET", tst.S.URL+"?cmd="+share.CmdAuth, nil, func(rsp *http.Response) {
		if err != nil || !isAuth {
			t.Fail()
		}
	})
	tst.Htreq(t, "PUT", tst.S.URL+"?cmd="+share.CmdAuth, nil, func(rsp *http.Response) {
		if err != noAuthWithThisMethod {
			t.Fail()
		}
	})
	tst.Htreq(t, "PUT", tst.S.URL+"?cmd="+share.CmdProps, nil, func(rsp *http.Response) {
		if err != nil {
			t.Fail()
		}
	})
	tst.Htreq(t, "PUT", tst.S.URL, nil, func(rsp *http.Response) {
		if err != nil {
			t.Fail()
		}
	})
}

func TestGetOneOrZero(t *testing.T) {
	var (
		it  string
		err error
	)

	it, err = getOneOrZero(nil, "key")
	if err != nil || len(it) != 0 {
		t.Fail()
	}
	it, err = getOneOrZero(map[string][]string{"some": []string{"val"}}, "key")
	if err != nil || len(it) != 0 {
		t.Fail()
	}
	it, err = getOneOrZero(map[string][]string{"key": []string{}}, "key")
	if err != nil || len(it) != 0 {
		t.Fail()
	}
	it, err = getOneOrZero(map[string][]string{"key": []string{"val0", "val1"}}, "key")
	if err != onlyOneItemAllowed {
		t.Fail()
	}
	it, err = getOneOrZero(map[string][]string{"key": []string{"val"}}, "key")
	if err != nil || it != "val" {
		t.Fail()
	}
}

func TestGetCredsXHeaderUser(t *testing.T) {
	var (
		user, pwd string
		err       error
		tuser     = "user"
		tpwd      = "pwd"
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, err = getCredsXHeaderUser(r)
	}

	r, err := http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || len(pwd) != 0 {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, tuser)
	r.Header.Add(credXHeaderUserKey, tuser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, "")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, tuser)
	r.Header.Add(credXHeaderPwdKey, tpwd)
	r.Header.Add(credXHeaderPwdKey, tpwd)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, tuser)
	r.Header.Add(credXHeaderPwdKey, tpwd)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, tuser)
	r.Header.Add(credXHeaderPwdKey, "")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != "" {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderUserKey, tuser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != "" {
			t.Fail()
		}
	})
}

func TestGetCredsHeaderUser(t *testing.T) {
	var (
		user, pwd    string
		err          error
		tuser        = "user"
		tpwd         = "pwd"
		invalidEnced = base64.StdEncoding.EncodeToString([]byte(tuser + "-" + tpwd))
		enced        = base64.StdEncoding.EncodeToString([]byte(tuser + ":" + tpwd))
		valid        = basicAuthType + " " + enced
		nouser       = basicAuthType + " " + base64.StdEncoding.EncodeToString([]byte(":"+tpwd))
		nopwd        = basicAuthType + " " + base64.StdEncoding.EncodeToString([]byte(tuser+":"))
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, err = getCredsHeaderUser(r)
	}

	r, err := http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if len(user) != 0 || len(pwd) != 0 || err != nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, valid)
	r.Header.Add(credHeaderUserKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, basicAuthType)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, "some "+enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, basicAuthType+" "+invalidEnced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, nouser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderUserKey, nopwd)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != "" {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.SetBasicAuth(tuser, tpwd)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd {
			t.Fail()
		}
	})
}

func TestGetCredsXHeaderToken(t *testing.T) {
	var (
		tk    string
		err   error
		r     *http.Request
		valid = base64.StdEncoding.EncodeToString([]byte("some"))
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		tk, err = getCredsXHeaderToken(r)
	}

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != "" {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderTokenKey, valid)
	r.Header.Add(credXHeaderTokenKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderTokenKey, "")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credXHeaderTokenKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == "" {
			t.Fail()
		}
	})
}

func TestGetCredsCookie(t *testing.T) {
	var (
		tk    string
		err   error
		r     *http.Request
		v     = []byte("some")
		valid = base64.StdEncoding.EncodeToString(v)
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		tk, err = getCredsCookie(r)
	}

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != "" {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: valid})
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: valid})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: valid})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != valid {
			t.Fail()
		}
	})
}

func TestGetCredsJson(t *testing.T) {
	var (
		user, pwd string
		tk        string
		err       error
		tuser     = "user"
		tpwd      = "pwd"
		bValid    = []byte("some")
		tkValid   = base64.StdEncoding.EncodeToString(bValid)
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = getCredsJson(r)
	}

	func() {
		mrb := maxRequestBody
		maxRequestBody = 4
		defer func() {
			maxRequestBody = mrb
		}()
		tst.Htreq(t, "AUTH", tst.S.URL, bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", userKey, tuser)),
			func(rsp *http.Response) {
				if err == nil {
					t.Fail()
				}
			})
	}()
	tst.Htreq(t, "AUTH", tst.S.URL, nil, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || len(pwd) != 0 || tk != "" {
			t.Fail()
		}
	})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"\"}", userKey)),
		func(rsp *http.Response) {
			if err != invalidData {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", userKey, tuser)),
		func(rsp *http.Response) {
			if err != nil || user != tuser || len(pwd) != 0 {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\", \"%s\":\"%s\"}", userKey, tuser, pwdKey, tpwd)),
		func(rsp *http.Response) {
			if err != nil || user != tuser || pwd != tpwd {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"\"}", tokenKey)),
		func(rsp *http.Response) {
			if err != invalidData {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, tkValid)),
		func(rsp *http.Response) {
			if err != nil || tk != tkValid {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, tuser, pwdKey, tpwd, tokenKey, tkValid)),
		func(rsp *http.Response) {
			if err != nil || user != tuser || pwd != tpwd || tk != tkValid {
				t.Fail()
			}
		})
}

func TestGetCredsValidation(t *testing.T) {
	var (
		isAuth       bool
		user, pwd    string
		tk           string
		err          error
		r            *http.Request
		tuser        = "user"
		tpwd         = "pwd"
		val          = []byte(tuser + ":" + tpwd)
		enced        = base64.StdEncoding.EncodeToString(val)
		invalidEnced = base64.StdEncoding.EncodeToString([]byte(tuser + "-" + tpwd))
		valid        = basicAuthType + " " + enced
		a            = &filter{allowCookies: true}
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = a.getCreds(r, isAuth)
	}

	isAuth = false

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderUserKey, "")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderUserKey, tuser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credHeaderUserKey, invalidEnced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credHeaderUserKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != enced {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != enced {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk != "" {
			t.Fail()
		}
	})

	isAuth = true

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, bytes.NewBufferString("%%"))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", userKey, tuser)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != enced {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, enced)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == "" || tk != enced {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != "" {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: enced})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != enced {
			t.Fail()
		}
	})
}

func TestGetCredsPrecedence(t *testing.T) {
	var (
		isAuth    bool
		user, pwd string
		tk        string
		err       error
		r         *http.Request
		userxh    = "userxh"
		pwdxh     = "pwdxh"
		userh     = "userh"
		pwdh      = "pwdh"
		userj     = "userj"
		pwdj      = "pwdj"
		txh       = []byte("txh")
		tc        = []byte("tc")
		tj        = []byte("tj")
		encedx    = base64.StdEncoding.EncodeToString(txh)
		encedh    = base64.StdEncoding.EncodeToString([]byte(userh + ":" + pwdh))
		encedj    = base64.StdEncoding.EncodeToString(tj)
		validh    = basicAuthType + " " + encedh
		encedc    = base64.StdEncoding.EncodeToString(tc)
		a         = &filter{allowCookies: true}
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = a.getCreds(r, isAuth)
	}

	isAuth = false

	// x header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Set(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderUserKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})

	// header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderUserKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})

	// x header token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == "" || tk != encedx {
			t.Fail()
		}
	})

	// cookie token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == "" || tk != encedc {
			t.Fail()
		}
	})

	// none
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk != "" {
			t.Fail()
		}
	})

	isAuth = true

	// x header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderUserKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})

	// header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderUserKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})

	// x header token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk == "" || tk != encedx {
			t.Fail()
		}
	})

	// json user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userj || pwd != pwdj {
			t.Fail()
		}
	})

	// json token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, encedj)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk == "" || tk != encedj {
			t.Fail()
		}
	})

	// cookie token
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk == "" || tk != encedc {
			t.Fail()
		}
	})

	// none
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != "" || tk != "" {
			t.Fail()
		}
	})
}

func TestCheckCreds(t *testing.T) {
	a := &filter{auth: new(auth)}

	tk, u, err := a.checkCreds("123", "123", nil)
	if err != nil || tk == nil || u != "123" {
		t.Fail()
	}

	tk, u, err = a.checkCreds("123", "456", nil)
	if err == nil {
		t.Fail()
	}

	tk, _, err = a.checkCreds("", "", []byte("123"))
	if err != nil || tk == nil || !bytes.Equal(tk, []byte("123")) {
		t.Fail()
	}

	tk, _, err = a.checkCreds("", "", []byte("456"))
	if err != nil || tk == nil || !bytes.Equal(tk, []byte("123")) {
		t.Fail()
	}

	tk, _, err = a.checkCreds("", "", []byte("789"))
	if err == nil {
		t.Fail()
	}

	tk, _, err = a.checkCreds("", "", nil)
	if err != nil || tk != nil {
		t.Fail()
	}
}

func TestServeHTTP(t *testing.T) {
	var (
		a = &filter{auth: new(auth)}
	)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		a.ServeHTTP(w, r)
	}

	rq, err := http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderUserKey, "123")
	rq.Header.Set(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderUserKey, "123")
	rq.Header.Set(credXHeaderPwdKey, "456")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderUserKey, "123")
	rq.Header.Set(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderUserKey, "123")
	rq.Header.Set(credXHeaderPwdKey, "456")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderTokenKey, "not base64")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Set(credXHeaderTokenKey, "not base64")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestFilter(t *testing.T) {
	var (
		a    = &filter{auth: new(auth)}
		res  interface{}
		h    bool
		t123 = base64.StdEncoding.EncodeToString([]byte("123"))
		t456 = base64.StdEncoding.EncodeToString([]byte("456"))
	)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		res, h = a.Filter(w, r, nil)
	}

	tst.Htreq(t, "OPTIONS", tst.S.URL, nil, func(rsp *http.Response) {
		if res != nil || h {
			t.Fail()
		}
	})

	tst.Htreq(t, "HEAD", tst.S.URL, nil, func(rsp *http.Response) {
		if res != nil || h {
			t.Fail()
		}
	})

	tst.Htreq(t, "AUTH", tst.S.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if res != nil || !h || rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	rq, err := http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "user")
	rq.Header.Add(credXHeaderUserKey, "user")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || !h || rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderTokenKey, "not base64")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || !h || rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "456")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || h {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "456")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || !h || rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderTokenKey, base64.StdEncoding.EncodeToString([]byte("789")))
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || h {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderTokenKey, base64.StdEncoding.EncodeToString([]byte("789")))
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != nil || !h || rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderTokenKey, t123)
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		tks := rsp.Header[credXHeaderTokenKey]
		if len(tks) != 1 || tks[0] != t123 {
			t.Log("so here", len(tks))
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderTokenKey, t456)
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		tks := rsp.Header[credXHeaderTokenKey]
		if len(tks) != 1 || tks[0] != t123 {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		tks := rsp.Header[credXHeaderTokenKey]
		if len(tks) != 1 || tks[0] != t123 {
			t.Fail()
		}
	})

	a.allowCookies = false
	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		tks := rsp.Header[credXHeaderTokenKey]
		if len(tks) != 1 || tks[0] != t123 {
			t.Fail()
		}
		for _, c := range rsp.Cookies() {
			if c.Name == tokenCookieName && c.Value == t123 {
				t.Fail()
				break
			}
		}
	})

	a.allowCookies = true
	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		tks := rsp.Header[credXHeaderTokenKey]
		if len(tks) != 1 || tks[0] != t123 {
			t.Fail()
		}
		var found bool
		for _, c := range rsp.Cookies() {
			if c.Name == tokenCookieName && c.Value == t123 {
				found = true
				break
			}
		}
		if !found {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("GET", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != "123" || h {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if len(b) > 0 {
			t.Fail()
		}
	})

	rq, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	rq.Header.Add(credXHeaderUserKey, "123")
	rq.Header.Add(credXHeaderPwdKey, "123")
	tst.Htreqr(t, rq, func(rsp *http.Response) {
		if res != "123" || !h {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(b, []byte(t123)) {
			t.Fail()
		}
	})
}
