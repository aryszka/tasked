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
	r.Header.Add(credHeaderUserKey, basicAuthType+" notbase64")
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
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
	r.Header.Set(credXHeaderTokenKey, notbase64)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
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
	r.Header.Set(credXHeaderTokenKey, notbase64)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err == nil {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Set(credXHeaderTokenKey, enced)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL,
		bytes.NewBufferString(fmt.Sprintf("{\"%s\":\"%s\"}", tokenKey, enced)))
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || tk == nil || !bytes.Equal(tk.Value(), val) {
			t.Fail()
		}
	})
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
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
	)
	tst.Thnd.Sh = func(_ http.ResponseWriter, r *http.Request) {
		user, pwd, tk, err = getCreds(r, isAuth)
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
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
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
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
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
		if err != nil || len(user) != 0 || tk != nil {
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
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), txh) {
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
			t.Log("here")
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
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tj) {
			t.Fail()
		}
	})

	// cookie token
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	r.Header.Add(share.HeaderContentType, mimeJson)
	r.AddCookie(&http.Cookie{Name: tokenCookieName, Value: encedc})
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk == nil || !bytes.Equal(tk.Value(), tc) {
			t.Fail()
		}
	})

	// none
	r, err = http.NewRequest("AUTH", tst.S.URL, nil)
	tst.ErrFatal(t, err)
	tst.Htreqr(t, r, func(rsp *http.Response) {
		if err != nil || len(user) != 0 || tk != nil {
			t.Fail()
		}
	})
}
