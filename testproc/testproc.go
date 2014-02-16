package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"strconv"
	"time"
	"io"
	"path"
	"net"
	"net/http"
	"bytes"
	"strings"
)

const (
	missingCommand = "Missing command."
	invalidCommand = "Invalid command."
	missingAddress = "Missing address."
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

func serve() {
	if len(os.Args) < 4 {
		log.Fatalln(missingAddress)
	}
	addr := os.Args[2]
	readyMsg := os.Args[3]
	if len(os.Args) <= 4 || os.Args[4] != "no-delete" {
		err := os.Remove(addr)
		if err != nil && !os.IsNotExist(err) {
			log.Fatalln(err)
		}
	}
	l, err := net.Listen("unixpacket", addr)
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var (
				status int
				err error
			)
			if status, err = strconv.Atoi(path.Base(r.URL.Path)); err != nil {
				status = 200
			}
			buf := bytes.NewBuffer(nil)
			_, err = io.Copy(buf, r.Body)
			if err != nil {
				log.Fatalln(err)
			}
			w.Header().Set("Content-Length", fmt.Sprint(buf.Len()))
			for k, v := range r.Header {
				if strings.Index(k, "X-") != 0 {
					continue
				}
				w.Header()[k] = v
			}
			w.WriteHeader(status)
			_, err = io.Copy(w, buf)
			if err != nil {
				log.Fatalln(err)
			}
		}))
	}()
	fmt.Println(readyMsg)
	select {}
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
	case "serve":
		serve()
	}
}
