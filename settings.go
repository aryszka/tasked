package main

import (
	"code.google.com/p/gcfg"
	"code.google.com/p/tasked/util"
	"flag"
	"os"
	"path"
)

const (
	defaultConfigBaseName = ".tasked"
	configEnvKey          = "taskedconf"
	sysConfig             = "/etc/tasked/settings"
)

type ReadConf struct {
	Sec struct {
		AesKeyFile    string
		AesIvFile     string
		TokenValidity int
	}
	Http struct {
		TlsKeyFile     string
		TlsCertFile    string
		Address        string
		MaxRequestBody int64
	}
	Files struct {
		Root             string
		MaxSearchResults int
	}
}

type settings struct {
	configFile string
	sec        struct {
		aes struct {
			keyFile string
			ivFile  string
		}
		tokenValidity int
	}
	http struct {
		tls struct {
			keyFile  string
			certFile string
		}
		address        string
		maxRequestBody int64
	}
	files struct {
		root   string
		search struct {
			maxResults int
		}
	}
}

func (s *settings) MaxRequestBody() int64 { return s.http.maxRequestBody }
func (s *settings) MaxSearchResults() int { return s.files.search.maxResults }

func readFlags(s *settings) error {
	flag.StringVar(&s.configFile, "config", "", "")
	flag.StringVar(&s.sec.aes.keyFile, "aeskeyfile", "", "")
	flag.StringVar(&s.sec.aes.ivFile, "aesivfile", "", "")
	flag.IntVar(&s.sec.tokenValidity, "tokenvalidity", 0, "")
	flag.StringVar(&s.http.tls.keyFile, "tlskeyfile", "", "")
	flag.StringVar(&s.http.tls.certFile, "tlscertfile", "", "")
	flag.StringVar(&s.http.address, "address", "", "")
	flag.Int64Var(&s.http.maxRequestBody, "maxrequestbody", 0, "")
	flag.StringVar(&s.files.root, "root", "", "")
	flag.IntVar(&s.files.search.maxResults, "maxsearchresults", 0, "")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	s.configFile = util.AbspathNotEmpty(s.configFile, wd)
	s.sec.aes.keyFile = util.AbspathNotEmpty(s.sec.aes.keyFile, wd)
	s.sec.aes.ivFile = util.AbspathNotEmpty(s.sec.aes.ivFile, wd)
	s.http.tls.keyFile = util.AbspathNotEmpty(s.http.tls.keyFile, wd)
	s.http.tls.certFile = util.AbspathNotEmpty(s.http.tls.certFile, wd)
	s.files.root = util.AbspathNotEmpty(s.files.root, wd)

	return nil
}

func getConfigPath() (string, error) {
	p := os.Getenv(configEnvKey)
	if len(p) > 0 {
		return p, nil
	}
	p = path.Join(os.Getenv("HOME"), defaultConfigBaseName)
	if ok, err := util.CheckPath(p, false); ok || err != nil {
		return p, err
	}
	if ok, err := util.CheckPath(sysConfig, false); ok || err != nil {
		return sysConfig, err
	}
	return "", nil
}

func readConfig(s *settings) error {
	if len(s.configFile) == 0 {
		var err error
		if s.configFile, err = getConfigPath(); err != nil || len(s.configFile) == 0 {
			return err
		}
	}

	var rcfg ReadConf
	if err := gcfg.ReadFileInto(&rcfg, s.configFile); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	dir := path.Dir(s.configFile)

	if len(s.sec.aes.keyFile) == 0 {
		s.sec.aes.keyFile = util.AbspathNotEmpty(rcfg.Sec.AesKeyFile, dir)
	}
	if len(s.sec.aes.ivFile) == 0 {
		s.sec.aes.ivFile = util.AbspathNotEmpty(rcfg.Sec.AesIvFile, dir)
	}
	if s.sec.tokenValidity <= 0 && rcfg.Sec.TokenValidity > 0 {
		s.sec.tokenValidity = rcfg.Sec.TokenValidity
	}
	if len(s.http.tls.keyFile) == 0 {
		s.http.tls.keyFile = util.AbspathNotEmpty(rcfg.Http.TlsKeyFile, dir)
	}
	if len(s.http.tls.certFile) == 0 {
		s.http.tls.certFile = util.AbspathNotEmpty(rcfg.Http.TlsCertFile, dir)
	}
	if len(s.http.address) == 0 {
		s.http.address = rcfg.Http.Address
	}
	if s.http.maxRequestBody <= 0 && rcfg.Http.MaxRequestBody > 0 {
		s.http.maxRequestBody = rcfg.Http.MaxRequestBody
	}
	if len(s.files.root) == 0 {
		s.files.root = util.AbspathNotEmpty(rcfg.Files.Root, dir)
	}
	if s.files.search.maxResults <= 0 && rcfg.Files.MaxSearchResults > 0 {
		s.files.search.maxResults = rcfg.Files.MaxSearchResults
	}

	return nil
}

func getSettings() (*settings, error) {
	var s settings
	if err := readFlags(&s); err != nil {
		return &s, err
	}
	return &s, readConfig(&s)
}
