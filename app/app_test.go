package app

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"path"
	"testing"
)

const (
	failedToInitTestdir = "Failed to initialize test directory."
	defaultTestdir      = "test"
	testdirKey          = "testdir"
	fnbase              = "file"
	testTlsKey          = `-----BEGIN PRIVATE KEY-----
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

var testdir = defaultTestdir

// duplicate
func init() {
	get := func() string {
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
	}
	testdir = get()
	err := ensureDir(testdir)
	if err != nil {
		panic(failedToInitTestdir)
	}
	fn = path.Join(testdir, fnbase)
}

// duplicate
// Makes sure that a directory with a given path exists.
func ensureDir(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
	} else if err == nil && !fi.IsDir() {
		err = errors.New("File exists and not a directory.")
	}
	return err
}

type httpTestConfig struct {
	address string
	tlsKey  []byte
	tlsCert []byte
}

func (tc *httpTestConfig) Address() string { return tc.address }
func (tc *httpTestConfig) TlsKey() []byte  { return tc.tlsKey }
func (tc *httpTestConfig) TlsCert() []byte { return tc.tlsCert }

func get(url string) (*http.Response, error) {
	return (&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}).Get(url)
}

func TestReadConfig(t *testing.T) {
	dtk, dtc := []byte(defaultTlsKey), []byte(defaultTlsCert)
	tk, tc, a := readConfig(nil)
	if !bytes.Equal(tk, dtk) || !bytes.Equal(tc, dtc) || a != defaultAddress {
		t.Fail()
	}
	vk, vc := []byte(testTlsKey), []byte(testTlsCert)
	tk, tc, a = readConfig(&httpTestConfig{tlsKey: vk})
	if !bytes.Equal(tk, vk) || !bytes.Equal(tc, dtc) || a != defaultAddress {
		t.Fail()
	}
	tk, tc, a = readConfig(&httpTestConfig{tlsCert: vc})
	if !bytes.Equal(tk, dtk) || !bytes.Equal(tc, vc) || a != defaultAddress {
		t.Fail()
	}
	tk, tc, a = readConfig(&httpTestConfig{address: ":8080"})
	if !bytes.Equal(tk, dtk) || !bytes.Equal(tc, dtc) || a != ":8080" {
		t.Fail()
	}
}

func TestServe(t *testing.T) {
	err := os.Remove(fn)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	err = Serve(nil, fn)
	if err != nil {
		t.Fatal(err)
	}
	defer func(c chan int) { c <- 0 }(Stop) // ask: how this is
	resp, err := get("https://localhost" + defaultAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fail()
	}
	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	t.Fail()
	// }
	// if !bytes.Equal(body, "hello") {
	// 	t.Fail()
	// }
}
