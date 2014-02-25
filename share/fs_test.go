package share

import (
	tst "code.google.com/p/tasked/testing"
	"os"
	"path"
	"testing"
)

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
	tp := path.Join(tst.Testdir, "some")
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
	p := path.Join(tst.Testdir, "test-file")
	tst.RemoveIfExistsF(t, p)
	ok, err := CheckPath(p, false)
	if ok || err != nil {
		t.Fail()
	}
	tst.WithNewFileF(t, p, nil)
	ok, err = CheckPath(p, false)
	if !ok || err != nil {
		t.Fail()
	}
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
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
	if tst.IsRoot {
		t.Skip()
	}

	dir := path.Join(tst.Testdir, "dir")
	tst.EnsureDirF(t, dir)
	p := path.Join(dir, "test-file")
	tst.WithNewFileF(t, p, nil)
	err := os.Chmod(dir, 0)
	defer func() {
		err = os.Chmod(dir, os.ModePerm)
		tst.ErrFatal(t, err)
	}()
	ok, err := CheckPath(p, false)
	if ok || err == nil {
		t.Fail()
	}
}
