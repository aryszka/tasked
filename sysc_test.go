package main

import (
	"os/user"
	"strconv"
	"testing"
)

func TestLookupGroupName(t *testing.T) {
	u, err := user.Current()
	gid, err := strconv.Atoi(u.Gid)
	errFatal(t, err)
	_, err = lookupGroupName(uint32(gid))
	if err != nil {
		t.Fail()
	}
}
