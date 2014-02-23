package auth

import (
	"bytes"
	. "code.google.com/p/tasked/testing"
	"crypto/aes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"testing"
	"time"
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

func defaultInstance() *It {
	i, err := New(PasswordCheckerFunc(checkFunc), &testConfig{makeKey(), makeKey(), 18})
	if err != nil {
		panic(err)
	}
	return i
}

func (a *It) durNoRefresh() time.Duration {
	return time.Duration(float64(a.tokenValidity) * renewThresholdRate / 3)
}

func (a *It) pastNoRefresh() int64 {
	return time.Now().Add(-a.durNoRefresh()).Unix()
}

func (a *It) pastRefresh() int64 {
	return time.Now().Add(-a.renewThreshold).Unix()
}

func (a *It) pastInvalid() int64 {
	return time.Now().Add(-a.tokenValidity).Unix()
}

func encryptf(t *testing.T, i *It, c int64, u string) []byte {
	val, err := i.encryptToken(c, u)
	ErrFatal(t, err)
	return val
}

func TestTokenEncryptDecrypt(t *testing.T) {
	i := defaultInstance()

	c := int64(42)
	u := "some"
	v, err := i.encryptToken(c, u)
	if err != nil || v == nil || len(v) == 0 {
		t.Fail()
	}

	cv, uv, err := i.decryptToken(v)
	if err != nil || cv != c || u != uv {
		t.Fail()
	}

	blank := make([]byte, len(v))
	c, u, err = i.decryptToken(blank)
	if err == nil || err != invalidToken {
		t.Fail()
	}

	random := makeRandom(len(v))
	c, u, err = i.decryptToken(random)
	if err == nil || err != invalidToken {
		t.Fail()
	}

	c = 42
	u = "some"
	v, err = i.encryptToken(c, u)
	ErrFatal(t, err)
	tiv := i.iv
	i.iv = makeKey()
	_, _, err = i.decryptToken(v)
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

func TestNew(t *testing.T) {
	_, err := New(nil, nil)
	if err == nil {
		t.Fail()
	}

	testError := errors.New("test error")
	pcf := PasswordCheckerFunc(func(u, p string) error {
		return testError
	})
	a, err := New(pcf, nil)
	if err != nil || a == nil ||
		a.checker.Check("", "") != testError ||
		len(a.key) != aes.BlockSize || len(a.iv) != aes.BlockSize ||
		a.tokenValidity != 0 || a.renewThreshold != 0 {
		t.Fail()
	}

	iv := make([]byte, 1.5 * aes.BlockSize)
	_, err = rand.Read(iv)
	ErrFatal(t, err)
	a, err = New(pcf, &testConfig{
		aesKey: []byte("012"),
		aesIv: iv,
		tokenValidity: 30})
	if err != nil || a == nil ||
		len(a.key) != aes.BlockSize ||
		len(a.iv) != aes.BlockSize ||
		a.tokenValidity != 30 * time.Second ||
		a.renewThreshold != time.Duration(float64(a.tokenValidity) * renewThresholdRate) {
		t.Fail()
	}
}

func TestAuthPwd(t *testing.T) {
	i := defaultInstance()
	tk, err := i.AuthPwd("c", "c")
	if err != nil || len(tk) == 0 {
		t.Fail()
	}
	_, err = i.AuthPwd("c", "d")
	if err == nil {
		t.Fail()
	}
}

func TestAuthToken(t *testing.T) {
	i := defaultInstance()
	_, _, err := i.AuthToken(nil)
	if err == nil {
		t.Fail()
	}
	tk, err := i.encryptToken(i.pastInvalid(), "c")
	ErrFatal(t, err)
	_, _, err = i.AuthToken(tk)
	if err != invalidToken {
		t.Fail()
	}
	tk, err = i.encryptToken(i.pastNoRefresh(), "c")
	ErrFatal(t, err)
	tback, u, err := i.AuthToken(tk)
	if !bytes.Equal(tback, tk) || u != "c" || err != nil {
		t.Fail()
	}
	tk, err = i.encryptToken(i.pastRefresh(), "c")
	ErrFatal(t, err)
	tback, u, err = i.AuthToken(tk)
	if bytes.Equal(tback, tk) || u != "c" || err != nil {
		t.Fail()
	}
}

func TestAuthFull(t *testing.T) {
	i := defaultInstance()
	tk, err := i.AuthPwd("cred", "cred")
	if err != nil || len(tk) == 0 {
		t.Fail()
	}
	tback, u, err := i.AuthToken(tk)
	if err != nil || !bytes.Equal(tback, tk) || u != "cred" {
		t.Fail()
	}
}

func TestEncryptionOfLargeData(t *testing.T) {
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
