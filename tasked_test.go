package main

import (
	"bytes"
	"code.google.com/p/tasked/sec"
	"errors"
	"flag"
	"os"
	"path"
	"testing"
	"time"
)

var (
	testAuth   bool
	testInTime bool
)

func init() {
	ta := flag.Bool("auth", false, "")
	ttp := flag.Bool("testInTime", false, "")
	flag.Parse()
	testAuth = *ta
	testInTime = *ttp
}

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func TestEnsureDir(t *testing.T) {
	const syserr = "Cannot create test file."

	// exists and directory
	tp := path.Join(testdir, "some")
	err := os.RemoveAll(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.MkdirAll(tp, os.ModePerm)
	if err != nil {
		t.Fatal(syserr)
	}
	err = ensureDir(tp)
	if err != nil {
		t.Fail()
	}

	// exists and not directory
	err = os.RemoveAll(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	var f *os.File
	f, err = os.Create(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	f.Close()
	err = ensureDir(tp)
	if err == nil {
		t.Fail()
	}

	// doesn't exist
	err = os.RemoveAll(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	err = ensureDir(tp)
	if err != nil {
		t.Fail()
	}
	var fi os.FileInfo
	fi, err = os.Stat(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	if !fi.IsDir() {
		t.Fail()
	}
}

func TestGetHttpDir(t *testing.T) {
	err := withEnv(testdirKey, "", func() error {
		return withEnv("HOME", "", func() error {
			dn := getHttpDir()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if dn != path.Join(wd, defaultTestdir) {
				return errors.New(dn)
			}
			return nil
		})
	})
	if err != nil {
		t.Fail()
	}
	err = withEnv(testdirKey, "", func() error {
		dn := getHttpDir()
		if dn != path.Join(os.Getenv("HOME"), defaultTestdir) {
			return errors.New(dn)
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
	err = withEnv(testdirKey, "test", func() error {
		dn := getHttpDir()
		if dn != "test" {
			return errors.New(dn)
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
}

func TestAuthPam(t *testing.T) {
	if !testAuth {
		t.Skip()
	}
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")

	if nil != authPam(user, pwd) {
		t.Fail()
	}
	if nil == authPam(user+"x", pwd) {
		t.Fail()
	}
	if nil == authPam(user, pwd+"x") {
		t.Fail()
	}
	if nil == authPam(user, "") {
		t.Fail()
	}
	if nil == authPam("", "") {
		t.Fail()
	}
}

func TestAuthPwd(t *testing.T) {
	if !testAuth {
		t.Skip()
	}
	sec.Init(&cfg, sec.AuthFunc(authPam))
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")

	tk, err := sec.AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	_, err = sec.AuthPwd(user+"x", pwd)
	if err == nil {
		t.Fail()
	}
	_, err = sec.AuthPwd(user, pwd+"x")
	if err == nil {
		t.Fail()
	}
	_, err = sec.AuthPwd(user, "")
	if err == nil {
		t.Fail()
	}
	_, err = sec.AuthPwd("", "")
	if err == nil {
		t.Fail()
	}
}

func TestAuthFull(t *testing.T) {
	if !testAuth {
		t.Skip()
	}
	sec.Init(&cfg, sec.AuthFunc(authPam))
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")
	tk, err := sec.AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	tback, err := sec.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = sec.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	userBack, err := sec.GetUser(tk)
	if err != nil || userBack != user {
		t.Fail()
	}
}

func TestAuthFullTime(t *testing.T) {
	if !testInTime {
		t.Skip()
	}
	if !testAuth {
		t.Skip()
	}
	sec.Init(&cfg, sec.AuthFunc(authPam))
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")
	tk, err := sec.AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	time.Sleep(time.Second)
	tback, err := sec.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = sec.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(2 * time.Second)
	tback, err = sec.AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = sec.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(20 * time.Second)
	tback, err = sec.AuthToken(tk)
	if err == nil {
		t.Fail()
	}
	tback, err = sec.AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}
