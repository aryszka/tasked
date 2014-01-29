package share

import (
	"os/user"
	"syscall"
)

var IsRoot bool // make this a function, move the variable to testing

func init() {
	syscall.Umask(0077)
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	IsRoot = usr.Uid == "0"
}
