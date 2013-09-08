package main

import (
	"code.google.com/p/tasked/app"
	"code.google.com/p/tasked/sec"
	"fmt"
	"os"
	"os/signal"
)

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
	err = app.Serve(cfg)
	if err != nil {
		printerrln(err)
		os.Exit(1)
	}
	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)
	<-sigint
}
