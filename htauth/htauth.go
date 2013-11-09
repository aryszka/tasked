package htauth

import (
	"code.google.com/p/tasked/util"
	"errors"
	"net/http"
)

const cmdAuth = "auth"

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

func (t *token) Value() []byte { return t.val }

type filter struct{
	auth Auth
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

	if r.Method == "OPTIONS" || r.Method == "HEAD" {
		return nil, false
	}
	return nil, false
}

func New(a Auth) (util.HttpFilter, error) {
	if a == nil {
		return nil, errors.New("Auth must be set.")
	}
	var f filter
	f.auth = a
	return &f, nil
}

func isAuthRequest(r *http.Request) {
	if r.Method == "AUTH" {
	}
}

func getCredentials(r *http.Request) {
	// accept:
	// - basic base64 header
	// if method AUTH, GET or POST accept:
	// - body form defined by encoding
	// - body JSON defined by encoding
}
