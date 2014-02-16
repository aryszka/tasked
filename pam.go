package main

import (
	pam "code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"errors"
)

var authFailed = errors.New("Authentication failed.")

type authSettings struct {
	s      *settings
	aesKey string
	aesIv  string
}

func (as *authSettings) AesKey() string     { return as.aesKey }
func (as *authSettings) AesIv() string      { return as.aesIv }
func (as *authSettings) TokenValidity() int { return as.s.TokenValidity() }

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

func newAuth(s *settings) (*auth.It, error) {
	if s == nil {
		s = &settings{}
	}
	as := &authSettings{}
	as.s = s
	as.aesKey = s.AesKey()
	as.aesIv = s.AesIv()
	// return auth.New(auth.PasswordCheckerFunc(authPam), as)
	return nil, nil
}
