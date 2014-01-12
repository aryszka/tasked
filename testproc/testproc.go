package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	missingCommand = "Missing command."
	invalidCommand = "Invalid command."
	timeout        = "Timeout."
)

func getToMillisecs() int {
	var (
		toSecs int
		err    error
	)
	if len(os.Args) > 2 {
		toSecs, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalln(invalidCommand, err)
		}
	} else {
		toSecs = 12000
	}
	return toSecs
}

func wait() {
	<-time.After(time.Duration(getToMillisecs()) * time.Millisecond)
}

func gulpTerm() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	for {
		select {
		case <-c:
		case <-time.After(time.Duration(getToMillisecs()) * time.Millisecond):
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
	<-time.After(time.Duration(getToMillisecs()) * time.Millisecond)
}

func main() {
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
}
