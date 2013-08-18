package main

import (
	"fmt"
	"os"
	"path"
	"testing"
)

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

func TestEnsureEnvDir(t *testing.T) {
	// env set/unset

	const envkey = "TEST_KEY"
	tp := path.Join(testdir, "envtest")

	os.Setenv(envkey, tp)
	res, err := ensureEnvDir(envkey, "")
	if err != nil || res != tp {
		t.Fail()
	}
}

func TestReadConfig(t *testing.T) {
	const (
		syserr        = "Cannot create test file."
		configTestDir = "config-test"
	)
	withFile := func(fn string, do func(*os.File) error) error {
		f, err := os.Create(fn)
		if err != nil {
			return err
		}
		defer f.Close()
		return do(f)
	}
	eqbytes := func(bs0, bs1 []byte) bool {
		if len(bs0) != len(bs1) {
			return false
		}
		for i, b := range bs0 {
			if b != bs1[i] {
				return false
			}
		}
		return true
	}

	fn := path.Join(testdir, configTestDir)
	err := ensureDir(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	fn = path.Join(fn, configBaseName)

	// settings file exists
	secdir := path.Join(testdir, configTestDir, "sec")
	err = os.RemoveAll(secdir)
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.MkdirAll(secdir, os.ModePerm)
	if err != nil {
		t.Fatal(syserr)
	}
	keypath := path.Join(secdir, "aeskey")
	err = withFile(keypath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "abc")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	ivpath := path.Join(secdir, "aesiv")
	err = withFile(ivpath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "def")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withFile(fn, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "[aes]\nkeypath = %s\nivpath = %s", keypath, ivpath)
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = defaultConfig()
	err = readConfig(fn, cfg)

	if err != nil {
		t.Fail()
	}
	if cfg == nil {
		t.Fail()
	}
	if !eqbytes(cfg.aes.key, []byte("abc")) {
		t.Fail()
	}
	if !eqbytes(cfg.aes.iv, []byte("def")) {
		t.Fail()
	}

	// settings file doesn't exist
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = defaultConfig()
	key, iv := cfg.aes.key, cfg.aes.iv
	err = readConfig(fn, cfg)

	if !os.IsNotExist(err) {
		t.Error(err)
	}
	if !eqbytes(cfg.aes.key, key) {
		t.Fail()
	}
	if !eqbytes(cfg.aes.iv, iv) {
		t.Fail()
	}

	// settings file exists with invalid content
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withFile(fn, func(f *os.File) error {
		_, err = fmt.Fprintf(f, "something invalid")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = defaultConfig()
	err = readConfig(fn, cfg)
	if err == nil {
		t.Fail()
	}
}

func TestInitConfig(t *testing.T) {
	os.Setenv(configEnvKey, testdir)
	err := os.RemoveAll(path.Join(testdir, configBaseName))
	if err != nil {
		t.Fatal("Cannot cleanup test data.")
	}
	err = initConfig(&options{})
	if err != nil {
		t.Fail()
	}
}
