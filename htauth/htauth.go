package htauth

import (
	"bytes"
	"code.google.com/p/tasked/share"
	"encoding/base64"
	"errors"
	"mime"
	"net/http"
	"strings"
	"log"
)

const (
	credXHeaderUserKey    = "X-Auth-Username"
	credXHeaderPwdKey     = "X-Auth-Password"
	credXHeaderTokenKey   = "X-Auth-Token"
	credHeaderUserKey     = "Authorization"
	tokenCookieName       = "tasked-auth"
	basicAuthType         = "Basic"
	userKey               = "username"
	pwdKey                = "password"
	tokenKey              = "token"
	defaultMaxRequestBody = int64(10 << 20)
	mimeJson              = "application/json"
)

var (
	authNotSet              = errors.New("Auth must be set.")
	noAlternativeCmdAllowed = errors.New("No alternative command allowed.")
	noAuthWithThisMethod    = errors.New("No auth command allowed with this method.")
	onlyOneItemAllowed      = errors.New("Only one item allowed.")
	invalidHeader           = errors.New("Invalid authorization header.")
	invalidData             = errors.New("Invalid data.")
	maxRequestBody          = defaultMaxRequestBody
)

type Auth interface {
	AuthPwd(string, string) ([]byte, error)
	AuthToken([]byte) ([]byte, string, error)
}

type Settings interface {
	AllowCookies() bool
	CookieMaxAge() int
}

func newTokenString(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

type filter struct {
	auth         Auth
	allowCookies bool
	cookieMaxAge int
}

func New(a Auth, s Settings) (share.HttpFilter, error) {
	if a == nil {
		return nil, authNotSet
	}
	ha := filter{auth: a}
	if s != nil {
		ha.allowCookies = s.AllowCookies()
	}
	return &ha, nil
}

func isAuthRequest(r *http.Request) (bool, error) {
	cmd, err := share.GetQryCmd(r, share.CmdAll)
	if err != nil {
		return false, err
	}
	switch r.Method {
	case "AUTH":
		if len(cmd) > 0 {
			return false, noAlternativeCmdAllowed
		}
		return true, nil
	case "GET", "POST":
		return cmd == share.CmdAuth, nil
	default:
		if cmd == share.CmdAuth {
			return false, noAuthWithThisMethod
		}
		return false, nil
	}
}

func getOneOrZero(m map[string][]string, k string) (string, error) {
	if m == nil {
		return "", nil
	}
	vs, ok := m[k]
	if !ok || len(vs) == 0 {
		return "", nil
	}
	if len(vs) > 1 {
		return "", onlyOneItemAllowed
	}
	return vs[0], nil
}

func getCredsXHeaderUser(r *http.Request) (string, string, error) {
	if _, ok := r.Header[credXHeaderUserKey]; !ok {
		return "", "", nil
	}
	user, err := getOneOrZero(r.Header, credXHeaderUserKey)
	if err != nil {
		return "", "", err
	}
	if user == "" {
		return "", "", invalidHeader
	}
	pwd, err := getOneOrZero(r.Header, credXHeaderPwdKey)
	return user, pwd, err
}

func getCredsHeaderUser(r *http.Request) (string, string, error) {
	if _, ok := r.Header[credHeaderUserKey]; !ok {
		return "", "", nil
	}
	h, err := getOneOrZero(r.Header, credHeaderUserKey)
	if err != nil {
		return "", "", err
	}
	ps := strings.Split(h, " ")
	if len(ps) != 2 || ps[0] != basicAuthType {
		return "", "", invalidHeader
	}
	css, err := base64.StdEncoding.DecodeString(ps[1])
	if err != nil {
		return "", "", err
	}
	cs := strings.Split(string(css), ":")
	if len(cs) != 2 || len(cs[0]) == 0 {
		return "", "", invalidHeader
	}
	return cs[0], cs[1], nil
}

func getCredsXHeaderToken(r *http.Request) (string, error) {
	if _, ok := r.Header[credXHeaderTokenKey]; !ok {
		return "", nil
	}
	ts, err := getOneOrZero(r.Header, credXHeaderTokenKey)
	if err != nil {
		return "", err
	}
	if ts == "" {
		err = invalidHeader
	}
	return ts, err
}

func getCredsCookie(r *http.Request) (string, error) {
	var tc *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name != tokenCookieName {
			continue
		}
		if tc != nil {
			return "", invalidHeader
		}
		tc = c
	}
	var v string
	if tc != nil {
		v = tc.Value
	}
	return v, nil
}

