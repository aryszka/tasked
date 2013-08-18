package main

import (
    "errors"
    "os"
    "path"
    "code.google.com/p/gcfg"
)

const (
    configEnvKey = "TASKED_CONF"
    configDefaultDir = ".tasked"
    configBaseName = "settings"
)

type ReadConf struct {
    Aes struct {
        Key string
        Iv string
    }
}

type config struct {
    aes struct {
        key []byte
        iv []byte
    }
}

var cfg *config

func ensureDir(dir string) error {
    fi, err := os.Stat(dir)
    if os.IsNotExist(err) {
        err = os.MkdirAll(dir, os.ModePerm)
    } else if err == nil && !fi.IsDir() {
        err = errors.New("File exists and not a directory.")
    }
    return err
}

func ensureEnvDir(envkey, defaultName string) (string, error) {
    var err error
    dir := os.Getenv(envkey)
    if len(dir) == 0 {
        dir, err = os.Getwd()
        if err != nil { return dir, err }
        dir = path.Join(dir, defaultName)
    }
    err = ensureDir(dir)
    return dir, err
}

func defaultConfig() *config {
    c := &config{}
    c.aes.key = []byte("123")
    c.aes.iv = []byte("456")
    return c
}

func readConfig(fn string, to *config) error {
    rcfg := &ReadConf{}
    err := gcfg.ReadFileInto(rcfg, fn)
    if err != nil { return err }

    to.aes.key = []byte(rcfg.Aes.Key)
    to.aes.iv = []byte(rcfg.Aes.Iv)

    return nil
}

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
