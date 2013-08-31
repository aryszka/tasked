package app

import (
	"crypto/aes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"
)

const testTokenValidity = 5400

type testConfig struct {
	aesKey []byte
	aesIv []byte
	tokenValidity int
}

func (c *testConfig) AesKey() []byte { return c.aesKey }
func (c *testConfig) AesIv() []byte { return c.aesIv }
func (c *testConfig) TokenValidity() int { return c.tokenValidity }

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func makeKey() ([]byte, error) {
	s := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, s); err != nil {
		return nil, err
	}
	return s, nil
}

// verify encryption keys by encrypting/decrypting a test datum
// used on startup
// TODO: move out from here into a generic startup health check
func verifyEncryption() error {
	test := "Test encryption message."
	enc, err := crypt([]byte(test))
	if err != nil {
		return err
	}
	dec, err := crypt(enc)
	if err != nil {
		return err
	}
	if string(dec) != test {
		errors.New("Failed to initialize encryption.")
	}
	return nil
}

func initTestSec() error {
	key, err := makeKey()
	if err != nil {
		return err
	}
	iv, err := makeKey()
	if err != nil {
		return err
	}
	InitSec(&testConfig{key, iv, testTokenValidity})
	return nil
}

func TestTokenEncryptDecrypt(t *testing.T) {
	err := initTestSec()
	if err != nil {
		t.Fatal(err)
	}

	tk := token{user: "some", created: 42}
	v, err := encrypt(tk)
	if err != nil || v == nil || len(v) == 0 {
		t.Fail()
	}

	verify, err := decryptToken(v)
	if err != nil || !eqbytes(verify.val, v) ||
		verify.user != tk.user || verify.created != tk.created {
		t.Fail()
	}
}

func TestTokenValue(t *testing.T) {
	tk := token{user: "some", created: 42}
	val := tk.Value()
	if val == nil || !eqbytes(tk.val, val) {
		t.Fail()
	}
	tk = token{user: "some", created: 42}
	verify, err := encrypt(tk)
	if err != nil || !eqbytes(val, verify) {
		t.Fail()
	}
}

func TestCheckCred(t *testing.T) {
	testAuth, err := strconv.ParseBool(os.Getenv("AUTH"))
	if err != nil || !testAuth {
		t.Skip()
	}
	testusr := envdef("TESTUSR", "test")
	testpwd := envdef("TESTPWD", "testpwd")

	if nil != checkCred(testusr, testpwd) {
		t.Fail()
	}
	if nil == checkCred(testusr + "x", testpwd) {
		t.Fail()
	}
	if nil == checkCred(testusr, testpwd + "x") {
		t.Fail()
	}
	if nil == checkCred(testusr, "") {
		t.Fail()
	}
	if nil == checkCred("", "") {
		t.Fail()
	}
}

func TestEncryption(t *testing.T) {
	var err error

	key, err = makeKey()
	if err != nil {
		t.Fail()
	}
	iv, err = makeKey()
	if err != nil {
		t.Fail()
	}
	if nil != verifyEncryption() {
		t.Fail()
	}

	key = nil
	if nil == verifyEncryption() {
		t.Fail()
	}

	key, err = makeKey()
	key = key[:15]
	if err != nil {
		t.Fail()
	}
	iv = iv[:15]
	if nil == verifyEncryption() {
		t.Fail()
	}
}
