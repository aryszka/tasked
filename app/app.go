// Package app runs the tasked backend process.
package app

import (
	"errors"
	"log"
	"net"
	"net/http"
	"crypto/tls"
)

const defaultAddress = ":9090"

var Stop chan int = make(chan int)

type HttpConfig interface {
	Address() string
	TlsKey() []byte
	TlsCert() []byte
}

func Serve(config HttpConfig) error {
	if config == nil {
		return errors.New("Server requires TLS configuration.")
	}
	address := defaultAddress
	a := config.Address()
	if len(a) > 0 {
		address = a
	}
	l, err := net.Listen("tcp", address) // todo: address as config, default 9090, but still https
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(config.TlsCert(), config.TlsKey())
	if err != nil {
		return err
	}
	l = tls.NewListener(l, &tls.Config{
		NextProtos: []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}})
	stopped := false
	go func() {
		<- Stop
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
	return nil
}
