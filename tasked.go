package main

import (
	"code.google.com/p/tasked/app"
	"os"
)

type options struct {
}

func main() {
	err := initConfig(&options{})
	if err != nil {
		os.Exit(1)
	}
	err = app.InitSec(cfg.aes.key, cfg.aes.iv, cfg.auth.tokenValiditySecs)
	if err != nil {
		os.Exit(1)
	}
}
