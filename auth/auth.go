// Package auth implements a simple authentication scheme. Instance of its defaultl type check user credentials,
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
	"log"
	"time"
)

const (
	defaultTokenValidity = 60 * 60 * 24 * 90
	renewThresholdRate   = 0.1
	invalidTokenMessage  = "Invalid token."
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

// A type that implements Settings can be used to pass initialization values to the new instances of auth.Type.
type Settings interface {
	AesKey() []byte     // AES key used for encryption
	AesIv() []byte      // AES iv used for encryption
	TokenValidity() int // Validity duration of the generated authentication tokens in seconds.
}

// Implementations of Token can be used to hold the encrypted authentication token.
type Token interface {
	Value() []byte
}

type token struct {
	user    string // username used during checking the credentials
	created int64  // creation time
	val     []byte // encrypted value
}

func (t *token) Value() []byte { return t.val }

// Structure enclosing authentication functions.
type Type struct {
	// aes key and iv: valid during the time the application is running
	// update now possible only through restarting the app
	// (if want to update during run time, will need to take care about the previously issued keys, too)
	key []byte
	iv  []byte

	tokenValidity  time.Duration
	renewThreshold time.Duration

	checker PasswordChecker
}

func (a *Type) decryptToken(v []byte) (token, error) {
	t := token{val: v}
	b, err := a.crypt(v)
	if err != nil {
		return t, err
	}
	userPos := len(a.iv) + 8
	if len(b) < userPos || !bytes.Equal(b[:len(a.iv)], a.iv) {
		return t, errors.New(invalidTokenMessage)
	}
	t.created, _ = binary.Varint(b[len(a.iv):userPos])
	t.user = string(b[userPos:])
	return t, nil
}

func (a *Type) encrypt(t token) ([]byte, error) {
	cap := len(a.iv) + 8 + len(t.user)
	b := make([]byte, cap, cap)
	copy(b, a.iv)
	binary.PutVarint(b[len(a.iv):], t.created)
	copy(b[cap-len(t.user):], t.user)
	return a.crypt(b)
}

// encryption/decryption with AES CTR
func (a *Type) crypt(in []byte) ([]byte, error) {
	b, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	s := cipher.NewCTR(b, a.iv)
	out := make([]byte, len(in))
	s.XORKeyStream(out, in)
	return out, nil
}

func (a *Type) validate(t token) (token, error) {
	d := time.Now().Sub(time.Unix(t.created, 0))
	if d > a.tokenValidity {
		return t, errors.New(invalidTokenMessage)
	}
	if d < a.renewThreshold {
		return t, nil
	}
	t.created = time.Now().Unix()
	val, err := a.encrypt(t)
	if err != nil {
		return t, err
	}
	t.val = val
	return t, nil
}

// Initializes an authentication instance by setting the key and iv for AES, and setting the token expiration
// interval. It expects an implementation of PasswordChecker, which will be used to check user credentials when
// AuthPwd is called. The names of files containing the AES key and iv must be set. (The token expiration's
// default is 90 days.)
func New(c PasswordChecker, s Settings) (*Type, error) {
	if c == nil {
		return nil, errors.New("PasswordChecker must be defined.")
	}
	var (
		i             Type
		tokenValidity int
	)
	i.checker = c
	if s != nil {
		i.key = s.AesKey()
		i.iv = s.AesIv()
		tokenValidity = s.TokenValidity()
	}
	if len(i.key) == 0 || len(i.iv) == 0 {
		log.Println("AES has not been configured.")
	}
	if len(i.key) == 0 {
		i.key = make([]byte, aes.BlockSize)
	}
	if len(i.iv) == 0 {
		i.iv = make([]byte, aes.BlockSize)
	}
	if tokenValidity == 0 {
		tokenValidity = defaultTokenValidity
	}
	i.tokenValidity = time.Duration(tokenValidity) * time.Second
	i.renewThreshold = time.Duration(float64(i.tokenValidity) * renewThresholdRate)
	return &i, nil
}

// Checks if the provided username and password are correct. If yes, an authentication token is returned,
// otherwise an error.
func (a *Type) AuthPwd(user, pwd string) (Token, error) {
	var t token
	err := a.checker.Check(user, pwd)
	if err != nil {
		return nil, err
	}
	t.user = user
	t.created = time.Now().Unix()
	t.val, err = a.encrypt(t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Validates previously provided tokens if they didn't expire. If not, it returns the same token or a new one
// with extended expiration, so that a session doesn't expire due to inactivity shorter than 90% of the token
// validity interval.
func (a *Type) AuthToken(t Token) (Token, error) {
	if t == nil {
		return nil, errors.New(invalidTokenMessage)
	}
	var (
		tc *token
		ok bool
	)
	if tc, ok = t.(*token); !ok {
		return a.AuthTokenBytes(t.Value())
	}
	tn, err := a.validate(*tc)
	return &tn, err
}

// The same as AuthToken but expects the byte representation of a token.
func (a *Type) AuthTokenBytes(v []byte) (Token, error) {
	t, err := a.decryptToken(v)
	if err == nil {
		t, err = a.validate(t)
	}
	return &t, err
}

// If the token was generated by package auth, GetUser returns the username whom the token belongs to. (Even if
// the token has expired.)
func (a *Type) GetUser(t Token) (string, error) {
	if t == nil {
		return "", errors.New(invalidTokenMessage)
	}
	if tc, ok := t.(*token); ok {
		return tc.user, nil
	}
	tc, err := a.decryptToken(t.Value())
	return tc.user, err
}
