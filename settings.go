package main

import (
	"code.google.com/p/tasked/keyval"
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
)

const (
	userConfigBase = ".nlet"
	envFilename    = "nletconf"
	envAltFilename = "NLETCONF"

	includeConfigKey = "include-config"

	rootKey             = "root"
	cachedirKey         = "cachedir"
	maxSearchResultsKey = "max-search-results"

	addressKey          = "address"
	tlsKeyKey           = "tls-key"
	tlsCertKey          = "tls-cert"
	tlsKeyFileKey       = "tls-key-file"
	tlsCertFileKey      = "tls-cert-file"
	allowCookiesKey     = "allow-cookies"
	maxRequestBodyKey   = "max-request-body"
	maxRequestHeaderKey = "max-request-header"
	proxyKey            = "proxy"
	proxyFileKey        = "proxy-file"

	authenticateKey     = "authenticate"
	publicUserKey       = "public-user"
	aesKeyKey           = "aes-key"
	aesIvKey            = "aes-iv"
	aesKeyFileKey       = "aes-key-file"
	aesIvFileKey        = "aes-iv-file"
	tokenValidityKey    = "token-validity"
	maxUserProcessesKey = "max-user-processes"
	processIdleTimeKey  = "process-idle-time"

	defaultAddress          = ":9090"
	defaultMaxRequestHeader = 1 << 20
	defaultTokenValidity    = 60 * 60 * 24 * 80
	defaultProcessIdleTime  = 360
)

var (
	sysConfig   = "/etc/nlet"
	userHomeKey = "HOME"
)

type flg struct {
	key    string
	val    *string
	isBool bool
}

func (f *flg) Set(v string) error {
	f.val = &v
	return nil
}

func (f *flg) String() string {
	if f.val == nil {
		return ""
	}
	return *f.val
}

func (f *flg) IsBoolFlag() bool {
	return f.isBool
}

type settings struct {
	root             string
	cachedir         string
	maxSearchResults int

	address          string
	tlsKey           string
	tlsCert          string
	tlsKeyFile       string
	tlsCertFile      string
	allowCookies     bool
	maxRequestBody   int64
	maxRequestHeader int64
	proxy            string
	proxyFile        string

	authenticate     bool
	publicUser       string
	aesKey           string
	aesIv            string
	aesKeyFile       string
	aesIvFile        string
	tokenValidity    int
	maxUserProcesses int
	processIdleTime  int
}

func (s *settings) Root() string          { return s.root }
func (s *settings) CacheDir() string      { return s.cachedir }
func (s *settings) MaxSearchResults() int { return s.maxSearchResults }

func (s *settings) Address() string         { return s.address }
func (s *settings) TlsKey() string          { return s.tlsKey }
func (s *settings) TlsCert() string         { return s.tlsCert }
func (s *settings) AllowCookies() bool      { return s.allowCookies }
func (s *settings) MaxRequestBody() int64   { return s.maxRequestBody }
func (s *settings) MaxRequestHeader() int64 { return s.maxRequestHeader }
func (s *settings) Proxy() string           { return s.proxy }

func (s *settings) Authenticate() bool    { return s.authenticate }
func (s *settings) PublicUser() string    { return s.publicUser }
func (s *settings) AesKey() string        { return s.aesKey }
func (s *settings) AesIv() string         { return s.aesIv }
func (s *settings) TokenValidity() int    { return s.tokenValidity }
func (s *settings) MaxUserProcesses() int { return s.maxUserProcesses }
func (s *settings) ProcessIdleTime() int  { return s.processIdleTime }

