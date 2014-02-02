package main

import (
	"os"
	"log"
	"net/http"
	"net"
	"bytes"
	"net/http/httputil"
	"io"
	"fmt"
	"strings"
)

func close(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("Missing method and address.")
	}
	addr := os.Args[1]
	method := os.Args[2]
	var path, data string
	if len(os.Args) > 3 {
		path = os.Args[3]
		if len(os.Args) > 4 {
			data = os.Args[4]
		}
	}
	r, err := http.NewRequest(method, path, bytes.NewBufferString(data))
	if err != nil {
		log.Fatalln(err)
	}
	nc, err := net.Dial("unixpacket", addr)
	if err != nil {
		log.Fatalln(err)
	}
	hc := httputil.NewClientConn(nc, nil)
	defer close(hc)
	rsp, err := hc.Do(r)
	if err != nil {
		log.Fatalln(err)
	}
	defer close(rsp.Body)
	fmt.Printf("%s %s\n", rsp.Proto, rsp.Status)
	for k, v := range rsp.Header {
		fmt.Printf("%s: %s\n", k, strings.Join(v, ", "))
	}
	io.Copy(os.Stdout, rsp.Body)
}
