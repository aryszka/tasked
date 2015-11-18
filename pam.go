package main

import (
	pam "code.google.com/p/gopam"
	"github.com/aryszka/tasked/auth"
)

type authOptions struct {
	aesKey        []byte
	aesIv         []byte
	tokenValidity int
}

func (ao *authOptions) AesKey() []byte     { return ao.aesKey }
func (ao *authOptions) AesIv() []byte      { return ao.aesIv }
func (ao *authOptions) TokenValidity() int { return ao.tokenValidity }

func authPam(user, pwd string) bool {
	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF, pam.PROMPT_ECHO_ON:
			return pwd, true
		default:
			return "", false
		}
	}))
	if s != pam.SUCCESS {
		return false
	}
	defer t.End(s)
	s = t.Authenticate(0)
	return s == pam.SUCCESS
}

func mkauth(o *options) (*auth.It, error) {
	aesKey, err := o.AesKey()
	if err != nil {
		return nil, err
	}
	aesIv, err := o.AesIv()
	if err != nil {
		return nil, err
	}
	if len(aesKey) == 0 && len(aesIv) == 0 {
		var err error
		aesKey, aesIv, err = genAes()
		if err != nil {
			return nil, err
		}
	}
	ao := new(authOptions)
	ao.aesKey = aesKey
	ao.aesIv = aesIv
	ao.tokenValidity = o.TokenValidity()
	cp := auth.PasswordCheckerFunc(authPam)
	return auth.New(cp, ao), nil
}
