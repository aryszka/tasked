// Package app runs the http server.
package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
)

const (
	noTlsWarning = "Tls has not been configured."
)

// Channel that, if signaled, stops the http server.
var stop chan int = make(chan int)

// Read the config or fall back to defaults.
func readHttpConfig() (tlsKey, tlsCert []byte, address string) {
	tlsKey = []byte(defaultTlsKey)
	tlsCert = []byte(defaultTlsCert)
	address = defaultAddress
	tk := cfg.http.tls.key
	if len(tk) == 0 {
		log.Println(noTlsWarning)
	} else {
		tlsKey = tk
	}
	tc := cfg.http.tls.cert
	if len(tc) == 0 {
		log.Println(noTlsWarning)
	} else {
		tlsCert = tc
	}
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
		return nil, err
	}
	l = tls.NewListener(l, &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}})
	return l, nil
}

// Wait for a single stop signal, and start the http server in the background.
func startStop(l net.Listener) {
	stopped := false
	go func() {
		<-stop
		stopped = true
		err := l.Close()
		if err != nil {
			log.Println(err)
		}
	}()
	go func() {
		err := http.Serve(l, http.HandlerFunc(handler))
		if _, ok := err.(*net.OpError); err != nil && (!ok && stopped || !stopped) {
			log.Println(err)
		}
	}()
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
	startStop(l)
	return nil
}
