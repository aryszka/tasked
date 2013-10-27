package main

import (
	"code.google.com/p/gopam"
	"code.google.com/p/tasked/auth"
	"errors"
	"log"
	"os"
	"path"
)

var authFailed = errors.New("Authentication failed.")

func getConfigPath() (string, error) {
	p := flags.config
	if len(p) > 0 {
		return abspath(p, "")
	}
	p = os.Getenv(configEnvKey)
	if len(p) > 0 {
		return abspath(p, "")
	}
	p = path.Join(os.Getenv("HOME"), defaultConfigBaseName)
	if ok, err := checkPath(p, false); ok || err != nil {
		return p, err
	}
	if ok, err := checkPath(sysConfig, false); ok || err != nil {
		return sysConfig, err
	}
	return "", nil
}

func getHttpDir() (string, error) {
	dn := flags.root
	if len(dn) > 0 {
		return abspath(dn, "")
	}
	dn = cfg.files.root
	if len(dn) > 0 {
		return dn, nil
	}
	return os.Getwd()
}

func authPam(user, pwd string) error {
	t, s := pam.Start("", user, pam.ResponseFunc(func(style int, _ string) (string, bool) {
		switch style {
		case pam.PROMPT_ECHO_OFF, pam.PROMPT_ECHO_ON:
			return pwd, true
		default:
			return "", false
		}
	}))
	if s != pam.SUCCESS {
		return authFailed
	}
	defer t.End(s)

	s = t.Authenticate(0)
	if s != pam.SUCCESS {
		return authFailed
	}
	return nil
}

func main() {
	parseFlags()

	configPath, err := getConfigPath()
	if err != nil {
		log.Panicln(err)
	}

	err = initConfig(configPath)
	if err != nil {
		log.Panicln(err)
	}

	err = auth.Init(&cfg, auth.AuthFunc(authPam))
	if err != nil {
		log.Panicln(err)
	}

	dn, err = getHttpDir()
	if err != nil {
		log.Panicln(err)
	}
	ensureDir(dn)

	err = serve()
	if err != nil {
		log.Panicln(err)
	}
}
