package app

import (
	"crypto/aes"
	"crypto/rand"
	"io"
	"os"
	"strconv"
	"testing"
)

const testTokenValidity = 5400

func makeKey() ([]byte, error) {
	s := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, s); err != nil {
		return nil, err
	}
	return s, nil
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
	return InitSec(key, iv, testTokenValidity)
}

func TestCheckCred(t *testing.T) {
	testAuth, err := strconv.ParseBool(os.Getenv("AUTH"))
	if err != nil || !testAuth {
		t.Skip()
	}
	if nil != checkCred("test", "testpwd") {
		t.Fail()
	}
	if nil == checkCred("testx", "testpwd") {
		t.Fail()
	}
	if nil == checkCred("test", "testpwdx") {
		t.Fail()
	}
	if nil == checkCred("test", "") {
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

func TestInit(t *testing.T) {
	key, err := makeKey()
	if err != nil {
		t.Fail()
	}
	iv, err := makeKey()
	if err != nil {
		t.Fail()
	}

	if nil != InitSec(key, iv, testTokenValidity) {
		t.Fail()
	}
	if nil == InitSec(nil, iv, testTokenValidity) {
		t.Fail()
	}
	if nil == InitSec(key[:15], iv[:15], testTokenValidity) {
		t.Fail()
	}
}

func TestTokenEncryptDecrypt(t *testing.T) {
	err := initTestSec()
	if err != nil {
		t.Fatal(err)
	}

	tk := token{user: "some", created: 42}
	v, err := tk.encrypt()
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
	if tk.val != nil {
		t.Fail()
	}
	tk = token{user: "some", created: 42}
	verify, err := tk.encrypt()
	if err != nil || !eqbytes(val, verify) {
		t.Fail()
	}
}
