package main

import "os"

func withEnv(key, val string, f func() error) error {
	orig := os.Getenv(key)
	defer os.Setenv(key, orig)
	err := os.Setenv(key, val)
	if err != nil {
		return err
	}
	return f()
}

func withNewFile(fn string, do func(*os.File) error) error {
	err := os.Remove(fn)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	if do == nil {
		return nil
	}
	return do(f)
}

func create(fn string) error {
	return withNewFile(fn, nil)
}
