package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"os"
)

const noTlsWarning = "TLS has not been set."

func readKey(fn string) ([]byte, error) {
	if len(fn) == 0 {
		return nil, nil
	}
	key, err := ioutil.ReadFile(fn)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return key, err
}

func getTcpSettings(s *settings) ([]byte, []byte, string, error) {
	tlsKey, err := readKey(s.http.tls.keyFile)
	if err != nil {
		return nil, nil, "", err
	}
	tlsCert, err := readKey(s.http.tls.certFile)
	if err != nil {
		return nil, nil, "", err
	}
	if len(tlsKey) == 0 || len(tlsCert) == 0 {
		log.Println(noTlsWarning)
	}
	if len(tlsKey) == 0 {
		tlsKey = []byte(defaultTlsKey)
	}
	if len(tlsCert) == 0 {
		tlsCert = []byte(defaultTlsCert)
	}
	address := s.http.address
	if len(address) == 0 {
		address = defaultAddress
	}
	return tlsKey, tlsCert, address, nil
}

func listen(s *settings) (net.Listener, error) {
	tlsKey, tlsCert, address, err := getTcpSettings(s)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, err
	}
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	l = tls.NewListener(l, &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}})
	return l, nil
}
