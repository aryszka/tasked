package main

import (
	"code.google.com/p/gcfg"
	"io/ioutil"
	"os"
	"path"
)

const (
	defaultConfigBaseName = ".tasked"
	configEnvKey          = "taskedconf"
	defaultSysConfig      = "/etc/tasked/settings"
)

var sysConfig = defaultSysConfig

type ReadConf struct {
	Sec struct {
		AesKeyPath        string
		AesIvPath         string
		TokenValiditySecs int
	}
	Http struct {
		TlsKeyPath  string
		TlsCertPath string
		Address     string
	}
	Files struct {
		Root             string
		MaxSearchResults int
	}
}

type config struct {
	sec struct {
		aes struct {
			key []byte
			iv  []byte
		}
		tokenValidity int
	}
	http struct {
		tls struct {
			key  []byte
			cert []byte
		}
		address string
	}
	files struct {
		root   string
		search struct {
			maxResults int
		}
	}
}

func (c *config) AesKey() []byte     { return c.sec.aes.key }
func (c *config) AesIv() []byte      { return c.sec.aes.iv }
func (c *config) TokenValidity() int { return c.sec.tokenValidity }

var cfg config

func checkPath(p string) (bool, error) {
	_, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func evalFile(p, dir string, val *[]byte) error {
	if len(p) == 0 {
		return nil
	}
	p, err := abspath(p, dir)
	if err != nil {
		return err
	}
	c, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}
	*val = c
	return nil
}

func evalIntP(inval int, val *int) {
	if inval <= 0 {
		return
	}
	*val = inval
}

func evalString(inval string, val *string) {
	if len(inval) == 0 {
		return
	}
	*val = inval
}

func readConfig(fn string) error {
	rcfg := &ReadConf{}
	err := gcfg.ReadFileInto(rcfg, fn)
	if err != nil {
		return err
	}

	dir := path.Dir(fn)

	if err = evalFile(rcfg.Sec.AesKeyPath, dir, &cfg.sec.aes.key); err != nil {
		return err
	}
	if err = evalFile(rcfg.Sec.AesIvPath, dir, &cfg.sec.aes.iv); err != nil {
		return err
	}
	evalIntP(rcfg.Sec.TokenValiditySecs, &cfg.sec.tokenValidity)

	if err = evalFile(rcfg.Http.TlsKeyPath, dir, &cfg.http.tls.key); err != nil {
		return err
	}
	if err = evalFile(rcfg.Http.TlsCertPath, dir, &cfg.http.tls.cert); err != nil {
		return err
	}
	evalString(rcfg.Http.Address, &cfg.http.address)

	evalIntP(rcfg.Files.MaxSearchResults, &cfg.files.search.maxResults)

	evalString(rcfg.Files.Root, &cfg.files.root)
	if len(cfg.files.root) > 0 {
		cfg.files.root, err = abspath(cfg.files.root, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func initConfig(p string) error {
	err := readConfig(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
