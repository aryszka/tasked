package htfile

import (
	"bytes"
	"code.google.com/p/tasked/share"
	tst "code.google.com/p/tasked/testing"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type testOptions struct {
	root             string
	maxRequestBody   int64
	maxSearchResults int
}

func (ts *testOptions) Root() string          { return ts.root }
func (ts *testOptions) MaxRequestBody() int64 { return ts.maxRequestBody }
func (ts *testOptions) MaxSearchResults() int { return ts.maxSearchResults }

var (
	dn           string
	propsRoot    bool
	modpropsRoot bool
	getDirRoot   bool
)

func init() {
	tpr := flag.Bool("test.propsroot", false, "")
	tmpr := flag.Bool("test.modpropsroot", false, "")
	tgdr := flag.Bool("test.getdirroot", false, "")
	flag.Parse()
	propsRoot = *tpr
	modpropsRoot = *tmpr
	getDirRoot = *tgdr

	dn = path.Join(tst.Testdir, "http")
	err := share.EnsureDir(dn)
	if err != nil {
		panic(err)
	}
}

type fileInfoT struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (fi *fileInfoT) Name() string       { return fi.name }
func (fi *fileInfoT) Size() int64        { return fi.size }
func (fi *fileInfoT) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfoT) ModTime() time.Time { return fi.modTime }
func (fi *fileInfoT) IsDir() bool        { return fi.isDir }
func (fi *fileInfoT) Sys() interface{}   { return fi.sys }

func compareProperties(left, right map[string]interface{}) bool {
	compareString := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalString, ok := lval.(string)
			if !ok {
				return false
			}
			rvalString, ok := right[key].(string)
			if !ok || rvalString != lvalString {
				return false
			}
		}
		return true
	}
	compareInt64 := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalInt, ok := lval.(int64)
			if !ok {
				return false
			}
			rvalInt, ok := right[key].(int64)
			if !ok || rvalInt != lvalInt {
				return false
			}
		}
		return true
	}
	compareFileMode := func(key string) bool { // make it int32
		lval, ok := left[key]
		if ok {
			lvalInt, ok := lval.(os.FileMode)
			if !ok {
				return false
			}
			rvalInt, ok := right[key].(os.FileMode)
			if !ok || rvalInt != lvalInt {
				return false
			}
		}
		return true
	}
	compareBool := func(key string) bool {
		lval, ok := left[key]
		if ok {
			lvalBool, ok := lval.(bool)
			if !ok {
				return false
			}
			rvalBool, ok := right[key].(bool)
			if !ok || rvalBool != lvalBool {
				return false
			}
		}
		return true
	}
	if len(left) != len(right) ||
		!compareString("name") ||
		!compareInt64("size") ||
		!compareInt64("modTime") ||
		!compareBool("isDir") ||
		!compareFileMode("mode") ||
		!compareString("user") ||
		!compareString("group") ||
		!compareInt64("accessTime") ||
		!compareInt64("changeTime") {
		return false
	}
	return true
}

func isPermission(err error) bool {
	if err == nil {
		return false
	}
	if perr, ok := err.(*os.PathError); ok {
		if errno, ok := perr.Err.(syscall.Errno); ok && (errno == syscall.EPERM || errno == syscall.EACCES) {
			return true
		}
	}
	if serr, ok := err.(*os.SyscallError); ok {
		if errno, ok := serr.Err.(syscall.Errno); ok && errno == syscall.EPERM {
			return true
		}
	}
	return false
}

func convert64(m map[string]interface{}, n string) (ok bool) {
	var (
		v interface{}
		f float64
	)
	if v, ok = m[n]; !ok || v == nil {
		return !ok
	}
	if f, ok = v.(float64); !ok {
		return false
	}
	m[n] = int64(f)
	return true
}

func convertFm(pr map[string]interface{}) (ok bool) {
	fv, ok := pr["mode"].(float64)
	if !ok {
		return false
	}
	pr["mode"] = os.FileMode(fv)
	return true
}

func verifyHeader(expect, have map[string][]string) bool {
	for k, vs := range expect {
		vvs, ok := have[k]
		if !ok || len(vs) != len(vvs) {
			return false
		}
		for _, v := range vs {
			found := false
			for _, vv := range vvs {
				if v == vv {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func newHandler(t *testing.T, o Options) http.Handler {
	h, err := New(o)
	tst.ErrFatal(t, err)
	return h
}

func TestToPropertyMap(t *testing.T) {
	var (
		defaultTime time.Time
		defaultMode os.FileMode
	)
	p := toPropertyMap(&fileInfoT{}, false)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false}) {
		t.Fail()
	}
	now := time.Now()
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, false)
	if !compareProperties(p, map[string]interface{}{
		"name":    "some",
		"size":    int64(42),
		"modTime": now.Unix(),
		"isDir":   true}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    defaultMode}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		mode:    os.ModePerm,
		modTime: now,
		isDir:   true}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "some",
		"size":    int64(42),
		"modTime": now.Unix(),
		"isDir":   true,
		"mode":    os.ModePerm}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfoT{
		mode: os.ModePerm + 1024}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    os.ModePerm}) {
		t.Fail()
	}
	p = toPropertyMap(&fileInfo{
		sys: &fileInfoT{mode: os.ModePerm + 1024}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":    "",
		"size":    int64(0),
		"modTime": defaultTime.Unix(),
		"isDir":   false,
		"mode":    os.ModePerm,
		"dirname": "/"}) {
		t.Fail()
	}
	u, err := user.Current()
	tst.ErrFatal(t, err)
	uid, err := strconv.Atoi(u.Uid)
	gid, err := strconv.Atoi(u.Gid)
	tst.ErrFatal(t, err)
	g, err := share.LookupGroupById(uint32(gid))
	tst.ErrFatal(t, err)
	p = toPropertyMap(&fileInfoT{
		name:    "some",
		size:    42,
		modTime: defaultTime,
		isDir:   false,
		mode:    os.ModePerm,
		sys: &syscall.Stat_t{
			Uid:  uint32(uid),
			Gid:  uint32(gid),
			Atim: syscall.Timespec{Sec: defaultTime.Unix() + 42},
			Ctim: syscall.Timespec{Sec: defaultTime.Unix() + 42<<1}}}, true)
	if !compareProperties(p, map[string]interface{}{
		"name":       "some",
		"size":       int64(42),
		"modTime":    defaultTime.Unix(),
		"isDir":      false,
		"mode":       os.ModePerm,
		"user":       u.Username,
		"group":      g.Name,
		"accessTime": defaultTime.Unix() + 42,
		"changeTime": defaultTime.Unix() + 42<<1}) {
		t.Fail()
	}
}

func TestPathIntersect(t *testing.T) {
	// equal length, not equal
	s0, s1 := "some/one", "some/two"
	if pathIntersect(s0, s1) != 0 || pathIntersect(s1, s0) != 0 {
		t.Fail()
	}

	// equal length, equal
	s0, s1 = "some/path", "some/path"
	if pathIntersect(s0, s1) != 3 {
		t.Fail()
	}

	// not equal length, not intersect
	s0, s1 = "some/path", "some/pathbutdifferent"
	if pathIntersect(s0, s1) != 0 || pathIntersect(s1, s0) != 0 {
		t.Fail()
	}

	// not equal length, intersect
	s0, s1 = "some/path", "some/path/inside"
	if pathIntersect(s0, s1) != 2 || pathIntersect(s1, s0) != 1 {
		t.Fail()
	}
}

