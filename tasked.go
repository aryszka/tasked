package main

import (
    "taskedserver.com/repo/tasked/app"
)

type options struct {
}

func main() {
    initConfig(&options{})
    app.InitSec(cfg.aes.key, cfg.aes.iv)
}
