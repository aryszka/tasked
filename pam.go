package main

import (
	pam "code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"errors"
	"io/ioutil"
)

var authFailed = errors.New("Authentication failed.")

type authSettings struct {
	s      *settings
	aesKey []byte
	aesIv  []byte
}

func (as *authSettings) AesKey() []byte     { return as.aesKey }
func (as *authSettings) AesIv() []byte      { return as.aesIv }
func (as *authSettings) TokenValidity() int { return as.s.sec.tokenValidity }

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

func newAuth(s *settings) (*auth.Type, error) {
	if s == nil {
		s = &settings{}
	}
	as := &authSettings{}
	as.s = s
	if len(s.sec.aes.keyFile) > 0 {
		b, err := ioutil.ReadFile(s.sec.aes.keyFile)
		if err != nil {
			return nil, err
		}
		as.aesKey = b
	}
	if len(s.sec.aes.ivFile) > 0 {
		b, err := ioutil.ReadFile(s.sec.aes.ivFile)
		if err != nil {
			return nil, err
		}
		as.aesIv = b
	}
	return auth.New(auth.PasswordCheckerFunc(authPam), as)
}
