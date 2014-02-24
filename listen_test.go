package main

import (
	. "code.google.com/p/tasked/testing"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"testing"
)

var testLong = false

func init() {
	tl := flag.Bool("test.long", false, "")
	flag.Parse()
	testLong = *tl
}

const (
	testTlsKey = `-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAK5zTQyrShHno1ON
zPCgwH71S02v+dsKuktLDHmQyJFSm22SAcl/VPT9cQOny7fhNGsZD36y47vuDStk
DDWdQ6JgdaB4vIgM5lcBIsbAIBzpu72qHY8C1SADc3kh/AJ01uZ0+YT7SdrTFbfQ
8+BlrNeRpuyZ5pYTpdN9WD/Ml0GnAgMBAAECgYBALW7VEFbpi1wMqwgQJjNrAXa4
l9rFeIbcmDl4p4kB3XAMuUArDssEbhCblaledl1AYTHQHKOnSYZSxjWvq7Freaj3
yVzl/w8afndhzwQMXXA6WE6mzDGpZys0Osl54tnx9pVsBow/0dWF5YqqzkkHvCgU
HneMhId+RlK4ovS4wQJBANcztfgoDtc0o3RxbGgbTQHZ6R9h96APh6pYeDpuWUXm
iz3KrJfcrC04uRR5OvvdMi9yvlKSS3XWUqw+gJ1/ffECQQDPhdA61WuDUkASoSsM
CbP29BMDNxwEVJpFj9mz8/gCBstLJr91J9e/0hUmcNTYLSXeu9VbkhlZHKqRmCeP
9QEXAkEAhi/vWqKXxmdDONt0zmGfnfTj4Ta0MnEclgJ+TWj7b5O25TvYbQUjszr4
ZSTGu8AMh/uTY2dneD3A5Tg/+HVD8QJAfZVRCgmVYocn1x5JWqOUkOHrj4qOHeDE
60u+RzRg5XkPFY+BVXijC6YqvnSRmbDYtg2ddCjZxh+e6TTW3Ds8SQJAIoHZku4v
0i/CibdiccC/7VeAWbpKxdRCDAD3V9GlOsLML2iSIWbsB5VRd5OVyZ8F5nTHaCkl
JYZRgZQ2tTdJPA==
-----END PRIVATE KEY-----
`
	testTlsCert = `-----BEGIN CERTIFICATE-----
MIIC7jCCAlegAwIBAgIJAJi4cHfxlouiMA0GCSqGSIb3DQEBBQUAMIGPMQswCQYD
VQQGEwJERTEPMA0GA1UECAwGQmVybGluMQ8wDQYDVQQHDAZCZXJsaW4xHDAaBgNV
BAoME0JlcmxpbmVyIFJvYm90d2Vya2UxGTAXBgNVBAMMEHRhc2tlZHNlcnZlci5j
b20xJTAjBgkqhkiG9w0BCQEWFmFycGFkLnJ5c3prYUBnbWFpbC5jb20wHhcNMTMw
OTA4MTIzNzQ5WhcNMTYwOTA3MTIzNzQ5WjCBjzELMAkGA1UEBhMCREUxDzANBgNV
BAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMRwwGgYDVQQKDBNCZXJsaW5lciBS
b2JvdHdlcmtlMRkwFwYDVQQDDBB0YXNrZWRzZXJ2ZXIuY29tMSUwIwYJKoZIhvcN
AQkBFhZhcnBhZC5yeXN6a2FAZ21haWwuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQCuc00Mq0oR56NTjczwoMB+9UtNr/nbCrpLSwx5kMiRUpttkgHJf1T0
/XEDp8u34TRrGQ9+suO77g0rZAw1nUOiYHWgeLyIDOZXASLGwCAc6bu9qh2PAtUg
A3N5IfwCdNbmdPmE+0na0xW30PPgZazXkabsmeaWE6XTfVg/zJdBpwIDAQABo1Aw
TjAdBgNVHQ4EFgQULF7xiQlOWP9BBLe0ZSFFmA35HeUwHwYDVR0jBBgwFoAULF7x
iQlOWP9BBLe0ZSFFmA35HeUwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOB
gQAktY9ZXYMknjOPNYLvHcj0rGVFdu7BQuRNGu+YfRrWaK7Hc+T6eXzyXrYOExZu
o3416vOFtSPB9jlxEkJFkQR303SRJmEurxkgfXNiBV5jKqj7jKCkoXG7fHCDZou0
ja5JCKq4V6B3O32gOEhgAdh6OUE4iWYxGhWd3wYUevdyFw==
-----END CERTIFICATE-----
`
)

type testOptions struct {
	address   string
	cachedir  string
	key       []byte
	cert      []byte
	keyError  error
	certError error
}

