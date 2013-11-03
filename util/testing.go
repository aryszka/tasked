package util

import (
	"os"
	"os/user"
	"path"
)

const (
	testdirKey          = "testdir"
	failedToInitTestdir = "Failed to initialize test directory."
)

type Fataler interface {
	Fatal(...interface{})
}

var (
	Testdir  = defaultTestdir
	IsRoot   bool
	Testuser string
	Testpwd  string
)

func initTestdir() {
	Testdir = func() string {
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
	}()
	err := EnsureDir(Testdir)
	if err != nil {
		panic(failedToInitTestdir)
	}
}

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	IsRoot = usr.Uid == "0"
	initTestdir()
	Testuser = envdef("testuser", "testuser")
	Testpwd = envdef("testpwd", "testpwd")
}

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func ErrFatal(f Fataler, err error) {
	if err != nil {
		f.Fatal(err)
	}
}

func EnsureDirF(f Fataler, dir string) {
	ErrFatal(f, EnsureDir(dir))
}

func WithEnv(key, val string, f func() error) error {
	orig := os.Getenv(key)
	defer Doretlog42(func() error { return os.Setenv(key, orig) })
	err := os.Setenv(key, val)
	if err != nil {
		return err
	}
	if f == nil {
		return nil
	}
	return f()
}

func WithNewFile(fn string, do func(*os.File) error) error {
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

func WithNewFileF(f Fataler, fn string, do func(*os.File) error) {
	ErrFatal(f, WithNewFile(fn, do))
}

func RemoveIfExists(fn string) error {
	err := os.RemoveAll(fn)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func RemoveIfExistsF(f Fataler, fn string) {
	ErrFatal(f, RemoveIfExists(fn))
}

func WithNewDir(dir string) error {
	err := RemoveIfExists(dir)
	if err != nil {
		return err
	}
	return EnsureDir(dir)
}

func WithNewDirF(f Fataler, dir string) {
	ErrFatal(f, WithNewDir(dir))
}
