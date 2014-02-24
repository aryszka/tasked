package main

import (
	. "code.google.com/p/tasked/share"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
)

const noTlsWarning = "TLS has not been configured."

type schema string
type port uint16

const (
	schemaHttp            schema = "http"
	schemaHttps           schema = "https"
	schemaUnix            schema = "unix"
	defaultPort           port   = 9090
	defaultUnixAddressFmt        = "nlet-%d"
)

type listenerOptions interface {
	Address() string
	Cachedir() string
	TlsKey() ([]byte, error)
	TlsCert() ([]byte, error)
}

type address struct {
	schema schema
	val    string
	port   port
}

var (
	addressRx = regexp.MustCompile(
		"^((((https?)|(unix)):)?((/{0,2}(([^:]*)|(\\[.*\\])))(:(\\d+))?)$)|" +
			"(unix:(.*)$)")
	invalidAddress = errors.New("Invalid address.")
)

func parseAddress(s string) (*address, error) {
	m := addressRx.FindStringSubmatch(s)
	if len(m) == 0 {
		return nil, invalidAddress
	}
	sch := schema(m[3])
	anyAddr := m[6]
	tcpAddr := m[8]
	tcpPort := m[12]
	unixAddr := m[14]
	a := new(address)
	switch {
	case sch == schemaUnix:
		a.schema = schemaUnix
		a.val = anyAddr
		return a, nil
	case unixAddr != "":
		a.schema = schemaUnix
		a.val = unixAddr
		return a, nil
	default:
		a.schema = sch
		a.val = tcpAddr
		if tcpPort != "" {
			p, err := strconv.Atoi(tcpPort)
			if err != nil {
				return nil, err
			}
			a.port = port(p)
		}
		return a, nil
	}
}

func getListenerParams(addr *address) (string, string) {
	n := "tcp"
	a := addr.val
	if addr.schema == schemaUnix {
		n = "unixpacket"
		if a == "" {
			a = fmt.Sprintf(defaultUnixAddressFmt, os.Getpid())
		}
	} else {
		p := addr.port
		if p <= 0 {
			p = 9090
		}
		a += ":" + strconv.FormatUint(uint64(p), 10)
	}
	return n, a
}

func listenTls(l net.Listener, a *address, o listenerOptions) (net.Listener, error) {
	tlsKey, err := o.TlsKey()
	if err != nil {
		return nil, err
	}
	tlsCert, err := o.TlsCert()
	if err != nil {
		return nil, err
	}
	if len(tlsKey) == 0 && len(tlsCert) == 0 {
		println("why 0")
		var host interface{}
		if len(a.val) > 0 {
			ip := net.ParseIP(a.val)
			if ip == nil && a.val[0] == '[' && a.val[len(a.val)-1] == ']' {
				ip = net.ParseIP(a.val[1 : len(a.val)-1])
			}
			if ip == nil {
				host = a.val
			} else {
				host = ip
			}
		}
		cachedir := o.Cachedir()
		if cachedir != "" {
			cachedir = path.Join(cachedir, "p"+strconv.Itoa(os.Getpid()), "tls")
			err = EnsureDir(cachedir)
			if err != nil {
				return nil, err
			}
		}
		tlsKey, tlsCert, err = selfCert(host, cachedir)
		if err != nil {
			return nil, err
		}
	}
	println("so actually here")
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		println("should come here")
		return nil, err
	}
	return tls.NewListener(l, &tls.Config{
		NextProtos:   []string{"http/1.1"},
		Certificates: []tls.Certificate{cert}}), nil
}

func listen(o listenerOptions) (net.Listener, error) {
	addr, err := parseAddress(o.Address())
	if err != nil {
		return nil, err
	}
	n, a := getListenerParams(addr)
	l, err := net.Listen(n, a)
	if err != nil || addr.schema != schemaHttps {
		return l, err
	}
	tl, err := listenTls(l, addr, o)
	if err != nil {
		println("this error is the one")
		defer Doretlog42(l.Close)
	}
	return tl, err
}
