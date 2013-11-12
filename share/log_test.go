package share

import (
	"errors"
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
