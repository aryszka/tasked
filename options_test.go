package main

import (
	"bytes"
	"code.google.com/p/tasked/keyval"
	. "code.google.com/p/tasked/testing"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

func testEntries(t *testing.T, es []*keyval.Entry, expected ...*keyval.Entry) {
	if len(es) != len(expected) {
		t.Fail()
		return
	}
	for i, e := range es {
		if e.Key != expected[i].Key ||
			e.Val != expected[i].Val {
			t.Fail()
			return
		}
	}
}

func TestFieldOrFile(t *testing.T) {
	b, err := fieldOrFile("hello", "")
	if err != nil || !bytes.Equal(b, []byte("hello")) {
		t.Fail()
	}

	fn := path.Join(Testdir, "file")
	WithNewFileF(t, fn, func(f *os.File) error {
		_, err := f.Write([]byte("olleh"))
		return err
	})
	b, err = fieldOrFile("hello", fn)
	if err != nil || !bytes.Equal(b, []byte("hello")) {
		t.Fail()
	}

	RemoveIfExistsF(t, fn)
	b, err = fieldOrFile("", fn)
	if err == nil {
		t.Fail()
	}

	WithNewFileF(t, fn, func(f *os.File) error {
		_, err := f.Write([]byte("hello"))
		return err
	})
	b, err = fieldOrFile("", fn)
	if err != nil || !bytes.Equal(b, []byte("hello")) {
		t.Fail()
	}

	b, err = fieldOrFile("", "")
	if len(b) > 0 || err != nil {
		t.Fail()
	}
}

func TestParseCommand(t *testing.T) {
	defer func(args []string) { os.Args = args }(os.Args)

	os.Args = []string{"tasked"}
	_, err := parseCommand()
	if err != missingCommand {
		t.Fail()
	}

	os.Args = []string{"tasked", "no-command"}
	_, err = parseCommand()
	if err != invalidCommand {
		t.Fail()
	}

	for _, cmd := range []string{
		cmdHelp,
		cmdServe,
		cmdOptions,
		cmdHead,
		cmdSearch,
		cmdGet,
		cmdProps,
		cmdModprops,
		cmdPut,
		cmdCopy,
		cmdRename,
		cmdDelete,
		cmdMkdir,
		cmdPost,
		cmdSync} {
		os.Args = []string{"tasked", cmd}
		pcmd, err := parseCommand()
		if pcmd != cmd || err != nil {
			t.Fail()
		}
	}
}

func TestParseFlags(t *testing.T) {
	defer func(args []string, ofe flag.ErrorHandling, stderr *os.File) {
		os.Args = args
		onFlagError = ofe
		os.Stderr = stderr
	}(os.Args, onFlagError, os.Stderr)

	// none set
	os.Args = []string{"tasked", "cmd"}
	e, a := parseFlags()
	if len(e) != 0 || len(a) != 0 {
		t.Fail()
	}

	// free args returned
	os.Args = []string{"tasked", "cmd", "arg0", "arg1"}
	_, a = parseFlags()
	if len(a) != 2 || a[0] != "arg0" || a[1] != "arg1" {
		t.Fail()
	}

	// all set
	os.Args = []string{"tasked", "cmd",
		"-" + includeConfigKey, "some-file",

		"-" + helpKey,
		"-" + rootKey, "some-file-0",
		"-" + cachedirKey, "some-file-1",
		"-" + allowCookiesKey,
		"-" + runasKey, "testuser",

		"-" + addressKey, "some-file-2",
		"-" + tlsKeyKey, "some-data-0",
		"-" + tlsCertKey, "some-data-1",
		"-" + tlsKeyFileKey, "some-file-3",
		"-" + tlsCertFileKey, "some-file-4",
		"-" + maxSearchResultsKey, "15",
		"-" + maxRequestBodyKey, "16",
		"-" + maxRequestHeaderKey, "17",
		"-" + proxyKey, "{}",
		"-" + proxyFileKey, "some-file-5",

		"-" + authenticateKey,
		"-" + publicUserKey, "some-user",
		"-" + aesKeyKey, "some-data-2",
		"-" + aesIvKey, "some-data-3",
		"-" + aesKeyFileKey, "some-file-6",
		"-" + aesIvFileKey, "some-file-7",
		"-" + tokenValidityKey, "18",
		"-" + maxUserProcessesKey, "19",
		"-" + processIdleTimeKey, "20",

		"not flag"}
	e, _ = parseFlags()
	testEntries(t, e,
		&keyval.Entry{Key: includeConfigKey, Val: "some-file"},

		&keyval.Entry{Key: helpKey, Val: "true"},
		&keyval.Entry{Key: rootKey, Val: "some-file-0"},
		&keyval.Entry{Key: cachedirKey, Val: "some-file-1"},
		&keyval.Entry{Key: allowCookiesKey, Val: "true"},
		&keyval.Entry{Key: runasKey, Val: "testuser"},

		&keyval.Entry{Key: addressKey, Val: "some-file-2"},
		&keyval.Entry{Key: tlsKeyKey, Val: "some-data-0"},
		&keyval.Entry{Key: tlsCertKey, Val: "some-data-1"},
		&keyval.Entry{Key: tlsKeyFileKey, Val: "some-file-3"},
		&keyval.Entry{Key: tlsCertFileKey, Val: "some-file-4"},
		&keyval.Entry{Key: maxSearchResultsKey, Val: "15"},
		&keyval.Entry{Key: maxRequestBodyKey, Val: "16"},
		&keyval.Entry{Key: maxRequestHeaderKey, Val: "17"},
		&keyval.Entry{Key: proxyKey, Val: "{}"},
		&keyval.Entry{Key: proxyFileKey, Val: "some-file-5"},

		&keyval.Entry{Key: authenticateKey, Val: "true"},
		&keyval.Entry{Key: publicUserKey, Val: "some-user"},
		&keyval.Entry{Key: aesKeyKey, Val: "some-data-2"},
		&keyval.Entry{Key: aesIvKey, Val: "some-data-3"},
		&keyval.Entry{Key: aesKeyFileKey, Val: "some-file-6"},
		&keyval.Entry{Key: aesIvFileKey, Val: "some-file-7"},
		&keyval.Entry{Key: tokenValidityKey, Val: "18"},
		&keyval.Entry{Key: maxUserProcessesKey, Val: "19"},
		&keyval.Entry{Key: processIdleTimeKey, Val: "20"})

	// usage
	d := path.Join(Testdir, "options")
	EnsureDirF(t, d)
	fakeStderr := path.Join(d, "fake-stderr")
	WithNewFileF(t, fakeStderr, func(f *os.File) error {
		os.Stderr = f
		os.Args = []string{"tasked", "cmd", "-" + includeConfigKey}
		onFlagError = flag.ContinueOnError
		parseFlags()
		return nil
	})
	output, err := ioutil.ReadFile(fakeStderr)
	ErrFatal(t, err)
	if strings.Index(string(output), usage) < 0 {
		t.Fail()
	}
}

func TestHasHelpFlag(t *testing.T) {
	has := hasHelpFlag(nil)
	if has {
		t.Fail()
	}

	has = hasHelpFlag([]*keyval.Entry{&keyval.Entry{"somekey", "someval"}})
	if has {
		t.Fail()
	}

	has = hasHelpFlag([]*keyval.Entry{
		&keyval.Entry{"somekey", "someval"},
		&keyval.Entry{helpKey, "someval"}})
	if !has {
		t.Fail()
	}
}

func TestGetIncludes(t *testing.T) {
	i := getIncludes(nil)
	if len(i) != 4 {
		t.Fail()
	}

	i = getIncludes([]*keyval.Entry{&keyval.Entry{
		Key: includeConfigKey, Val: "some-file"}})
	if len(i) != 5 || i[4] != "some-file" {
		t.Fail()
	}
}

func TestApplyDefaults(t *testing.T) {
	o := new(options)
	applyDefaults(o)
	if o.address != defaultAddress ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.tokenValidity != defaultTokenValidity ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}
}

