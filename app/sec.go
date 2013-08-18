package app

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"github.com/krockot/gopam/pam"
)

// aes key and iv, valid during the time the application is running
// update now possible only through restarting the app
// (if want to update during running, will need to take care about the previously issued keys, too)
var key, iv []byte

// serializable token to serve as an authentication key
type Token struct {
	user  string
	valid int64
}

func (t Token) Encrypt() string {
	return ""
}

func DecryptToken(s string) (Token, error) {
	return Token{}, nil
}

// checks a username and a password if they are valid on the current system
func checkCred(user, pwd string) error {
	fail := func() error { return errors.New("Authentication failed.") }

	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF:
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

// encryption/decryption with AES CTR
func dencrypt(in []byte) ([]byte, error) {
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
	enc, err := dencrypt([]byte(test))
	if err != nil {
		return err
	}
	dec, err := dencrypt(enc)
	if err != nil {
		return err
	}
	if string(dec) != test {
		errors.New("Failed to initialize encryption.")
	}
	return nil
}

// security related initialization:
// - store aes key and iv
func InitSec(aesKey, aesIv []byte) error {
	key = aesKey
	iv = aesIv
	return verifyEncryption()
}
