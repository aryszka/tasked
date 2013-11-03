package main

import (
	"code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"code.google.com/p/tasked/htfile"
	"code.google.com/p/tasked/util"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type authSettings struct {
	s      *settings
	aesKey []byte
	aesIv  []byte
}

func newAuthSettings(s *settings) (*authSettings, error) {
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
	return as, nil
}

func (as *authSettings) AesKey() []byte     { return as.aesKey }
func (as *authSettings) AesIv() []byte      { return as.aesIv }
func (as *authSettings) TokenValidity() int { return as.s.sec.tokenValidity }

var authFailed = errors.New("Authentication failed.")

func getHttpDir(s *settings) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return util.Abspath(s.files.root, wd), nil
}

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

func main() {
	s, err := getSettings()
	if err != nil {
		log.Panic(err)
	}

	as, err := newAuthSettings(s)
	if err != nil {
		log.Panic(err)
	}
	err = auth.Init(as, auth.AuthFunc(authPam))
	if err != nil {
		log.Panic(err)
	}

	dn, err := getHttpDir(s)
	if err != nil {
		log.Panicln(err)
	}
	util.EnsureDir(dn)
	ht := htfile.New(dn, s)

	l, err := listen(s)
	if err != nil {
		log.Panic(err)
	}
	defer util.Doretlog42(l.Close)

	log.Panic(http.Serve(l, ht))
}
