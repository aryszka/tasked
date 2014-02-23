package main

import (
	"math/big"
	"crypto/rsa"
	"crypto/rand"
	"crypto/x509"
	"time"
	"encoding/pem"
	"io/ioutil"
	"path"
	"net"
	"os"
)

const (
	bits = 2048
	tlsValidityDays = 11499
	keyFilename = "key.pem"
	certFilename = "cert.pem"
)

func selfCert(host interface{}, cachedir string) ([]byte, []byte, error) {
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	notAfter := now.Add(tlsValidityDays * 24 * time.Hour)
	t := x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		NotBefore: now,
		NotAfter: notAfter,
		IsCA: true,
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if dname, ok := host.(string); ok {
		t.DNSNames = []string{dname}
	} else if ip, ok := host.(net.IP); ok {
		t.IPAddresses = []net.IP{ip}
	}
	c, err := x509.CreateCertificate(rand.Reader, &t, &t, &k.PublicKey, k)
	if err != nil {
		return nil, nil, err
	}
	key := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c})
	if cachedir != "" {
		keyfn := path.Join(cachedir, keyFilename)
		err = ioutil.WriteFile(keyfn, key, os.FileMode(0600))
		if err != nil {
			return nil, nil, err
		}
		certfn := path.Join(cachedir, certFilename)
		err = ioutil.WriteFile(certfn, cert, os.FileMode(0600))
		if err != nil {
			return nil, nil, err
		}
	}
	return key, cert, nil
}
