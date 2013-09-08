// Package app runs the http server.
package app

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
var Stop chan int = make(chan int)

// Interface containing the configuration options accepted by the app package.
type HttpConfig interface {
	Address() string
	TlsKey() []byte
	TlsCert() []byte
}

// Read the config or fall back to defaults.
func readConfig(config HttpConfig) (tlsKey, tlsCert []byte, address string) {
	tlsKey = []byte(defaultTlsKey)
	tlsCert = []byte(defaultTlsCert)
	address = defaultAddress
	if config == nil {
		log.Println(noTlsWarning)
	} else {
		tk := config.TlsKey()
		if len(tk) == 0 {
			log.Println(noTlsWarning)
		} else {
			tlsKey = tk
		}
		tc := config.TlsCert()
		if len(tc) == 0 {
			log.Println(noTlsWarning)
		} else {
			tlsCert = tc
		}
		a := config.Address()
		if len(a) > 0 {
			address = a
		}
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
		<-Stop
		stopped = true
		err := l.Close()
		if err != nil {
			log.Println(err)
		}
	}()
	go func() {
		err := http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello"))
		}))
		if _, ok := err.(*net.OpError); err != nil && (!ok && stopped || !stopped) {
			log.Println(err)
		}
	}()
}

// Starts a http server that can be stopped by signaling the Stop channel.
// Within the config, TLS certification and key must be provided. (The hardcoded default serves only testing
// purpose.)
func Serve(config HttpConfig) error {
	tlsKey, tlsCert, address := readConfig(config)
	l, err := listen(tlsKey, tlsCert, address)
	if err != nil {
		return err
	}
	startStop(l)
	return nil
}
