package main

import (
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

func TestParseFlags(t *testing.T) {
	defer func(args []string, fs *flag.FlagSet, stderr *os.File) {
		os.Args = args
		flag.CommandLine = fs
		os.Stderr = stderr
	}(os.Args, flag.CommandLine, os.Stderr)

	// none set
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	e := parseFlags()
	if len(e) != 0 {
		t.Fail()
	}

	// all set
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	os.Args = []string{"cmd",
		"-" + includeConfigKey, "some-file",

		"-" + rootKey, "some-file-0",
		"-" + cachedirKey, "some-file-1",
		"-" + maxSearchResultsKey, "15",

		"-" + addressKey, "some-file-2",
		"-" + tlsKeyKey, "some-data-0",
		"-" + tlsCertKey, "some-data-1",
		"-" + tlsKeyFileKey, "some-file-3",
		"-" + tlsCertFileKey, "some-file-4",
		"-" + allowCookiesKey,
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
	e = parseFlags()
	testEntries(t, e,
		&keyval.Entry{Val: "not flag"},

		&keyval.Entry{Key: includeConfigKey, Val: "some-file"},

		&keyval.Entry{Key: rootKey, Val: "some-file-0"},
		&keyval.Entry{Key: cachedirKey, Val: "some-file-1"},
		&keyval.Entry{Key: maxSearchResultsKey, Val: "15"},

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
		&keyval.Entry{Key: processIdleTimeKey, Val: "20"})

	// usage
	d := path.Join(Testdir, "settings")
	fakeStderr := path.Join(d, "fake-stderr")
	WithNewFileF(t, fakeStderr, func(f *os.File) error {
		os.Stderr = f
		os.Args = []string{"cmd", "-" + includeConfigKey}
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
		parseFlags()
		return nil
	})
	output, err := ioutil.ReadFile(fakeStderr)
	ErrFatal(t, err)
	if strings.Index(string(output), usage) < 0 {
		t.Fail()
	}
}

func TestParseSettings(t *testing.T) {
	// none
	s, err := parseSettings(nil)
	if err != nil || s == nil ||
		s.root != "" ||
		s.cachedir != "" ||
		s.maxSearchResults != 0 ||

		s.address != "" ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != 0 ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != 0 ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != 0 {
		t.Fail()
	}

	// all
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Val: "not flag"},

		&keyval.Entry{Key: rootKey, Val: "some-file-0"},
		&keyval.Entry{Key: cachedirKey, Val: "some-file-1"},
		&keyval.Entry{Key: maxSearchResultsKey, Val: "15"},

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
	if err != nil || s == nil ||
		s.root != "some-file-0" ||
		s.cachedir != "some-file-1" ||
		s.maxSearchResults != 15 ||

		s.address != "some-file-2" ||
		s.tlsKey != "some-data-0" ||
		s.tlsCert != "some-data-1" ||
		s.tlsKeyFile != "some-file-3" ||
		s.tlsCertFile != "some-file-4" ||
		!s.allowCookies ||
		s.maxRequestBody != 16 ||
		s.maxRequestHeader != 17 ||
		s.proxy != "{}" ||
		s.proxyFile != "some-file-5" ||

		!s.authenticate ||
		s.publicUser != "some-user" ||
		s.aesKey != "some-data-2" ||
		s.aesIv != "some-data-3" ||
		s.aesKeyFile != "some-file-6" ||
		s.aesIvFile != "some-file-7" ||
		s.tokenValidity != 18 ||
		s.maxUserProcesses != 19 ||
		s.processIdleTime != 20 {
		t.Fail()
	}

	// parse int
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: maxSearchResultsKey, Val: "not int"}})
	if err == nil {
		t.Fail()
	}
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: maxSearchResultsKey, Val: fmt.Sprintf("%d", ^uint(0)>>1+1) + "0"}})
	if err == nil {
		t.Fail()
	}

	// parse int64
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: maxRequestBodyKey, Val: "not int"}})
	if err == nil {
		t.Fail()
	}
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: maxRequestBodyKey, Val: fmt.Sprintf("%d", ^uint64(0)>>1+1) + "0"}})
	if err == nil {
		t.Fail()
	}

	// parse bool
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "not bool"}})
	if err == nil {
		t.Fail()
	}
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "false"}})
	if err != nil || s.allowCookies {
		t.Fail()
	}
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "1"}})
	if err != nil || !s.allowCookies {
		t.Fail()
	}
	s, err = parseSettings([]*keyval.Entry{
		&keyval.Entry{Key: allowCookiesKey, Val: "0"}})
	if err != nil || s.allowCookies {
		t.Fail()
	}
}

func TestApplyDefaults(t *testing.T) {
	s := new(settings)
	applyDefaults(s)
	if s.address != defaultAddress ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.tokenValidity != defaultTokenValidity ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}
}

