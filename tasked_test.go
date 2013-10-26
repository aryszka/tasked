package main

import (
	"crypto/aes"
	"crypto/rand"
	"flag"
	"io"
	"os"
	"os/user"
	"path"
	"testing"
)

const (
	defaultTestdir      = "test"
	testdirKey          = "testdir"
	failedToInitTestdir = "Failed to initialize test directory."
)

var (
	isRoot       bool
	testdir      = defaultTestdir
	testPam      bool
	testuser     string
	testpwd      string
	propsRoot    bool
	modpropsRoot bool
	getDirRoot   bool
)

func initTestdir() {
	if testdir != defaultTestdir {
		return
	}
	testdir = func() string {
		td := os.Getenv(testdirKey)
		if len(td) > 0 {
			return td
		}
		td = os.Getenv("GOPATH")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td = os.Getenv("HOME")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td, err := os.Getwd()
		if err != nil {
			panic(failedToInitTestdir)
		}
		return path.Join(td, defaultTestdir)
	}()
	trace(testdir)
	err := ensureDir(testdir)
	if err != nil {
		panic(failedToInitTestdir)
	}
}

func init() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	isRoot = usr.Uid == "0"

	initTestdir()

	tp := flag.Bool("test.pam", false, "")
	tpr := flag.Bool("test.propsroot", false, "")
	tmpr := flag.Bool("test.modpropsroot", false, "")
	tgdr := flag.Bool("test.getdirroot", false, "")
	flag.Parse()
	testPam = *tp
	propsRoot = *tpr
	modpropsRoot = *tmpr
	getDirRoot = *tgdr

	testuser = envdef("testuser", "testuser")
	testpwd = envdef("testpwd", "testpwd")
}

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func makeRandom(l int) []byte {
	b := make([]byte, l)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("Failed to generate random bytes.")
	}
	return b
}

func makeKey() []byte { return makeRandom(aes.BlockSize) }

func TestGetConfigPath(t *testing.T) {
	wd, err := os.Getwd()
	errFatal(t, err)
	confSys := sysConfig
	defer func() { sysConfig = confSys }()
	hp := path.Join(os.Getenv("HOME"), defaultConfigBaseName)
	hpBak := hp + ".test.bak"
	err = os.Rename(hp, hpBak)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	defer func() {
		removeIfExistsF(t, hp)
		err = os.Rename(hpBak, hp)
		if os.IsNotExist(err) {
			return
		}
		errFatal(t, err)
	}()
	confEnv := os.Getenv(configEnvKey)
	defer func() {
		err = os.Setenv(configEnvKey, confEnv)
		errFatal(t, err)
	}()
	confFlag := flags.config
	defer func() { flags.config = confFlag }()

	// none
	flags.config = ""
	err = os.Setenv(configEnvKey, "")
	errFatal(t, err)
	removeIfExistsF(t, hp)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal()
	}
	sysConfig = path.Join(testdir, "noexist")
	removeIfExistsF(t, sysConfig)
	p, err := getConfigPath()
	if p != "" || err != nil {
		t.Fail()
	}

	// sys
	withNewFileF(t, sysConfig, nil)
	p, err = getConfigPath()
	if p != sysConfig || err != nil {
		t.Fail()
	}

	// home
	withNewFileF(t, hp, nil)
	p, err = getConfigPath()
	if p != hp || err != nil {
		t.Fail()
	}

	// env
	err = os.Setenv(configEnvKey, "env/path")
	errFatal(t, err)
	p, err = getConfigPath()
	if p != path.Join(wd, "env/path") || err != nil {
		t.Fail()
	}
	err = os.Setenv(configEnvKey, "/env/path")
	errFatal(t, err)
	p, err = getConfigPath()
	if p != "/env/path" || err != nil {
		t.Fail()
	}

	// flag
	flags.config = "flag/path"
	p, err = getConfigPath()
	if p != path.Join(wd, "flag/path") || err != nil {
		t.Fail()
	}
	flags.config = "/flag/path"
	p, err = getConfigPath()
	if p != "/flag/path" || err != nil {
		t.Fail()
	}
}

func TestGetHttpDir(t *testing.T) {
	wd, err := os.Getwd()
	errFatal(t, err)
	rootCfg := cfg.files.root
	defer func() { cfg.files.root = rootCfg }()
	rootFlag := flags.root
	defer func() { flags.root = rootFlag }()

	// none
	flags.root = ""
	cfg.files.root = ""
	dn, err := getHttpDir()
	if err != nil || dn != wd {
		t.Fail()
	}

	// config
	cfg.files.root = "/cfg/path"
	dn, err = getHttpDir()
	if err != nil || dn != "/cfg/path" {
		t.Log(err)
		t.Log(dn)
		t.Fail()
	}

	// flag
	flags.root = "flag/path"
	dn, err = getHttpDir()
	if err != nil || dn != path.Join(wd, "flag/path") {
		t.Fail()
	}
	flags.root = "/flag/path"
	dn, err = getHttpDir()
	if err != nil || dn != "/flag/path" {
		t.Fail()
	}
}

func TestAuthPam(t *testing.T) {
	if !isRoot || !testPam {
		t.Skip()
	}

	if nil != authPam(testuser, testpwd) {
		t.Fail()
	}
	if nil == authPam(testuser+"x", testpwd) {
		t.Fail()
	}
	if nil == authPam(testuser, testpwd+"x") {
		t.Fail()
	}
	if nil == authPam(testuser, "") {
		t.Fail()
	}
	if nil == authPam("", "") {
		t.Fail()
	}
}
