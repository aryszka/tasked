package util

import (
	"errors"
	"os"
	"path"
	"testing"
	"time"
)

func TestDoRetryReport(t *testing.T) {
	// succeed first
	c := 0
	Doretrep(func() error {
		c = c + 1
		return nil
	}, 0, nil)
	if c != 1 {
		t.Fail()
	}

	// succeed second
	done := make(chan int)
	c = 0
	Doretrep(func() error {
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
	Doretrep(func() error {
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
	p := Abspath("/", "")
	if p != "/" {
		t.Fail()
	}
	p = Abspath("/some/path", "")
	if p != "/some/path" {
		t.Fail()
	}
	p = Abspath("", "")
	if p != "" {
		t.Fail()
	}
	p = Abspath("some", "")
	if p != "some" {
		t.Fail()
	}
	p = Abspath("some/path", "")
	if p != "some/path" {
		t.Fail()
	}
	p = Abspath("some/path", "/absolute/path")
	if p != "/absolute/path/some/path" {
		t.Fail()
	}
}

func TestAbspathNotEmpty(t *testing.T) {
	p := AbspathNotEmpty("some", "/root")
	if p != "/root/some" {
		t.Fail()
	}
	p = AbspathNotEmpty("", "/root")
	if p != "" {
		t.Fail()
	}
}

func TestEnsureDir(t *testing.T) {
	const syserr = "Cannot create test file."

	// exists and directory
	tp := path.Join(Testdir, "some")
	err := os.RemoveAll(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	err = os.MkdirAll(tp, os.ModePerm)
	if err != nil {
		t.Fatal(syserr)
	}
	err = EnsureDir(tp)
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
	err = EnsureDir(tp)
	if err == nil {
		t.Fail()
	}

	// doesn't exist
	err = os.RemoveAll(tp)
	if err != nil {
		t.Fatal(syserr)
	}
	err = EnsureDir(tp)
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

func TestCheckPath(t *testing.T) {
	p := path.Join(Testdir, "test-file")
	RemoveIfExistsF(t, p)
	ok, err := CheckPath(p, false)
	if ok || err != nil {
		t.Fail()
	}
	WithNewFileF(t, p, nil)
	ok, err = CheckPath(p, false)
	if !ok || err != nil {
		t.Fail()
	}
	RemoveIfExistsF(t, p)
	EnsureDirF(t, p)
	ok, err = CheckPath(p, false)
	if ok || err != nil {
		t.Fail()
	}
	ok, err = CheckPath(p, true)
	if !ok || err != nil {
		t.Fail()
	}
}

func TestCheckPathNotRoot(t *testing.T) {
	if IsRoot {
		t.Skip()
	}

	dir := path.Join(Testdir, "dir")
	EnsureDirF(t, dir)
	p := path.Join(dir, "test-file")
	WithNewFileF(t, p, nil)
	err := os.Chmod(dir, 0)
	defer func() {
		err = os.Chmod(dir, os.ModePerm)
		ErrFatal(t, err)
	}()
	ok, err := CheckPath(p, false)
	if ok || err == nil {
		t.Fail()
	}
}
