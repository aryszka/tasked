// Package app runs the tasked backend process.
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

var Stop chan int = make(chan int)

type HttpConfig interface {
	Address() string
	TlsKey() []byte
	TlsCert() []byte
}

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

func Serve(config HttpConfig) error {
	tlsKey, tlsCert, address := readConfig(config)
	l, err := listen(tlsKey, tlsCert, address)
	if err != nil {
		return err
	}
	startStop(l)
	return nil
}
