package main

import (
	"code.google.com/p/tasked/app"
)

type options struct {
}

func main() {
	initConfig(&options{})
	app.InitSec(cfg.aes.key, cfg.aes.iv)
}
