package app

import (
    "crypto/aes"
    "crypto/cipher"
    "errors"
    "github.com/krockot/gopam/pam"
)

var key, iv []byte

type Token struct {
    user string
    valid int64
}

func (t Token) Encrypt() string {
    return ""
}

func DecryptToken(s string) (Token, error) {
    return Token{}, nil
}

func checkCred(user, pwd string) error {
    fail := func() error { return errors.New("Authentication failed.") }

    t, s:= pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
        switch style {
        case pam.PROMPT_ECHO_OFF:
            return pwd, true
        default:
            return "", false
        }
    }))
    if s != pam.SUCCESS { return fail() }
    defer t.End(s)

    s = t.Authenticate(0)
    if s != pam.SUCCESS { return fail() }
    return nil
}

func encrypt(d []byte) ([]byte, error) {
    b, err := aes.NewCipher(key)
    if err != nil { return nil, errors.New("Invalid encryption key.") }
    s := cipher.NewCTR(b, iv)
    c := make([]byte, len(d))
    s.XORKeyStream(c, d)
    return c, nil
}

func decrypt(c []byte) ([]byte, error) {
    b, err := aes.NewCipher(key)
    if err != nil { return nil, errors.New("Invalid encryption key.") }
    d := make([]byte, len(c))
    s := cipher.NewCTR(b, iv)
    s.XORKeyStream(d, c)
    return d, nil
}

func verifyEncryption() error {
    test := "Test encryption message."
    enc, err := encrypt([]byte(test))
    if err != nil { return err }
    dec, err := decrypt(enc)
    if err != nil { return err }
    if string(dec) != test {
        errors.New("Failed to initialize encryption.")
    }
    return nil
}

func InitSec(aesKey, aesIv []byte) error {
    key = aesKey
    iv = aesIv
    return verifyEncryption()
}
