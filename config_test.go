package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"
)

const (
	failedToInitTestdir = "Failed to initialize test directory."
)

var (
	testTokenValidity    = 1200
	testMaxSearchResults = 30
)

// duplicate
func init() {
	get := func() string {
		td := os.Getenv(testdirKey)
		if len(td) > 0 {
			return td
		}
		td = os.Getenv("GOPATH")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td = os.Getenv("HOME")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td, err := os.Getwd()
		if err != nil {
			panic(failedToInitTestdir)
		}
		return path.Join(td, defaultTestdir)
	}
	testdir = get()
	err := ensureDir(testdir)
	if err != nil {
		panic(failedToInitTestdir)
	}
}

func TestGetConfdir(t *testing.T) {
	const configTestdir = "config-test"
	err := withEnv(configEnvKey, configTestdir, func() error {
		dir, err := getConfdir()
		if err != nil {
			return err
		}
		if dir != configTestdir {
			return errors.New("Failed.")
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
	err = withEnv(configEnvKey, "", func() error {
		dir, err := getConfdir()
		if err != nil {
			return err
		}
		if dir != path.Join(os.Getenv("HOME"), configDefaultDir) {
			return errors.New("Failed.")
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal()
	}
	err = withEnv(configEnvKey, "", func() error {
		return withEnv("HOME", "", func() error {
			dir, err := getConfdir()
			if err != nil {
				return err
			}
			if dir != path.Join(wd, configDefaultDir) {
				return errors.New("Failed.")
			}
			return nil
		})
	})
}

func TestEvalFile(t *testing.T) {
	dir := path.Join(testdir, "config")
	err := ensureDir(dir)
	errFatal(t, err)
	p := path.Join(testdir, "some")

	err = evalFile("", nil)
	if err != nil {
		t.Fail()
	}

	err = removeIfExists(p)
	errFatal(t, err)
	err = evalFile(p, nil)
	if err == nil {
		t.Fail()
	}

	err = withNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("some"))
		return err
	})
	errFatal(t, err)
	var b []byte
	err = evalFile(p, &b)
	if err != nil || !bytes.Equal(b, []byte("some")) {
		t.Fail()
	}
}

func TestEvalIntP(t *testing.T) {
	i := -42
	evalIntP(-1, &i)
	if i != -42 {
		t.Fail()
	}
	evalIntP(0, &i)
	if i != -42 {
		t.Fail()
	}
	evalIntP(42, &i)
	if i != 42 {
		t.Fail()
	}
}

func TestEvalString(t *testing.T) {
	s := "some-before"
	evalString("", &s)
	if s != "some-before" {
		t.Fail()
	}
	evalString("some-after", &s)
	if s != "some-after" {
		t.Fail()
	}
}

func TestReadConfig(t *testing.T) {
	const (
		syserr        = "Cannot create test file."
		configTestDir = "config-test"
	)

	configEqual := func(left, right config) bool {
		return bytes.Equal(left.sec.aes.key, right.sec.aes.key) &&
			bytes.Equal(left.sec.aes.iv, right.sec.aes.iv) &&
			left.sec.tokenValidity == right.sec.tokenValidity &&
			bytes.Equal(left.http.tls.key, right.http.tls.key) &&
			bytes.Equal(left.http.tls.cert, right.http.tls.cert) &&
			left.http.address == right.http.address &&
			left.files.search.maxResults == right.files.search.maxResults
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

	cfg = config{}
	verify := config{}
	err = readConfig(fn)

	if !os.IsNotExist(err) || !configEqual(cfg, verify) {
		t.Fail()
	}

	// settings file exists, empty
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withNewFile(fn, nil)
	if err != nil {
		t.Fatal()
	}
	cfg = config{}
	err = readConfig(fn)
	if err != nil || !configEqual(cfg, verify) {
		t.Fail()
	}

	// settings file exists
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}

	aesdir := path.Join(testdir, configTestDir, "aes")
	err = os.RemoveAll(aesdir)
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.MkdirAll(aesdir, os.ModePerm)
	if err != nil {
		t.Fatal(syserr)
	}
	aesKeypath := path.Join(aesdir, "aeskey")
	err = withNewFile(aesKeypath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "abc")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	aesIvpath := path.Join(aesdir, "aesiv")
	err = withNewFile(aesIvpath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "def")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	tlsdir := path.Join(testdir, configTestDir, "tls")
	err = os.RemoveAll(tlsdir)
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.MkdirAll(tlsdir, os.ModePerm)
	if err != nil {
		t.Fatal(syserr)
	}
	tlsKeypath := path.Join(tlsdir, "tlskey")
	err = withNewFile(tlsKeypath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "123")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	tlsCertpath := path.Join(tlsdir, "tlscert")
	err = withNewFile(tlsCertpath, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "456")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withNewFile(fn, func(f *os.File) error {
		print := func(ft string, args ...interface{}) bool {
			_, err := fmt.Fprintf(f, ft, args...)
			return err == nil
		}
		if !(print("[Sec]\n") &&
			print("aeskeypath = %s\n", aesKeypath) &&
			print("aesivpath = %s\n", aesIvpath) &&
			print("tokenvaliditysecs = %d\n", testTokenValidity) &&
			print("[Http]\n") &&
			print("tlskeypath = %s\n", tlsKeypath) &&
			print("tlscertpath = %s\n", tlsCertpath) &&
			print("address = %s\n", ":9091") &&
			print("[Files]\n") &&
			print("maxsearchresults = %d\n", testMaxSearchResults)) {
			return errors.New(syserr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = config{}
	err = readConfig(fn)

	if err != nil {
		t.Fail()
	}
	if !bytes.Equal(cfg.sec.aes.key, []byte("abc")) {
		t.Fail()
	}
	if !bytes.Equal(cfg.sec.aes.iv, []byte("def")) {
		t.Fail()
	}
	if !bytes.Equal(cfg.http.tls.key, []byte("123")) {
		t.Fail()
	}
	if !bytes.Equal(cfg.http.tls.cert, []byte("456")) {
		t.Fail()
	}
	if cfg.http.address != ":9091" {
		t.Fail()
	}

	// settings file exists with invalid content
	err = os.RemoveAll(fn)
	if err != nil {
		t.Fatal(syserr)
	}
	err = withNewFile(fn, func(f *os.File) error {
		_, err = fmt.Fprintf(f, "something invalid")
		return err
	})
	if err != nil {
		t.Fatal(syserr)
	}

	cfg = config{}
	err = readConfig(fn)
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
	err = initConfig()
	if err != nil {
		t.Fail()
	}
}
