package main

import (
	"errors"
	"log"
	"os"
	"path"
	"testing"
	"time"
)

func errFatal(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func withEnv(key, val string, f func() error) error {
	orig := os.Getenv(key)
	defer doretlog42(func() error { return os.Setenv(key, orig) })
	err := os.Setenv(key, val)
	if err != nil {
		return err
	}
	if f == nil {
		return nil
	}
	return f()
}

func withNewFile(fn string, do func(*os.File) error) error {
	err := os.Remove(fn)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	if do == nil {
		return nil
	}
	err = do(f)
	if err != nil {
		return err
	}
	return f.Close()
}

func withNewFileF(t *testing.T, fn string, do func(*os.File) error) {
	errFatal(t, withNewFile(fn, do))
}

func removeIfExists(fn string) error {
	err := os.RemoveAll(fn)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func removeIfExistsF(t *testing.T, fn string) {
	errFatal(t, removeIfExists(fn))
}

func ensureDirF(t *testing.T, dir string) {
	errFatal(t, ensureDir(dir))
}

func withNewDir(dir string) error {
	err := removeIfExists(dir)
	if err != nil {
		return err
	}
	return ensureDir(dir)
}

func withNewDirF(t *testing.T, dir string) {
	errFatal(t, withNewDir(dir))
}

func trace(args ...interface{}) {
	log.Println(args...)
}

func TestDoRetryReport(t *testing.T) {
	// succeed first
	c := 0
	doretrep(func() error {
		c = c + 1
		return nil
	}, 0, nil)
	if c != 1 {
		t.Fail()
	}

	// succeed second
	done := make(chan int)
	c = 0
	doretrep(func() error {
		c = c + 1
		switch c {
		case 1:
			return errors.New("error")
		default:
			done <- 0
			return nil
		}
	}, 42*time.Millisecond, nil)
	<-done
	if c != 2 {
		t.Fail()
	}

	// fail
	c = 0
	var errs []interface{} = nil
	doretrep(func() error {
		c = c + 1
		return errors.New("error")
	}, 42*time.Millisecond, func(es ...interface{}) {
		errs = es
		done <- 0
	})
	<-done
	if c != 2 || len(errs) != 2 {
		t.Fail()
	}
}

func TestAbspath(t *testing.T) {
	wd, err := os.Getwd()
	errFatal(t, err)
	p, err := abspath("/", "")
	if err != nil || p != "/" {
		t.Fail()
	}
	p, err = abspath("/some/path", "")
	if err != nil || p != "/some/path" {
		t.Fail()
	}
	p, err = abspath("", "")
	if err != nil || p != wd {
		t.Fail()
	}
	p, err = abspath("some", "")
	if err != nil || p != path.Join(wd, "some") {
		t.Fail()
	}
	p, err = abspath("some/path", "")
	if err != nil || p != path.Join(wd, "some/path") {
		t.Fail()
	}
	p, err = abspath("some/path", "not/absolute")
	if err == nil {
		t.Fail()
	}
	p, err = abspath("some/path", "/absolute/path")
	if p != "/absolute/path/some/path" || err != nil {
		t.Fail()
	}
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
