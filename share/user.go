package share

import (
	"os/user"
	"syscall"
)

var IsRoot bool

func init() {
	syscall.Umask(0077)
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	IsRoot = usr.Uid == "0"
}
