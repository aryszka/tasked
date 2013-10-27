package main

import (
	"os/user"
	"strconv"
	"testing"
)

func TestLookupGroupById(t *testing.T) {
	u, err := user.Current()
	gid, err := strconv.Atoi(u.Gid)
	errFatal(t, err)
	grp, err := lookupGroupById(uint32(gid))
	if err != nil || grp.id != uint32(gid) {
		t.Fail()
	}
	grp, err = lookupGroupById(1 << 31)
	if err == nil || grp != nil {
		t.Fail()
	}
}

func TestLookupGroupByName(t *testing.T) {
	u, err := user.Current()
	gid, err := strconv.Atoi(u.Gid)
	errFatal(t, err)
	grpv, err := lookupGroupById(uint32(gid))
	errFatal(t, err)
	grp, err := lookupGroupByName(grpv.name)
	if err != nil || grp.id != uint32(gid) || grp.name != grpv.name {
		t.Fail()
	}
	grp, err = lookupGroupByName("nouserlikethis")
	if err == nil || grp != nil {
		t.Fail()
	}
}
