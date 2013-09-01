package main

import (
	"code.google.com/p/tasked/sec"
	"os"
)

type options struct {
}

func main() {
	err := initConfig(&options{})
	if err != nil {
		os.Exit(1)
	}
	sec.Init(cfg)
}
