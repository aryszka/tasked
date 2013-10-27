package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"
)

func doretrep(do func() error, delay time.Duration, report func(...interface{})) {
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

func doretlog(do func() error, delay time.Duration) {
	doretrep(do, delay, log.Println)
}

func doretlog42(do func() error) {
	doretlog(do, 42*time.Millisecond)
}

func abspath(p, dir string) (string, error) {
	if path.IsAbs(p) {
		return p, nil
	}
	if len(dir) == 0 {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else if !path.IsAbs(dir) {
		return "", fmt.Errorf("Not an absolute path: %s.", dir)
	}
	return path.Join(dir, p), nil
}

func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = fmt.Errorf("File exists and not a directory: %s.", dir)
	}
	return err
}

func checkPath(p string, dir bool) (bool, error) {
	fi, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return err == nil && (fi.IsDir() && dir || !fi.IsDir() && !dir), err
}