func TestApplyFreeArgs(t *testing.T) {
	o := new(options)
	o.command = cmdHelp
	err := applyFreeArgs(o, []string{"some0", "some1", "some2"})
	if err != nil {
		t.Fail()
	}

	o = new(options)
	o.command = cmdServe
	err = applyFreeArgs(o, []string{"some0", "some1", "some2"})
	if err != invalidArgs {
		t.Fail()
	}

	o = new(options)
	o.command = cmdServe
	err = applyFreeArgs(o, []string{"some0", "some1"})
	if err != nil || o.root != "some0" || o.address != "some1" {
		t.Fail()
	}

	o = new(options)
	o.command = cmdServe
	err = applyFreeArgs(o, []string{"some0"})
	if err != nil || o.root != "some0" {
		t.Fail()
	}
}

func TestParseOptions(t *testing.T) {
	// none
	o := new(options)
	err := parseOptions(o, nil)
	if err != nil || o == nil ||
		o.root != "" ||
		o.cachedir != "" ||
		o.maxSearchResults != 0 ||
		o.runas != "" ||

		o.address != "" ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != 0 ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != 0 ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != 0 {
		t.Fail()
	}

	// all
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Val: "not flag"},

		&keyval.Entry{Key: rootKey, Val: "some-file-0"},
		&keyval.Entry{Key: cachedirKey, Val: "some-file-1"},
		&keyval.Entry{Key: maxSearchResultsKey, Val: "15"},
		&keyval.Entry{Key: runasKey, Val: "testuser"},

		&keyval.Entry{Key: addressKey, Val: "some-file-2"},
		&keyval.Entry{Key: tlsKeyKey, Val: "some-data-0"},
		&keyval.Entry{Key: tlsCertKey, Val: "some-data-1"},
		&keyval.Entry{Key: tlsKeyFileKey, Val: "some-file-3"},
		&keyval.Entry{Key: tlsCertFileKey, Val: "some-file-4"},
		&keyval.Entry{Key: allowCookiesKey, Val: "true"},
		&keyval.Entry{Key: maxRequestBodyKey, Val: "16"},
		&keyval.Entry{Key: maxRequestHeaderKey, Val: "17"},
		&keyval.Entry{Key: proxyKey, Val: "{}"},
		&keyval.Entry{Key: proxyFileKey, Val: "some-file-5"},

		&keyval.Entry{Key: authenticateKey, Val: "true"},
		&keyval.Entry{Key: publicUserKey, Val: "some-user"},
		&keyval.Entry{Key: aesKeyKey, Val: "some-data-2"},
		&keyval.Entry{Key: aesIvKey, Val: "some-data-3"},
		&keyval.Entry{Key: aesKeyFileKey, Val: "some-file-6"},
		&keyval.Entry{Key: aesIvFileKey, Val: "some-file-7"},
		&keyval.Entry{Key: tokenValidityKey, Val: "18"},
		&keyval.Entry{Key: maxUserProcessesKey, Val: "19"},
		&keyval.Entry{Key: processIdleTimeKey, Val: "20"}})
	if err != nil || o == nil ||
		o.root != "some-file-0" ||
		o.cachedir != "some-file-1" ||
		o.maxSearchResults != 15 ||
		o.runas != "testuser" ||

		o.address != "some-file-2" ||
		o.tlsKey != "some-data-0" ||
		o.tlsCert != "some-data-1" ||
		o.tlsKeyFile != "some-file-3" ||
		o.tlsCertFile != "some-file-4" ||
		!o.allowCookies ||
		o.maxRequestBody != 16 ||
		o.maxRequestHeader != 17 ||
		o.proxy != "{}" ||
		o.proxyFile != "some-file-5" ||

		!o.authenticate ||
		o.publicUser != "some-user" ||
		o.aesKey != "some-data-2" ||
		o.aesIv != "some-data-3" ||
		o.aesKeyFile != "some-file-6" ||
		o.aesIvFile != "some-file-7" ||
		o.tokenValidity != 18 ||
		o.maxUserProcesses != 19 ||
		o.processIdleTime != 20 {
		t.Fail()
	}

	// parse int
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: maxSearchResultsKey, Val: "not int"}})
	if err == nil {
		t.Fail()
	}
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: maxSearchResultsKey, Val: fmt.Sprintf("%d", ^uint(0)>>1+1) + "0"}})
	if err == nil {
		t.Fail()
	}

	// parse int64
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: maxRequestBodyKey, Val: "not int"}})
	if err == nil {
		t.Fail()
	}
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: maxRequestBodyKey, Val: fmt.Sprintf("%d", ^uint64(0)>>1+1) + "0"}})
	if err == nil {
		t.Fail()
	}

	// parse bool
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "not bool"}})
	if err == nil {
		t.Fail()
	}
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "false"}})
	if err != nil || o.allowCookies {
		t.Fail()
	}
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "1"}})
	if err != nil || !o.allowCookies {
		t.Fail()
	}
	o = new(options)
	err = parseOptions(o, []*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "0"}})
	if err != nil || o.allowCookies {
		t.Fail()
	}
}

