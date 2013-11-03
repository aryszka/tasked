package main

import (
	"bytes"
	"code.google.com/p/tasked/util"
	"os"
	"path"
	"testing"
)

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

func TestReadKey(t *testing.T) {
	key, err := readKey("")
	if err != nil || key != nil {
		t.Fail()
	}

	tkey := []byte("1234567890123456")
	p := path.Join(util.Testdir, "test-key")
	util.WithNewFileF(t, p, func(f *os.File) error {
		_, err := f.Write(tkey)
		return err
	})
	key, err = readKey(p)
	if err != nil || !bytes.Equal(key, tkey) {
		t.Fail()
	}

	util.RemoveIfExistsF(t, p)
	key, err = readKey(p)
	if err != nil || key != nil {
		t.Fail()
	}
}

func TestGetTcpSettings(t *testing.T) {
	s := settings{}
	key, cert, address, err := getTcpSettings(&s)
	if err != nil ||
		!bytes.Equal(key, []byte(defaultTlsKey)) ||
		!bytes.Equal(cert, []byte(defaultTlsCert)) ||
		address != defaultAddress {
		t.Fail()
	}

	pk := path.Join(util.Testdir, "test-key")
	util.WithNewFileF(t, pk, func(f *os.File) error {
		_, err := f.Write([]byte(testTlsKey))
		return err
	})
	pc := path.Join(util.Testdir, "test-cert")
	util.WithNewFileF(t, pc, func(f *os.File) error {
		_, err := f.Write([]byte(testTlsCert))
		return err
	})
	s.http.tls.keyFile = pk
	s.http.tls.certFile = pc
	s.http.address = ":9091"
	key, cert, address, err = getTcpSettings(&s)
	if err != nil ||
		!bytes.Equal(key, []byte(testTlsKey)) ||
		!bytes.Equal(cert, []byte(testTlsCert)) ||
		address != ":9091" {
		t.Fail()
	}
}

func TestGetTcpSettingsNotRoot(t *testing.T) {
	if util.IsRoot {
		t.Skip()
	}

	p := path.Join(util.Testdir, "test-key")
	util.WithNewFileF(t, p, nil)
	err := os.Chmod(p, 0)
	util.ErrFatal(t, err)
	defer func() {
		err := os.Chmod(p, os.ModePerm)
		util.ErrFatal(t, err)
	}()
	s := settings{}
	s.http.tls.keyFile = p
	_, _, _, err = getTcpSettings(&s)
	if err == nil {
		t.Fail()
	}
}

func TestListen(t *testing.T) {
	pk := path.Join(util.Testdir, "key-file")
	util.WithNewFileF(t, pk, func(f *os.File) error {
		_, err := f.Write([]byte("123"))
		return err
	})
	var s settings
	s.http.tls.keyFile = pk
	_, err := listen(&s)
	if err == nil {
		t.Fail()
	}
}