func parseFlags() []*keyval.Entry {
	flags := []*flg{
		&flg{key: includeConfigKey},

		&flg{key: rootKey},
		&flg{key: cachedirKey},
		&flg{key: maxSearchResultsKey},

		&flg{key: addressKey},
		&flg{key: tlsKeyKey},
		&flg{key: tlsCertKey},
		&flg{key: tlsKeyFileKey},
		&flg{key: tlsCertFileKey},
		&flg{key: allowCookiesKey, isBool: true},
		&flg{key: maxRequestBodyKey},
		&flg{key: maxRequestHeaderKey},
		&flg{key: proxyKey},
		&flg{key: proxyFileKey},

		&flg{key: authenticateKey, isBool: true},
		&flg{key: publicUserKey},
		&flg{key: aesKeyKey},
		&flg{key: aesIvKey},
		&flg{key: aesKeyFileKey},
		&flg{key: aesIvFileKey},
		&flg{key: tokenValidityKey},
		&flg{key: maxUserProcessesKey},
		&flg{key: processIdleTimeKey}}

	for _, f := range flags {
		flag.Var(f, f.key, "")
	}
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	var e []*keyval.Entry
	args := flag.Args()
	if len(args) > 0 {
		e = append(e, &keyval.Entry{Val: args[0]})
	}
	for _, f := range flags {
		if f.val == nil {
			continue
		}
		e = append(e, &keyval.Entry{Key: f.key, Val: *f.val})
	}

	return e
}

func parseSettings(e []*keyval.Entry) (*settings, error) {
	s := new(settings)
	for _, ei := range e {
		switch ei.Key {
		// general
		case rootKey, "":
			s.root = ei.Val
		case cachedirKey:
			s.cachedir = ei.Val
		case maxSearchResultsKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return nil, err
			}
			s.maxSearchResults = int(v)

		// http
		case addressKey:
			s.address = ei.Val
		case tlsKeyKey:
			s.tlsKey = ei.Val
		case tlsCertKey:
			s.tlsCert = ei.Val
		case tlsKeyFileKey:
			s.tlsKeyFile = ei.Val
		case tlsCertFileKey:
			s.tlsCertFile = ei.Val
		case allowCookiesKey:
			v, err := strconv.ParseBool(ei.Val)
			if err != nil {
				return nil, err
			}
			s.allowCookies = v
		case maxRequestBodyKey:
			v, err := strconv.ParseInt(ei.Val, 0, 64)
			if err != nil {
				return nil, err
			}
			s.maxRequestBody = v
		case maxRequestHeaderKey:
			v, err := strconv.ParseInt(ei.Val, 0, 64)
			if err != nil {
				return nil, err
			}
			s.maxRequestHeader = v
		case proxyKey:
			s.proxy = ei.Val
		case proxyFileKey:
			s.proxyFile = ei.Val

		// auth
		case authenticateKey:
			v, err := strconv.ParseBool(ei.Val)
			if err != nil {
				return nil, err
			}
			s.authenticate = v
		case publicUserKey:
			s.publicUser = ei.Val
		case aesKeyKey:
			s.aesKey = ei.Val
		case aesIvKey:
			s.aesIv = ei.Val
		case aesKeyFileKey:
			s.aesKeyFile = ei.Val
		case aesIvFileKey:
			s.aesIvFile = ei.Val
		case tokenValidityKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return nil, err
			}
			s.tokenValidity = int(v)
		case maxUserProcessesKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return nil, err
			}
			s.maxUserProcesses = int(v)
		case processIdleTimeKey:
			println("parsing")
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return nil, err
			}
			println(v)
			s.processIdleTime = int(v)
		}
	}
	return s, nil
}

func applyDefaults(s *settings) {
	if s.address == "" {
		s.address = defaultAddress
	}
	if s.maxRequestHeader <= 0 {
		s.maxRequestHeader = defaultMaxRequestHeader
	}
	if s.tokenValidity <= 0 {
		s.tokenValidity = defaultTokenValidity
	}
	if s.processIdleTime <= 0 {
		println("it is smaller")
		s.processIdleTime = defaultProcessIdleTime
	}
}

func readSettings() (*settings, error) {
	includes := []string{
		sysConfig,
		path.Join(os.Getenv(userHomeKey), userConfigBase),
		os.Getenv(envAltFilename),
		os.Getenv(envFilename)}
	flags := parseFlags()
	for _, ei := range flags {
		if ei.Key != includeConfigKey {
			continue
		}
		includes = append(includes, string(ei.Val))
		break
	}
	e, err := keyval.Parse(includes, includeConfigKey)
	if err != nil {
		return nil, err
	}
	e = append(e, flags...)
	s, err := parseSettings(e)
	if err != nil {
		return nil, err
	}
	applyDefaults(s)
	return s, nil
}
