package main

import (
	"bytes"
	"code.google.com/p/tasked/util"
	"flag"
	"os"
	"path"
	"testing"
)

var testPam bool

func init() {
	tp := flag.Bool("test.pam", false, "")
	flag.Parse()
	testPam = *tp
}

func TestNewAuthSettings(t *testing.T) {
	as, err := newAuthSettings(nil)
	if err != nil || as.s == nil {
		t.Fail()
	}

	kf := path.Join(util.Testdir, "key")
	util.WithNewFileF(t, kf, func(f *os.File) error {
		_, err := f.Write([]byte("123"))
		return err
	})
	ivf := path.Join(util.Testdir, "iv")
	util.WithNewFileF(t, ivf, func(f *os.File) error {
		_, err := f.Write([]byte("456"))
		return err
	})
	s := &settings{}
	s.sec.aes.keyFile = kf
	s.sec.aes.ivFile = ivf
	as, err = newAuthSettings(s)
	if err != nil ||
		!bytes.Equal(as.aesKey, []byte("123")) ||
		!bytes.Equal(as.aesIv, []byte("456")) {
		t.Fail()
	}

	util.RemoveIfExistsF(t, kf)
	as, err = newAuthSettings(s)
	if err == nil {
		t.Fail()
	}
}

func TestGetHttpDir(t *testing.T) {
	wd, err := os.Getwd()
	util.ErrFatal(t, err)

	s := &settings{}

	s.files.root = "some/path"
	p, err := getHttpDir(s)
	if err != nil || p != path.Join(wd, "some/path") {
		t.Fail()
	}

	s.files.root = "/some/absolute/path"
	p, err = getHttpDir(s)
	if err != nil || p != "/some/absolute/path" {
		t.Fail()
	}
}

func TestAuthPam(t *testing.T) {
	if !util.IsRoot || !testPam {
		t.Skip()
	}

	if nil != authPam(util.Testuser, util.Testpwd) {
		t.Fail()
	}
	if nil == authPam(util.Testuser+"x", util.Testpwd) {
		t.Fail()
	}
	if nil == authPam(util.Testuser, util.Testpwd+"x") {
		t.Fail()
	}
	if nil == authPam(util.Testuser, "") {
		t.Fail()
	}
	if nil == authPam("", "") {
		t.Fail()
	}
}
