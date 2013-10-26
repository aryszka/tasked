package main

import (
	"log"
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