func getCredsJson(r *http.Request) (user, pwd, t string, err error) {
	var m map[string]string
	err = share.ReadJsonRequest(r, &m, maxRequestBody)
	if err != nil {
		return
	}
	user, isDefined := m[userKey]
	if isDefined && user == "" {
		err = invalidData
		return
	}
	pwd = m[pwdKey]
	t, isDefined = m[tokenKey]
	if isDefined && t == "" {
		err = invalidData
	}
	return user, pwd, t, err
}

func (a *filter) getCreds(r *http.Request, isAuth bool) (user, pwd, t string, err error) {
	user, pwd, err = getCredsXHeaderUser(r)
	if err != nil || user != "" {
		return
	}

	user, pwd, err = getCredsHeaderUser(r)
	if err != nil || user != "" {
		return
	}

	if !isAuth {
		t, err = getCredsXHeaderToken(r)
		if err != nil || t != "" {
			return
		}
		if a.allowCookies {
			t, err = getCredsCookie(r)
		}
		return
	}

	t, err = getCredsXHeaderToken(r)
	if err != nil || t != "" {
		return
	}

	ct, err := getOneOrZero(r.Header, share.HeaderContentType)
	if err != nil {
		return
	}
	if ct != "" {
		if ct, _, err = mime.ParseMediaType(ct); err != nil {
			return
		}
	}
	if ct == mimeJson {
		user, pwd, t, err = getCredsJson(r)
	}
	if err != nil || user != "" || t != "" {
		return
	}

	t, err = getCredsCookie(r)
	return
}

func (a *filter) checkCreds(user, pwd string, t []byte) ([]byte, string, error) {
	if len(user) > 0 {
		t, err := a.auth.AuthPwd(user, pwd)
		return t, user, err
	}
	if t != nil {
		return a.auth.AuthToken(t)
	}
	return nil, "", nil
}

func (a *filter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, h := a.Filter(w, r, nil)
	us, ok := u.(string)
	share.CheckHandle(w, h || ok && us != "", http.StatusNotFound)
}

func (a *filter) Filter(w http.ResponseWriter, r *http.Request, _ interface{}) (interface{}, bool) {
	if r.Method == "OPTIONS" || r.Method == "HEAD" {
		return nil, false
	}

	isAuth, err := isAuthRequest(r)
	if !share.CheckBadReq(w, err == nil) {
		return nil, true
	}

	user, pwd, ts, err := a.getCreds(r, isAuth)
	if !share.CheckBadReq(w, err == nil) {
		return nil, true
	}

	var tp []byte
	if user == "" && ts != "" {
		tp, err = newTokenString(ts)
		if !share.CheckBadReq(w, err == nil) {
			return nil, true
		}
	}

	tn, user, err := a.checkCreds(user, pwd, tp)
	if err != nil || len(tn) == 0 || user == "" {
		if isAuth {
			share.ErrorResponse(w, http.StatusNotFound)
		}
		log.Println("here", len(tn), len(user), err)
		return nil, isAuth
	}

	isNew := len(tp) == 0 || !bytes.Equal(tn, tp)
	if isNew {
		ts = base64.StdEncoding.EncodeToString(tn)
	}

	h := w.Header()
	h.Set(credXHeaderTokenKey, ts)
	if a.allowCookies {
		http.SetCookie(w, &http.Cookie{
			Name:   tokenCookieName,
			Value:  ts,
			MaxAge: a.cookieMaxAge})
	}
	if isAuth {
		w.Write([]byte(ts))
	}

	return user, isAuth
}