func TestIsOwner(t *testing.T) {
	cu, err := user.Current()
	tst.ErrFatal(t, err)
	cui, err := strconv.Atoi(cu.Uid)
	tst.ErrFatal(t, err)
	cuui := uint32(cui)

	fi := &fileInfoT{sys: &syscall.Stat_t{Uid: cuui}}
	is, err := isOwner(cu, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	cu, err := user.Current()
	tst.ErrFatal(t, err)
	fi := &fileInfoT{}
	is, err := isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(cu.Uid)
	tst.ErrFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(cu, fi)
	if is || err != nil {
		t.Fail()
	}
}

func TestIsOwnerRoot(t *testing.T) {
	if !share.IsRoot {
		t.Skip()
	}

	fi := &fileInfoT{}
	u, err := user.Current()
	tst.ErrFatal(t, err)
	is, err := isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}

	cui, err := strconv.Atoi(u.Uid)
	tst.ErrFatal(t, err)
	cuui := uint32(cui)
	fi = &fileInfoT{sys: &syscall.Stat_t{Uid: cuui + 1}}
	is, err = isOwner(u, fi)
	if !is || err != nil {
		t.Fail()
	}
}

func TestDetectContentType(t *testing.T) {
	ct, err := detectContentType("some.html", nil)
	if err != nil || ct != textMimeTypes["html"] {
		t.Fail()
	}
	p := path.Join(tst.Testdir, "some-file")
	err = tst.WithNewFile(p, func(f *os.File) error {
		_, err := f.Write([]byte("This suppose to be some human readable text."))
		return err
	})
	tst.ErrFatal(t, err)
	f, err := os.Open(p)
	tst.ErrFatal(t, err)
	ct, err = detectContentType("some-file", f)
	if err != nil || ct != textMimeTypes["txt"] {
		t.Fail()
	}
}

func TestSearchFiles(t *testing.T) {
	p := path.Join(tst.Testdir, "search")
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	di, err := os.Lstat(p)
	tst.ErrFatal(t, err)
	dii := &fileInfo{sys: di, dirname: tst.Testdir}
	tst.WithNewFileF(t, path.Join(p, "file0"), nil)
	tst.WithNewFileF(t, path.Join(p, "file1"), nil)

	// 0 max
	if len(searchFiles([]*fileInfo{dii}, 0, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// negative max
	if len(searchFiles([]*fileInfo{dii}, -1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// no root dirs
	if len(searchFiles(nil, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// not existing root dirs
	tst.RemoveIfExistsF(t, p)
	if len(searchFiles([]*fileInfo{dii}, 1, func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// predicate always false
	tst.EnsureDirF(t, p)
	di, err = os.Lstat(p)
	tst.ErrFatal(t, err)
	dii = &fileInfo{sys: di, dirname: tst.Testdir}
	tst.WithNewFileF(t, path.Join(p, "file0"), nil)
	tst.WithNewFileF(t, path.Join(p, "file1"), nil)
	if len(searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return false })) != 0 {
		t.Fail()
	}

	// predicate always true
	result := searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return true })
	if len(result) != 2 {
		t.Fail()
	}

	// root is a file
	fi, err := os.Lstat(path.Join(p, "file0"))
	tst.ErrFatal(t, err)
	if len(searchFiles([]*fileInfo{&fileInfo{sys: fi, dirname: p}}, 42,
		func(_ *fileInfo) bool { return true })) != 0 {
		t.Fail()
	}

	// find all in a recursively
	tst.EnsureDirF(t, p)
	di, err = os.Lstat(p)
	tst.ErrFatal(t, err)
	dii = &fileInfo{sys: di, dirname: tst.Testdir}
	tst.WithNewFileF(t, path.Join(p, "file0"), nil)
	tst.WithNewFileF(t, path.Join(p, "file1"), nil)
	p0 := path.Join(p, "dir0")
	tst.EnsureDirF(t, p0)
	tst.WithNewFileF(t, path.Join(p0, "file0"), nil)
	tst.WithNewFileF(t, path.Join(p0, "file1"), nil)
	result = searchFiles([]*fileInfo{dii}, 42, func(_ *fileInfo) bool { return true })
	if len(result) != 5 {
		t.Fail()
	}
	m := make(map[string]int)
	for _, fii := range result {
		m[path.Join(fii.dirname, fii.Name())] = 1
	}
	if len(m) != len(result) {
		t.Fail()
	}

	// find max, breadth first
	result = searchFiles([]*fileInfo{dii}, 4, func(_ *fileInfo) bool { return true })
	if len(result) != 4 {
		t.Fail()
	}
	var oneOfThem bool
	for _, fii := range result {
		if path.Join(fii.dirname, fii.Name()) == path.Join(p0, "file0") ||
			path.Join(fii.dirname, fii.Name()) == path.Join(p0, "file1") {
			if oneOfThem {
				t.Fail()
			}
			oneOfThem = true
		}
	}

	// find all, filtered
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return fi.Name() == "file0"
	})) != 2 {
		t.Fail()
	}
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "file") == 0
	})) != 4 {
		t.Fail()
	}

	// find max, filtered, step levels
	// don't count max for stepping directories
	// (failure was: Readdir(max))
	p1 := path.Join(p, "dir1")
	tst.EnsureDirF(t, p1)
	if len(searchFiles([]*fileInfo{dii}, 2, func(fi *fileInfo) bool {
		if fi.Name() == "dir0" {
			err = os.Rename(path.Join(p0, "file0"), path.Join(p1, "file0"))
			tst.ErrFatal(t, err)
			return false
		}
		return fi.Name() == "file0"
	})) != 2 {
		t.Fail()
	}
	err = os.Rename(path.Join(p1, "file0"), path.Join(p1, "file0"))
	tst.ErrFatal(t, err)
	err = os.RemoveAll(p1)
	tst.ErrFatal(t, err)

	// find all dirs, too
	p1 = path.Join(p0, "dir1")
	tst.EnsureDirF(t, p1)
	p2 := path.Join(p0, "dir2")
	tst.EnsureDirF(t, p2)
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool {
		return strings.Index(fi.Name(), "dir") == 0
	})) != 3 {
		t.Fail()
	}

	// verify data
	tst.WithNewFileF(t, path.Join(p, "file1"), func(f *os.File) error {
		_, err := f.Write([]byte("012345678"))
		return err
	})
	tst.WithNewFileF(t, path.Join(p0, "file0"), func(f *os.File) error {
		_, err := f.Write([]byte("012345"))
		return err
	})
	sizeOfADir := dii.Size()
	vm := map[string]map[string]interface{}{
		path.Join(p, "dir0"): map[string]interface{}{
			"name":    "dir0",
			"dirname": p,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p0, "dir1"): map[string]interface{}{
			"name":    "dir1",
			"dirname": p0,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p0, "dir2"): map[string]interface{}{
			"name":    "dir2",
			"dirname": p0,
			"isDir":   true,
			"size":    sizeOfADir},
		path.Join(p, "file0"): map[string]interface{}{
			"name":    "file0",
			"dirname": p,
			"isDir":   false,
			"size":    int64(0)},
		path.Join(p, "file1"): map[string]interface{}{
			"name":    "file1",
			"dirname": p,
			"isDir":   false,
			"size":    int64(9)},
		path.Join(p0, "file0"): map[string]interface{}{
			"name":    "file0",
			"dirname": p0,
			"isDir":   false,
			"size":    int64(6)},
		path.Join(p0, "file1"): map[string]interface{}{
			"name":    "file1",
			"dirname": p0,
			"isDir":   false,
			"size":    int64(0)}}
	result = searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool { return true })
	for _, fi := range result {
		fp := path.Join(fi.dirname, fi.Name())
		if fi.Name() != vm[fp]["name"].(string) ||
			fi.dirname != vm[fp]["dirname"].(string) ||
			fi.IsDir() != vm[fp]["isDir"].(bool) ||
			fi.Size() != vm[fp]["size"].(int64) {
			t.Fail()
		}
	}

	// verify breadth first, order
	level0 := result[:3]
	for _, fi := range level0 {
		if fi.dirname != p {
			t.Fail()
		}
	}
	result = searchFiles([]*fileInfo{dii}, 5, func(fi *fileInfo) bool { return true })
	level0 = result[:3]
	for _, fi := range level0 {
		if fi.dirname != p {
			t.Fail()
		}
	}
	result = searchFiles([]*fileInfo{dii}, 2, func(fi *fileInfo) bool { return true })
	for _, fi := range result {
		if fi.dirname != p {
			t.Fail()
		}
	}
}

func TestSearchFilesNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	p := path.Join(tst.Testdir, "search")
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	p0 := path.Join(p, "dir0")
	tst.EnsureDirF(t, p0)
	tst.WithNewFileF(t, path.Join(p, "file0"), nil)
	tst.WithNewFileF(t, path.Join(p, "file1"), nil)
	di, err := os.Lstat(p)
	tst.ErrFatal(t, err)
	dii := &fileInfo{sys: di, dirname: tst.Testdir}

	// not finding when no rights for dir
	err = os.Chmod(p0, 0)
	tst.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(p0, os.ModePerm)
		tst.ErrFatal(t, err)
	}()
	if len(searchFiles([]*fileInfo{dii}, 42, func(fi *fileInfo) bool { return true })) != 3 {
		t.Fail()
	}
}

func TestCopyTree(t *testing.T) {
	dir := path.Join(dn, "copy-tree")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)

	// no copy of the same path
	fi0, err := os.Lstat(dir)
	tst.ErrFatal(t, err)
	err = copyTree(dir, dir)
	if err != nil {
		t.Fail()
	}
	fi1, err := os.Lstat(dir)
	tst.ErrFatal(t, err)
	if !fi1.ModTime().Equal(fi0.ModTime()) {
		t.Fail()
	}

	// not found
	dir0 := path.Join(dir, "dir0")
	err = tst.RemoveIfExists(dir0)
	dir1 := path.Join(dir, "dir1")
	tst.RemoveIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if err == nil || !os.IsNotExist(err) {
		t.Fail()
	}

	// tree
	tst.EnsureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	tst.WithNewFileF(t, fp, nil)
	tst.RemoveIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if err != nil {
		t.Fail()
	}
	_, err = os.Lstat(path.Join(dir1, "some"))
	if err != nil {
		t.Fail()
	}

	// copy file
	f0, f1 := path.Join(dir, "file0"), path.Join(dir, "file1")
	tst.WithNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	tst.RemoveIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	cf, err := os.Open(f1)
	tst.ErrFatal(t, err)
	defer cf.Close()
	ccf, err := ioutil.ReadAll(cf)
	tst.ErrFatal(t, err)
	if !bytes.Equal(ccf, []byte("some content")) {
		t.Fail()
	}
}

func TestCopyTreeNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	dir := path.Join(dn, "copy")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)

	// no access
	dir0 := path.Join(dir, "dir0")
	tst.EnsureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	tst.RemoveIfExistsF(t, dir1)
	err := os.Chmod(dir, 0600)
	tst.ErrFatal(t, err)
	err = copyTree(dir0, dir1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, os.ModePerm)
	tst.ErrFatal(t, err)

	// copy abort
	tst.EnsureDirF(t, dir0)
	fp := path.Join(dir0, "some")
	tst.WithNewFileF(t, fp, nil)
	err = os.Chmod(fp, 0000)
	tst.RemoveIfExistsF(t, dir1)
	err = copyTree(dir0, dir1)
	if !isPermission(err) {
		t.Fail()
	}
	_, err = os.Lstat(dir1)
	if err != nil {
		t.Fail()
	}
	_, err = os.Lstat(path.Join(dir1, "some"))
	if !os.IsNotExist(err) {
		t.Fail()
	}
	err = os.Chmod(fp, 0666)
	tst.ErrFatal(t, err)

	// copy file, no write permission
	f0, f1 := path.Join(dir, "file0"), path.Join(dir, "file1")
	tst.WithNewFileF(t, f0, nil)
	tst.RemoveIfExistsF(t, f1)
	err = os.Chmod(dir, 0600)
	tst.ErrFatal(t, err)
	err = copyTree(f0, f1)
	if !isPermission(err) {
		t.Fail()
	}
	err = os.Chmod(dir, 0777)
	tst.ErrFatal(t, err)

	// preserve mode
	tst.WithNewFileF(t, f0, nil)
	err = os.Chmod(f0, 0400)
	tst.ErrFatal(t, err)
	tst.RemoveIfExistsF(t, f1)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
	fi, err := os.Lstat(f1)
	tst.ErrFatal(t, err)
	if fi.Mode() != os.FileMode(0400) {
		t.Fail()
	}

	// copy to not existing
	p0, p1 := path.Join(dir, "dir0"), path.Join(dir, "dir1")
	tst.EnsureDirF(t, p0)
	tst.RemoveIfExistsF(t, p1)
	err = copyTree(p0, p1)
	if err != nil {
		t.Fail()
	}

	// copy to existing file
	f0, f1 = path.Join(dir, "file0"), path.Join(dir, "file1")
	tst.WithNewFileF(t, f0, nil)
	tst.WithNewFileF(t, f1, nil)
	err = copyTree(f0, f1)
	if err != nil {
		t.Fail()
	}
}

func TestQryNum(t *testing.T) {
	qry := make(url.Values)
	n, err := getQryNum(qry, "some")
	if n != 0 || err != nil {
		t.Fail()
	}

	qry.Add("some", "0")
	qry.Add("some", "1")
	n, err = getQryNum(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "val")
	n, err = getQryNum(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "42")
	n, err = getQryNum(qry, "some")
	if n != 42 || err != nil {
		t.Fail()
	}
}

func TestGetQryExpression(t *testing.T) {
	qry := make(url.Values)
	qry.Add("some", "val0")
	qry.Add("some", "val1")
	x, err := getQryExpression(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	x, err = getQryExpression(qry, "some")
	if x != nil || err != nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "")
	x, err = getQryExpression(qry, "some")
	if x != nil || err != nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "(")
	x, err = getQryExpression(qry, "some")
	if err == nil {
		t.Fail()
	}

	qry = make(url.Values)
	qry.Add("some", "val")
	x, err = getQryExpression(qry, "some")
	if x == nil || err != nil {
		t.Fail()
	}
}

func TestGetPath(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	p, err := ht.getPath("..")
	if err == nil {
		t.Fail()
	}
	p, err = ht.getPath("some")
	if err != nil || p != path.Join(dn, "some") {
		t.Fail()
	}
}

