package share

import (
	tst "code.google.com/p/tasked/testing"
	"os/user"
	"strconv"
	"testing"
)

func TestLookupGroupById(t *testing.T) {
	u, err := user.Current()
	gid, err := strconv.Atoi(u.Gid)
	tst.ErrFatal(t, err)
	grp, err := LookupGroupById(uint32(gid))
	if err != nil || grp.Id != uint32(gid) {
		t.Fail()
	}
	grp, err = LookupGroupById(1 << 31)
	if err == nil || grp != nil {
		t.Fail()
	}
}

func TestLookupGroupByName(t *testing.T) {
	u, err := user.Current()
	gid, err := strconv.Atoi(u.Gid)
	tst.ErrFatal(t, err)
	grpv, err := LookupGroupById(uint32(gid))
	tst.ErrFatal(t, err)
	grp, err := LookupGroupByName(grpv.Name)
	if err != nil || grp.Id != uint32(gid) || grp.Name != grpv.Name {
		t.Fail()
	}
	grp, err = LookupGroupByName("nouserlikethis")
	if err == nil || grp != nil {
		t.Fail()
	}
}