func (o *testOptions) Address() string          { return o.address }
func (o *testOptions) Cachedir() string         { return o.cachedir }
func (o *testOptions) TlsKey() ([]byte, error)  { return o.key, o.keyError }
func (o *testOptions) TlsCert() ([]byte, error) { return o.cert, o.certError }

func TestParseAddress(t *testing.T) {
	test := func(addr string, errx error, s, v string, p int) {
		a, err := parseAddress(addr)
		if err != errx || errx == nil &&
			(a.schema != schema(s) ||
				a.val != v ||
				a.port != port(p)) {
			t.Fail()
		}
	}
	test("", nil, "", "", 0)
	test("http", nil, "", "http", 0)
	test("https", nil, "", "https", 0)
	test("unix", nil, "", "unix", 0)
	test("host", nil, "", "host", 0)
	test("//", nil, "", "", 0)
	test(":9090", nil, "", "", 9090)
	test("host:9090", nil, "", "host", 9090)
	test("host9:9090", nil, "", "host9", 9090)
	test("//host:9090", nil, "", "host", 9090)
	test("//host", nil, "", "host", 0)
	test("host:", invalidAddress, "", "", 0)
	test("//host:", invalidAddress, "", "", 0)
	test(":host", invalidAddress, "", "", 0)
	test("//:host", invalidAddress, "", "", 0)
	test(":", invalidAddress, "", "", 0)
	test("::", invalidAddress, "", "", 0)
	test("2001:db8:85a3:8d3:1319:8a2e:370:7348", invalidAddress, "", "", 0)
	test("[2001:db8:85a3:8d3:1319:8a2e:370:7348]", nil, "", "[2001:db8:85a3:8d3:1319:8a2e:370:7348]", 0)
	test("[2001:", invalidAddress, "", "", 0)
	test("[2001:db8]:9090", nil, "", "[2001:db8]", 9090)

	test("http:", nil, "http", "", 0)
	test("http://", nil, "http", "", 0)
	test("http::9090", nil, "http", "", 9090)
	test("http:host:9090", nil, "http", "host", 9090)
	test("http:host9:9090", nil, "http", "host9", 9090)
	test("http://host:9090", nil, "http", "host", 9090)
	test("http:host", nil, "http", "host", 0)
	test("http://host", nil, "http", "host", 0)
	test("http:host:", invalidAddress, "", "", 0)
	test("http://host:", invalidAddress, "", "", 0)
	test("http::host", invalidAddress, "", "", 0)
	test("http://:host", invalidAddress, "", "", 0)
	test("http::", invalidAddress, "", "", 0)
	test("http:::", invalidAddress, "", "", 0)
	test("http:[2001:db8]", nil, "http", "[2001:db8]", 0)
	test("http:[2001:db8]:9090", nil, "http", "[2001:db8]", 9090)
	test("http://[2001:db8]:9090", nil, "http", "[2001:db8]", 9090)
	test("http://[2001:db8]:a", invalidAddress, "", "", 0)

	test("https:", nil, "https", "", 0)
	test("https://", nil, "https", "", 0)
	test("https::9090", nil, "https", "", 9090)
	test("https:host:9090", nil, "https", "host", 9090)
	test("https:host9:9090", nil, "https", "host9", 9090)
	test("https://host:9090", nil, "https", "host", 9090)
	test("https:host", nil, "https", "host", 0)
	test("https://host", nil, "https", "host", 0)
	test("https:[2001:db8]", nil, "https", "[2001:db8]", 0)
	test("https:[2001:db8]:9090", nil, "https", "[2001:db8]", 9090)
	test("https://[2001:db8]:9090", nil, "https", "[2001:db8]", 9090)

	test("unix:", nil, "unix", "", 0)
	test("unix://", nil, "unix", "//", 0)
	test("unix:9090", nil, "unix", "9090", 0)
	test("unix:host:9090", nil, "unix", "host:9090", 0)
	test("unix://host:9090", nil, "unix", "//host:9090", 0)
	test("unix:host", nil, "unix", "host", 0)
	test("unix://host", nil, "unix", "//host", 0)
	test("unix:host:", nil, "unix", "host:", 0)
	test("unix://host:", nil, "unix", "//host:", 0)
	test("unix::host", nil, "unix", ":host", 0)
	test("unix://:host", nil, "unix", "//:host", 0)
	test("unix::", nil, "unix", ":", 0)
}