func TestValidateOptions(t *testing.T) {
	o := new(options)
	err := validateOptions(o)
	if err != nil || o.root != "" {
		t.Fail()
	}
	o = new(options)
	o.root = "/some/path"
	err = validateOptions(o)
	if err != nil || o.root != "/some/path" {
		t.Fail()
	}
	o = new(options)
	o.root = "some/path"
	wd, err := os.Getwd()
	ErrFatal(t, err)
	err = validateOptions(o)
	if err != nil || o.root != path.Join(wd, "some/path") {
		t.Fail()
	}
}

func TestReadOptions(t *testing.T) {
	defer func(sc, hk string, args []string, stderr *os.File) {
		sysConfig = sc
		userHomeKey = hk
		os.Args = args
		os.Stderr = stderr
	}(sysConfig, userHomeKey, os.Args, os.Stderr)

	sc := path.Join(Testdir, "sysconfig")
	sysConfig = sc

	uh := path.Join(Testdir, "home")
	EnsureDirF(t, uh)
	userHomeKey = "testhome"
	err := os.Setenv(userHomeKey, uh)
	ErrFatal(t, err)
	uhc := path.Join(uh, userConfigBase)

	efa := path.Join(Testdir, "enva")
	err = os.Setenv(envAltFilename, efa)
	ErrFatal(t, err)

	ef := path.Join(Testdir, "env")
	err = os.Setenv(envFilename, ef)
	ErrFatal(t, err)

	// command parse fails
	fakeStderr := path.Join(Testdir, "stderr")
	WithNewFileF(t, fakeStderr, func(f *os.File) error {
		os.Stderr = f
		os.Args = []string{"tasked", "no command"}
		_, err = readOptions()
		if err == nil {
			t.Fail()
		}
		return nil
	})
	output, err := ioutil.ReadFile(fakeStderr)
	ErrFatal(t, err)
	if strings.Index(string(output), usage) < 0 {
		t.Fail()
	}

	// command parsed
	os.Args = []string{"tasked", cmdServe}
	o, err := readOptions()
	if err != nil || o.Command() != cmdServe {
		t.Fail()
	}

	// help flag
	fakeStderr = path.Join(Testdir, "stderr")
	WithNewFileF(t, fakeStderr, func(f *os.File) error {
		os.Stderr = f
		os.Args = []string{"tasked", cmdServe, "-help", "me"}
		o, err := readOptions()
		if err != nil || o != nil {
			t.Fail()
		}
		return nil
	})
	output, err = ioutil.ReadFile(fakeStderr)
	ErrFatal(t, err)
	if strings.Index(string(output), usage) < 0 {
		t.Fail()
	}

	// free args fail
	fakeStderr = path.Join(Testdir, "stderr")
	WithNewFileF(t, fakeStderr, func(f *os.File) error {
		os.Stderr = f
		os.Args = []string{"tasked", cmdServe, "/arg0", "arg1", "arg2"}
		_, err := readOptions()
		if err != invalidArgs {
			t.Fail()
		}
		return nil
	})
	output, err = ioutil.ReadFile(fakeStderr)
	ErrFatal(t, err)
	if strings.Index(string(output), usage) < 0 {
		t.Fail()
	}

	// free args
	RemoveIfExistsF(t, sc)
	RemoveIfExistsF(t, uhc)
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"tasked", cmdServe, "/arg0", "arg1"}
	o, err = readOptions()
	if err != nil || o.Command() != cmdServe ||
		o.Root() != "/arg0" || o.Address() != "arg1" {
		t.Fail()
	}

	// validate
	RemoveIfExistsF(t, sc)
	RemoveIfExistsF(t, uhc)
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	wd, err := os.Getwd()
	ErrFatal(t, err)
	os.Args = []string{"tasked", cmdServe, "some/path"}
	o, err = readOptions()
	if err != nil || o.root != path.Join(wd, "some/path") {
		t.Log(o.root)
		t.Fail()
	}

	// empty
	RemoveIfExistsF(t, sc)
	RemoveIfExistsF(t, uhc)
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"tasked", cmdServe}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != "" ||
		o.cachedir != "" ||
		o.maxSearchResults != 0 ||
		o.runas != "" ||

		o.address != defaultAddress ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// sys
	d0 := path.Join(Testdir, "dir0")
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0))
		return err
	})
	RemoveIfExistsF(t, uhc)
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"tasked", cmdServe}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d0 ||
		o.cachedir != "" ||
		o.maxSearchResults != 0 ||
		o.runas != "" ||

		o.address != defaultAddress ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// home
	cd0 := path.Join(Testdir, "cachedir0")
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d0 + "\n" +
			cachedirKey + "=" + cd0))
		return err
	})
	d1 := path.Join(Testdir, "dir1")
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d1))
		return err
	})
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"tasked", cmdServe}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d1 ||
		o.cachedir != cd0 ||
		o.maxSearchResults != 0 ||
		o.runas != "" ||

		o.address != defaultAddress ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// alt env
	msr0 := 15
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d0 + "\n" +
			cachedirKey + "=" + cd0 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr0)))
		return err
	})
	cd1 := path.Join(Testdir, "cachedir1")
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d1 + "\n" +
			cachedirKey + "=" + cd1))
		return err
	})
	d2 := path.Join(Testdir, "dir2")
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d2))
		return err
	})
	RemoveIfExistsF(t, ef)
	os.Args = []string{"tasked", cmdServe}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d2 ||
		o.cachedir != cd1 ||
		o.maxSearchResults != msr0 ||
		o.runas != "" ||

		o.address != defaultAddress ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// env
	ad0 := ":9091"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d0 + "\n" +
			cachedirKey + "=" + cd0 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
			addressKey + "=" + ad0))
		return err
	})
	msr1 := 16
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d1 + "\n" +
			cachedirKey + "=" + cd1 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr1)))
		return err
	})
	cd2 := path.Join(Testdir, "cachedir2")
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d2 + "\n" +
			cachedirKey + "=" + cd2))
		return err
	})
	d3 := path.Join(Testdir, "dir3")
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d3))
		return err
	})
	os.Args = []string{"tasked", cmdServe}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d3 ||
		o.cachedir != cd2 ||
		o.maxSearchResults != msr1 ||
		o.runas != "" ||

		o.address != ad0 ||
		o.tlsKey != "" ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// include
	tk0 := "some-key-0"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d0 + "\n" +
			cachedirKey + "=" + cd0 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
			addressKey + "=" + ad0 + "\n" +
			tlsKeyKey + "=" + tk0))
		return err
	})
	ad1 := ":9092"
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d1 + "\n" +
			cachedirKey + "=" + cd1 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr1) + "\n" +
			addressKey + "=" + ad1))
		return err
	})
	msr2 := 17
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d2 + "\n" +
			cachedirKey + "=" + cd2 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr2)))
		return err
	})
	cd3 := path.Join(Testdir, "cachedir3")
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d3 + "\n" +
			cachedirKey + "=" + cd3))
		return err
	})
	ic := path.Join(Testdir, "include")
	d4 := path.Join(Testdir, "dir4")
	WithNewFileF(t, ic, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d4))
		return err
	})
	os.Args = []string{"tasked", cmdServe,
		"-" + includeConfigKey, ic}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d4 ||
		o.cachedir != cd3 ||
		o.maxSearchResults != msr2 ||
		o.runas != "" ||

		o.address != ad1 ||
		o.tlsKey != tk0 ||
		o.tlsCert != "" ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// flag
	tc0 := "some-cert-0"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d0 + "\n" +
			cachedirKey + "=" + cd0 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
			addressKey + "=" + ad0 + "\n" +
			tlsKeyKey + "=" + tk0 + "\n" +
			tlsCertKey + "=" + tc0))
		return err
	})
	tk1 := "some-key-1"
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d1 + "\n" +
			cachedirKey + "=" + cd1 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr1) + "\n" +
			addressKey + "=" + ad1 + "\n" +
			tlsKeyKey + "=" + tk1))
		return err
	})
	ad2 := ":9092"
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d2 + "\n" +
			cachedirKey + "=" + cd2 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr2) + "\n" +
			addressKey + "=" + ad2))
		return err
	})
	msr3 := 18
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d3 + "\n" +
			cachedirKey + "=" + cd3 + "\n" +
			maxSearchResultsKey + "=" + fmt.Sprint(msr3)))
		return err
	})
	cd4 := path.Join(Testdir, "cachedir4")
	WithNewFileF(t, ic, func(f *os.File) error {
		_, err := f.Write([]byte(rootKey + "=" + d4 + "\n" +
			cachedirKey + "=" + cd4))
		return err
	})
	d5 := path.Join(Testdir, "dir5")
	os.Args = []string{"tasked", cmdServe,
		"-" + includeConfigKey, ic,
		"-" + rootKey, d5}
	o, err = readOptions()
	if err != nil || o == nil ||
		o.root != d5 ||
		o.cachedir != cd4 ||
		o.maxSearchResults != msr3 ||
		o.runas != "" ||

		o.address != ad2 ||
		o.tlsKey != tk1 ||
		o.tlsCert != tc0 ||
		o.tlsKeyFile != "" ||
		o.tlsCertFile != "" ||
		o.allowCookies ||
		o.maxRequestBody != 0 ||
		o.maxRequestHeader != defaultMaxRequestHeader ||
		o.proxy != "" ||
		o.proxyFile != "" ||

		o.authenticate ||
		o.publicUser != "" ||
		o.aesKey != "" ||
		o.aesIv != "" ||
		o.aesKeyFile != "" ||
		o.aesIvFile != "" ||
		o.tokenValidity != defaultTokenValidity ||
		o.maxUserProcesses != 0 ||
		o.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// error
	RemoveIfExistsF(t, ic)
	EnsureDirF(t, ic)
	os.Args = []string{"tasked", "cmd",
		"-" + includeConfigKey, ic}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	o, err = readOptions()
	if err == nil {
		t.Fail()
	}
	os.Args = []string{"tasked", "cmd",
		"-" + maxSearchResultsKey, "not int"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	o, err = readOptions()
	if err == nil {
		t.Fail()
	}

	// command and args
	os.Args = []string{"tasked", cmdServe, "arg0", "arg1"}
	o, err = readOptions()
	if err != nil {
		t.Fail()
	}
}
