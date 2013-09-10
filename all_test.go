package main

import (
	"os"
)

func withEnv(key, val string, f func() error) error {
	orig := os.Getenv(key)
	defer os.Setenv(key, orig)
	err := os.Setenv(key, val)
	if err != nil {
		return err
	}
	return f()
}