func TestGetListenerParams(t *testing.T) {
	test := func(addr *address, nv, av string) {
		n, a := getListenerParams(addr)
		if n != nv || a != av {
			t.Fail()
		}
	}
	test(&address{}, "tcp", ":9090")
	test(&address{val: "host.domain"}, "tcp", "host.domain:9090")
	test(&address{port: 80}, "tcp", ":80")
	test(&address{val: "host.domain", port: 80}, "tcp", "host.domain:80")
	test(&address{schema: "http"}, "tcp", ":9090")
	test(&address{schema: "http", val: "host.domain"}, "tcp", "host.domain:9090")
	test(&address{schema: "http", port: 80}, "tcp", ":80")
	test(&address{schema: "http", val: "host.domain", port: 80}, "tcp", "host.domain:80")
	test(&address{schema: "https"}, "tcp", ":9090")
	test(&address{schema: "https", val: "host.domain"}, "tcp", "host.domain:9090")
	test(&address{schema: "https", port: 80}, "tcp", ":80")
	test(&address{schema: "https", val: "host.domain", port: 80}, "tcp", "host.domain:80")
	test(&address{schema: "unix"}, "unixpacket", fmt.Sprintf(defaultUnixAddressFmt, os.Getpid()))
	test(&address{schema: "unix", val: "filename"}, "unixpacket", "filename")
	test(&address{schema: "unix", port: 80}, "unixpacket", fmt.Sprintf(defaultUnixAddressFmt, os.Getpid()))
	test(&address{schema: "unix", val: "filename", port: 80}, "unixpacket", "filename")
}

func TestListenTls(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	o := new(testOptions)
	o.keyError = errors.New("test error")
	_, err := listenTls(nil, nil, o)
	if err != o.keyError {
		t.Fail()
	}

	o = new(testOptions)
	o.certError = errors.New("test error")
	_, err = listenTls(nil, nil, o)
	if err != o.certError {
		t.Fail()
	}

	o = new(testOptions)
	o.key = []byte("invalid key")
	o.cert = []byte(testTlsCert)
	_, err = listenTls(nil, nil, o)
	if err == nil {
		t.Fail()
	}

	o = new(testOptions)
	o.key = []byte(testTlsKey)
	o.cert = []byte("invalid cert")
	_, err = listenTls(nil, nil, o)
	if err == nil {
		t.Fail()
	}

	file := path.Join(Testdir, "file")
	WithNewFileF(t, file, nil)
	o = new(testOptions)
	o.cachedir = file
	_, err = listenTls(nil, new(address), o)
	if err == nil {
		t.Fail()
	}

	func() {
		l, err := net.Listen("tcp", "127.0.0.1:9090")
		ErrFatal(t, err)
		defer CloseFatal(t, l)
		o = new(testOptions)
		o.key = []byte(testTlsKey)
		o.cert = []byte(testTlsCert)
		_, err = listenTls(l, new(address), o)
		if err != nil {
			t.Fail()
		}
	}()

	func() {
		l, err := net.Listen("tcp", "127.0.0.1:9090")
		ErrFatal(t, err)
		defer CloseFatal(t, l)
		a := new(address)
		a.val = "127.0.0.1"
		a.port = 9090
		_, err = listenTls(l, a, new(testOptions))
		if err != nil {
			t.Fail()
		}
	}()

	func() {
		l, err := net.Listen("tcp", "127.0.0.1:9090")
		ErrFatal(t, err)
		defer CloseFatal(t, l)
		a := new(address)
		a.val = "[::1]"
		a.port = 9090
		_, err = listenTls(l, a, new(testOptions))
		if err != nil {
			t.Fail()
		}
	}()

	func() {
		l, err := net.Listen("tcp", "127.0.0.1:9090")
		ErrFatal(t, err)
		defer CloseFatal(t, l)
		a := new(address)
		a.val = "localhost.localdomain"
		a.port = 9090
		_, err = listenTls(l, a, new(testOptions))
		if err != nil {
			t.Fail()
		}
	}()
}

func TestListen(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	o := new(testOptions)
	o.address = ":"
	_, err := listen(o)
	if err == nil {
		t.Fail()
	}

	o = new(testOptions)
	o.address = " "
	_, err = listen(o)
	if err == nil {
		t.Fail()
	}

	func() {
		lb, err := net.Listen("tcp", ":9090")
		ErrFatal(t, err)
		defer CloseFatal(t, lb)
		_, err = listen(new(testOptions))
		if err == nil {
			t.Fail()
		}
	}()

	o = new(testOptions)
	o.address = "https:"
	o.key = []byte("invalid key")
	func() {
		_, err = listen(o)
		if err == nil {
			t.Fail()
		}
	}()

	func() {
		l, err := listen(new(testOptions))
		if err != nil {
			t.Fail()
			return
		}
		defer CloseFatal(t, l)
	}()

	func() {
		o = new(testOptions)
		o.address = "https:"
		l, err := listen(o)
		if err != nil {
			t.Fail()
			return
		}
		defer CloseFatal(t, l)
	}()
}
