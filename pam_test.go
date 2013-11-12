package main

import (
	"code.google.com/p/tasked/share"
	tst "code.google.com/p/tasked/testing"
	"flag"
	"path"
	"testing"
)

var testPam bool

func init() {
	tp := flag.Bool("test.pam", false, "")
	flag.Parse()
	testPam = *tp
}

func TestAuthPam(t *testing.T) {
	if !share.IsRoot || !testPam {
		t.Skip()
	}

	if nil != authPam(tst.Testuser, tst.Testpwd) {
		t.Fail()
	}
	if nil == authPam(tst.Testuser+"x", tst.Testpwd) {
		t.Fail()
	}
	if nil == authPam(tst.Testuser, tst.Testpwd+"x") {
		t.Fail()
	}
	if nil == authPam(tst.Testuser, "") {
		t.Fail()
	}
	if nil == authPam("", "") {
		t.Fail()
	}
}

func TestNewAuth(t *testing.T) {
	a, err := newAuth(nil)
	if a == nil || err != nil {
		t.Fail()
	}
	s := &settings{}
	kp := path.Join(tst.Testdir, "keyFile")
	tst.RemoveIfExistsF(t, kp)
	s.sec.aes.keyFile = kp
	a, err = newAuth(s)
	if err == nil {
		t.Fail()
	}
	s = &settings{}
	ki := path.Join(tst.Testdir, "ivFile")
	tst.RemoveIfExistsF(t, ki)
	s.sec.aes.ivFile = ki
	a, err = newAuth(s)
	if err == nil {
		t.Fail()
	}
}
