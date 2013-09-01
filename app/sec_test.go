package app

import (
	"crypto/aes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

const (
	testTokenValidity = 5400
	testValidity      = 20
)

type testConfig struct {
	aesKey        []byte
	aesIv         []byte
	tokenValidity int
}

func (c *testConfig) AesKey() []byte     { return c.aesKey }
func (c *testConfig) AesIv() []byte      { return c.aesIv }
func (c *testConfig) TokenValidity() int { return c.tokenValidity }

type testToken struct {
	val []byte
}

func (t *testToken) Value() []byte { return t.val }

func envdef(key, dflt string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		return dflt
	}
	return val
}

func makeRandom(l int) []byte {
	b := make([]byte, l)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("Failed to generate random bytes.")
	}
	return b
}

func makeKey() []byte { return makeRandom(aes.BlockSize) }

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

func initTestSec() {
	key := makeKey()
	iv := makeKey()
	InitSec(&testConfig{key, iv, testTokenValidity})
}

func TestEqbytes(t *testing.T) {
	if !eqbytes() {
		t.Fail()
	}
	if !eqbytes(nil) {
		t.Fail()
	}
	if !eqbytes(nil, nil) {
		t.Fail()
	}
	if !eqbytes([]byte{1, 2}, []byte{1, 2}) {
		t.Fail()
	}
	if eqbytes([]byte{1, 1}, []byte{1, 2}) {
		t.Fail()
	}
	if !eqbytes([]byte{1, 2}, []byte{1, 2}, []byte{1, 2}) {
		t.Fail()
	}
	if eqbytes([]byte{1, 2}, []byte{1, 1}, []byte{1, 2}) {
		t.Fail()
	}
}

func TestTokenEncryptDecrypt(t *testing.T) {
	initTestSec()

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

	blank := make([]byte, len(v))
	tk, err = decryptToken(blank)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}

	random := makeRandom(len(v))
	tk, err = decryptToken(random)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}

	tk = token{user: "some", created: 42}
	v, _ = encrypt(tk)
	tiv := iv
	iv = makeKey()
	_, err = decryptToken(v)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
	iv = tiv
}

