package htauth

import (
	"code.google.com/p/tasked/share"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"mime"
	"net/http"
	"strings"
)

const (
	credXHeaderUserKey    = "X-Auth-Username"
	credXHeaderPwdKey     = "X-Auth-Password"
	credXHeaderTokenKey   = "X-Auth-Token"
	credHeaderKey         = "Authorization"
	tokenCookieName       = "tasked-auth"
	basicAuthType         = "Basic"
	userKey               = "username"
	pwdKey                = "password"
	tokenKey              = "token"
	defaultMaxRequestBody = int64(10 << 20)
	mimeForm              = "application/x-www-form-urlencoded"
	mimeMForm             = "multipart/form-data"
	mimeJson              = "application/json"
	mimeText              = "text/plain"
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

type Token interface {
	Value() []byte
}

type Auth interface {
	AuthPwd(string, string) (Token, error)
	AuthToken(Token) (Token, error)
	GetUser(Token) (string, error)
}

type token struct {
	val []byte
}

func newTokenString(s string) (Token, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	return &token{b}, err
}

func (t *token) Value() []byte { return t.val }

type filter struct {
	auth Auth
}

func New(a Auth) (share.HttpFilter, error) {
	if a == nil {
		return nil, authNotSet
	}
	return &filter{auth: a}, nil
}

func (a *filter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Filter(w, r, nil)
}

func (a *filter) Filter(w http.ResponseWriter, r *http.Request, _ interface{}) (interface{}, bool) {
	// if method OPTIONS or HEAD, do nothing
	// check for credentials
	// if credentials, authenticate credentials
	// else check for token
	// if token authenticate
	// if method AUTH, return handled
	// if no credentials and no token, and method AUTH fail
	// if is auth request and no credentials, return 401 with basic authentication request

	var (
		isAuth bool
		err    error
	)
	if r.Method == "OPTIONS" || r.Method == "HEAD" {
		return nil, false
	}
	isAuth, err = isAuthRequest(r)
	if !share.CheckBadReq(w, err == nil) {
		return nil, true
	}
	_, _, _, err = getCreds(r, isAuth)
	if !share.CheckBadReq(w, err == nil) {
		return nil, true
	}
	return nil, false
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
	if len(user) == 0 {
		return "", "", invalidHeader
	}
	pwd, err := getOneOrZero(r.Header, credXHeaderPwdKey)
	return user, pwd, err
}

func getCredsHeader(r *http.Request) (string, string, error) {
	if _, ok := r.Header[credHeaderKey]; !ok {
		return "", "", nil
	}
	h, err := getOneOrZero(r.Header, credHeaderKey)
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

func getCredsXHeaderToken(r *http.Request) (Token, error) {
	if _, ok := r.Header[credXHeaderTokenKey]; !ok {
		return nil, nil
	}
	ts, err := getOneOrZero(r.Header, credXHeaderTokenKey)
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, invalidHeader
	}
	return newTokenString(ts)
}

func getCredsCookie(r *http.Request) (Token, error) {
	var tc *http.Cookie
	for _, c := range r.Cookies() {
		if c.Name != tokenCookieName {
			continue
		}
		if tc != nil {
			return nil, invalidHeader
		}
		tc = c
	}
	if tc == nil || len(tc.Value) == 0 {
		return nil, nil
	}
	return newTokenString(tc.Value)
}

func getCredsForm(r *http.Request) (string, string, Token, error) {
	var (
		user, pwd, ts string
		t             Token
		err           error
	)

	// try parse the form even if it's a GET request
	err = func() error {
		m := r.Method
		r.Method = "POST"
		defer func() {
			r.Method = m
		}()
		return r.ParseMultipartForm(maxRequestBody)
	}()
	if err != nil {
		return "", "", nil, err
	}

	user, err = getOneOrZero(r.PostForm, userKey)
	if err != nil {
		return "", "", nil, err
	}
	pwd, err = getOneOrZero(r.PostForm, pwdKey)
	if err != nil {
		return "", "", nil, err
	}
	ts, err = getOneOrZero(r.PostForm, tokenKey)
	if err != nil {
		return "", "", nil, err
	}
	if len(ts) > 0 {
		t, err = newTokenString(ts)
	}
	return user, pwd, t, err
}

func getCredsJson(r *http.Request) (string, string, Token, error) {
	var (
		user, pwd, ts string
		t             Token
		isDefined     bool
		err           error
		m             map[string]string
	)
	err = share.ReadJsonRequest(r, &m, maxRequestBody)
	if err != nil {
		return "", "", nil, err
	}
	user, isDefined = m[userKey]
	if isDefined && len(user) == 0 {
		return "", "", nil, invalidData
	}
	pwd = m[pwdKey]
	ts, isDefined = m[tokenKey]
	if isDefined {
		if len(ts) == 0 {
			return "", "", nil, invalidData
		}
		t, err = newTokenString(ts)
	}
	return user, pwd, t, err
}

func getCredsText(r *http.Request) (Token, error) {
	tb, err := ioutil.ReadAll(r.Body)
	if err != nil || len(tb) == 0 {
		return nil, err
	}
	return newTokenString(string(tb))
}

func getCredsHeaderToken(r *http.Request) (Token, error) {
	t, err := getCredsXHeaderToken(r)
	if err == nil && t == nil {
		t, err = getCredsCookie(r)
	}
	return t, err
}

func getCreds(r *http.Request, isAuth bool) (string, string, Token, error) {
	user, pwd, err := getCredsXHeaderUser(r)
	if err != nil || len(user) > 0 {
		return user, pwd, nil, err
	}
	user, pwd, err = getCredsHeader(r)
	if err != nil || len(user) > 0 {
		return user, pwd, nil, err
	}
	if !isAuth {
		t, err := getCredsHeaderToken(r)
		return "", "", t, err
	}

	ct, err := getOneOrZero(r.Header, share.HeaderContentType)
	if err != nil {
		return "", "", nil, err
	}
	if len(ct) > 0 {
		if ct, _, err = mime.ParseMediaType(ct); err != nil {
			return "", "", nil, err
		}
	}
	var bt Token
	switch ct {
	case mimeForm, mimeMForm:
		user, pwd, bt, err = getCredsForm(r)
	case mimeJson:
		user, pwd, bt, err = getCredsJson(r)
	}
	if err != nil || len(user) > 0 {
		return user, pwd, nil, err
	}

	t, err := getCredsHeaderToken(r)
	if err != nil || t != nil {
		return "", "", t, err
	}
	if bt != nil {
		return "", "", bt, nil
	}
	if ct == mimeText {
		t, err = getCredsText(r)
	}
	return "", "", t, err
}
