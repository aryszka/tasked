package keyval

import (
	"bytes"
	. "code.google.com/p/tasked/testing"
	"os"
	"path"
	"testing"
)

func testEntries(t *testing.T, es entries, expected ...*Entry) {
	if len(es) != len(expected) {
		t.Fail()
		return
	}
	for i, e := range es {
		if e.Key != expected[i].Key ||
			e.Val != expected[i].Val {
			t.Fail()
			return
		}
	}
}

func testParse(t *testing.T, fs string, expected ...*Entry) {
	es, err := parseFile(bytes.NewBufferString(fs))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es, expected...)
}

func TestParseFile(t *testing.T) {
	var fs string

	// empties
	testParse(t, "")
	testParse(t, "\n")
	testParse(t, "\n\n")
	testParse(t, "\n \n")
	testParse(t, " ")
	testParse(t, "  ")
	fs = "some = 1\n" +
		"  some other=2	 "
	testParse(t, fs,
		&Entry{Key: "some", Val: "1"},
		&Entry{Key: "some other", Val: "2"})

	// escaped empties
	testParse(t, "\\")
	testParse(t, "'")
	testParse(t, "\"")
	testParse(t, "\\\n", &Entry{Val: "\n"})
	testParse(t, "\\\n\n", &Entry{Val: "\n"})
	testParse(t, "\\\n\\\n", &Entry{Val: "\n\n"})
	testParse(t, "\n\\\n", &Entry{Val: "\n"})
	testParse(t, "\\\n \n", &Entry{Val: "\n"})
	testParse(t, "\\\n \\\n", &Entry{Val: "\n \n"})
	testParse(t, "\n\\ \n", &Entry{Val: " "})
	testParse(t, "\n \\\n", &Entry{Val: "\n"})
	testParse(t, "\\ ", &Entry{Val: " "})
	testParse(t, "\\  ", &Entry{Val: " "})
	testParse(t, " \\ ", &Entry{Val: " "})
	testParse(t, "\\\\", &Entry{Val: "\\"})
	testParse(t, "'\n'", &Entry{Val: "\n"})
	testParse(t, "'\n'\n", &Entry{Val: "\n"})
	testParse(t, "'\n''\n'", &Entry{Val: "\n\n"})
	testParse(t, "\n'\n'", &Entry{Val: "\n"})
	testParse(t, "'\n' \n", &Entry{Val: "\n"})
	testParse(t, "'\n' '\n'", &Entry{Val: "\n \n"})
	testParse(t, "\n' '\n", &Entry{Val: " "})
	testParse(t, "\n '\n'", &Entry{Val: "\n"})
	testParse(t, "' '", &Entry{Val: " "})
	testParse(t, "' ' ", &Entry{Val: " "})
	testParse(t, " ' '", &Entry{Val: " "})
	testParse(t, "'\\\\'", &Entry{Val: "\\"})
	testParse(t, "'\\'", &Entry{Val: "'"})
	testParse(t, "\"\n\"", &Entry{Val: "\n"})
	testParse(t, "\"\n\"\n", &Entry{Val: "\n"})
	testParse(t, "\"\n\"\"\n\"", &Entry{Val: "\n\n"})
	testParse(t, "\n\"\n\"", &Entry{Val: "\n"})
	testParse(t, "\"\n\" \n", &Entry{Val: "\n"})
	testParse(t, "\"\n\" \"\n\"", &Entry{Val: "\n \n"})
	testParse(t, "\n\" \"\n", &Entry{Val: " "})
	testParse(t, "\n \"\n\"", &Entry{Val: "\n"})
	testParse(t, "\" \"", &Entry{Val: " "})
	testParse(t, "\" \" ", &Entry{Val: " "})
	testParse(t, " \" \"", &Entry{Val: " "})
	testParse(t, "\"\\\\\"", &Entry{Val: "\\"})
	testParse(t, "\"\\\"", &Entry{Val: "\""})

	// unicode
	testParse(t, "世 界= 界 世", &Entry{Key: "世 界", Val: "界 世"})

	// comments
	testParse(t, "#")
	testParse(t, "# a comment")
	testParse(t, " #")
	testParse(t, " # a comment")
	testParse(t, "\n# a comment")
	testParse(t, "# a comment")
	fs = "# a title comment\n" +
		"an entry = a value # and entry comment"
	testParse(t, fs, &Entry{Key: "an entry", Val: "a value"})
	fs = "# a title comment\n" +
		"an entry = a value # and entry comment\n" +
		"# a footer comment"
	testParse(t, fs, &Entry{Key: "an entry", Val: "a value"})

	// entry
	testParse(t, "value only", &Entry{Val: "value only"})
	testParse(t, "= value only", &Entry{Val: "value only"})
	testParse(t, "key = value", &Entry{Key: "key", Val: "value"})
	testParse(t, "entry format = key = value", &Entry{Key: "entry format", Val: "key = value"})

	// escaping
	testParse(t, "\\a", &Entry{Val: "a"})
	testParse(t, "\\\\", &Entry{Val: "\\"})
	testParse(t, "\\\\\\", &Entry{Val: "\\"})
	testParse(t, "\\\\=\\\\", &Entry{Key: "\\", Val: "\\"})
	testParse(t, "\\===", &Entry{Key: "=", Val: "="})
	testParse(t, "\\#", &Entry{Val: "#"})
	testParse(t, "\\#=\\#", &Entry{Key: "#", Val: "#"})
	testParse(t, "\\\"=\\\"", &Entry{Key: "\"", Val: "\""})
	testParse(t, "'='='='", &Entry{Key: "=", Val: "="})
	testParse(t, "\"=\"=\"=\"", &Entry{Key: "=", Val: "="})
	testParse(t, "'\n'='\n'", &Entry{Key: "\n", Val: "\n"})
	testParse(t, "\"\n\"=\"\n\"", &Entry{Key: "\n", Val: "\n"})
	testParse(t, "'entry as a \\'key = value\\'' = 'key'",
		&Entry{Key: "entry as a 'key = value'", Val: "key"})

	// trimming
	testParse(t, " a b = b a ", &Entry{Key: "a b", Val: "b a"})
	testParse(t, "' a b '=' b a '", &Entry{Key: " a b ", Val: " b a "})
	testParse(t, "'\na\nb\n'='\nb\na\n'", &Entry{Key: "\na\nb\n", Val: "\nb\na\n"})

	// full, real life
	fs = `# include
include-config = filename0
include-config = filename1
/var/data

# general
root               = /var/data # also as default parameter, when not set then stdio
cachedir           = filename
allow-cookies      = false
max-request-body   = 1<<30
max-request-header = 1<<20

# http
address            = :9090 # when filename, then unix

tls-key            = \
"-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBANfK7I6VGd4yxNNK
rg1+GdveTB4aAiqC916Yl5vFTlpCg6LIhAYmDvXPM9XJZc/h8N4jh/JNC39wgcEG
/RV2wl9T63+NR6TBLVx6nJbKjCvEuzwpB3BIun4827cU6PCksBc4hke9pTgD9v0y
DOtECKl+HuxRuKJLGoRCQ9rcoJL1AgMBAAECgYBz0R+hbvjRPuJQnNZJu5JZZTfp
OABNnLjzdmZ4Xi8tVmGcLo5dVnPVDf4+EbepGRTTxLIkI6G2JkYduYh/ypuK3TtD
JQ2j2Wb4hSFXc3jGGGmx3SFYrmajM6nW7vnBw7Ld6PaJqo5lZtYcFzpOSrzP5G0p
TPEJ1091aOrhoNexgQJBAP7M2XMw4TJqddT03/y4y46ESq4bNYOIyMd3X9yYM77Q
KH5v1x+95znBkb8hJoPgO2+un4uLr2A8L8umxByTHJECQQDYzw2BxF6D9GSDjQr6
BEX1UxfM96DiSE2N3i+1YJWOdcqg9dvJRByYzvdlqEobY2DB8Cnh1HS94V3vyruw
R1IlAkEA9NTnuTzllukfEiK+O3th9S5/B+8TK7G6o5e8IB6L0jT4RA25W0HBtgie
wFXdSWikE/tqSM9PFByhHIHA/WgKUQJALTMlbrtgtQPbfK2H7026xAV5vcqWaPaH
7J64tYiYRWX7Q4leM9yWVak4XKI0KPeT8Xq/UIx5diio69gJPxvvXQJAM1lr5o49
D0qEjXcpHjsMHcrYgQLGZPCfNn3gkGZ/pxr/3N36SyaqF6/7NRe7BLHbll9lb+8f
8FF/8F+a66TGLw==
-----END PRIVATE KEY-----
"
tls-cert           = \
"-----BEGIN CERTIFICATE-----
MIIC7jCCAlegAwIBAgIJAIvCpMZ/RhydMA0GCSqGSIb3DQEBBQUAMIGPMQswCQYD
VQQGEwJERTEPMA0GA1UECAwGQmVybGluMQ8wDQYDVQQHDAZCZXJsaW4xHDAaBgNV
BAoME0JlcmxpbmVyIFJvYm90d2Vya2UxGTAXBgNVBAMMEHRhc2tlZHNlcnZlci5j
b20xJTAjBgkqhkiG9w0BCQEWFmFycGFkLnJ5c3prYUBnbWFpbC5jb20wHhcNMTMw
OTA3MTk1MzU1WhcNMTYwOTA2MTk1MzU1WjCBjzELMAkGA1UEBhMCREUxDzANBgNV
BAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMRwwGgYDVQQKDBNCZXJsaW5lciBS
b2JvdHdlcmtlMRkwFwYDVQQDDBB0YXNrZWRzZXJ2ZXIuY29tMSUwIwYJKoZIhvcN
AQkBFhZhcnBhZC5yeXN6a2FAZ21haWwuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQDXyuyOlRneMsTTSq4Nfhnb3kweGgIqgvdemJebxU5aQoOiyIQGJg71
zzPVyWXP4fDeI4fyTQt/cIHBBv0VdsJfU+t/jUekwS1cepyWyowrxLs8KQdwSLp+
PNu3FOjwpLAXOIZHvaU4A/b9MgzrRAipfh7sUbiiSxqEQkPa3KCS9QIDAQABo1Aw
TjAdBgNVHQ4EFgQUrAUcn4JJ13CSKXdKquzs03OHl0gwHwYDVR0jBBgwFoAUrAUc
n4JJ13CSKXdKquzs03OHl0gwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOB
gQB2VmcD9Hde1Bf9lgk3iWw+ZU8JbdJvhK0MoU4RhCDEl01K2omxoT4B8OVWlFD5
GWX4rnIZtcLahM1eu8h+QxdcTNGwCpIiait2pmpVcV6pjNKv8LUxAcaemq178OfK
h3I2CsHAUTwxT1ca8SGLCsFTm03AyXaU0Q061+RX1Do/Iw==
-----END CERTIFICATE-----
"

tls-key-file       = filename
tls-cert-file      = filename
max-search-results = 0

# e.g. [{"method": "AUTH", "address": "/var/sockets/auth-socket"}]
proxy              = \
'[{
	"include": "proxy.json"
}, {
	"method": "AUTH",
	"path-expression": "path-regexp",
	"address": "name.domain:8080",
	"rewrite-method": "POST",
	"rewrite-path": [{
		"expression": "match-regexp",
		"replace": "literal&"
	}],
	"response": {
		"status": 404,
		"header": {
			"Header-Key": "header-value",
		},
		"body": "data"
	}
}]'
proxy-file         = filename

# auth
authenticate       = false
public-user        = nobody # when not set and auth enabled then no public access
# aes-key            = ""
# aes-iv             = ""
aes-key-file       = filename
aes-iv-file        = filename
token-validity     = 6912000
max-user-processes = 0
process-idle-time  = 360`

	testParse(t, fs,
		&Entry{Key: "include-config", Val: "filename0"},
		&Entry{Key: "include-config", Val: "filename1"},
		&Entry{Key: "", Val: "/var/data"},
		&Entry{Key: "root", Val: "/var/data"},
		&Entry{Key: "cachedir", Val: "filename"},
		&Entry{Key: "allow-cookies", Val: "false"},
		&Entry{Key: "max-request-body", Val: "1<<30"},
		&Entry{Key: "max-request-header", Val: "1<<20"},
		&Entry{Key: "address", Val: ":9090"},
		&Entry{Key: "tls-key", Val: `
-----BEGIN PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBANfK7I6VGd4yxNNK
rg1+GdveTB4aAiqC916Yl5vFTlpCg6LIhAYmDvXPM9XJZc/h8N4jh/JNC39wgcEG
/RV2wl9T63+NR6TBLVx6nJbKjCvEuzwpB3BIun4827cU6PCksBc4hke9pTgD9v0y
DOtECKl+HuxRuKJLGoRCQ9rcoJL1AgMBAAECgYBz0R+hbvjRPuJQnNZJu5JZZTfp
OABNnLjzdmZ4Xi8tVmGcLo5dVnPVDf4+EbepGRTTxLIkI6G2JkYduYh/ypuK3TtD
JQ2j2Wb4hSFXc3jGGGmx3SFYrmajM6nW7vnBw7Ld6PaJqo5lZtYcFzpOSrzP5G0p
TPEJ1091aOrhoNexgQJBAP7M2XMw4TJqddT03/y4y46ESq4bNYOIyMd3X9yYM77Q
KH5v1x+95znBkb8hJoPgO2+un4uLr2A8L8umxByTHJECQQDYzw2BxF6D9GSDjQr6
BEX1UxfM96DiSE2N3i+1YJWOdcqg9dvJRByYzvdlqEobY2DB8Cnh1HS94V3vyruw
R1IlAkEA9NTnuTzllukfEiK+O3th9S5/B+8TK7G6o5e8IB6L0jT4RA25W0HBtgie
wFXdSWikE/tqSM9PFByhHIHA/WgKUQJALTMlbrtgtQPbfK2H7026xAV5vcqWaPaH
7J64tYiYRWX7Q4leM9yWVak4XKI0KPeT8Xq/UIx5diio69gJPxvvXQJAM1lr5o49
D0qEjXcpHjsMHcrYgQLGZPCfNn3gkGZ/pxr/3N36SyaqF6/7NRe7BLHbll9lb+8f
8FF/8F+a66TGLw==
-----END PRIVATE KEY-----
`},
		&Entry{Key: "tls-cert", Val: `
-----BEGIN CERTIFICATE-----
MIIC7jCCAlegAwIBAgIJAIvCpMZ/RhydMA0GCSqGSIb3DQEBBQUAMIGPMQswCQYD
VQQGEwJERTEPMA0GA1UECAwGQmVybGluMQ8wDQYDVQQHDAZCZXJsaW4xHDAaBgNV
BAoME0JlcmxpbmVyIFJvYm90d2Vya2UxGTAXBgNVBAMMEHRhc2tlZHNlcnZlci5j
b20xJTAjBgkqhkiG9w0BCQEWFmFycGFkLnJ5c3prYUBnbWFpbC5jb20wHhcNMTMw
OTA3MTk1MzU1WhcNMTYwOTA2MTk1MzU1WjCBjzELMAkGA1UEBhMCREUxDzANBgNV
BAgMBkJlcmxpbjEPMA0GA1UEBwwGQmVybGluMRwwGgYDVQQKDBNCZXJsaW5lciBS
b2JvdHdlcmtlMRkwFwYDVQQDDBB0YXNrZWRzZXJ2ZXIuY29tMSUwIwYJKoZIhvcN
AQkBFhZhcnBhZC5yeXN6a2FAZ21haWwuY29tMIGfMA0GCSqGSIb3DQEBAQUAA4GN
ADCBiQKBgQDXyuyOlRneMsTTSq4Nfhnb3kweGgIqgvdemJebxU5aQoOiyIQGJg71
zzPVyWXP4fDeI4fyTQt/cIHBBv0VdsJfU+t/jUekwS1cepyWyowrxLs8KQdwSLp+
PNu3FOjwpLAXOIZHvaU4A/b9MgzrRAipfh7sUbiiSxqEQkPa3KCS9QIDAQABo1Aw
TjAdBgNVHQ4EFgQUrAUcn4JJ13CSKXdKquzs03OHl0gwHwYDVR0jBBgwFoAUrAUc
n4JJ13CSKXdKquzs03OHl0gwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQUFAAOB
gQB2VmcD9Hde1Bf9lgk3iWw+ZU8JbdJvhK0MoU4RhCDEl01K2omxoT4B8OVWlFD5
GWX4rnIZtcLahM1eu8h+QxdcTNGwCpIiait2pmpVcV6pjNKv8LUxAcaemq178OfK
h3I2CsHAUTwxT1ca8SGLCsFTm03AyXaU0Q061+RX1Do/Iw==
-----END CERTIFICATE-----
`},
		&Entry{Key: "tls-key-file", Val: "filename"},
		&Entry{Key: "tls-cert-file", Val: "filename"},
		&Entry{Key: "max-search-results", Val: "0"},
		&Entry{Key: "proxy", Val: `
[{
	"include": "proxy.json"
}, {
	"method": "AUTH",
	"path-expression": "path-regexp",
	"address": "name.domain:8080",
	"rewrite-method": "POST",
	"rewrite-path": [{
		"expression": "match-regexp",
		"replace": "literal&"
	}],
	"response": {
		"status": 404,
		"header": {
			"Header-Key": "header-value",
		},
		"body": "data"
	}
}]`},
		&Entry{Key: "proxy-file", Val: "filename"},
		&Entry{Key: "authenticate", Val: "false"},
		&Entry{Key: "public-user", Val: "nobody"},
		&Entry{Key: "aes-key-file", Val: "filename"},
		&Entry{Key: "aes-iv-file", Val: "filename"},
		&Entry{Key: "token-validity", Val: "6912000"},
		&Entry{Key: "max-user-processes", Val: "0"},
		&Entry{Key: "process-idle-time", Val: "360"})
}

