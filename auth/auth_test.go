package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"errors"
	"flag"
	"io"
	"os"
	"testing"
	"time"
	tst "code.google.com/p/tasked/testing"
)

var testLong = false

func init() {
	tl := flag.Bool("test.long", false, "")
	flag.Parse()
	testLong = *tl
}

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
	if val == "" {
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

func checkFunc(u, p string) error {
	if len(u) > 0 && u == p {
		return nil
	}
	return errors.New("Authentication failed.")
}

func defaultInstance() *Type {
	i, err := New(PasswordCheckerFunc(checkFunc), &testConfig{makeKey(), makeKey(), 18})
	if err != nil {
		panic(err)
	}
	return i
}

func (a *Type) durNoRefresh() time.Duration {
	return time.Duration(float64(a.tokenValidity) * renewThresholdRate / 3)
}

func (a *Type) pastNoRefresh() int64 {
	return time.Now().Add(-a.durNoRefresh()).Unix()
}

func (a *Type) pastRefresh() int64 {
	return time.Now().Add(-a.renewThreshold).Unix()
}

func (a *Type) pastInvalid() int64 {
	return time.Now().Add(-a.tokenValidity).Unix()
}

func encryptf(t *testing.T, i *Type, tk *token) {
	val, err := i.encrypt(*tk)
	tst.ErrFatal(t, err)
	tk.val = val
}

func TestTokenEncryptDecrypt(t *testing.T) {
	i := defaultInstance()

	tk := token{user: "some", created: 42}
	v, err := i.encrypt(tk)
	if err != nil || v == nil || len(v) == 0 {
		t.Fail()
	}

	verify, err := i.decryptToken(v)
	if err != nil || !bytes.Equal(verify.val, v) ||
		verify.user != tk.user || verify.created != tk.created {
		t.Fail()
	}

	blank := make([]byte, len(v))
	tk, err = i.decryptToken(blank)
	if err == nil || err != invalidToken {
		t.Fail()
	}

	random := makeRandom(len(v))
	tk, err = i.decryptToken(random)
	if err == nil || err != invalidToken {
		t.Fail()
	}

	tk = token{user: "some", created: 42}
	v, _ = i.encrypt(tk)
	tiv := i.iv
	i.iv = makeKey()
	_, err = i.decryptToken(v)
	if err == nil || err != invalidToken {
		t.Fail()
	}
	i.iv = tiv
}

func TestEncryption(t *testing.T) {
	i := defaultInstance()
	verifyEncryption := func() error {
		test := "Test encryption message."
		enc, err := i.crypt([]byte(test))
		if err != nil {
			return err
		}
		dec, err := i.crypt(enc)
		if err != nil {
			return err
		}
		if string(dec) != test {
			errors.New("Failed to initialize encryption.")
		}
		return nil
	}
	i.key = makeKey()
	i.iv = makeKey()

	if nil != verifyEncryption() {
		t.Fail()
	}

	i.key = nil
	if nil == verifyEncryption() {
		t.Fail()
	}

	i.key = makeKey()[:15]
	i.iv = makeKey()[:15]
	if nil == verifyEncryption() {
		t.Fail()
	}
}

func TestValidate(t *testing.T) {
	i := defaultInstance()

	tk := token{}
	tback, err := i.validate(tk)
	if err == nil || err != invalidToken {
		t.Fail()
	}
	tk = token{user: "test", created: time.Now().Unix()}
	tback, err = i.validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created ||
		!bytes.Equal(tback.val, tk.val) {
		t.Fail()
	}
	tk = token{user: "test", created: time.Now().Unix()}
	encryptf(t, i, &tk)
	tk.Value()
	tback, err = i.validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created ||
		!bytes.Equal(tback.val, tk.val) {
		t.Fail()
	}

	created := i.pastNoRefresh()
	tk = token{created: created}
	encryptf(t, i, &tk)
	val := tk.Value()
	tk, err = i.validate(tk)
	nval := tk.val
	if err != nil || tk.created != created ||
		!bytes.Equal(val, tk.Value()) || !bytes.Equal(val, nval) {
		t.Fail()
	}

	created = i.pastRefresh()
	tk = token{created: created}
	val, err = i.encrypt(tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	tk, err = i.validate(tk)
	if err != nil || tk.created <= created ||
		bytes.Equal(val, tk.Value()) {
		t.Fail()
	}

	tk = token{created: i.pastInvalid()}
	_, err = i.validate(tk)
	if err == nil || err != invalidToken {
		t.Fail()
	}
}

func TestValidateTime(t *testing.T) {
	if !testLong {
		t.Skip()
	}
	i := defaultInstance()
	created := time.Now().Unix()
	tk := token{created: created}
	encryptf(t, i, &tk)
	val := tk.Value()

	time.Sleep(i.durNoRefresh())
	tk, err := i.validate(tk)
	nval := tk.val
	if err != nil || tk.created != created ||
		!bytes.Equal(val, tk.Value()) || !bytes.Equal(val, nval) {
		t.Log("here 0")
		t.Fail()
	}

	time.Sleep(i.renewThreshold)
	tk, err = i.validate(tk)
	if err != nil || tk.created <= created ||
		bytes.Equal(val, tk.Value()) {
		t.Fail()
	}

	time.Sleep(i.tokenValidity)
	_, err = i.validate(tk)
	if err == nil || err != invalidToken {
		t.Log("here 2")
		t.Fail()
	}
}

func TestAuthToken(t *testing.T) {
	i := defaultInstance()
	_, err := i.AuthToken(nil)
	if err == nil {
		t.Fail()
	}

	tt := &testToken{}
	_, err = i.AuthToken(tt)
	if err == nil {
		t.Fail()
	}

	tk := &token{created: time.Now().Unix()}
	val, err := i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	tt = &testToken{tk.Value()}
	tback, err := i.AuthToken(tt)
	if err != nil || tback == nil || !bytes.Equal(tk.Value(), tback.Value()) {
		t.Fail()
	}

	random := makeRandom(len(tk.Value()))
	tt = &testToken{random}
	_, err = i.AuthToken(tt)
	if err == nil {
		t.Fail()
	}

	tback, err = i.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tk.Value(), tback.Value()) {
		t.Fail()
	}

	_, err = i.AuthToken(&token{})
	if err == nil {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	tiv := i.iv
	i.iv = makeKey()
	_, err = i.AuthToken(&token{})
	if err == nil {
		t.Fail()
	}
	i.iv = tiv

	tk = &token{created: i.pastNoRefresh()}
	encryptf(t, i, tk)
	tback, err = i.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: i.pastRefresh()}
	encryptf(t, i, tk)
	tback, err = i.AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: i.pastInvalid()}
	_, err = i.AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthTokenTime(t *testing.T) {
	if !testLong {
		t.Skip()
	}

	i := defaultInstance()
	tk := &token{created: time.Now().Unix()}
	encryptf(t, i, tk)
	time.Sleep(i.durNoRefresh())
	tback, err := i.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(i.renewThreshold)
	tback, err = i.AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(i.tokenValidity)
	_, err = i.AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytes(t *testing.T) {
	i := defaultInstance()
	_, err := i.AuthTokenBytes(nil)
	if err == nil {
		t.Fail()
	}

	tk := &token{}
	val, err := i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	blank := make([]byte, len(tk.Value()))
	_, err = i.AuthTokenBytes(blank)
	if err == nil {
		t.Fail()
	}

	random := makeRandom(len(tk.Value()))
	_, err = i.AuthTokenBytes(random)
	if err == nil {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	val, err = i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	tback, err := i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{}
	val, err = i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	_, err = i.AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}

	tk = &token{created: i.pastNoRefresh()}
	val, err = i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: i.pastRefresh()}
	val, err = i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: i.pastInvalid()}
	val, err = i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	_, err = i.AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytesTime(t *testing.T) {
	if !testLong {
		t.Skip()
	}
	i := defaultInstance()

	tk := &token{created: time.Now().Unix()}
	encryptf(t, i, tk)
	time.Sleep(i.durNoRefresh())
	tback, err := i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	encryptf(t, i, tk)
	time.Sleep(i.renewThreshold)
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	encryptf(t, i, tk)
	time.Sleep(i.tokenValidity)
	_, err = i.AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestGetUser(t *testing.T) {
	i := defaultInstance()
	_, err := i.GetUser(nil)
	if err == nil {
		t.Fail()
	}

	tk := &token{user: "test"}
	val, err := i.encrypt(*tk)
	if err != nil {
		t.Fatal()
	}
	tk.val = val
	user, err := i.GetUser(tk)
	if err != nil || user != "test" {
		t.Fail()
	}

	tt := &testToken{val: tk.Value()}
	user, err = i.GetUser(tt)
	if err != nil || user != "test" {
		t.Fail()
	}

	tt = &testToken{val: makeRandom(len(tk.Value()))}
	_, err = i.GetUser(tt)
	if err == nil {
		t.Fail()
	}
}

func TestAuthPwd(t *testing.T) {
	i := defaultInstance()
	tk, err := i.AuthPwd("c", "c")
	if err != nil || tk == nil || tk.Value() == nil {
		t.Fail()
	}
	_, err = i.AuthPwd("c", "d")
	if err == nil {
		t.Fail()
	}
}

func TestAuthFull(t *testing.T) {
	i := defaultInstance()
	tk, err := i.AuthPwd("cred", "cred")
	if err != nil || tk == nil {
		t.Fail()
	}
	tback, err := i.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	userBack, err := i.GetUser(tk)
	if err != nil || userBack != "cred" {
		t.Fail()
	}
}

func TestAuthFullTime(t *testing.T) {
	if !testLong {
		t.Skip()
	}
	i := defaultInstance()
	tk, err := i.AuthPwd("c", "c")
	if err != nil || tk == nil {
		t.Fail()
	}
	time.Sleep(time.Second)
	tback, err := i.AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(2 * time.Second)
	tback, err = i.AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = i.AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(20 * time.Second)
	tback, err = i.AuthToken(tk)
	if err == nil {
		t.Fail()
	}
	tback, err = i.AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestEncryptionOfLongData(t *testing.T) {
	i := defaultInstance()
	data := makeRandom(1 << 18)
	encrypted, err := i.crypt(data)
	if err != nil {
		t.Fail()
	}
	decrypted, err := i.crypt(encrypted)
	if err != nil {
		t.Fail()
	}
	if !bytes.Equal(data, decrypted) {
		t.Fail()
	}
}
