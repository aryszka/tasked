package main

import (
	tst "github.com/aryszka/tasked/testing"
	"testing"
	"path"
	. "github.com/aryszka/tasked/testing"
	"io"
	"crypto/rand"
	"crypto/aes"
)

func TestAuthPam(t *testing.T) {
	if !IsRoot || !testLong {
		t.Skip()
	}

	if valid := authPam(tst.Testuser, tst.Testpwd); !valid {
		t.Fail()
	}
	if valid := authPam(tst.Testuser+"x", tst.Testpwd); valid {
		t.Fail()
	}
	if valid := authPam(tst.Testuser, tst.Testpwd+"x"); valid {
		t.Fail()
	}
	if valid := authPam(tst.Testuser, ""); valid {
		t.Fail()
	}
	if valid := authPam("", ""); valid {
		t.Fail()
	}
}

func TestMkauth(t *testing.T) {
	defer func(r io.Reader) { rand.Reader = r }(rand.Reader)

	// key read fail
	o := new(options)
	f := path.Join(Testdir, "file")
	o.aesKeyFile = f
	RemoveIfExistsF(t, f)
	_, err := mkauth(o)
	if err == nil {
		t.Fail()
	}

	// iv read fail
	o = new(options)
	f = path.Join(Testdir, "file")
	o.aesIvFile = f
	RemoveIfExistsF(t, f)
	_, err = mkauth(o)
	if err == nil {
		t.Fail()
	}

	// no key and iv, genAes fails
	o = new(options)
	rr := rand.Reader
	rand.Reader = new(errorReader)
	_, err = mkauth(o)
	if err == nil {
		t.Fail()
	}
	rand.Reader = rr

	// no key and iv
	o = new(options)
	a, err := mkauth(o)
	if a == nil || err != nil {
		t.Fail()
	}

	// key and iv set
	key := make([]byte, aes.BlockSize)
	iv := make([]byte, aes.BlockSize)
	o = new(options)
	o.aesKey = string(key)
	o.aesIv = string(iv)
	a, err = mkauth(o)
	if a == nil || err != nil {
		t.Fail()
	}
}
