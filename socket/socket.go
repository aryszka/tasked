// package to make and to fake http connections on unix sockets
package socket

import "net/http"

// server
func Serve(name string, hnd http.Handler) error {
	return nil
}

// or better if just returning a listener? fits more the rest of the code

// client
func Send(name string, w http.ResponseWriter, r *http.Request) error {
	return nil
}
