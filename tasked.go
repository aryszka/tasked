package main

import (
	"code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"errors"
	"log"
	"os"
	"path"
)

const (
	defaultTestdir = "test"
	testdirKey     = "testdir"
)

func getHttpDir() string {
	dn := os.Getenv(testdirKey)
	if len(dn) == 0 {
		dn = os.Getenv("HOME")
		if len(dn) == 0 {
			var err error
			dn, err = os.Getwd()
			if err != nil {
				panic(err)
			}
		}
		dn = path.Join(dn, defaultTestdir)
	}
	return dn
}

func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = errors.New("File exists and not a directory.")
	}
	return err
}

func authPam(user, pwd string) error {
	fail := func() error { return errors.New("Authentication failed.") }
	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF, pam.PROMPT_ECHO_ON:
			return pwd, true
		default:
			return "", false
		}
	}))
	if s != pam.SUCCESS {
		return fail()
	}
	defer t.End(s)

	s = t.Authenticate(0)
	if s != pam.SUCCESS {
		return fail()
	}
	return nil
}

func main() {
	err := initConfig()
	if err != nil {
		log.Panicln(err)
	}

	err = auth.Init(&cfg, auth.AuthFunc(authPam))
	if err != nil {
		log.Panicln(err)
	}

	dn := getHttpDir()
	ensureDir(dn)

	err = serve()
	if err != nil {
		log.Panicln(err)
	}
}
