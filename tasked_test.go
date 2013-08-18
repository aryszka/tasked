package main

const testenvKey = "TEST_DATA"

var testdir string

func init() {
	var err error
	testdir, err = ensureEnvDir(testenvKey, "")
	if err != nil {
		panic("Cannot ensure test directory.")
	}
}