func TestReadSettings(t *testing.T) {
	defer func(sc, hk string, args []string, fs *flag.FlagSet, stderr *os.File) {
		sysConfig = sc
		userHomeKey = hk
		os.Args = args
		flag.CommandLine = fs
		os.Stderr = stderr
	}(sysConfig, userHomeKey, os.Args, flag.CommandLine, os.Stderr)

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

	// empty
	RemoveIfExistsF(t, sc)
	RemoveIfExistsF(t, uhc)
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err := readSettings()
	if err != nil || s == nil ||
		s.root != "" ||
		s.cachedir != "" ||
		s.maxSearchResults != 0 ||

		s.address != defaultAddress ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
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
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d0 ||
		s.cachedir != "" ||
		s.maxSearchResults != 0 ||

		s.address != defaultAddress ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// home
	cd0 := path.Join(Testdir, "cachedir0")
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0 + "\n" +
				cachedirKey + "=" + cd0))
		return err
	})
	d1 := path.Join(Testdir, "dir1")
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d1))
		return err
	})
	RemoveIfExistsF(t, efa)
	RemoveIfExistsF(t, ef)
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d1 ||
		s.cachedir != cd0 ||
		s.maxSearchResults != 0 ||

		s.address != defaultAddress ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Fail()
	}

	// alt env
	msr0 := 15
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0 + "\n" +
				cachedirKey + "=" + cd0 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr0)))
		return err
	})
	cd1 := path.Join(Testdir, "cachedir1")
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d1 + "\n" +
				cachedirKey + "=" + cd1))
		return err
	})
	d2 := path.Join(Testdir, "dir2")
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d2))
		return err
	})
	RemoveIfExistsF(t, ef)
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d2 ||
		s.cachedir != cd1 ||
		s.maxSearchResults != msr0 ||

		s.address != defaultAddress ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Log(s)
		t.Fail()
	}

	// env
	ad0 := ":9091"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0 + "\n" +
				cachedirKey + "=" + cd0 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
				addressKey + "=" + ad0))
		return err
	})
	msr1 := 16
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d1 + "\n" +
				cachedirKey + "=" + cd1 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr1)))
		return err
	})
	cd2 := path.Join(Testdir, "cachedir2")
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d2 + "\n" +
				cachedirKey + "=" + cd2))
		return err
	})
	d3 := path.Join(Testdir, "dir3")
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d3))
		return err
	})
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d3 ||
		s.cachedir != cd2 ||
		s.maxSearchResults != msr1 ||

		s.address != ad0 ||
		s.tlsKey != "" ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Log(s)
		t.Fail()
	}

	// include
	tk0 := "some-key-0"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0 + "\n" +
				cachedirKey + "=" + cd0 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
				addressKey + "=" + ad0 + "\n" +
				tlsKeyKey + "=" + tk0))
		return err
	})
	ad1 := ":9092"
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d1 + "\n" +
				cachedirKey + "=" + cd1 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr1) + "\n" +
				addressKey + "=" + ad1))
		return err
	})
	msr2 := 17
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d2 + "\n" +
				cachedirKey + "=" + cd2 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr2)))
		return err
	})
	cd3 := path.Join(Testdir, "cachedir3")
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d3 + "\n" +
				cachedirKey + "=" + cd3))
		return err
	})
	ic := path.Join(Testdir, "include")
	d4 := path.Join(Testdir, "dir4")
	WithNewFileF(t, ic, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d4))
		return err
	})
	os.Args = []string{"cmd",
		"-" + includeConfigKey, ic}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d4 ||
		s.cachedir != cd3 ||
		s.maxSearchResults != msr2 ||

		s.address != ad1 ||
		s.tlsKey != tk0 ||
		s.tlsCert != "" ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Log(s)
		t.Fail()
	}

	// flag
	tc0 := "some-cert-0"
	WithNewFileF(t, sc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d0 + "\n" +
				cachedirKey + "=" + cd0 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr0) + "\n" +
				addressKey + "=" + ad0 + "\n" +
				tlsKeyKey + "=" + tk0 + "\n" +
				tlsCertKey + "=" + tc0))
		return err
	})
	tk1 := "some-key-1"
	WithNewFileF(t, uhc, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d1 + "\n" +
				cachedirKey + "=" + cd1 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr1) + "\n" +
				addressKey + "=" + ad1 + "\n" +
				tlsKeyKey + "=" + tk1))
		return err
	})
	ad2 := ":9092"
	WithNewFileF(t, efa, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d2 + "\n" +
				cachedirKey + "=" + cd2 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr2) + "\n" +
				addressKey + "=" + ad2))
		return err
	})
	msr3 := 18
	WithNewFileF(t, ef, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d3 + "\n" +
				cachedirKey + "=" + cd3 + "\n" +
				maxSearchResultsKey + "=" + fmt.Sprint(msr3)))
		return err
	})
	cd4 := path.Join(Testdir, "cachedir4")
	WithNewFileF(t, ic, func(f *os.File) error {
		_, err := f.Write([]byte(
			rootKey + "=" + d4 + "\n" +
				cachedirKey + "=" + cd4))
		return err
	})
	d5 := path.Join(Testdir, "dir5")
	os.Args = []string{"cmd",
		"-" + includeConfigKey, ic,
		"-" + rootKey, d5}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	s, err = readSettings()
	if err != nil || s == nil ||
		s.root != d5 ||
		s.cachedir != cd4 ||
		s.maxSearchResults != msr3 ||

		s.address != ad2 ||
		s.tlsKey != tk1 ||
		s.tlsCert != tc0 ||
		s.tlsKeyFile != "" ||
		s.tlsCertFile != "" ||
		s.allowCookies ||
		s.maxRequestBody != 0 ||
		s.maxRequestHeader != defaultMaxRequestHeader ||
		s.proxy != "" ||
		s.proxyFile != "" ||

		s.authenticate ||
		s.publicUser != "" ||
		s.aesKey != "" ||
		s.aesIv != "" ||
		s.aesKeyFile != "" ||
		s.aesIvFile != "" ||
		s.tokenValidity != defaultTokenValidity ||
		s.maxUserProcesses != 0 ||
		s.processIdleTime != defaultProcessIdleTime {
		t.Log(s)
		t.Fail()
	}
}
