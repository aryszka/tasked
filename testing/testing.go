package util

import (
	"os"
	"path"
	"fmt"
	"log"
	"net/http"
	"sync"
	"crypto/tls"
	"net/http/httptest"
	"io"
)

const (
	testTlsKey = `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBANfK7I6VGd4yxNNK
rg1+GdveTB4aAiqC916Yl5vFTlpCg6LIhAYmDvXPM9XJZc/h8N4jh/JNC39wgcEG
/RV2wl9T63+NR6TBLVx6nJbKjCvEuzwpB3BIun4827cU6PCksBc4hke9pTgD9v0y
DOtECKl+HuxRuKJLGoRCQ9rcoJL1AgMBAAECgYBz0R+hbvjRPuJQnNZJu5JZZTfp
OABNnLjzdmZ4Xi8tVmGcLo5dVnPVDf4+EbepGRTTxLIkI6G2JkYduYh/ypuK3TtD
JQ2j2Wb4hSFXc3jGGGmx3SFYrmajM6nW7vnBw7Ld6PaJqo5lZtYcFzpOSrzP5G0p
TPEJ1091aOrhoNexgQJBAP7M2XMw4TJqddT03/y4y46ESq4bNYOIyMd3X9yYM77Q
KH5v1x+95znBkb8hJoPgO2+un4uLr2A8L8umxByTHJECQQDYzw2BxF6D9GSDjQr6
BEX1UxfM96DiSE2N3i+1YJWOdcqg9dvJRByYzvdlqEobY2DB8Cnh1HS94V3vyruw
R1IlAkEA9NTnuTzllukfEiK+O3th9S5/B+8TK7G6o5e8IB6L0jT4RA25W0HBtgie
wFXdSWikE/tqSM9PFByhHIHA/WgKUQJALTMlbrtgtQPbfK2H7026xAV5vcqWaPaH
7J64tYiYRWX7Q4leM9yWVak4XKI0KPeT8Xq/UIx5diio69gJPxvvXQJAM1lr5o49
D0qEjXcpHjsMHcrYgQLGZPCfNn3gkGZ/pxr/3N36SyaqF6/7NRe7BLHbll9lb+8f
8FF/8F+a66TGLw==
-----END PRIVATE KEY-----
`
	testTlsCert = `-----BEGIN CERTIFICATE-----
MIIC7jCCAlegAwIBAgIJAIvCpMZ/RhydMA0GCSqGSIb3DQEBBQUAMIGPMQswCQYD
VQQGEwJERTEPMA0GA1UECAwGQmVybGluMQ8wDQYDVQQHDAZCZXJsaW4xHDAaBgNV
BAoME0JlcmxpbmVyIFJvYm90d2Vya2UxGTAXBgNVBAMMEHRhc2tlZHNlcnZlci5j
b20xJTAjBgkqhkiG9w0BCQEWFmFycGFkLnJ5c3prYUBnbWFpbC5jb20wHhcNMTMw
OTA3MTk1MzU1WhcNMTYwOTA2MTk1MzU1WjCBjzELMAkGA1UEBhMCREUxDzANBgNV
BAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMRwwGgYDVQQKDBNCZXJsaW5lciBS
b2JvdHdlcmtlMRkwFwYDVQQDDBB0YXNrZWRzZXJ2ZXIuY29tMSUwIwYJKoZIhvcN
AQkBFhZhcnBhZC5yeXN6a2FAZ21haWwuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQDXyuyOlRneMsTTSq4Nfhnb3kweGgIqgvdemJebxU5aQoOiyIQGJg71
zzPVyWXP4fDeI4fyTQt/cIHBBv0VdsJfU+t/jUekwS1cepyWyowrxLs8KQdwSLp+
PNu3FOjwpLAXOIZHvaU4A/b9MgzrRAipfh7sUbiiSxqEQkPa3KCS9QIDAQABo1Aw
TjAdBgNVHQ4EFgQUrAUcn4JJ13CSKXdKquzs03OHl0gwHwYDVR0jBBgwFoAUrAUc
n4JJ13CSKXdKquzs03OHl0gwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOB
gQB2VmcD9Hde1Bf9lgk3iWw+ZU8JbdJvhK0MoU4RhCDEl01K2omxoT4B8OVWlFD5
GWX4rnIZtcLahM1eu8h+QxdcTNGwCpIiait2pmpVcV6pjNKv8LUxAcaemq178OfK
h3I2CsHAUTwxT1ca8SGLCsFTm03AyXaU0Q061+RX1Do/Iw==
-----END CERTIFICATE-----
`
)

