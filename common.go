package main

import (
	"log"
	"time"
)

// Tries executing a function. If it fails, retries after a delay but in a new
// goroutine. If it fails again, reports (e.g. log) both errors.
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

// The same as doretrep, but with log.Println as report.
func doretlog(do func() error, delay time.Duration) {
	doretrep(do, delay, log.Println)
}

// The same as doretlog, but with 42 milliseconds as delay.
func doretlog42(do func() error) {
	doretlog(do, 42*time.Millisecond)
}
