package main

import (
	"code.google.com/p/tasked/keyval"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

const (
	userConfigBase = ".nlet"
	envFilename    = "nletconf"
	envAltFilename = "NLETCONF"

	cmdServe    = "serve"
	cmdOptions  = "options"
	cmdHead     = "head"
	cmdSearch   = "search"
	cmdGet      = "get"
	cmdProps    = "props"
	cmdModprops = "modprops"
	cmdPut      = "put"
	cmdCopy     = "copy"
	cmdRename   = "rename"
	cmdDelete   = "delete"
	cmdMkdir    = "mkdir"
	cmdPost     = "post"

	argKey           = "arg"
	includeConfigKey = "include-config"

	rootKey             = "root"
	cachedirKey         = "cachedir"
	maxSearchResultsKey = "max-search-results"

	addressKey          = "address" // todo: document that address is a non-standard format
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

type options struct {
	command string
	args    []string

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

func fieldOrFile(field string, fn string) ([]byte, error) {
	if len(field) > 0 {
		return []byte(field), nil
	}
	return ioutil.ReadFile(fn)
}

func (o *options) Command() string { return o.command }
func (o *options) Args() []string  { return o.args }

func (o *options) Root() string          { return o.root }
func (o *options) CacheDir() string      { return o.cachedir }
func (o *options) MaxSearchResults() int { return o.maxSearchResults }

func (o *options) Address() string          { return o.address }
func (o *options) TlsKey() ([]byte, error)  { return fieldOrFile(o.tlsKey, o.tlsKeyFile) }
func (o *options) TlsCert() ([]byte, error) { return fieldOrFile(o.tlsCert, o.tlsCertFile) }
func (o *options) AllowCookies() bool       { return o.allowCookies }
func (o *options) MaxRequestBody() int64    { return o.maxRequestBody }
func (o *options) MaxRequestHeader() int64  { return o.maxRequestHeader }
func (o *options) Proxy() string            { return o.proxy }

func (o *options) Authenticate() bool      { return o.authenticate }
func (o *options) PublicUser() string      { return o.publicUser }
func (o *options) AesKey() ([]byte, error) { return fieldOrFile(o.aesKey, o.aesKeyFile) }
func (o *options) AesIv() ([]byte, error)  { return fieldOrFile(o.aesIv, o.aesIvFile) }
func (o *options) TokenValidity() int      { return o.tokenValidity }
func (o *options) MaxUserProcesses() int   { return o.maxUserProcesses }
func (o *options) ProcessIdleTime() int    { return o.processIdleTime }

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
	for _, arg := range flag.Args() {
		e = append(e, &keyval.Entry{Key: argKey, Val: arg})
	}
	for _, f := range flags {
		if f.val == nil {
			continue
		}
		e = append(e, &keyval.Entry{Key: f.key, Val: *f.val})
	}

	return e
}

func applyDefaults(o *options) {
	o.address = defaultAddress
	o.maxRequestHeader = defaultMaxRequestHeader
	o.tokenValidity = defaultTokenValidity
	o.processIdleTime = defaultProcessIdleTime
}

func parseOptions(o *options, e []*keyval.Entry) error {
	for _, ei := range e {
		switch ei.Key {
		// command
		case argKey:
			switch {
			case o.command == "":
				o.command = ei.Val
			default:
				o.args = append(o.args, ei.Val)
			}

		// general
		case rootKey:
			o.root = ei.Val
		case cachedirKey:
			o.cachedir = ei.Val
		case maxSearchResultsKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return err
			}
			o.maxSearchResults = int(v)

		// http
		case addressKey:
			o.address = ei.Val
		case tlsKeyKey:
			o.tlsKey = ei.Val
		case tlsCertKey:
			o.tlsCert = ei.Val
		case tlsKeyFileKey:
			o.tlsKeyFile = ei.Val
		case tlsCertFileKey:
			o.tlsCertFile = ei.Val
		case allowCookiesKey:
			v, err := strconv.ParseBool(ei.Val)
			if err != nil {
				return err
			}
			o.allowCookies = v
		case maxRequestBodyKey:
			v, err := strconv.ParseInt(ei.Val, 0, 64)
			if err != nil {
				return err
			}
			o.maxRequestBody = v
		case maxRequestHeaderKey:
			v, err := strconv.ParseInt(ei.Val, 0, 64)
			if err != nil {
				return err
			}
			o.maxRequestHeader = v
		case proxyKey:
			o.proxy = ei.Val
		case proxyFileKey:
			o.proxyFile = ei.Val

		// auth
		case authenticateKey:
			v, err := strconv.ParseBool(ei.Val)
			if err != nil {
				return err
			}
			o.authenticate = v
		case publicUserKey:
			o.publicUser = ei.Val
		case aesKeyKey:
			o.aesKey = ei.Val
		case aesIvKey:
			o.aesIv = ei.Val
		case aesKeyFileKey:
			o.aesKeyFile = ei.Val
		case aesIvFileKey:
			o.aesIvFile = ei.Val
		case tokenValidityKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return err
			}
			o.tokenValidity = int(v)
		case maxUserProcessesKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return err
			}
			o.maxUserProcesses = int(v)
		case processIdleTimeKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return err
			}
			o.processIdleTime = int(v)
		}
	}
	return nil
}

func readOptions() (*options, error) {
	flags := parseFlags()
	includes := []string{
		sysConfig,
		path.Join(os.Getenv(userHomeKey), userConfigBase),
		os.Getenv(envAltFilename),
		os.Getenv(envFilename)}
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
	o := new(options)
	applyDefaults(o)
	err = parseOptions(o, e)
	return o, err
}
