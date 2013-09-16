package main

import (
	"code.google.com/p/gcfg"
	"io/ioutil"
	"os"
	"path"
)

const (
	configDefaultDir = ".tasked"    // default directory name for tasked config if environment not set
	configEnvKey     = "taskedconf" // environment variable of the tasked config directory path
	configBaseName   = "settings"   // filename of storing general settings inside the config directory
)

// Structure used to parse the general settings from an ini file.
// TODO:
// - replace the serialization with parsing from/serializing to a simple map
type ReadConf struct {
	Sec struct {
		AesKeyPath        string // path to the file containing the aes key (no line break at the end)
		AesIvPath         string // path to the file containing the aes iv (no line break at the end)
		TokenValiditySecs int    // seconds of validity of the authentication token
	}
	Http struct {
		TlsKeyPath  string
		TlsCertPath string
		Address     string
	}
}

// Structure holding application settings.
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
}

func (c *config) AesKey() []byte     { return c.sec.aes.key }
func (c *config) AesIv() []byte      { return c.sec.aes.iv }
func (c *config) TokenValidity() int { return c.sec.tokenValidity }

// Current settings available for the whole package.
var cfg config

// Gets the configuration directory specified by the taskedconf environment key.
// If the environment variable is empty, $HOME/.tasked or $(pwd)/.tasked is used.
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

// Reads the specified configuration file into cfg.
func readConfig(fn string) error {
	rcfg := &ReadConf{}
	err := gcfg.ReadFileInto(rcfg, fn)
	if err != nil {
		return err
	}

	if len(rcfg.Sec.AesKeyPath) > 0 {
		cfg.sec.aes.key, err = ioutil.ReadFile(rcfg.Sec.AesKeyPath)
		if err != nil {
			return err
		}
	}
	if len(rcfg.Sec.AesIvPath) > 0 {
		cfg.sec.aes.iv, err = ioutil.ReadFile(rcfg.Sec.AesIvPath)
		if err != nil {
			return err
		}
	}
	if rcfg.Sec.TokenValiditySecs > 0 {
		cfg.sec.tokenValidity = rcfg.Sec.TokenValiditySecs
	}
	if len(rcfg.Http.TlsKeyPath) > 0 {
		cfg.http.tls.key, err = ioutil.ReadFile(rcfg.Http.TlsKeyPath)
		if err != nil {
			return err
		}
	}
	if len(rcfg.Http.TlsCertPath) > 0 {
		cfg.http.tls.cert, err = ioutil.ReadFile(rcfg.Http.TlsCertPath)
		if err != nil {
			return err
		}
	}
	if len(rcfg.Http.Address) > 0 {
		cfg.http.address = rcfg.Http.Address
	}

	return nil
}

// Initializes the configuration.
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