func TestSearchf(t *testing.T) {
	var queryString url.Values
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.searchf(w, r, queryString)
	}

	p := path.Join(dn, "search")
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)

	checkError := func(err int) {
		tst.Htreqx(t, "SEARCH", tst.S.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != err {
				t.Fail()
			}
		})
	}
	checkBadReq := func() { checkError(http.StatusBadRequest) }
	checkLen := func(l int) {
		tst.Htreqx(t, "SEARCH", tst.S.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			js, err := ioutil.ReadAll(rsp.Body)
			tst.ErrFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			tst.ErrFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// max results set
	for i := 0; i < 42; i++ {
		pi := path.Join(p, fmt.Sprintf("file%d", i))
		tst.WithNewFileF(t, pi, nil)
	}
	queryString = make(url.Values)
	checkLen(defaultMaxSearchResults)
	queryString.Set("max", "42")
	checkLen(defaultMaxSearchResults)
	queryString.Set("max", "3")
	checkLen(3)

	// only one name
	queryString = make(url.Values)
	queryString.Add("name", "some")
	queryString.Add("name", "someOther")
	checkBadReq()

	// only one content
	queryString = make(url.Values)
	queryString.Add("content", "some")
	queryString.Add("content", "someOther")
	checkBadReq()

	// invalid regexp
	queryString = make(url.Values)
	queryString.Set("name", "(")
	checkBadReq()

	// not found
	queryString = make(url.Values)
	err := os.RemoveAll(p)
	tst.ErrFatal(t, err)
	checkError(http.StatusNotFound)

	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	tst.WithNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	tst.WithNewFileF(t, path.Join(p, "fileB"), func(f *os.File) error {
		_, err := f.Write([]byte("b"))
		return err
	})
	tst.EnsureDirF(t, path.Join(p, "dirA"))

	// filtering by name
	queryString = make(url.Values)
	queryString.Set("name", "A")
	checkLen(2)

	// filtering by content
	queryString = make(url.Values)
	queryString.Set("content", "b")
	checkLen(1)

	// filtering by name and content
	queryString = make(url.Values)
	queryString.Set("name", "A")
	queryString.Set("content", "a")
	checkLen(1)
	queryString = make(url.Values)
	queryString.Set("name", "A")
	queryString.Set("content", "b")
	checkLen(0)

	// check data
	fia, err := os.Lstat(path.Join(p, "fileA"))
	tst.ErrFatal(t, err)
	fib, err := os.Lstat(path.Join(p, "fileB"))
	tst.ErrFatal(t, err)
	fid, err := os.Lstat(path.Join(p, "dirA"))
	tst.ErrFatal(t, err)
	mts := make(map[string]int64)
	mts["fileA"] = fia.ModTime().Unix()
	mts["fileB"] = fib.ModTime().Unix()
	mts["dirA"] = fid.ModTime().Unix()
	sizeOfADir := fid.Size()
	queryString = make(url.Values)
	tst.Htreqx(t, "SEARCH", tst.S.URL+"/search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		tst.ErrFatal(t, err)
		if len(m) != 3 {
			t.Fail()
		}
		for _, pr := range m {
			convert64(pr, "size")
			convert64(pr, "modTime")
			n, ok := pr["name"].(string)
			if !ok {
				t.Fail()
			}
			if !compareProperties(pr, map[string]interface{}{
				"dirname": p,
				"name":    n,
				"isDir":   n == "dirA",
				"size": func() int64 {
					if n == "dirA" {
						return sizeOfADir
					} else {
						return int64(1)
					}
				}(),
				"modTime": mts[n]}) {
				t.Fail()
			}
		}
	})
}

func TestSearchNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	var queryString url.Values
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.searchf(w, r, queryString)
	}

	p := path.Join(dn, "search")
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)

	checkLen := func(l int) {
		tst.Htreqx(t, "SEARCH", tst.S.URL+"/search", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			js, err := ioutil.ReadAll(rsp.Body)
			tst.ErrFatal(t, err)
			var m []map[string]interface{}
			err = json.Unmarshal(js, &m)
			tst.ErrFatal(t, err)
			if len(m) != l {
				t.Fail()
			}
		})
	}

	// no permissions
	tst.EnsureDirF(t, p)
	err := os.Chmod(p, 0)
	tst.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(p, os.ModePerm)
		tst.ErrFatal(t, err)
	}()
	checkLen(0)

	// filtering by content, no rights
	err = os.Chmod(p, 0777)
	tst.ErrFatal(t, err)
	tst.WithNewFileF(t, path.Join(p, "fileA"), func(f *os.File) error {
		_, err := f.Write([]byte("a"))
		return err
	})
	err = os.Chmod(path.Join(p, "fileA"), 0)
	tst.ErrFatal(t, err)
	defer func() {
		err = os.Chmod(path.Join(p, "fileA"), os.ModePerm)
		tst.ErrFatal(t, err)
	}()
	queryString = make(url.Values)
	queryString.Set("content", "a")
	checkLen(0)
}

func TestPropsf(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.propsf(w, r)
	}

	fn := "some-file"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn

	tst.RemoveIfExistsF(t, p)
	tst.Htreqx(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	tst.Htreqx(t, "PROPS", tst.S.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	tst.WithNewFileF(t, p, nil)
	fiVerify, err := os.Stat(p)
	tst.ErrFatal(t, err)
	prVerify := toPropertyMap(fiVerify, true)
	jsVerify, err := json.Marshal(prVerify)
	tst.ErrFatal(t, err)

	tst.Htreqx(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(share.HeaderContentType) != share.JsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
		tst.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		tst.ErrFatal(t, err)
		if !convert64(pr, "modTime") || !convert64(pr, "size") ||
			!convert64(pr, "accessTime") || !convert64(pr, "changeTime") ||
			!convertFm(pr) {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Fail()
		}
	})

	tst.WithNewFileF(t, p, nil)
	tst.Htreqx(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			rsp.Header.Get(share.HeaderContentType) != share.JsonContentType {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
		tst.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if len(js) != 0 {
			t.Fail()
		}
	})
}

func TestPropsfRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !share.IsRoot || !propsRoot {
		t.Skip()
	}

	t.Parallel()
	tst.Mx.Lock()
	defer tst.Mx.Unlock()

	err := os.Chmod(tst.Testdir, 0777)
	tst.ErrFatal(t, err)
	err = os.Chmod(dn, 0777)
	tst.ErrFatal(t, err)
	fn := "some-file-uid"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn
	tst.WithNewFileF(t, p, nil)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.propsf(w, r)
	}

	// uid := syscall.Getuid()
	usr, err := user.Lookup(tst.Testuser)
	tst.ErrFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	tst.ErrFatal(t, err)
	err = syscall.Setuid(tuid)
	tst.ErrFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	tst.ErrFatal(t, err)
	// }()

	fiVerify, err := os.Stat(p)
	tst.ErrFatal(t, err)
	prVerify := toPropertyMap(fiVerify, false)
	jsVerify, err := json.Marshal(prVerify)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "PROPS", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
		tst.ErrFatal(t, err)
		if len(jsVerify) != clen {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(js, jsVerify) {
			t.Fail()
		}
		var pr map[string]interface{}
		err = json.Unmarshal(js, &pr)
		tst.ErrFatal(t, err)
		if !convert64(pr, "modTime") || !convert64(pr, "size") {
			t.Fail()
		}
		if !compareProperties(pr, prVerify) {
			t.Fail()
		}
		_, ok := pr["mode"]
		if ok {
			t.Fail()
		}
		_, ok = pr["owner"]
		if ok {
			t.Fail()
		}
		_, ok = pr["group"]
		if ok {
			t.Fail()
		}
	})
}

