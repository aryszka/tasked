package app

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"github.com/krockot/gopam/pam"
	"time"
)

const (
	renewThresholdRate = 0.1
	invalidTokenMessage = "Invalid token."
	invalidEncryptionKeyMessage = "Invalid encryption key."
)

var (
	// aes key and iv: valid during the time the application is running
	// update now possible only through restarting the app
	// (if want to update during running, will need to take care about the previously issued keys, too)
	key, iv []byte

	// validity time of the security token in seconds
	tokenValidity time.Duration
	renewThreshold time.Duration
)

type SecConfig interface {
	AesKey() []byte
	AesIv() []byte
	TokenValidity() int
}

type Token interface {
	Value() []byte
}

// serializable token to serve as an authentication key
type token struct {
	user    string
	created int64
	val     []byte
}

func (t *token) Value() []byte {
	if t.val != nil {
		return t.val
	}
	val, err := encrypt(*t)
	if err != nil {
		panic(err)
	}
	t.val = val
	return val
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

func encrypt(t token) ([]byte, error) {
	b := make([]byte, 8)
	binary.PutVarint(b, t.created)
	b = append(b, t.user...)
	return crypt(b)
}

// same as AuthToken
func validate(t token) (token, error) {
	d := time.Now().Sub(time.Unix(t.created, 0))
	if d > tokenValidity {
		return t, errors.New(invalidTokenMessage)
	}
	if d < renewThreshold {
		return t, nil
	}
	t.val = nil
	t.created = time.Now().Unix()
	return t, nil
}

// encryption/decryption with AES CTR
func crypt(in []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New(invalidEncryptionKeyMessage)
	}
	s := cipher.NewCTR(b, iv)
	out := make([]byte, len(in))
	s.XORKeyStream(out, in)
	return out, nil
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

// checks username and password, and if they are
// not valid credentials, returns error, or a
// valid security token
func AuthPwd(user, pwd string) (Token, error) {
	var t token
	err := checkCred(user, pwd)
	if err != nil {
		return nil, err
	}
	t.user = user
	t.created = time.Now().Unix()
	return &t, nil
}

// validates a token and returns error if not valid, or
// returns the token, or if the age of the token is above
// the threshold, it creates and returns a fresh token
func AuthToken(t Token) (Token, error) {
	if t == nil {
		return nil, errors.New(invalidTokenMessage)
	}
	var (
		tc *token
		ok bool
	)
	if tc, ok = t.(*token); !ok {
		return AuthTokenBytes(t.Value())
	}
	tn, err := validate(*tc)
	return &tn, err
}

// same as AuthToken but with the byte representation of a token
func AuthTokenBytes(v []byte) (Token, error) {
	t, err := decryptToken(v)
	if err == nil {
		t, err = validate(t)
	}
	return &t, err
}

func GetUser(t Token) (string, error) {
	return "", nil
}

// security related initialization:
// - store aes key and iv
// - store token validity time
func InitSec(c SecConfig) {
	key = c.AesKey()
	iv = c.AesIv()
	tokenValidity = time.Duration(c.TokenValidity()) * time.Second
	renewThreshold = time.Duration(float64(tokenValidity) * renewThresholdRate)
}
