package app

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/krockot/gopam/pam"
	"time"
)

var (
	// aes key and iv: valid during the time the application is running
	// update now possible only through restarting the app
	// (if want to update during running, will need to take care about the previously issued keys, too)
	key, iv []byte

	// validity time of the security token in seconds
	tokenValidity int
)

type Token interface {
	Value() []byte
}

// serializable token to serve as an authentication key
type token struct {
	user    string
	created int64
	val     []byte
}

func decryptToken(v []byte) (token, error) {
	t := token{val: v}
	b, err := crypt(v)
	if err != nil {
		return t, err
	}
	if len(b) < 8 {
		return t, nil
	}
	t.created, _ = binary.Varint(b[:8])
	t.user = string(b[8:])
	return t, nil
}

func (t token) encrypt() ([]byte, error) {
	b := make([]byte, 8)
	binary.PutVarint(b, t.created)
	b = append(b, t.user...)
	return crypt(b)
}

func (t token) Value() []byte {
	if t.val != nil {
		return t.val
	}
	val, err := t.encrypt()
	if err != nil {
		panic(err)
	}
	t.val = val
	return val
}

// encryption/decryption with AES CTR
func crypt(in []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("Invalid encryption key.")
	}
	s := cipher.NewCTR(b, iv)
	out := make([]byte, len(in))
	s.XORKeyStream(out, in)
	return out, nil
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

// checks a username and a password if they are valid on the current system
func checkCred(user, pwd string) error {
	fail := func() error { return errors.New("Authentication failed.") }

	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF, pam.PROMPT_ECHO_ON:
			return pwd, true
		default:
			return "", false
		}
	}))
	if s != pam.SUCCESS {
		return fail()
	}
	defer t.End(s)

	s = t.Authenticate(0)
	if s != pam.SUCCESS {
		return fail()
	}
	return nil
}

func AuthPwd(user, pwd string) (Token, error) {
	err := checkCred(user, pwd)
	if err != nil {
		return nil, err
	}

	return token{
		user: user,
		// user rather now, and compare later if it is still valid
		created: time.Now().Add(time.Duration(tokenValidity) * time.Second).Unix()}, nil
}

func AuthToken(t Token) (Token, error) {
	var (
		tk  token
		ok  bool
		err error
	)
	if tk, ok = t.(token); !ok {
		tk, err = decryptToken(t.Value())
		if err != nil {
			return tk, err
		}
	}
	return tk, nil
}

func GetUser(t fmt.Stringer) (Token, error) {
	return token{}, nil
}

// security related initialization:
// - store aes key and iv
// - store token validity time
func InitSec(aesKey, aesIv []byte, tokenValiditySecs int) error {
	key = aesKey
	iv = aesIv
	tokenValidity = tokenValiditySecs
	return verifyEncryption()
}
