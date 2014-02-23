package main

import (
	"testing"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"time"
	"net"
	"path"
	"io/ioutil"
	. "code.google.com/p/tasked/testing"
)

func TestSelfCert(t *testing.T) {
	test := func(host interface{}, cachedir string) {
		before := time.Now()
		before = time.Unix(before.Unix(), 0)
		notAfter := before.Add(tlsValidityDays * 24 * time.Hour)
		notAfter = time.Unix(notAfter.Unix(), 0)
		k, c, err := selfCert(host, cachedir)
		if len(k) == 0 || len(c) == 0 || err != nil {
			t.Log(err)
			t.Fail()
			return
		}
		after := time.Now()
		notAfterCap := after.Add(tlsValidityDays * 24 * time.Hour)
		testCert := func(c, k []byte) {
			_, err = tls.X509KeyPair(c, k)
			if err != nil {
				t.Fail()
			}
			b, rest := pem.Decode(c)
			if len(rest) != 0 {
				t.Fail()
			}
			tc, err := x509.ParseCertificate(b.Bytes)
			if tc == nil || err != nil {
				t.Fail()
			}
			if tc.SerialNumber == nil ||
				tc.SerialNumber.Int64() < before.Unix() ||
				tc.SerialNumber.Int64() > after.Unix() {
				t.Fail()
			}
			if tc.NotBefore.Before(before) || tc.NotBefore.After(after) {
				t.Fail()
			}
			if tc.NotAfter.Before(notAfter) || tc.NotAfter.After(notAfterCap) {
				t.Fail()
			}
			if tc.KeyUsage & x509.KeyUsageKeyEncipherment == 0 ||
				tc.KeyUsage & x509.KeyUsageDigitalSignature == 0 ||
				tc.KeyUsage & x509.KeyUsageCertSign == 0 {
				t.Fail()
			}
			if len(tc.ExtKeyUsage) == 0 || tc.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
				t.Fail()
			}
			if dname, ok := host.(string); !ok && len(tc.DNSNames) != 0 ||
				ok && (len(tc.DNSNames) != 1 || tc.DNSNames[0] != dname) {
				t.Fail()
			}
			if ip, ok := host.(net.IP); !ok && len(tc.IPAddresses) != 0 ||
				ok && (len(tc.IPAddresses) != 1 || !tc.IPAddresses[0].Equal(ip)) {
				t.Fail()
			}
		}
		testCert(c, k)
		if cachedir != "" {
			kfn := path.Join(cachedir, keyFilename)
			cfn := path.Join(cachedir, certFilename)
			k, err := ioutil.ReadFile(kfn)
			ErrFatal(t, err)
			c, err := ioutil.ReadFile(cfn)
			ErrFatal(t, err)
			t.Log(string(k))
			t.Log(string(c))
			t.Log("here")
			testCert(c, k)
		}
	}

	test(nil, "")
	test("host.domain", "")
	test(net.ParseIP("42.42.42.42"), "")

	cachedir := path.Join(Testdir, "cache")
	EnsureDirF(t, cachedir)
	test(nil, cachedir)
	test("host.domain", cachedir)
	test(net.ParseIP("42.42.42.42"), cachedir)
}
