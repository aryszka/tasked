package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
)

const noTlsWarning = "TLS has not been configured."

func getHttpConfig() (tlsKey, tlsCert []byte, address string) {
	tlsKey = []byte(defaultTlsKey)
	tlsCert = []byte(defaultTlsCert)
	tk := cfg.http.tls.key
	tc := cfg.http.tls.cert
	if len(tk) == 0 || len(tc) == 0 {
		log.Println(noTlsWarning)
	}
	if len(tk) > 0 {
		tlsKey = tk
	}
	if len(tc) > 0 {
		tlsCert = tc
	}
	address = defaultAddress
	a := cfg.http.address
	if len(a) > 0 {
		address = a
	}
	return tlsKey, tlsCert, address
}

func listen(tlsKey, tlsCert []byte, address string) (net.Listener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		doretlog42(l.Close)
		return nil, err
	}
	l = tls.NewListener(l, &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}})
	return l, nil
}

func serve() error {
	l, err := listen(getHttpConfig())
	if err != nil {
		return err
	}
	defer doretlog42(l.Close)
	return http.Serve(l, http.HandlerFunc(handler))
}
