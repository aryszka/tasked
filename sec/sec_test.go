package sec

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
)

var (
	testAuth   = false
	testInTime = false
)

func init() {
	tap := flag.Bool("auth", false, "")
	ttp := flag.Bool("testInTime", false, "")
	flag.Parse()
	testAuth = *tap
	testInTime = *ttp
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

func resetConfig() {
	Init(&testConfig{makeKey(), makeKey(), 18})
}

func durNoRefresh() time.Duration {
	return time.Duration(float64(tokenValidity) * renewThresholdRate / 3)
}

func pastNoRefresh() int64 {
	return time.Now().Add(-durNoRefresh()).Unix()
}

func pastRefresh() int64 {
	return time.Now().Add(-renewThreshold).Unix()
}

func pastInvalid() int64 {
	return time.Now().Add(-tokenValidity).Unix()
}

func TestTokenEncryptDecrypt(t *testing.T) {
	resetConfig()
	tk := token{user: "some", created: 42}
	v, err := encrypt(tk)
	if err != nil || v == nil || len(v) == 0 {
		t.Fail()
	}

	verify, err := decryptToken(v)
	if err != nil || !bytes.Equal(verify.val, v) ||
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
	resetConfig()
	tk := token{user: "some", created: 42}
	val := tk.Value()
	if val == nil || !bytes.Equal(tk.val, val) {
		t.Fail()
	}

	tk = token{user: "some", created: 42}
	verify, err := encrypt(tk)
	if err != nil || !bytes.Equal(val, verify) {
		t.Fail()
	}
}

func TestEncryption(t *testing.T) {
	resetConfig()
	verifyEncryption := func() error {
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
	resetConfig()
	if !testAuth {
		t.Skip()
	}
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")

	if nil != checkCred(user, pwd) {
		t.Fail()
	}
	if nil == checkCred(user+"x", pwd) {
		t.Fail()
	}
	if nil == checkCred(user, pwd+"x") {
		t.Fail()
	}
	if nil == checkCred(user, "") {
		t.Fail()
	}
	if nil == checkCred("", "") {
		t.Fail()
	}
}

func TestValidate(t *testing.T) {
	resetConfig()
	tk := token{}
	tback, err := validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
	tk = token{user: "test", created: time.Now().Unix()}
	tback, err = validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created {
		t.Fail()
	}
	tk = token{user: "test", created: time.Now().Unix()}
	tk.Value()
	tback, err = validate(tk)
	if err != nil || tback.user != tk.user || tback.created != tk.created ||
		!bytes.Equal(tback.val, tk.val) {
		t.Fail()
	}

	created := pastNoRefresh()
	tk = token{created: created}
	val := tk.Value()
	tk, err = validate(tk)
	nval := tk.val
	if err != nil || tk.created != created ||
		!bytes.Equal(val, tk.Value()) || !bytes.Equal(val, nval) {
		t.Fail()
	}

	created = pastRefresh()
	tk = token{created: created}
	val = tk.Value()
	tk, err = validate(tk)
	nval = tk.val
	if err != nil || tk.created <= created ||
		nval != nil || bytes.Equal(val, tk.Value()) {
		t.Fail()
	}

	tk = token{created: pastInvalid()}
	_, err = validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
}

func TestValidateTime(t *testing.T) {
	resetConfig()
	if !testInTime {
		t.Skip()
	}
	created := time.Now().Unix()
	tk := token{created: created}
	val := tk.Value()

	time.Sleep(durNoRefresh())
	tk, err := validate(tk)
	nval := tk.val
	if err != nil || tk.created != created ||
		!bytes.Equal(val, tk.Value()) || !bytes.Equal(val, nval) {
		t.Fail()
	}

	time.Sleep(renewThreshold)
	tk, err = validate(tk)
	nval = tk.val
	if err != nil || tk.created <= created ||
		nval != nil || bytes.Equal(val, tk.Value()) {
		t.Fail()
	}

	time.Sleep(tokenValidity)
	_, err = validate(tk)
	if err == nil || err.Error() != invalidTokenMessage {
		t.Fail()
	}
}

func TestAuthPwd(t *testing.T) {
	resetConfig()
	if !testAuth {
		t.Skip()
	}
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")

	tk, err := AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	_, err = AuthPwd(user+"x", pwd)
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd(user, pwd+"x")
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd(user, "")
	if err == nil {
		t.Fail()
	}
	_, err = AuthPwd("", "")
	if err == nil {
		t.Fail()
	}
}

func TestAuthToken(t *testing.T) {
	resetConfig()
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
	if err != nil || tback == nil || !bytes.Equal(tk.Value(), tback.Value()) {
		t.Fail()
	}

	random := makeRandom(len(tk.Value()))
	tt = &testToken{random}
	_, err = AuthToken(tt)
	if err == nil {
		t.Fail()
	}

	tback, err = AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tk.Value(), tback.Value()) {
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

	tk = &token{created: pastNoRefresh()}
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: pastRefresh()}
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: pastInvalid()}
	_, err = AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthTokenTime(t *testing.T) {
	resetConfig()
	if !testInTime {
		t.Skip()
	}

	tk := &token{created: time.Now().Unix()}
	time.Sleep(durNoRefresh())
	tback, err := AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(renewThreshold)
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	time.Sleep(tokenValidity)
	_, err = AuthToken(tk)
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytes(t *testing.T) {
	resetConfig()
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
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{}
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}

	tk = &token{created: pastNoRefresh()}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: pastRefresh()}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: pastInvalid()}
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestAuthBytesTime(t *testing.T) {
	resetConfig()
	if !testInTime {
		t.Skip()
	}

	tk := &token{created: time.Now().Unix()}
	time.Sleep(durNoRefresh())
	tback, err := AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	time.Sleep(renewThreshold)
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}

	tk = &token{created: time.Now().Unix()}
	time.Sleep(tokenValidity)
	_, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}

func TestGetUser(t *testing.T) {
	resetConfig()
	_, err := GetUser(nil)
	if err == nil {
		t.Fail()
	}

	tk := &token{user: "test"}
	user, err := GetUser(tk)
	if err != nil || user != "test" {
		t.Fail()
	}

	tt := &testToken{val: tk.Value()}
	user, err = GetUser(tt)
	if err != nil || user != "test" {
		t.Fail()
	}

	tt = &testToken{val: makeRandom(len(tk.Value()))}
	_, err = GetUser(tt)
	if err == nil {
		t.Fail()
	}
}

func TestAuthFull(t *testing.T) {
	resetConfig()
	if !testAuth {
		t.Skip()
	}
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")
	tk, err := AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	tback, err := AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	userBack, err := GetUser(tk)
	if err != nil || userBack != user {
		t.Fail()
	}
}

func TestAuthFullTime(t *testing.T) {
	resetConfig()
	if !testInTime {
		t.Skip()
	}
	if !testAuth {
		t.Skip()
	}
	user := envdef("testusr", "test")
	pwd := envdef("testpwd", "testpwd")
	tk, err := AuthPwd(user, pwd)
	if err != nil || tk == nil {
		t.Fail()
	}
	time.Sleep(time.Second)
	tback, err := AuthToken(tk)
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || !bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(2 * time.Second)
	tback, err = AuthToken(tk)
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	tback, err = AuthTokenBytes(tk.Value())
	if err != nil || tback == nil || bytes.Equal(tback.Value(), tk.Value()) {
		t.Fail()
	}
	time.Sleep(20 * time.Second)
	tback, err = AuthToken(tk)
	if err == nil {
		t.Fail()
	}
	tback, err = AuthTokenBytes(tk.Value())
	if err == nil {
		t.Fail()
	}
}
