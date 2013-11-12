package share

import (
	"log"
	"time"
)

// YOLO
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