func TestReadFile(t *testing.T) {
	d := path.Join(Testdir, "settings")
	EnsureDirF(t, d)
	f := path.Join(d, "file")

	RemoveIfExistsF(t, f)
	es, err := readFile(f)
	if len(es) != 0 || err != nil {
		t.Fail()
	}

	WithNewFileF(t, f, func(f *os.File) error {
		_, err := f.Write([]byte("key = value"))
		return err
	})
	es, err = readFile(f)
	testEntries(t, es, &Entry{Key: "key", Val: "value"})
}

func TestExpandEntries(t *testing.T) {
	d := path.Join(Testdir, "settings")
	EnsureDirF(t, d)
	f := path.Join(d, "file")
	includeKey := "include-config"

	// only once, only once
	WithNewFileF(t, f, func(f *os.File) error {
		_, err := f.Write([]byte("key=value"))
		return err
	})
	es, err := expandIncludes(entries{
		&Entry{Key: includeKey, Val: f},
		&Entry{Key: includeKey, Val: f}}, []string{includeKey}, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es, &Entry{Key: "key", Val: "value"})
	WithNewFileF(t, f, func(f *os.File) error {
		_, err := f.Write([]byte(includeKey + "=" + f.Name() + "\nkey=value"))
		return err
	})
	es, err = expandIncludes(entries{
		&Entry{Key: includeKey, Val: f},
		&Entry{Key: "key", Val: "value"}}, []string{includeKey}, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es,
		&Entry{Key: "key", Val: "value"},
		&Entry{Key: "key", Val: "value"})

	// positioning
	WithNewFileF(t, f, func(f *os.File) error {
		_, err := f.Write([]byte("key1=value1"))
		return err
	})
	es, err = expandIncludes(entries{
		&Entry{Key: "key0", Val: "value0"},
		&Entry{Key: includeKey, Val: f},
		&Entry{Key: "key2", Val: "value2"}}, []string{includeKey}, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es,
		&Entry{Key: "key0", Val: "value0"},
		&Entry{Key: "key1", Val: "value1"},
		&Entry{Key: "key2", Val: "value2"})
	f0 := path.Join(d, "file0")
	f1 := path.Join(d, "file1")
	WithNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte(includeKey + "=" + f1 + "\nkey0=value0"))
		return err
	})
	WithNewFileF(t, f1, func(f *os.File) error {
		_, err := f.Write([]byte(includeKey + "=" + f0 + "\nkey1=value1"))
		return err
	})
	es, err = expandIncludes(entries{&Entry{Key: includeKey, Val: f0}},
		[]string{includeKey}, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es,
		&Entry{Key: "key1", Val: "value1"},
		&Entry{Key: "key0", Val: "value0"})

	// no include keys
	es, err = expandIncludes(entries{&Entry{Key: includeKey, Val: f0}}, nil, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es, &Entry{Key: includeKey, Val: f0})

	// multiple include keys
	WithNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte("key0=val0"))
		return err
	})
	WithNewFileF(t, f1, func(f *os.File) error {
		_, err := f.Write([]byte("key1=val1"))
		return err
	})
	es, err = expandIncludes(entries{
		&Entry{Key: "include0", Val: f0},
		&Entry{Key: "include1", Val: f1}},
		[]string{"include0", "include1"}, make(map[string]bool))
	if err != nil {
		t.Fail()
	}
	testEntries(t, es,
		&Entry{Key: "key0", Val: "val0"},
		&Entry{Key: "key1", Val: "val1"})
}

