// Package auth implements a simple authentication scheme. Instances of its default type check user credentials,
// and on success, they generate encrypted, time limited tokens, that can be used for subsequent checks.
//
// For checking credentials, the package uses external credential checking implementations.
package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"time"
)

const renewThresholdRate = 0.1

var (
	invalidToken      = errors.New("Invalid token.")
	noPasswordChecker = errors.New("PasswordChecker must be defined.")
)

// Package auth uses implementations of PasswordChecker to verify the validity of a username and password pair.
type PasswordChecker interface {
	// Returns nil if username and password are valid credentials and no errors occur during verification.
	Check(username, password string) error
}

// Wrapper for standalone function implementations of PasswordChecker.
type PasswordCheckerFunc func(string, string) error

// Calls f.
func (f PasswordCheckerFunc) Check(username, password string) error {
	return f(username, password)
}

// A type that implements Options can be used to pass initialization values to the new instances of auth.It.
type Options interface {
	AesKey() []byte     // AES key used for encryption
	AesIv() []byte      // AES iv used for encryption
	TokenValidity() int // Validity duration of the generated authentication tokens in seconds.
}

// Structure enclosing authentication functions.
type It struct {
	// aes key and iv: valid during the time the application is running
	// update now possible only through restarting the app
	// (if want to update during run time, will need to take care about the previously issued keys, too)
	key []byte
	iv  []byte

	tokenValidity  time.Duration
	renewThreshold time.Duration

	checker PasswordChecker
}

// crypting with AES CTR
func (a *It) crypt(in []byte) ([]byte, error) {
	b, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	s := cipher.NewCTR(b, a.iv)
	out := make([]byte, len(in))
	s.XORKeyStream(out, in)
	return out, nil
}

func (a *It) decryptToken(v []byte) (int64, string, error) {
	b, err := a.crypt(v)
	if err != nil {
		return 0, "", err
	}
	userPos := len(a.iv) + 8
	if len(b) < userPos || !bytes.Equal(b[:len(a.iv)], a.iv) {
		return 0, "", invalidToken
	}
	c, _ := binary.Varint(b[len(a.iv):userPos])
	u := string(b[userPos:])
	return c, u, nil
}

func (a *It) encryptToken(c int64, u string) ([]byte, error) {
	cap := len(a.iv) + 8 + len(u)
	b := make([]byte, cap, cap)
	copy(b, a.iv)
	binary.PutVarint(b[len(a.iv):], c)
	copy(b[cap-len(u):], u)
	return a.crypt(b)
}

// Initializes an authentication instance by setting the key and iv for AES, and setting the token expiration
// interval. It expects an implementation of PasswordChecker, which will be used to check user credentials when
// AuthPwd is called.
func New(c PasswordChecker, o Options) (*It, error) {
	if c == nil {
		return nil, noPasswordChecker
	}
	a := new(It)
	a.checker = c
	a.key = make([]byte, aes.BlockSize)
	a.iv = make([]byte, aes.BlockSize)
	if o == nil {
		return a, nil
	}
	copy(a.key, o.AesKey())
	copy(a.iv, o.AesIv())
	a.tokenValidity = time.Duration(o.TokenValidity()) * time.Second
	a.renewThreshold = time.Duration(float64(a.tokenValidity) * renewThresholdRate)
	return a, nil
}

// Checks if the provided username and password are correct. If yes, an authentication token is returned,
// otherwise an error.
func (a *It) AuthPwd(user, pwd string) ([]byte, error) {
	err := a.checker.Check(user, pwd)
	if err != nil {
		return nil, err
	}
	c := time.Now().Unix()
	return a.encryptToken(c, user)
}

// Validates previously provided tokens if they didn't expire. If not, it returns the same token or a new one
// with extended expiration, so that a session doesn't expire due to inactivity shorter than 90% of the token
// validity interval.
func (a *It) AuthToken(v []byte) ([]byte, string, error) {
	c, u, err := a.decryptToken(v)
	if err != nil {
		return nil, "", err
	}
	d := time.Now().Sub(time.Unix(c, 0))
	if d > a.tokenValidity {
		return nil, "", invalidToken
	}
	if d < a.renewThreshold {
		return v, u, nil
	}
	c = time.Now().Unix()
	v, err = a.encryptToken(c, u)
	return v, u, err
}
