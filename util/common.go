package util

import (
	"fmt"
	"log"
	"os"
	"path"
	"syscall"
	"time"
)

const defaultTestdir = "test"

func init() {
	syscall.Umask(0077)
}

func Doretrep(do func() error, delay time.Duration, report func(...interface{})) {
	err0 := do()
	if err0 == nil {
		return
	}
	go func() {
		time.Sleep(delay)
		err1 := do()
		if err1 != nil {
			report(err0, err1)
		}
	}()
}

func Doretlog(do func() error, delay time.Duration) {
	Doretrep(do, delay, log.Println)
}

func Doretlog42(do func() error) {
	Doretlog(do, 42*time.Millisecond)
}

func Abspath(p, dir string) string {
	if path.IsAbs(p) {
		return p
	}
	return path.Join(dir, p)
}

func AbspathNotEmpty(p, dir string) string {
	if len(p) == 0 {
		return p
	}
	return Abspath(p, dir)
}

func EnsureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = fmt.Errorf("File exists and not a directory: %s.", dir)
	}
	return err
}

func CheckPath(p string, dir bool) (bool, error) {
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return err == nil && (fi.IsDir() && dir || !fi.IsDir() && !dir), err
}
