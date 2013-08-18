package main

import (
	"code.google.com/p/gcfg"
	"errors"
	"os"
	"path"
)

const (
	configEnvKey     = "TASKED_CONF" // environment variable of the tasked config directory path
	configDefaultDir = ".tasked"     // default directory name for tasked config if environment not set
	configBaseName   = "settings"    // filename of storing general settings inside the config directory
)

// structure used to parse the general settings from an ini file
// TODO:
// - replace the serialization with parsing to a simple map
type ReadConf struct {
	Aes struct {
		Key string
		Iv  string
	}
}

// structure holding general settings
type config struct {
	aes struct {
		key []byte
		iv  []byte
	}
}

// settings parsed and evaluated on startup
var cfg *config

// default config values
func defaultConfig() *config {
	c := &config{}
	c.aes.key = []byte("123")
	c.aes.iv = []byte("456")
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
		dir, err = os.Getwd()
		if err != nil {
			return dir, err
		}
		dir = path.Join(dir, defaultName)
	}
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

	to.aes.key = []byte(rcfg.Aes.Key)
	to.aes.iv = []byte(rcfg.Aes.Iv)

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
