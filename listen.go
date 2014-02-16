package main

import (
	"crypto/tls"
	"io/ioutil"
	"net"
	"os"
)

const noTlsWarning = "TLS has not been configured."

func readKey(fn string) ([]byte, error) {
	if fn == "" {
		return nil, nil
	}
	key, err := ioutil.ReadFile(fn)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return key, err
}

func selfCert() {
}

func getTcpSettings(s *settings) ([]byte, []byte, string, error) {
	/*
		var (
			tlsKey, tlsCert []byte
			address         string
			err             error
		)
		if s != nil {
			tlsKey, err = s.TlsKey()
			if err != nil {
				return nil, nil, "", err
			}
			tlsCert, err = s.TlsCert()
			if err != nil {
				return nil, nil, "", err
			}
			address = s.Address()
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
		if address == "" {
			address = defaultAddress
		}
		return tlsKey, tlsCert, address, nil
	*/
	return nil, nil, "", nil
}

func listenTcp(s *settings) (net.Listener, error) {
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

func listenUnix(s *settings) (net.Listener, error) {
	var (
		l   net.Listener
		err error
	)
	addr := s.Address()
	if err = os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return l, err
	}
	return net.Listen("unixpacket", addr)
}

// shall it be able to listen on multiple channels?
// no: use multiple processes. check if it is possible to differentiate between unix and tcp address, and if
// yes, then use only one address flag.
