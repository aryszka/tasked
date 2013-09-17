package main

import (
	"errors"
	"os"
	"path"
	"testing"
	"io"
	"crypto/rand"
	"crypto/aes"
	"os/user"
)

var isRoot bool

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	isRoot = usr.Uid == "0"
}

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func makeRandom(l int) []byte {
	b := make([]byte, l)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("Failed to generate random bytes.")
	}
	return b
}

func makeKey() []byte { return makeRandom(aes.BlockSize) }

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
	if !isRoot {
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
