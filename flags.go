package main

import "flag"

type flagsType struct {
	root   string
	config string
}

var flags flagsType

func parseFlags() {
	flag.StringVar(&flags.root, "root", "", "")
	flag.StringVar(&flags.config, "config", "", "")
	flag.Parse()
}
