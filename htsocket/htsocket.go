package htsocket

import "code.google.com/p/tasked/share"

type Settings interface {}

func New(s Settings) share.HttpFilter {
	return nil
}
