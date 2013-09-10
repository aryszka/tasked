package main

import (
	"code.google.com/p/tasked/app"
	"code.google.com/p/tasked/sec"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
)

const (
	defaultTestdir = "test"
	testdirKey     = "testdir"
	fnbase         = "file"
)

func getHttpDir() string {
	// todo: in the final logic, wd probably will need to be before home, and home used only if wd is not
	// writeable, or rather not used at all
	dn := os.Getenv(testdirKey)
	if len(dn) == 0 {
		dn = os.Getenv("HOME")
		if len(dn) == 0 {
			var err error
			dn, err = os.Getwd()
			if err != nil {
				panic(err)
			}
		}
		dn = path.Join(dn, defaultTestdir)
	}
	return dn
}

// duplicate
// Makes sure that a directory with a given path exists.
func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = errors.New("File exists and not a directory.")
	}
	return err
}

func printerrln(e ...interface{}) {
	_, err := fmt.Fprintln(os.Stderr, e...)
	if err != nil {
		panic("Failed to write to stderr.")
	}
}

func main() {
	err := initConfig()
	if err != nil {
		printerrln(err)
		os.Exit(1)
	}

	sec.Init(cfg)

	dn := getHttpDir()
	ensureDir(dn)
	fn := path.Join(dn, fnbase)

	err = app.Serve(cfg, fn)
	if err != nil {
		printerrln(err)
		os.Exit(1)
	}
	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)
	<-sigint
}