func TestModpropsf(t *testing.T) {
	fn := "some-file"
	p := path.Join(dn, fn)
	tst.WithNewFileF(t, p, nil)
	st := &testOptions{root: dn, maxRequestBody: defaultMaxRequestBody}
	ht := newHandler(t, st).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.modpropsf(w, r)
	}

	// max req length
	st.maxRequestBody = 8
	ht = newHandler(t, st).(*handler)
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"something\": \"long enough\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fail()
			}
		})
	st.maxRequestBody = defaultMaxRequestBody
	ht = newHandler(t, st).(*handler)

	// json number
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"mode\":  \"not a number\"}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"mode\": 0.1}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"mode\": -2}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "MODPROPS", tst.S.URL+"/"+fn, tst.NewByteReaderString(fmt.Sprintf("{\"mode\": %d}", 0600)),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	err := os.Chmod(p, 0600)
	tst.ErrFatal(t, err)

	// valid json
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("not json"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// map only or nil
	tst.Htreqx(t, "MODPROP", tst.S.URL, tst.NewByteReaderString("[]"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("null"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	tst.Htreqx(t, "MODPROPS", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// one map only
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"mode\": 0}{\"mode\": 0}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// valid fields only
	tst.Htreqx(t, "MODPROPS", tst.S.URL, tst.NewByteReaderString("{\"some\": \"value\"}"), func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// not found
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		tst.RemoveIfExistsF(t, p)
		htr(t, "MODPROPS", tst.S.URL+"/"+fn, tst.NewByteReaderString("{\"mode\":0}"), func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusNotFound {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// bad req, path
	tst.Htreqx(t, "MODPROPS", tst.S.URL+"/"+string([]byte{0}), tst.NewByteReaderString("{\"t\":0}"),
		func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusBadRequest {
				t.Fail()
			}
		})

	// mod, success
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		tst.WithNewFileF(t, p, nil)
		err = os.Chmod(p, os.ModePerm)
		tst.ErrFatal(t, err)
		htr(t, "MODPROPS", tst.S.URL+"/"+fn, tst.NewByteReaderString(fmt.Sprintf("{\"mode\": %d}", 0744)),
			func(rsp *http.Response) {
				if rsp.StatusCode != http.StatusOK {
					t.Fail()
				}
				fi, err := os.Stat(p)
				tst.ErrFatal(t, err)
				if fi.Mode() != os.FileMode(0744) {
					t.Fail()
				}
			})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// mod, success, masked
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		err = os.Chmod(p, os.ModePerm)
		tst.ErrFatal(t, err)
		htr(t, "MODPROPS", tst.S.URL+"/"+fn, tst.NewByteReaderString(fmt.Sprintf("{\"mode\": %d}", 01744)),
			func(rsp *http.Response) {
				if rsp.StatusCode != http.StatusOK {
					t.Fail()
				}
				fi, err := os.Stat(p)
				tst.ErrFatal(t, err)
				if fi.Mode() != os.FileMode(0744) {
					t.Fail()
				}
			})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestModpropsRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !share.IsRoot || !modpropsRoot {
		t.Skip()
	}

	t.Parallel()
	tst.Mx.Lock()
	defer tst.Mx.Unlock()

	fn := "some-file-uid-mod"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn

	usr, err := user.Lookup(tst.Testuser)
	tst.ErrFatal(t, err)

	err = os.Chmod(dn, os.ModePerm)
	tst.ErrFatal(t, err)
	err = os.Chmod(tst.Testdir, os.ModePerm)
	tst.ErrFatal(t, err)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.modpropsf(w, r)
	}

	// chown
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		tst.WithNewFileF(t, p, nil)
		htr(t, "MODPROPS", url, bytes.NewBufferString(fmt.Sprintf("{\"owner\": \"%s\"}", tst.Testuser)),
			func(rsp *http.Response) {
				if rsp.StatusCode != http.StatusOK {
					t.Fail()
				}
				fi, err := os.Lstat(p)
				tst.ErrFatal(t, err)
				sstat, ok := fi.Sys().(*syscall.Stat_t)
				if !ok {
					t.Fatal()
				}
				if strconv.Itoa(int(sstat.Uid)) != usr.Uid {
					t.Fail()
				}
			})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// uid := syscall.Getuid()
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		tst.WithNewFileF(t, p, nil)
		err = os.Chmod(p, os.ModePerm)
		tst.ErrFatal(t, err)
		tuid, err := strconv.Atoi(usr.Uid)
		tst.ErrFatal(t, err)
		err = syscall.Setuid(tuid)
		tst.ErrFatal(t, err)

		htr(t, "MODPROPS", url,
			bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0744)),
			func(rsp *http.Response) {
				if rsp.StatusCode != http.StatusNotFound {
					t.Fail()
				}
				fi, err := os.Lstat(p)
				tst.ErrFatal(t, err)
				if fi.Mode() != os.ModePerm {
					t.Fail()
				}
			})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestGetDir(t *testing.T) {
	fn := "some-dir"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn
	var d *os.File
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		tst.WithNewFileF(t, path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
	}

	// not found
	tst.EnsureDirF(t, p)
	err := os.Chmod(p, os.ModePerm)
	tst.ErrFatal(t, err)
	d, err = os.Open(p)
	tst.ErrFatal(t, err)
	tst.RemoveIfExistsF(t, p)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if strings.Trim(string(b), "\n") != http.StatusText(http.StatusNotFound) {
			t.Fail()
		}
	})

	// empty dir
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	d, err = os.Open(p)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var res []interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 0 {
			t.Fail()
		}
	})

	// dir with files
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var res []map[string]interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 3 {
			t.Fail()
		}
		for _, m := range res {
			n, ok := m["name"].(string)
			if !ok {
				t.Fail()
			}
			switch n {
			case "some0", "some1", "some2":
				if !convert64(m, "modTime") || !convert64(m, "size") ||
					!convert64(m, "accessTime") || !convert64(m, "changeTime") ||
					!convertFm(m) {
					t.Fail()
				}
				fi, err := os.Stat(path.Join(p, n))
				tst.ErrFatal(t, err)
				if !compareProperties(m, toPropertyMap(fi, true)) {
					t.Fail()
				}
			default:
				t.Fail()
			}
		}
	})

	// tests the same with HEAD
	tst.RemoveIfExistsF(t, p)
	tst.EnsureDirF(t, p)
	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})
	d, err = os.Open(p)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "HEAD", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if len(b) > 0 {
			t.Fail()
		}
	})
}

func TestGetDirRoot(t *testing.T) {
	// Tests using Setuid cannot be run together until they're replaced by Seteuid
	if !share.IsRoot || !getDirRoot {
		t.Skip()
	}

	t.Parallel()
	tst.Mx.Lock()
	defer tst.Mx.Unlock()

	err := os.Chmod(tst.Testdir, 0777)
	tst.ErrFatal(t, err)
	err = os.Chmod(dn, 0777)
	tst.ErrFatal(t, err)

	fn := "some-dir"
	p := path.Join(dn, fn)
	tst.EnsureDirF(t, p)
	err = os.Chmod(p, 0777)
	tst.ErrFatal(t, err)
	url := tst.S.URL + "/" + fn
	var d *os.File
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getDir(w, r, d)
	}
	mkfile := func(n string, c []byte) {
		tst.WithNewFileF(t, path.Join(p, n), func(f *os.File) error {
			n, err := f.Write(c)
			if n != len(c) {
				return errors.New("Failed to write all bytes.")
			}
			return err
		})
	}

	mkfile("some0", nil)
	mkfile("some1", []byte{0})
	mkfile("some2", []byte{0, 0})

	// uid := syscall.Getuid()
	usr, err := user.Lookup(tst.Testuser)
	tst.ErrFatal(t, err)
	tuid, err := strconv.Atoi(usr.Uid)
	tst.ErrFatal(t, err)
	tgid, err := strconv.Atoi(usr.Gid)
	tst.ErrFatal(t, err)
	err = os.Chown(path.Join(p, "some1"), tuid, tgid)
	tst.ErrFatal(t, err)
	err = syscall.Setuid(tuid)
	tst.ErrFatal(t, err)

	// makes no sense at the moment
	// defer func() {
	// 	err = syscall.Setuid(uid)
	// 	tst.ErrFatal(t, err)
	// }()

	d, err = os.Open(p)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var res []map[string]interface{}
		err = json.Unmarshal(b, &res)
		if err != nil || len(res) > 3 {
			t.Fail()
		}
		for _, m := range res {
			n, ok := m["name"].(string)
			if !ok {
				t.Fail()
			}
			switch n {
			case "some0", "some1", "some2":
				if !convert64(m, "modTime") || !convert64(m, "size") ||
					!convert64(m, "accessTime") || !convert64(m, "changeTime") {
					t.Fail()
				}
				if n == "some1" && !convertFm(m) {
					t.Fail()
				}
				fi, err := os.Stat(path.Join(p, n))
				tst.ErrFatal(t, err)
				if !compareProperties(m, toPropertyMap(fi, n == "some1")) {
					t.Fail()
				}
			default:
				t.Fail()
			}
		}
	})
}

