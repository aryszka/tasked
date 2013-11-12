package htauth

import (
	"bytes"
	"code.google.com/p/tasked/share"
	tst "code.google.com/p/tasked/testing"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
)

func TestNewTokenString(t *testing.T) {
	tk, err := newTokenString("")
	if err != nil || t == nil || len(tk.Value()) != 0 {
		t.Fail()
	}
	tk, err = newTokenString("not base64")
	if err == nil {
		t.Fail()
	}
	b := []byte("some")
	tk, err = newTokenString(base64.StdEncoding.EncodeToString(b))
	if err != nil || !bytes.Equal(tk.Value(), b) {
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

func TestGetCredsHeader(t *testing.T) {
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
		user, pwd, err = getCredsHeader(r)
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
	r.Header.Add(credHeaderKey, valid)
	r.Header.Add(credHeaderKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, basicAuthType)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, "some "+enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, basicAuthType+" notbase64")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, basicAuthType+" "+invalidEnced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, nouser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != invalidHeader {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(credHeaderKey, nopwd)
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
		tk    Token
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
		if err != nil || tk != nil {
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
		if err != nil || tk == nil {
			t.Fail()
		}
	})
}

func TestGetCredsCookie(t *testing.T) {
	var (
		tk      Token
		err     error
		r       *http.Request
		v       = []byte("some")
		valid   = base64.StdEncoding.EncodeToString(v)
		invalid = "not+base64"
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		tk, err = getCredsCookie(r)
	}

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != nil {
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
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: invalid})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Log("here")
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: valid})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), v) {
			t.Fail()
		}
	})
}

func TestGetCredsForm(t *testing.T) {
	var (
		user, pwd string
		tk        Token
		err       error
		tuser     = "user"
		tpwd      = "pwd"
		bValid    = []byte("some")
		tkValid   = base64.StdEncoding.EncodeToString(bValid)
		tkInvalid = "not+base64"
		r         *http.Request
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		m := r.Method
		user, pwd, tk, err = getCredsForm(r)
		if r.Method != m {
			t.Fail()
		}
	}

	r, err = http.NewRequest("POST", tst.S.URL, bytes.NewBufferString("%%"))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("POST", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s", userKey, tuser, userKey, tuser)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("POST", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s", pwdKey, tpwd, pwdKey, tpwd)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("POST", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", userKey, tuser)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || len(pwd) != 0 {
			t.Fail()
		}
	})
	r, err = http.NewRequest("POST", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s", userKey, tuser, pwdKey, tpwd)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd {
			t.Fail()
		}
	})
	r, err = http.NewRequest("GET", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s", tokenKey, tkValid, tokenKey, tkValid)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("GET", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", tokenKey, tkInvalid)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("GET", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", tokenKey, tkValid)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), bValid) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("GET", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, tuser, pwdKey, tpwd, tokenKey, tkValid)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser || pwd != tpwd || !bytes.Equal(tk.Value(), bValid) {
			t.Fail()
		}
	})
}

func TestGetCredsJson(t *testing.T) {
	var (
		user, pwd string
		tk        Token
		err       error
		tuser     = "user"
		tpwd      = "pwd"
		bValid    = []byte("some")
		tkValid   = base64.StdEncoding.EncodeToString(bValid)
		tkInvalid = "not+base64"
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
		if err != nil || len(user) != 0 || len(pwd) != 0 || tk != nil {
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
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, tkInvalid)),
		func(rsp *http.Response) {
			if err == nil {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, tkValid)),
		func(rsp *http.Response) {
			if err != nil || !bytes.Equal(tk.Value(), bValid) {
				t.Fail()
			}
		})
	tst.Htreq(t, "AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, tuser, pwdKey, tpwd, tokenKey, tkValid)),
		func(rsp *http.Response) {
			if err != nil || user != tuser || pwd != tpwd || !bytes.Equal(tk.Value(), bValid) {
				t.Fail()
			}
		})
}

func TestGetCredsText(t *testing.T) {
	var (
		tk        Token
		err       error
		bValid    = []byte("some")
		tkValid   = base64.StdEncoding.EncodeToString(bValid)
		tkInvalid = "not+base64"
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		tk, err = getCredsText(r)
	}

	tst.Htreq(t, "AUTH", tst.S.URL, nil, func(rsp *http.Response) {
		if err != nil || tk != nil {
			t.Fail()
		}
	})
	tst.Htreq(t, "AUTH", tst.S.URL, bytes.NewBufferString(tkInvalid), func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	tst.Htreq(t, "AUTH", tst.S.URL, bytes.NewBufferString(tkValid), func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), bValid) {
			t.Fail()
		}
	})
}

