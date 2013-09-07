package main

import (
	"code.google.com/p/gcfg"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

const (
	configEnvKey     = "taskedconf" // environment variable of the tasked config directory path
	configDefaultDir = ".tasked"     // default directory name for tasked config if environment not set
	configBaseName   = "settings"    // filename of storing general settings inside the config directory
)

// structure used to parse the general settings from an ini file
// TODO:
// - replace the serialization with parsing from/serializing to a simple map
type ReadConf struct {
	Aes struct {
		KeyPath string // path to the file containing the aes key (no line break at the end)
		IvPath  string // path to the file containing the aes iv (no line break at the end)
	}
	Auth struct {
		TokenValiditySecs int // seconds of validity of the authentication token
	}
	Tls struct {
		KeyPath string
		CertPath string
	}
}

// structure holding general settings
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
			key []byte
			cert []byte
		}
	}
}

func (c *config) AesKey() []byte     { return c.sec.aes.key }
func (c *config) AesIv() []byte      { return c.sec.aes.iv }
func (c *config) TokenValidity() int { return c.sec.tokenValidity }

// settings parsed and evaluated on startup
var cfg *config

// default config values
func defaultConfig() *config {
	c := &config{}
	c.sec.aes.key = []byte("0123456789abcdef")
	c.sec.aes.iv = []byte("0123456789abcdef")
	c.sec.tokenValidity = 7776000
	return c
}

// makes sure that a directory with a given path exists
func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = errors.New("File exists and not a directory.")
	}
	return err
}

// makes sure that a directory specified by an environment key exists
// if the environment variable is empty, pwd/defaultName is used
func ensureEnvDir(envkey, defaultName string) (string, error) {
	var err error
	dir := os.Getenv(envkey)
	if len(dir) == 0 {
		dir = os.Getenv("HOME")
		if err != nil {
			return dir, err
		}
	}
	if len(dir) == 0 {
		dir, err = os.Getwd()
		if err != nil {
			return dir, err
		}
	}
	dir = path.Join(dir, defaultName)
	err = ensureDir(dir)
	return dir, err
}

// reads the specified configuration file into to
func readConfig(fn string, to *config) error {
	rcfg := &ReadConf{}
	err := gcfg.ReadFileInto(rcfg, fn)
	if err != nil {
		return err
	}

	if len(rcfg.Aes.KeyPath) > 0 {
		to.sec.aes.key, err = ioutil.ReadFile(rcfg.Aes.KeyPath)
		if err != nil {
			return err
		}
	}
	if len(rcfg.Aes.IvPath) > 0 {
		to.sec.aes.iv, err = ioutil.ReadFile(rcfg.Aes.IvPath)
		if err != nil {
			return err
		}
	}
	if rcfg.Auth.TokenValiditySecs > 0 {
		to.sec.tokenValidity = rcfg.Auth.TokenValiditySecs
	}
	if len(rcfg.Tls.KeyPath) > 0 {
		to.http.tls.key, err = ioutil.ReadFile(rcfg.Tls.KeyPath)
		if err != nil {
			return err
		}
	}
	if len(rcfg.Tls.CertPath) > 0 {
		to.http.tls.cert, err = ioutil.ReadFile(rcfg.Tls.CertPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// initializes the configuration settings
// override rules of configuration values: default -> config -> startup options
func initConfig(opt *options) error {
	// any config value can be overridden with options
	// get env for tasked config dir, default pwd/.config
	// load aes key and iv from configured path, default $config/sec
	// aes key and iv cannot be overridden with options, only optional path can be given for them

	// evaluate config dir
	// parse config
	// evaluate options

	cfg = defaultConfig()
	cfgdir, err := ensureEnvDir(configEnvKey, configDefaultDir)
	err = readConfig(path.Join(cfgdir, configBaseName), cfg)
	if err != nil && !os.IsNotExist(err) {
		return errors.New("Failed to read configuration.")
	}

	return nil
}
