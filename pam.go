package main

import (
	pam "code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"errors"
)

var authFailed = errors.New("Authentication failed.")

type authOptions struct {
	o      *options
	aesKey []byte
	aesIv  []byte
}

func (ao *authOptions) AesKey() []byte     { return ao.aesKey }
func (ao *authOptions) AesIv() []byte      { return ao.aesIv }
func (ao *authOptions) TokenValidity() int { return ao.o.TokenValidity() }

func authPam(user, pwd string) error {
	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF, pam.PROMPT_ECHO_ON:
			return pwd, true
		default:
			return "", false
		}
	}))
	if s != pam.SUCCESS {
		return authFailed
	}
	defer t.End(s)

	s = t.Authenticate(0)
	if s != pam.SUCCESS {
		return authFailed
	}
	return nil
}

func newAuth(o *options) (*auth.It, error) {
	if o == nil {
		o = &options{}
	}
	ao := &authOptions{}
	ao.o = o
	var err error
	ao.aesKey, err = o.AesKey()
	if err != nil {
		return nil, err
	}
	ao.aesIv, err = o.AesIv()
	if err != nil {
		return nil, err
	}
	// return auth.New(auth.PasswordCheckerFunc(authPam), ao)
	return nil, nil
}
