package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"strconv"
	"time"
)

const (
	missingCommand = "Missing command."
	invalidCommand = "Invalid command."
	timeout        = "Timeout."
)

func getToMillisecs() time.Duration {
	var (
		toMillisecs int
		err    error
	)
	if len(os.Args) > 2 {
		toMillisecs, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalln(invalidCommand, err)
		}
	} else {
		toMillisecs = 12000
	}
	return time.Duration(toMillisecs) * time.Millisecond
}

func wait() {
	<-time.After(getToMillisecs())
}

func gulpTerm() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	for {
		select {
		case <-c:
		case <-time.After(time.Duration(getToMillisecs())):
			return
		}
	}
}

func printWait() {
	if len(os.Args) > 3 {
		for _, l := range os.Args[3:] {
			fmt.Println(l)
		}
	}
	<-time.After(time.Duration(getToMillisecs()))
}

func main() {
	log.Println(time.Now())
	if len(os.Args) == 1 {
		log.Fatalln(missingCommand)
	}
	switch os.Args[1] {
	default:
		log.Fatalln(invalidCommand)
	case "noop":
	case "wait":
		wait()
	case "gulpterm":
		gulpTerm()
	case "printwait":
		printWait()
	}
	log.Println(time.Now())
}
