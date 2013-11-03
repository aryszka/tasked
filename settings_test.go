package main

import (
	"code.google.com/p/tasked/util"
	"os"
	"path"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	err := util.WithEnv(configEnvKey, "some/path", func() error {
		p, err := getConfigPath()
		if err != nil || p != "some/path" {
			t.Fail()
		}
		return nil
	})
	util.ErrFatal(t, err)
	homeConfig := path.Join(os.Getenv("HOME"), defaultConfigBaseName)
	homeConfigExists, err := util.CheckPath(homeConfig, false)
	util.ErrFatal(t, err)
	sysConfigExists, err := util.CheckPath(sysConfig, false)
	util.ErrFatal(t, err)
	err = util.WithEnv(configEnvKey, "", func() error {
		p, err := getConfigPath()
		if err != nil ||
			homeConfigExists && p != homeConfig ||
			sysConfigExists && p != sysConfig ||
			p != "" {
			t.Fail()
		}
		return nil
	})
	util.ErrFatal(t, err)
}

func TestReadConfig(t *testing.T) {
	var s settings
	err := util.WithEnv(configEnvKey, "", func() error {
		err := readConfig(&s)
		if err != nil || s.configFile != "" {
			t.Fail()
		}
		return nil
	})
	util.ErrFatal(t, err)
	p := path.Join(util.Testdir, "test-config")
	util.RemoveIfExistsF(t, p)
	s.configFile = p
	err = readConfig(&s)
	if err != nil {
		t.Fail()
	}
	util.ErrFatal(t, err)

	util.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("aeskeyfile = some-file")); err != nil {
			return err
		}
		return nil
	})
	err = readConfig(&s)
	if err != nil || s.sec.aes.keyFile != path.Join(util.Testdir, "some-file") {
		t.Fail()
	}
	util.WithNewFileF(t, p, nil)
	s.sec.aes.keyFile = ""
	err = readConfig(&s)
	if err != nil || s.sec.aes.keyFile != "" {
		t.Fail()
	}
	util.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("tokenvalidity = -42")); err != nil {
			return err
		}
		return nil
	})
	err = readConfig(&s)
	if err != nil || s.sec.tokenValidity != 0 {
		t.Fail()
	}
	util.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("tokenvalidity = 42")); err != nil {
			return err
		}
		return nil
	})
	err = readConfig(&s)
	if err != nil || s.sec.tokenValidity != 42 {
		t.Fail()
	}
}

func TestReadConfigNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}
	var s settings
	p := path.Join(util.Testdir, "test-config")
	util.WithNewFileF(t, p, nil)
	err := os.Chmod(p, 0)
	util.ErrFatal(t, err)
	defer func() {
		err := os.Chmod(p, os.ModePerm)
		util.ErrFatal(t, err)
	}()
	s.configFile = p
	err = util.WithEnv(configEnvKey, "", func() error {
		err := readConfig(&s)
		if err == nil {
			t.Fail()
		}
		return nil
	})
	util.ErrFatal(t, err)
}

func TestGetSettings(t *testing.T) {
	p := path.Join(util.Testdir, "test-settings")
	util.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("tokenvalidity = 42")); err != nil {
			return err
		}
		return nil
	})
	err := util.WithEnv(configEnvKey, p, func() error {
		s, err := getSettings()
		if err != nil || s.sec.tokenValidity != 42 {
			t.Log(err)
			t.Log(s.sec.tokenValidity)
			t.Fail()
		}
		return nil
	})
	util.ErrFatal(t, err)
}