func TestParse(t *testing.T) {
	// no files
	e, err := Parse(nil, "")
	if err != nil || len(e) != 0 {
		t.Fail()
	}
	e, err = Parse(nil, "includeKey")
	if err != nil || len(e) != 0 {
		t.Fail()
	}

	d := path.Join(Testdir, "keyval")
	EnsureDirF(t, d)
	f0 := path.Join(d, "file0")
	f1 := path.Join(d, "file1")

	// no includes
	WithNewFileF(t, f0, func(f *os.File) error {
		_, err := f.Write([]byte("key0=val0"))
		return err
	})
	WithNewFileF(t, f1, func(f *os.File) error {
		_, err := f.Write([]byte("key1=val1"))
		return err
	})
	e, err = Parse([]string{f0, f1}, "includeKey")
	if err != nil {
		t.Fail()
	}
	testEntries(t, e,
		&Entry{Key: "key0", Val: "val0"},
		&Entry{Key: "key1", Val: "val1"})

	// includes
	f2 := path.Join(d, "file2")
	WithNewFileF(t, f1, func(f *os.File) error {
		_, err := f.Write([]byte("key1=val1\nincludeKey=" + f2))
		return err
	})
	WithNewFileF(t, f2, func(f *os.File) error {
		_, err := f.Write([]byte("key2=val2"))
		return err
	})
	e, err = Parse([]string{f0, f1}, "includeKey")
	if err != nil {
		t.Fail()
	}
	testEntries(t, e,
		&Entry{Key: "key0", Val: "val0"},
		&Entry{Key: "key1", Val: "val1"},
		&Entry{Key: "key2", Val: "val2"})
}
