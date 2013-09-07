package app

import (
	"net/http"
	"testing"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"crypto/tls"
)

const testdataKey = "testdata"

type httpTestConfig struct {
	address string
	tlsKey []byte
	tlsCert []byte
}

func (tc *httpTestConfig) Address() string { return tc.address }
func (tc *httpTestConfig) TlsKey() []byte { return tc.tlsKey }
func (tc *httpTestConfig) TlsCert() []byte { return tc.tlsCert }

func newTestConfig() (*httpTestConfig, error) {
	testdir := os.Getenv(testdataKey)
	key, err := ioutil.ReadFile(path.Join(testdir, "tls/key"))
	if err != nil {
		return nil, err
	}
	cert, err := ioutil.ReadFile(path.Join(testdir, "tls/cert"))
	if err != nil {
		return nil, err
	}
	return &httpTestConfig{tlsKey: key, tlsCert: cert}, nil
}

func get(url string) (*http.Response, error) {
	return (&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true}}}).Get(url)
}

func TestServe(t *testing.T) {
	tc, err := newTestConfig()
	if err != nil {
		t.Fatal()
	}
	err = Serve(tc)
	if err != nil {
		t.Fatal(err)
	}
	defer func(c chan int) { c <- 0 }(Stop) // ask: how this is
	resp, err := get("https://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fail()
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fail()
	}
	if !bytes.Equal(body, []byte("hello")) {
		t.Fail()
	}
}