func TestTokenValue(t *testing.T) {
	initTestSec()

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

func TestEncryption(t *testing.T) {
	initTestSec()

	key = makeKey()
	iv = makeKey()

	if nil != verifyEncryption() {
		t.Fail()
	}

	key = nil
	if nil == verifyEncryption() {
		t.Fail()
	}

	key = makeKey()[:15]
	iv = makeKey()[:15]
	if nil == verifyEncryption() {
		t.Fail()
	}
}

func TestCheckCred(t *testing.T) {
	initTestSec()

	testAuth, err := strconv.ParseBool(os.Getenv("AUTH"))
	if err != nil || !testAuth {
		t.Skip()
	}
	testusr := envdef("TESTUSR", "test")
	testpwd := envdef("TESTPWD", "testpwd")

	if nil != checkCred(testusr, testpwd) {
		t.Fail()
	}
	if nil == checkCred(testusr+"x", testpwd) {
		t.Fail()
	}
	if nil == checkCred(testusr, testpwd+"x") {
		t.Fail()
	}
	if nil == checkCred(testusr, "") {
		t.Fail()
	}
	if nil == checkCred("", "") {
		t.Fail()
	}
}

func TestValidate(t *testing.T) {
	initTestSec()

	testusr := envdef("TESTUSR", "test")
	tk := token{}
	tback, err := validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
	tk = token{user: testusr, created: time.Now().Unix()}
	tback, err = validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created {
		t.Fail()
	}
	tk = token{user: testusr, created: time.Now().Unix()}
	tk.Value()
	tback, err = validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created ||
		!eqbytes(tback.val, tk.val) {
		t.Fail()
	}

	InitSec(&testConfig{key, iv, testValidity})

	created := time.Now().Add(-time.Second).Unix()
	tk = token{created: created}
	val := tk.Value()
	tk, err = validate(tk)
	nval := tk.val
	if err != nil || tk.created != created || !eqbytes(val, tk.Value(), nval) {
		t.Fail()
	}

	created = time.Now().Add(-2 * time.Second).Unix()
	tk = token{created: created}
	val = tk.Value()
	tk, err = validate(tk)
	nval = tk.val
	if err != nil || tk.created <= created ||
		nval != nil || eqbytes(val, tk.Value()) {
		t.Fail()
	}

	tk = token{created: time.Now().Add(-21 * time.Second).Unix()}
	_, err = validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
}

func TestValidateTime(t *testing.T) {
	initTestSec()

	testTime, err := strconv.ParseBool(os.Getenv("TEST_IN_TIME"))
	if err != nil || !testTime {
		t.Skip()
	}
	InitSec(&testConfig{key, iv, testValidity})

	created := time.Now().Unix()
	tk := token{created: created}
	val := tk.Value()

	time.Sleep(1 * time.Second)
	tk, err = validate(tk)
	nval := tk.val
	if err != nil || tk.created != created || !eqbytes(val, tk.Value(), nval) {
		t.Fail()
	}

	time.Sleep(2 * time.Second)
	tk, err = validate(tk)
	nval = tk.val
	if err != nil || tk.created <= created ||
		nval != nil || eqbytes(val, tk.Value()) {
		t.Fail()
	}

	time.Sleep(testValidity * time.Second)
	_, err = validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
}

func TestAuthPwd(t *testing.T) {
	initTestSec()

	testAuth, err := strconv.ParseBool(os.Getenv("AUTH"))
	if err != nil || !testAuth {
		t.Skip()
	}
	testusr := envdef("TESTUSR", "test")
	testpwd := envdef("TESTPWD", "testpwd")

	tk, err := AuthPwd(testusr, testpwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	_, err = AuthPwd(testusr+"x", testpwd)
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd(testusr, testpwd+"x")
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd(testusr, "")
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd("", "")
	if err == nil {
		t.Fail()
	}
}

func TestAuthToken(t *testing.T) {
	initTestSec()

	_, err := AuthToken(nil)
	if err == nil {
		t.Fail()
	}

	tt := &testToken{}
	_, err = AuthToken(tt)
	if err == nil {
		t.Fail()
	}

	tk := &token{created: time.Now().Unix()}
	tt = &testToken{tk.Value()}
	tback, err := AuthToken(tt)
	if err != nil || tback == nil || !eqbytes(tk.Value(), tback.Value()) {
		t.Fail()
	}

	random := makeRandom(len(tk.Value()))
	tt = &testToken{random}
	_, err = AuthToken(tt)
	if err == nil {
		t.Fail()
	}

	tback, err = AuthToken(tk)
	if err != nil || tback == nil || !eqbytes(tk.Value(), tback.Value()) {
		t.Fail()
	}

	_, err = AuthToken(&token{})
	if err == nil {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	tiv := iv
	iv = makeKey()
	_, err = AuthToken(&token{})
	if err == nil {
		t.Fail()
	}
	iv = tiv

	InitSec(&testConfig{key, iv, testValidity})
	tk = &token{created: time.Now().Add(-1 * time.Second).Unix()}
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || !eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Add(-2 * time.Second).Unix()}
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Add(-21 * time.Second).Unix()}
	_, err = AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthTokenTime(t *testing.T) {
	testTime, err := strconv.ParseBool(os.Getenv("TEST_IN_TIME"))
	if err != nil || !testTime {
		t.Skip()
	}
	InitSec(&testConfig{key, iv, testValidity})

	tk := &token{created: time.Now().Unix()}
	time.Sleep(1 * time.Second)
	tback, err := AuthToken(tk)
	if err != nil || tback == nil || !eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(2 * time.Second)
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(20 * time.Second)
	_, err = AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytes(t *testing.T) {
	initTestSec()

	_, err := AuthTokenBytes(nil)
	if err == nil {
		t.Fail()
	}

	tk := &token{}
	blank := make([]byte, len(tk.Value()))
	_, err = AuthTokenBytes(blank)
	if err == nil {
		t.Fail()
	}

	random := makeRandom(len(tk.Value()))
	_, err = AuthTokenBytes(random)
	if err == nil {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	tback, err := AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{}
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}

	InitSec(&testConfig{key, iv, testValidity})
	tk = &token{created: time.Now().Add(-1 * time.Second).Unix()}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Add(-2 * time.Second).Unix()}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Add(-21 * time.Second).Unix()}
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytesTime(t *testing.T) {
	testTime, err := strconv.ParseBool(os.Getenv("TEST_IN_TIME"))
	if err != nil || !testTime {
		t.Skip()
	}
	InitSec(&testConfig{key, iv, testValidity})
	tk := &token{created: time.Now().Unix()}
	time.Sleep(1 * time.Second)
	tback, err := AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	time.Sleep(2 * time.Second)
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || eqbytes(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	time.Sleep(20 * time.Second)
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestGetUser(t *testing.T) {
	initTestSec()
}

func TestGetUserTime(t *testing.T) {
	testTime, err := strconv.ParseBool(os.Getenv("TEST_IN_TIME"))
	if err != nil || !testTime {
		t.Skip()
	}
	InitSec(&testConfig{key, iv, testValidity})
}

func TestAuthState(t *testing.T) {
	initTestSec()
}

func TestAuthStateTime(t *testing.T) {
	testTime, err := strconv.ParseBool(os.Getenv("TEST_IN_TIME"))
	if err != nil || !testTime {
		t.Skip()
	}
	InitSec(&testConfig{key, iv, testValidity})
}
