package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
)

var testTokenValidity = 1200

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
	const envkey = "testkey"
	tp := path.Join(testdir, "envtest")

	orig := os.Getenv(envkey)
	err := os.Setenv(envkey, tp)
	if err != nil {
		t.Fatal()
	}
	defer os.Setenv(envkey, orig)

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

	configEqual := func(left, right *config) bool {
		return bytes.Equal(left.sec.aes.key, right.sec.aes.key) &&
			bytes.Equal(left.sec.aes.iv, right.sec.aes.iv) &&
			left.sec.tokenValidity == right.sec.tokenValidity &&
			bytes.Equal(left.http.tls.key, right.http.tls.key) &&
			bytes.Equal(left.http.tls.cert, right.http.tls.cert)
	}

	fn := path.Join(testdir, configTestDir)
	err := ensureDir(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	fn = path.Join(fn, configBaseName)

	// settings file doesn't exist
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = defaultConfig()
	verify := defaultConfig()
	err = readConfig(fn, cfg)

	if !os.IsNotExist(err) || !configEqual(cfg, verify) {
		t.Log("here")
		t.Log(os.IsNotExist(err))
		t.Log(cfg)
		t.Log(verify)
		t.Log(configEqual(cfg, verify))
		t.Fail()
	}

	// settings file exists, empty
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withFile(fn, func(f *os.File) error { return nil })
	if err != nil {
		t.Fatal()
	}
	cfg = defaultConfig()
	err = readConfig(fn, cfg)
	if err != nil || !configEqual(cfg, verify) {
		t.Fail()
	}

	// settings file exists
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}

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
		print := func(ft string, args ...interface{}) bool {
			_, err := fmt.Fprintf(f, ft, args...)
			return err == nil
		}
		if !print("[aes]\n") ||
			!print("keypath = %s\n", keypath) ||
			!print("ivpath = %s\n", ivpath) ||
			!print("[auth]\n") ||
			!print("tokenvaliditysecs = %d", testTokenValidity) {
			return errors.New(syserr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = defaultConfig()
	err = readConfig(fn, cfg)

	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if cfg == nil {
		t.Fail()
	}
	if !bytes.Equal(cfg.sec.aes.key, []byte("abc")) {
		t.Fail()
	}
	if !bytes.Equal(cfg.sec.aes.iv, []byte("def")) {
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
