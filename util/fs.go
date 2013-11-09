package util

import (
	"os"
	"path"
	"fmt"
)

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