func TestGetCredsHeaderToken(t *testing.T) {
	var (
		tk       Token
		err      error
		r        *http.Request
		bValid0  = []byte("some0")
		tkValid0 = base64.StdEncoding.EncodeToString(bValid0)
		bValid1  = []byte("some1")
		tkValid1 = base64.StdEncoding.EncodeToString(bValid1)
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		tk, err = getCredsHeaderToken(r)
	}

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk != nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, tkValid0)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), bValid0) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, tkValid0)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: tkValid1})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), bValid0) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: tkValid1})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), bValid1) {
			t.Fail()
		}
	})
}

func TestGetCredsValidation(t *testing.T) {
	var (
		isAuth       bool
		user, pwd    string
		tk           Token
		err          error
		r            *http.Request
		tuser        = "user"
		tpwd         = "pwd"
		notbase64    = "not+base64"
		val          = []byte(tuser + ":" + tpwd)
		enced        = base64.StdEncoding.EncodeToString(val)
		invalidEnced = base64.StdEncoding.EncodeToString([]byte(tuser + "-" + tpwd))
		valid        = basicAuthType + " " + enced
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = getCreds(r, isAuth)
	}

	isAuth = false

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderUserKey, "")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderUserKey, tuser)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credHeaderKey, invalidEnced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credHeaderKey, valid)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderTokenKey, notbase64)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})

	isAuth = true

	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Add(share.HeaderContentType, "application/json")
	r.Header.Add(share.HeaderContentType, "application/json")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Add(share.HeaderContentType, "no mime")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, bytes.NewBufferString("%%"))
	r.Header.Add(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", userKey, tuser)))
	r.Header.Add(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, bytes.NewBufferString("%%"))
	r.Header.Add(share.HeaderContentType, mimeMForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, bytes.NewBufferString("%%"))
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", userKey, tuser)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != tuser {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderTokenKey, notbase64)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Log("here")
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), val) {
			t.Log(err)
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, enced)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, bytes.NewBufferString(enced))
	r.Header.Add(share.HeaderContentType, mimeText)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})
}

func TestGetCredsPrecedence(t *testing.T) {
	var (
		isAuth    bool
		user, pwd string
		tk        Token
		err       error
		r         *http.Request
		userxh    = "userxh"
		pwdxh     = "pwdxh"
		userh     = "userh"
		pwdh      = "pwdh"
		userf     = "userf"
		pwdf      = "pwdf"
		userj     = "userj"
		pwdj      = "pwdj"
		txh       = []byte("txh")
		tc        = []byte("tc")
		tf        = []byte("tf")
		tj        = []byte("tj")
		tt        = []byte("tt")
		encedx    = base64.StdEncoding.EncodeToString(txh)
		encedh    = base64.StdEncoding.EncodeToString([]byte(userh + ":" + pwdh))
		encedf    = base64.StdEncoding.EncodeToString(tf)
		encedj    = base64.StdEncoding.EncodeToString(tj)
		encedt    = base64.StdEncoding.EncodeToString(tt)
		validh    = basicAuthType + " " + encedh
		encedc    = base64.StdEncoding.EncodeToString(tc)
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = getCreds(r, isAuth)
	}

	isAuth = false

	// x header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Set(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Set(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})

	// header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})

	// x header token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})

	// cookie token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})

	// none
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})

	isAuth = true

	// x header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderUserKey, userxh)
	r.Header.Set(credXHeaderPwdKey, pwdxh)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userxh || pwd != pwdxh {
			t.Fail()
		}
	})

	// header user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.Header.Set(credHeaderKey, validh)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userh || pwd != pwdh {
			t.Fail()
		}
	})

	// form user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s&%s=%s&%s=%s",
			userKey, userf, pwdKey, pwdf, tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userf || pwd != pwdf {
			t.Fail()
		}
	})

	// json user
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\",\"%s\":\"%s\",\"%s\":\"%s\"}",
			userKey, userj, pwdKey, pwdj, tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || user != userj || pwd != pwdj {
			t.Fail()
		}
	})

	// x header token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.Header.Set(credXHeaderTokenKey, encedx)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
			t.Fail()
		}
	})

	// cookie token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})

	// form token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("%s=%s", tokenKey, encedf)))
	r.Header.Set(share.HeaderContentType, mimeForm)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tf) {
			t.Fail()
		}
	})

	// json token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, encedj)))
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tj) {
			t.Fail()
		}
	})

	// text token
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(encedt))
	r.Header.Add(share.HeaderContentType, mimeText)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tt) {
			t.Fail()
		}
	})

	// none
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})
}
