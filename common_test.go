package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

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
	defer doretlog42(f.Close)
	if do == nil {
		return nil
	}
	return do(f)
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
