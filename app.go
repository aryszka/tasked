// Package app runs the http server.
package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
)

const noTlsWarning = "Tls has not been configured."

// Read the config or fall back to defaults.
func readHttpConfig() (tlsKey, tlsCert []byte, address string) {
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

// Start a TLS wrapped TCP listener.
func listen(tlsKey, tlsCert []byte, address string) (net.Listener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		errClose := l.Close()
		if errClose != nil {
			log.Println(errClose)
		}
		return nil, err
	}
	l = tls.NewListener(l, &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}})
	return l, nil
}

// Starts a http server that can be stopped by signaling the Stop channel.
// Within the config, TLS certification and key must be provided. (The hardcoded default serves only testing
// purpose.)
func serve() error {
	tlsKey, tlsCert, address := readHttpConfig()
	l, err := listen(tlsKey, tlsCert, address)
	if err != nil {
		return err
	}
	return http.Serve(l, http.HandlerFunc(handler))
}
