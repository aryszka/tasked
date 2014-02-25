package main

import (
	"code.google.com/p/tasked/keyval"
	"errors"
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

	cmdHelp     = "help"
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
	cmdSync     = "sync"

	includeConfigKey = "include-config"

	helpKey         = "help"
	rootKey         = "root"
	cachedirKey     = "cachedir"
	allowCookiesKey = "allow-cookies"
	runasKey        = "runas"

	addressKey          = "address" // todo: document that address is a non-standard format
	tlsKeyKey           = "tls-key"
	tlsCertKey          = "tls-cert"
	tlsKeyFileKey       = "tls-key-file"
	tlsCertFileKey      = "tls-cert-file"
	maxSearchResultsKey = "max-search-results"
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
	sysConfig      = "/etc/nlet"
	userHomeKey    = "HOME"
	missingCommand = errors.New("missing command")
	invalidCommand = errors.New("invalid command")
	invalidArgs    = errors.New("invalid args")
	onFlagError    = flag.ExitOnError
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

	root         string
	cachedir     string
	allowCookies bool
	runas        string

	address          string
	tlsKey           string
	tlsCert          string
	tlsKeyFile       string
	tlsCertFile      string
	maxSearchResults int
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
	if len(fn) > 0 {
		return ioutil.ReadFile(fn)
	}
	return nil, nil
}

func (o *options) Command() string    { return o.command }
func (o *options) Root() string       { return o.root }
func (o *options) Cachedir() string   { return o.cachedir }
func (o *options) AllowCookies() bool { return o.allowCookies }
func (o *options) Runas() string      { return o.runas }

func (o *options) Address() string          { return o.address }
func (o *options) TlsKey() ([]byte, error)  { return fieldOrFile(o.tlsKey, o.tlsKeyFile) }
func (o *options) TlsCert() ([]byte, error) { return fieldOrFile(o.tlsCert, o.tlsCertFile) }
func (o *options) MaxSearchResults() int    { return o.maxSearchResults }
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

func parseCommand() (string, error) {
	if len(os.Args) < 2 {
		return "", missingCommand
	}
	cmd := os.Args[1]
	switch cmd {
	case
		cmdHelp,
		cmdServe,
		cmdOptions,
		cmdHead,
		cmdSearch,
		cmdGet,
		cmdProps,
		cmdModprops,
		cmdPut,
		cmdCopy,
		cmdRename,
		cmdDelete,
		cmdMkdir,
		cmdPost,
		cmdSync:
	default:
		return "", invalidCommand
	}
	return cmd, nil
}

func printUsage() {
	fmt.Fprint(os.Stderr, usage)
}

func parseFlags() ([]*keyval.Entry, []string) {
	flags := []*flg{
		&flg{key: includeConfigKey},

		&flg{key: helpKey, isBool: true},
		&flg{key: rootKey},
		&flg{key: cachedirKey},
		&flg{key: allowCookiesKey, isBool: true},
		&flg{key: runasKey},

		&flg{key: addressKey},
		&flg{key: tlsKeyKey},
		&flg{key: tlsCertKey},
		&flg{key: tlsKeyFileKey},
		&flg{key: tlsCertFileKey},
		&flg{key: maxSearchResultsKey},
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

	fs := flag.NewFlagSet("tasked", onFlagError)
	fs.Usage = printUsage
	for _, f := range flags {
		fs.Var(f, f.key, "")
	}
	fs.Parse(os.Args[2:])
	var e []*keyval.Entry
	for _, f := range flags {
		if f.val == nil {
			continue
		}
		e = append(e, &keyval.Entry{Key: f.key, Val: *f.val})
	}

	return e, fs.Args()
}

func hasHelpFlag(e []*keyval.Entry) bool {
	for _, ei := range e {
		if ei.Key == helpKey {
			return true
		}
	}
	return false
}

func getIncludes(flags []*keyval.Entry) []string {
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
	return includes
}

func applyDefaults(o *options) {
	o.address = defaultAddress
	o.maxRequestHeader = defaultMaxRequestHeader
	o.tokenValidity = defaultTokenValidity
	o.processIdleTime = defaultProcessIdleTime
}

func applyFreeArgs(o *options, args []string) error {
	switch o.command {
	case cmdHelp:
	case cmdServe:
		if len(args) > 2 {
			return invalidArgs
		}
		if len(args) > 0 {
			o.root = args[0]
		}
		if len(args) == 2 {
			o.address = args[1]
		}
	}
	return nil
}

func parseOptions(o *options, e []*keyval.Entry) error {
	for _, ei := range e {
		switch ei.Key {
		// general
		case rootKey:
			o.root = ei.Val
		case cachedirKey:
			o.cachedir = ei.Val
		case allowCookiesKey:
			v, err := strconv.ParseBool(ei.Val)
			if err != nil {
				return err
			}
			o.allowCookies = v
		case runasKey:
			o.runas = ei.Val

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
		case maxSearchResultsKey:
			v, err := strconv.ParseInt(ei.Val, 0, 32)
			if err != nil {
				return err
			}
			o.maxSearchResults = int(v)
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
	cmd, err := parseCommand()
	if err != nil || cmd == cmdHelp {
		printUsage()
		return nil, err
	}

	flags, args := parseFlags()
	if hasHelpFlag(flags) {
		printUsage()
		return nil, nil
	}

	includes := getIncludes(flags)
	e, err := keyval.Parse(includes, includeConfigKey)
	if err != nil {
		return nil, err
	}
	e = append(e, flags...)

	o := new(options)
	o.command = cmd
	applyDefaults(o)
	err = applyFreeArgs(o, args)
	if err != nil {
		printUsage()
		return nil, err
	}
	err = parseOptions(o, e)
	if err != nil {
		printUsage()
		return nil, err
	}
	return o, nil
}
