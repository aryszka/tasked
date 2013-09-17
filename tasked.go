// Tasked is not yet defined.
//
// It uses PAM (Pluggable Authentication Module) to check user credentials.
package main

import (
	"code.google.com/p/gopam"
	"code.google.com/p/tasked/sec"
	"errors"
	"fmt"
	"os"
	"path"
)

const (
	defaultTestdir = "test"
	testdirKey     = "testdir"
	fnbase         = "file"
)

func getHttpDir() string {
	// todo: in the final logic, wd probably will need to be before home, and home used only if wd is not
	// writeable, or rather not used at all
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

// Makes sure that a directory with a given path exists.
func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = errors.New("File exists and not a directory.")
	}
	return err
}

func printerrln(e ...interface{}) {
	_, err := fmt.Fprintln(os.Stderr, e...)
	if err != nil {
		panic("Failed to write to stderr.")
	}
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
		printerrln(err)
		os.Exit(1)
	}

	err = sec.Init(&cfg, sec.AuthFunc(authPam))
	if err != nil {
		printerrln(err)
		os.Exit(1)
	}

	dn := getHttpDir()
	ensureDir(dn)
	fn = path.Join(dn, fnbase)

	err = serve()
	if err != nil {
		printerrln(err)
		os.Exit(1)
	}
}
