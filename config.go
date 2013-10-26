package main

import (
	"code.google.com/p/gcfg"
	"io/ioutil"
	"os"
	"path"
)

const (
	configDefaultDir = ".tasked"
	configEnvKey     = "taskedconf"
	configBaseName   = "settings"
)

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
		search struct {
			maxResults int
		}
	}
}

func (c *config) AesKey() []byte     { return c.sec.aes.key }
func (c *config) AesIv() []byte      { return c.sec.aes.iv }
func (c *config) TokenValidity() int { return c.sec.tokenValidity }

var cfg config

func getConfdir() (string, error) {
	dir := os.Getenv(configEnvKey)
	if len(dir) > 0 {
		return dir, nil
	}
	dir = os.Getenv("HOME")
	if len(dir) == 0 {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return path.Join(dir, configDefaultDir), nil
}

func evalFile(path string, val *[]byte) error {
	if len(path) == 0 {
		return nil
	}
	c, err := ioutil.ReadFile(path)
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

	if err = evalFile(rcfg.Sec.AesKeyPath, &cfg.sec.aes.key); err != nil {
		return err
	}
	if err = evalFile(rcfg.Sec.AesIvPath, &cfg.sec.aes.iv); err != nil {
		return err
	}
	evalIntP(rcfg.Sec.TokenValiditySecs, &cfg.sec.tokenValidity)

	if err = evalFile(rcfg.Http.TlsKeyPath, &cfg.http.tls.key); err != nil {
		return err
	}
	if err = evalFile(rcfg.Http.TlsCertPath, &cfg.http.tls.cert); err != nil {
		return err
	}
	evalString(rcfg.Http.Address, &cfg.http.address)

	evalIntP(rcfg.Files.MaxSearchResults, &cfg.files.search.maxResults)

	return nil
}

func initConfig() error {
	cfgdir, err := getConfdir()
	if err != nil {
		return err
	}
	err = readConfig(path.Join(cfgdir, configBaseName))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
