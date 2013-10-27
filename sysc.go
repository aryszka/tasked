package main

import (
	"fmt"
	"syscall"
)

/*

#include <stdlib.h>
#include <unistd.h>
#include <grp.h>

int cgetgrgid(int gid, struct group *grp, char *buf, size_t buflen, struct group **result) {
	return getgrgid_r(gid, grp, buf, buflen, result);
}

int cgetgrnam(char* nam, struct group *grp, char *buf, size_t buflen, struct group **result) {
	return getgrnam_r(nam, grp, buf, buflen, result);
}

*/
import "C"

type group struct {
	id   uint32
	name string
}

func lookupGroup(
	getgrp func(*C.struct_group, *C.char, C.size_t, **C.struct_group) C.int,
	nf func() error) (*group, error) {
	const maxGetGrpSize = 1024 // sorry, sysconf
	var (
		grp C.struct_group
		res *C.struct_group
		bs  = C.size_t(maxGetGrpSize)
	)
	buf := C.malloc(bs)
	defer C.free(buf)
	ev := getgrp(&grp, (*C.char)(buf), bs, &res)
	if ev != 0 {
		return nil, syscall.Errno(ev)
	}
	if res == nil {
		return nil, nf()
	}
	return &group{id: uint32(grp.gr_gid), name: C.GoString(grp.gr_name)}, nil
}

func lookupGroupById(gid uint32) (*group, error) {
	return lookupGroup(func(grp *C.struct_group, buf *C.char, bs C.size_t, res **C.struct_group) C.int {
		return C.cgetgrgid(C.int(gid), grp, buf, bs, res)
	}, func() error {
		return fmt.Errorf("Unknown group id: %d.", gid)
	})
}

func lookupGroupByName(gn string) (*group, error) {
	return lookupGroup(func(grp *C.struct_group, buf *C.char, bs C.size_t, res **C.struct_group) C.int {
		return C.cgetgrnam(C.CString(gn), grp, buf, bs, res)
	}, func() error {
		return fmt.Errorf("Unknown group name: %s.", gn)
	})
}
