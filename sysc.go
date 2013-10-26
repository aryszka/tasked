package main

import (
	"errors"
	"syscall"
)

/*

#include <stdlib.h>
#include <unistd.h>
#include <grp.h>

int cgetgrgid(int gid, struct group *grp, char *buf, size_t buflen, struct group **result) {
	return getgrgid_r(gid, grp, buf, buflen, result);
}

*/
import "C"

func lookupGroupName(gid uint32) (string, error) {
	const maxGetGrpSize = 1024 // that's what she said
	var (
		grp C.struct_group
		res *C.struct_group
		bs  = C.size_t(maxGetGrpSize)
	)
	buf := C.malloc(bs)
	defer C.free(buf)
	ev := C.cgetgrgid(C.int(gid), &grp, (*C.char)(buf), bs, &res)
	if ev != 0 {
		return "", syscall.Errno(ev)
	}
	if res == nil {
		return "", errors.New("Unknown group id.")
	}
	return C.GoString(grp.gr_name), nil
}
