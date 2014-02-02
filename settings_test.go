package main

import (
	"code.google.com/p/tasked/share"
	tst "code.google.com/p/tasked/testing"
	"os"
	"path"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	err := tst.WithEnv(configEnvKey, "some/path", func() error {
		p, err := getConfigPath()
		if err != nil || p != "some/path" {
			t.Fail()
		}
		return nil
	})
	tst.ErrFatal(t, err)
	homeConfig := path.Join(os.Getenv("HOME"), defaultConfigBaseName)
	homeConfigExists, err := share.CheckPath(homeConfig, false)
	tst.ErrFatal(t, err)
	sysConfigExists, err := share.CheckPath(sysConfig, false)
	tst.ErrFatal(t, err)
	err = tst.WithEnv(configEnvKey, "", func() error {
		p, err := getConfigPath()
		if err != nil ||
			homeConfigExists && p != homeConfig ||
			sysConfigExists && p != sysConfig ||
			p != "" {
			t.Fail()
		}
		return nil
	})
	tst.ErrFatal(t, err)
}

func TestReadConfig(t *testing.T) {
	var s settings
	err := tst.WithEnv(configEnvKey, "", func() error {
		err := readConfig(&s)
		if err != nil || s.configFile != "" {
			t.Fail()
		}
		return nil
	})
	tst.ErrFatal(t, err)
	p := path.Join(tst.Testdir, "test-config")
	tst.RemoveIfExistsF(t, p)
	s.configFile = p
	err = readConfig(&s)
	if err != nil {
		t.Fail()
	}
	tst.ErrFatal(t, err)

	tst.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("aeskeyfile = some-file")); err != nil {
			return err
		}
		return nil
	})
	err = readConfig(&s)
	if err != nil || s.sec.aes.keyFile != path.Join(tst.Testdir, "some-file") {
		t.Fail()
	}
	tst.WithNewFileF(t, p, nil)
	s.sec.aes.keyFile = ""
	err = readConfig(&s)
	if err != nil || s.sec.aes.keyFile != "" {
		t.Fail()
	}
	tst.WithNewFileF(t, p, func(f *os.File) error {
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
	tst.WithNewFileF(t, p, func(f *os.File) error {
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
	if share.IsRoot {
		t.Skip()
	}
	var s settings
	p := path.Join(tst.Testdir, "test-config")
	tst.WithNewFileF(t, p, nil)
	err := os.Chmod(p, 0)
	tst.ErrFatal(t, err)
	defer func() {
		err := os.Chmod(p, os.ModePerm)
		tst.ErrFatal(t, err)
	}()
	s.configFile = p
	err = tst.WithEnv(configEnvKey, "", func() error {
		err := readConfig(&s)
		if err == nil {
			t.Fail()
		}
		return nil
	})
	tst.ErrFatal(t, err)
}

func TestGetSettings(t *testing.T) {
	p := path.Join(tst.Testdir, "test-settings")
	tst.WithNewFileF(t, p, func(f *os.File) error {
		if _, err := f.Write([]byte("[sec]\n")); err != nil {
			return err
		}
		if _, err := f.Write([]byte("tokenvalidity = 42")); err != nil {
			return err
		}
		return nil
	})
	err := tst.WithEnv(configEnvKey, p, func() error {
		s, err := getSettings()
		if err != nil || s.sec.tokenValidity != 42 {
			t.Fail()
		}
		return nil
	})
	tst.ErrFatal(t, err)
}
