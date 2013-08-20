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