const (
	testdirKey          = "testdir"
	failedToInitTestdir = "Failed to initialize test directory."
	defaultTestdir = "test"
)

type TestHandler struct {
	Sh func(w http.ResponseWriter, r *http.Request)
}

type Fataler interface {
	Fatal(...interface{})
}

func (th *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if th.Sh == nil {
		panic("Test handler not initialized.")
	}
	th.Sh(w, r)
}

var (
	Testdir  = defaultTestdir
	Testuser string
	Testpwd  string
	Thnd         = new(TestHandler)
	S            *httptest.Server
	Mx           = new(sync.Mutex)
)

func initTestdir() {
	Testdir = func() string {
		td := os.Getenv(testdirKey)
		if len(td) > 0 {
			return td
		}
		td = os.Getenv("GOPATH")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td = os.Getenv("HOME")
		if len(td) > 0 {
			return path.Join(td, defaultTestdir)
		}
		td, err := os.Getwd()
		if err != nil {
			panic(failedToInitTestdir)
		}
		return path.Join(td, defaultTestdir)
	}()
	err := EnsureDir(Testdir)
	if err != nil {
		panic(failedToInitTestdir)
	}
}

func init() {
	initTestdir()
	Testuser = envdef("testuser", "testuser")
	Testpwd = envdef("testpwd", "testpwd")
	c, err := tls.X509KeyPair([]byte(testTlsCert), []byte(testTlsKey))
	if err != nil {
		panic(err)
	}
	S = httptest.NewUnstartedServer(Thnd)
	S.TLS = &tls.Config{Certificates: []tls.Certificate{c}}
	S.StartTLS()
}

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func EnsureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = fmt.Errorf("File exists and not a directory: %s.", dir)
	}
	return err
}

func ErrFatal(f Fataler, err error) {
	if err != nil {
		f.Fatal(err)
	}
}

func EnsureDirF(f Fataler, dir string) {
	ErrFatal(f, EnsureDir(dir))
}

func WithEnv(key, val string, f func() error) error {
	orig := os.Getenv(key)
	defer func() {
		err := os.Setenv(key, orig)
		if err != nil {
			log.Panicln(err)
		}
	}()
	err := os.Setenv(key, val)
	if err != nil {
		return err
	}
	if f == nil {
		return nil
	}
	return f()
}

func WithNewFile(fn string, do func(*os.File) error) error {
	err := os.Remove(fn)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	if do == nil {
		return nil
	}
	err = do(f)
	if err != nil {
		return err
	}
	return f.Close()
}

func WithNewFileF(f Fataler, fn string, do func(*os.File) error) {
	ErrFatal(f, WithNewFile(fn, do))
}

func RemoveIfExists(fn string) error {
	err := os.RemoveAll(fn)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func RemoveIfExistsF(f Fataler, fn string) {
	ErrFatal(f, RemoveIfExists(fn))
}

func WithNewDir(dir string) error {
	err := RemoveIfExists(dir)
	if err != nil {
		return err
	}
	return EnsureDir(dir)
}

func WithNewDirF(f Fataler, dir string) {
	ErrFatal(f, WithNewDir(dir))
}

func Mkclient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}
}

func Htreq(t Fataler, method, url string, body io.Reader, clb func(rsp *http.Response)) {
	r, err := http.NewRequest(method, url, body)
	ErrFatal(t, err)
	client := Mkclient()
	rsp, err := client.Do(r)
	ErrFatal(t, err)
	defer rsp.Body.Close()
	clb(rsp)
}