func TestGetFile(t *testing.T) {
	var (
		f    *os.File
		fi   os.FileInfo
		err  error
		html = []byte("<html></html>")
	)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.getFile(w, r, f, fi)
	}

	// extension tested
	fn := "some-file.html"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn
	tst.WithNewFileF(t, p, nil)
	f, err = os.Open(p)
	tst.ErrFatal(t, err)
	fi, err = f.Stat()
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.Header.Get(share.HeaderContentType) != "text/html; charset=utf-8" {
			t.Fail()
		}
	})
	err = f.Close()
	tst.ErrFatal(t, err)

	// content tested, length set, content sent
	ff := func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		fn = "some-file"
		p = path.Join(dn, fn)
		url = tst.S.URL + "/" + fn
		tst.WithNewFileF(t, p, func(f *os.File) error {
			_, err = f.Write(html)
			return err
		})
		f, err = os.Open(p)
		tst.ErrFatal(t, err)
		fi, err = f.Stat()
		tst.ErrFatal(t, err)
		htr(t, "GET", url, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if rsp.Header.Get(share.HeaderContentType) != "text/html; charset=utf-8" {
				t.Fail()
			}
			clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
			tst.ErrFatal(t, err)
			if clen != len(html) {
				t.Fail()
			}
			b, err := ioutil.ReadAll(rsp.Body)
			tst.ErrFatal(t, err)
			if !bytes.Equal(b, html) {
				t.Fail()
			}
		})
		err = f.Close()
		tst.ErrFatal(t, err)
	}
	ff(tst.Htreq)
	ff(tst.Htrex)

	// HEAD handled
	ff = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		f, err = os.Open(p)
		tst.ErrFatal(t, err)
		fi, err = f.Stat()
		tst.ErrFatal(t, err)
		htr(t, "HEAD", url, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if rsp.Header.Get(share.HeaderContentType) != "text/html; charset=utf-8" {
				t.Fail()
			}
			clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
			tst.ErrFatal(t, err)
			if clen != len(html) {
				t.Fail()
			}
			b, err := ioutil.ReadAll(rsp.Body)
			tst.ErrFatal(t, err)
			if len(b) != 0 {
				t.Fail()
			}
		})
		err = f.Close()
		tst.ErrFatal(t, err)
	}
	ff(tst.Htreq)
	ff(tst.Htrex)

	// emulate copy failure
	// file handler still open, but file deleted, can't help this without performance penalty
	ff = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		f, err = os.Open(p)
		tst.ErrFatal(t, err)
		fi, err = f.Stat()
		tst.ErrFatal(t, err)
		err = os.Remove(p)
		tst.ErrFatal(t, err)
		htr(t, "GET", url, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if rsp.Header.Get(share.HeaderContentType) != "text/html; charset=utf-8" {
				t.Fail()
			}
			clen, err := strconv.Atoi(rsp.Header.Get(share.HeaderContentLength))
			tst.ErrFatal(t, err)
			if clen != len(html) {
				t.Fail()
			}
		})
		err = f.Close()
		tst.ErrFatal(t, err)
	}
	ff(tst.Htreq)
	// ff(tst.Htrex)
}

