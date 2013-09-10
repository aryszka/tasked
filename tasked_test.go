package main

import (
	"errors"
	"os"
	"path"
	"testing"
)

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

func TestGetHttpDir(t *testing.T) {
	err := withEnv(testdirKey, "", func() error {
		return withEnv("HOME", "", func() error {
			dn := getHttpDir()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if dn != path.Join(wd, defaultTestdir) {
				return errors.New(dn)
			}
			return nil
		})
	})
	if err != nil {
		t.Fail()
	}
	err = withEnv(testdirKey, "", func() error {
		dn := getHttpDir()
		if dn != path.Join(os.Getenv("HOME"), defaultTestdir) {
			return errors.New(dn)
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
	t.Log("starting")
	err = withEnv(testdirKey, "test", func() error {
		dn := getHttpDir()
		if dn != "test" {
			return errors.New(dn)
		}
		return nil
	})
	if err != nil {
		t.Fail()
	}
}