func TestPutf(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn, maxRequestBody: defaultMaxRequestBody}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.putf(w, r)
	}

	// invalid path
	tst.Htreqx(t, "PUT", tst.S.URL+"/"+string([]byte{0}), nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// existing dir
	err := os.MkdirAll(path.Join(dn, "dir"), os.ModePerm)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "PUT", tst.S.URL+"/dir", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// existing file
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		p := path.Join(dn, "file")
		tst.RemoveIfExistsF(t, p)
		tst.WithNewFileF(t, p, func(f *os.File) error {
			_, err := f.Write([]byte("old content"))
			return err
		})
		htr(t, "PUT", tst.S.URL+"/file", bytes.NewBufferString("new content"), func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			f, err := os.Open(p)
			tst.ErrFatal(t, err)
			defer f.Close()
			content, err := ioutil.ReadAll(f)
			tst.ErrFatal(t, err)
			if !bytes.Equal(content, []byte("new content")) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// new file
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		p := path.Join(dn, "file")
		tst.RemoveIfExistsF(t, p)
		htr(t, "PUT", tst.S.URL+"/file", bytes.NewBufferString("some content"), func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			f, err := os.Open(p)
			tst.ErrFatal(t, err)
			defer f.Close()
			content, err := ioutil.ReadAll(f)
			tst.ErrFatal(t, err)
			if !bytes.Equal(content, []byte("some content")) {
				t.Fail()
			}
			fi, err := os.Lstat(p)
			tst.ErrFatal(t, err)
			if fi.Mode() != os.FileMode(0600) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// clear file
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		p := path.Join(dn, "file")
		tst.RemoveIfExistsF(t, p)
		tst.WithNewFileF(t, p, func(f *os.File) error {
			_, err := f.Write([]byte("old content"))
			return err
		})
		htr(t, "PUT", tst.S.URL+"/file", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			f, err := os.Open(p)
			tst.ErrFatal(t, err)
			defer f.Close()
			content, err := ioutil.ReadAll(f)
			tst.ErrFatal(t, err)
			if len(content) != 0 {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// create full path
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		d0 := path.Join(dn, "dir0")
		tst.RemoveIfExistsF(t, d0)
		htr(t, "PUT", tst.S.URL+"/dir0/dir1/file", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			_, err := os.Lstat(path.Join(dn, "dir0/dir1/file"))
			if err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// max the body
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(rsp *http.Response))) {
		ht = newHandler(t, &testOptions{root: dn, maxRequestBody: 8}).(*handler)
		htr(t, "PUT", tst.S.URL+"/file", io.LimitReader(rand.Reader, 16), func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusRequestEntityTooLarge {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestPutfNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	ht := newHandler(t, &testOptions{root: dn, maxRequestBody: defaultMaxRequestBody}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.putf(w, r)
	}

	// no permission to write dir
	dp := path.Join(dn, "dir")
	tst.EnsureDirF(t, dp)
	p := path.Join(dp, "file")
	tst.RemoveIfExistsF(t, p)
	err := os.Chmod(dp, 0555)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "PUT", tst.S.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to execute dir
	dp = path.Join(dn, "dir")
	tst.EnsureDirF(t, dp)
	p = path.Join(dp, "file")
	tst.RemoveIfExistsF(t, p)
	err = os.Chmod(dp, 0666)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "PUT", tst.S.URL+"/dir/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dp, 0777)

	// no permission to write file
	p = path.Join(dn, "file")
	tst.WithNewFileF(t, p, nil)
	err = os.Chmod(p, 0444)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "PUT", tst.S.URL+"/file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestCopyRename(t *testing.T) {
	var (
		multiple bool
		qry      url.Values
		f        = func(_, _ string) error { return nil }
	)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.copyRename(w, r, qry, multiple, f)
	}

	// no to
	qry = make(url.Values)
	tst.Htreqx(t, "COPY", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// multiple to not allowed
	multiple = false
	qry = make(url.Values)
	qry.Add("to", "path0")
	qry.Add("to", "path1")
	tst.Htreqx(t, "COPY", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// multiple to allowed
	ff := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		multiple = true
		qry = make(url.Values)
		qry.Add("to", "path0")
		qry.Add("to", "path1")
		htr(t, "COPY", tst.S.URL+"/from", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	ff(tst.Htreq)
	ff(tst.Htrex)

	// copy tree above self
	qry = make(url.Values)
	qry.Add("to", "/path/path0")
	tst.Htreqx(t, "COPY", tst.S.URL+"/path/path0/path1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// copy tree below self
	qry = make(url.Values)
	qry.Add("to", "/path/path0/path1")
	tst.Htreqx(t, "COPY", tst.S.URL+"/path/path0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// func called
	pfrom, pto := "from", "to"
	f = func(from, to string) error {
		if from != path.Join(dn, pfrom) || to != path.Join(dn, pto) {
			t.Fail()
		}
		return os.ErrNotExist
	}
	qry = make(url.Values)
	qry.Set("to", pto)
	tst.Htreqx(t, "COPY", tst.S.URL+"/"+pfrom, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestCopyf(t *testing.T) {
	var (
		qry url.Values
		fn0 string
		fn1 string
	)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.copyf(w, r, qry)
	}

	dir := path.Join(dn, "copy")
	tst.EnsureDirF(t, dir)

	// copy over not existing file
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		fn0 = path.Join(dir, "file0")
		tst.WithNewFileF(t, fn0, nil)
		fn1 = path.Join(dir, "file1")
		tst.RemoveIfExistsF(t, fn1)
		qry = make(url.Values)
		qry.Set("to", "/copy/file1")
		htr(t, "COPY", tst.S.URL+"/copy/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// copy over existing file
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		tst.WithNewFileF(t, fn1, nil)
		qry = make(url.Values)
		qry.Set("to", "/copy/file1")
		htr(t, "COPY", tst.S.URL+"/copy/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// copy under not existing dir
	dir0 := path.Join(dir, "dir0")
	tst.RemoveIfExistsF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0/file1")
	tst.Htreqx(t, "COPY", tst.S.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over empty directory
	tst.WithNewDirF(t, dir0)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	tst.Htreqx(t, "COPY", tst.S.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// copy over not empty directory
	tst.WithNewDirF(t, dir0)
	fn1 = path.Join(dir0, "file1")
	tst.WithNewFileF(t, fn1, nil)
	qry = make(url.Values)
	qry.Set("to", "/copy/dir0")
	tst.Htreqx(t, "COPY", tst.S.URL+"/copy/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRenamef(t *testing.T) {
	var (
		qry  url.Values
		dir0 string
		dir1 string
		fn0  string
		fn1  string
	)
	dir := path.Join(dn, "rename")
	tst.EnsureDirF(t, dir)
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.renamef(w, r, qry)
	}

	// rename file
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		dir0 = path.Join(dir, "dir0")
		tst.EnsureDirF(t, dir0)
		dir1 = path.Join(dir, "dir1")
		tst.EnsureDirF(t, dir1)
		fn0 = path.Join(dir0, "file0")
		tst.WithNewFileF(t, fn0, func(f *os.File) error {
			_, err := f.Write([]byte("some content"))
			return err
		})
		fn1 = path.Join(dir1, "file1")
		tst.WithNewFileF(t, fn1, nil)
		qry = make(url.Values)
		qry.Set("to", "rename/dir1/file1")
		htr(t, "RENAME", tst.S.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			_, err := os.Lstat(fn0)
			if !os.IsNotExist(err) {
				t.Fail()
			}
			fi1, err := os.Lstat(fn1)
			if err != nil || fi1.Size() != int64(len("some content")) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// to not existing dir
	dir0 = path.Join(dir, "dir0")
	tst.RemoveIfExistsF(t, dir0)
	fn0 = path.Join(dir, "file0")
	tst.WithNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0/file1")
	tst.Htreq(t, "RENAME", tst.S.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// over existing dir
	tst.EnsureDirF(t, dir0)
	tst.WithNewFileF(t, fn0, nil)
	qry = make(url.Values)
	qry.Set("to", "/rename/dir0")
	tst.Htreq(t, "RENAME", tst.S.URL+"/rename/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
}

func TestRenamefNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	var qry url.Values
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.renamef(w, r, qry)
	}

	dir := path.Join(dn, "rename")
	tst.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	tst.EnsureDirF(t, dir0)
	dir1 := path.Join(dir, "dir1")
	tst.EnsureDirF(t, dir1)
	fn0 := path.Join(dir0, "file0")
	tst.WithNewFileF(t, fn0, func(f *os.File) error {
		_, err := f.Write([]byte("some content"))
		return err
	})
	fn1 := path.Join(dir1, "file1")
	tst.WithNewFileF(t, fn1, nil)

	// no write source
	err := os.Chmod(dir0, 0500)
	tst.ErrFatal(t, err)
	qry = make(url.Values)
	qry.Set("to", "rename/dir1/file1")
	tst.Htreqx(t, "RENAME", tst.S.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		_, err := os.Lstat(fn0)
		if err != nil {
			t.Fail()
		}
		fi1, err := os.Lstat(fn1)
		if err != nil || fi1.Size() != 0 {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, os.ModePerm)
	tst.ErrFatal(t, err)

	// no write to target
	err = os.Chmod(dir1, 0500)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "RENAME", tst.S.URL+"/rename/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		_, err := os.Lstat(fn0)
		if err != nil {
			t.Fail()
		}
		fi1, err := os.Lstat(fn1)
		if err != nil || fi1.Size() != 0 {
			t.Fail()
		}
	})
	err = os.Chmod(dir1, os.ModePerm)
	tst.ErrFatal(t, err)
}

func TestDeletef(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.deletef(w, r)
	}
	dir := path.Join(dn, "delete")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)
	dir0 := path.Join(dir, "dir0")
	tst.EnsureDirF(t, dir0)
	file0 := path.Join(dir0, "file0")
	tst.WithNewFileF(t, file0, nil)

	// doesn't exist
	tst.RemoveIfExistsF(t, file0)
	tst.Htreqx(t, "DELETE", tst.S.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// exists, deleted
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		tst.WithNewFileF(t, file0, nil)
		htr(t, "DELETE", tst.S.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(file0); !os.IsNotExist(err) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestDeletefNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.deletef(w, r)
	}
	dir := path.Join(dn, "delete")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	tst.EnsureDirF(t, dir0)
	file0 := path.Join(dir0, "file0")
	tst.WithNewFileF(t, file0, nil)
	err := os.Chmod(dir0, 0500)
	tst.ErrFatal(t, err)
	tst.Htreqx(t, "DELETE", tst.S.URL+"/delete/dir0/file0", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	tst.ErrFatal(t, err)
}

func TestMkdirf(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.mkdirf(w, r)
	}
	dir := path.Join(dn, "mkdir")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)

	// doesn't exist, created
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		dir0 := path.Join(dir, "dir0")
		tst.EnsureDirF(t, dir0)
		dir1 := path.Join(dir0, "dir1")
		tst.RemoveIfExistsF(t, dir1)
		htr(t, "MKDIR", tst.S.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(dir1); err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// exists, not touched
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		dir0 := path.Join(dir, "dir0")
		tst.EnsureDirF(t, dir0)
		dir1 := path.Join(dir0, "dir1")
		tst.EnsureDirF(t, dir1)
		file0 := path.Join(dir1, "file0")
		tst.WithNewFileF(t, file0, nil)
		htr(t, "MKDIR", tst.S.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(file0); err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestMkdirfNotRoot(t *testing.T) {
	if share.IsRoot {
		t.Skip()
	}

	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		ht.mkdirf(w, r)
	}
	dir := path.Join(dn, "mkdir")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)

	// no permission
	dir0 := path.Join(dir, "dir0")
	tst.EnsureDirF(t, dir0)
	err := os.Chmod(dir0, 0500)
	tst.ErrFatal(t, err)
	dir1 := path.Join(dir0, "dir1")
	tst.Htreqx(t, "MKDIR", tst.S.URL+"/mkdir/dir0/dir1", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
		if _, err := os.Lstat(dir1); !os.IsNotExist(err) {
			t.Fail()
		}
	})
	err = os.Chmod(dir0, 0700)
	tst.ErrFatal(t, err)
}

func TestNoCmd(t *testing.T) {
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		noCmd(w, r, func(w http.ResponseWriter, r *http.Request) {})
	}
	tst.Htreqx(t, "GET", tst.S.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "GET", tst.S.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
}

func TestQueryNoCmd(t *testing.T) {
	var testQuery url.Values
	tst.Thnd.Sh = func(w http.ResponseWriter, r *http.Request) {
		queryNoCmd(w, r, func(w http.ResponseWriter, r *http.Request, qry url.Values) {
			if len(qry) != len(testQuery) {
				t.Fail()
				return
			}
			for key, testVals := range testQuery {
				vals, ok := qry[key]
				if !ok || len(vals) != len(testVals) {
					t.Fail()
					return
				}
				for _, testVal := range testVals {
					found := false
					for _, val := range vals {
						if val == testVal {
							found = true
							break
						}
					}
					if !found {
						t.Fail()
						return
					}
				}
			}
		})
	}

	// no query
	testQuery = make(url.Values)
	tst.Htreqx(t, "GET", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// invalid query format
	testQuery = make(url.Values)
	tst.Htreqx(t, "GET", tst.S.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// contains query, command
	testQuery = make(url.Values)
	testQuery.Set("param", "some")
	tst.Htreqx(t, "GET", tst.S.URL+"?param=some&cmd=somecmd", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// contains query
	testQuery = make(url.Values)
	testQuery.Set("param", "some")
	tst.Htreqx(t, "GET", tst.S.URL+"?param=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
}

func TestNew(t *testing.T) {
	wd, err := os.Getwd()
	tst.ErrFatal(t, err)
	h, err := New(nil)
	if err != nil {
		t.Fail()
	}
	ht := h.(*handler)
	if ht.dn != wd ||
		ht.maxRequestBody != defaultMaxRequestBody ||
		ht.maxSearchResults != defaultMaxSearchResults {
		t.Fail()
	}
	h, err = New(&testOptions{
		root:             dn,
		maxRequestBody:   42,
		maxSearchResults: 1764})
	if err != nil {
		t.Fail()
	}
	ht = h.(*handler)
	if ht.dn != dn ||
		ht.maxRequestBody != 42 ||
		ht.maxSearchResults != 1764 {
		t.Fail()
	}
}

func TestOptionsHandler(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP
	tst.Htreqx(t, "OPTIONS", tst.S.URL, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK ||
			!verifyHeader(map[string][]string{"Content-Length": []string{"0"}}, rsp.Header) {
			t.Fail()
		}
	})
}

func TestProps(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP
	fn := "some-file"
	p := path.Join(dn, fn)
	tst.WithNewFileF(t, p, nil)
	tst.Htreqx(t, "PROPS", tst.S.URL+"/"+fn, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	tst.Htreqx(t, "PROPS", tst.S.URL+"/"+fn+"?cmd=anything", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestModprops(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP
	fn := "some-file"
	p := path.Join(dn, fn)
	tst.WithNewFileF(t, p, nil)
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		htr(t, "MODPROPS", tst.S.URL+"/"+fn, bytes.NewBufferString(fmt.Sprintf("{\"mode\": %d}", 0777)),
			func(rsp *http.Response) {
				if rsp.StatusCode != http.StatusOK {
					t.Fail()
				}
			})
	}
	f(tst.Htreq)
	f(tst.Htrex)
	tst.Htreqx(t, "MODPROPS", tst.S.URL+"/"+fn+"?cmd=anything", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestPut(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP

	// invalid command
	tst.Htreqx(t, "PUT", tst.S.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// put
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		p := path.Join(dn, "some-file")
		tst.RemoveIfExistsF(t, p)
		htr(t, "PUT", tst.S.URL+"/some-file", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			_, err := os.Lstat(p)
			if err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestSearch(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn}).(*handler)
	tst.Thnd.Sh = ht.ServeHTTP
	tst.Htreqx(t, "SEARCH", tst.S.URL+"?cmd=search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	for i := 0; i < 3; i++ {
		tst.WithNewFileF(t, path.Join(dn, fmt.Sprintf("file%d", i)), nil)
	}
	tst.Htreqx(t, "SEARCH", tst.S.URL+"?max=3", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		js, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var m []map[string]interface{}
		err = json.Unmarshal(js, &m)
		tst.ErrFatal(t, err)
		if len(m) != 3 {
			t.Fail()
		}
	})
}

func TestCopy(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP

	// invalid query
	tst.Htreqx(t, "COPY", tst.S.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// invalid command
	tst.Htreqx(t, "COPY", tst.S.URL+"?cmd=some", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
}

func TestGet(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP

	// cmd can be props or search only
	tst.Htreqx(t, "GET", tst.S.URL+"?cmd=invalid", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "GET", tst.S.URL+"?cmd=search&cmd=props", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})
	tst.Htreqx(t, "GET", tst.S.URL+"?cmd=search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})
	tst.Htreqx(t, "GET", tst.S.URL+"?cmd=props", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
	})

	// not found
	fn := "some-file"
	p := path.Join(dn, fn)
	url := tst.S.URL + "/" + fn
	tst.RemoveIfExistsF(t, p)
	tst.Htreqx(t, "GET", url, nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusNotFound {
			t.Fail()
		}
	})

	// listing if directory
	dir := path.Join(dn, "search")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)
	c := []byte("some content")
	p = path.Join(dir, "some-file")
	tst.WithNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write(c)
		return err
	})
	tst.Htreqx(t, "GET", tst.S.URL+"/search", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		var d []map[string]interface{}
		err = json.Unmarshal(b, &d)
		if err != nil || len(d) != 1 || d[0]["name"] != fn {
			t.Fail()
		}
	})

	// file otherwise
	tst.Htreqx(t, "GET", tst.S.URL+"/search/some-file", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusOK {
			t.Fail()
		}
		b, err := ioutil.ReadAll(rsp.Body)
		tst.ErrFatal(t, err)
		if !bytes.Equal(b, c) {
			t.Fail()
		}
	})
}

func TestPostHandler(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP

	// invalid query
	tst.Htreqx(t, "POST", tst.S.URL+"?%%", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	// invalid command
	tst.Htreqx(t, "POST", tst.S.URL+"?cmd=invalid", nil, func(rsp *http.Response) {
		if rsp.StatusCode != http.StatusBadRequest {
			t.Fail()
		}
	})

	dir := path.Join(dn, "post")
	tst.RemoveIfExistsF(t, dir)
	tst.EnsureDirF(t, dir)
	var file string

	// modprops
	f := func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		file = path.Join(dir, "file")
		tst.WithNewFileF(t, file, nil)
		err := os.Chmod(file, 0600)
		tst.ErrFatal(t, err)
		props := map[string]interface{}{"mode": 0660}
		js, err := json.Marshal(props)
		htr(t, "POST", tst.S.URL+"/post/file?cmd=modprops", bytes.NewBuffer(js), func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			fi, err := os.Lstat(file)
			tst.ErrFatal(t, err)
			if fi.Mode() != os.FileMode(0660) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// delete
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		tst.WithNewFileF(t, file, nil)
		htr(t, "POST", tst.S.URL+"/post/file?cmd=delete", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			_, err := os.Lstat(file)
			if !os.IsNotExist(err) {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// mkdir
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		dir0 := path.Join(dir, "dir0")
		tst.RemoveIfExistsF(t, dir0)
		htr(t, "POST", tst.S.URL+"/post/dir0?cmd=mkdir", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(dir0); err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// copy
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		tst.WithNewFileF(t, file, nil)
		file0 := path.Join(dir, "file0")
		tst.RemoveIfExistsF(t, file0)
		htr(t, "POST", tst.S.URL+"/post/file?cmd=copy&to=/post/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(file0); err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)

	// rename
	f = func(htr func(tst.Fataler, string, string, io.Reader, func(*http.Response))) {
		tst.WithNewFileF(t, file, nil)
		file0 := path.Join(dir, "file0")
		tst.RemoveIfExistsF(t, file0)
		htr(t, "POST", tst.S.URL+"/post/file?cmd=rename&to=/post/file0", nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusOK {
				t.Fail()
			}
			if _, err := os.Lstat(file); !os.IsNotExist(err) {
				t.Fail()
			}
			if _, err := os.Lstat(file0); err != nil {
				t.Fail()
			}
		})
	}
	f(tst.Htreq)
	f(tst.Htrex)
}

func TestNotSupported(t *testing.T) {
	ht := newHandler(t, &testOptions{root: dn})
	tst.Thnd.Sh = ht.ServeHTTP
	test := func(method string) {
		tst.Htreqx(t, method, tst.S.URL, nil, func(rsp *http.Response) {
			if rsp.StatusCode != http.StatusMethodNotAllowed {
				t.Fail()
			}
		})
	}
	test("TRACE")
	test("CONNECT")
	test("TINAM")
}
