package main

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	original := os.TempDir()
	isolated, err := os.MkdirTemp(original, "pql-test-tmp-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := func() int {
		if err := os.Setenv("TMPDIR", isolated); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if os.TempDir() != isolated || os.TempDir() == original {
			fmt.Fprintln(os.Stderr, "test TMPDIR isolation failed")
			return 1
		}
		return m.Run()
	}()
	if err := os.RemoveAll(isolated); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}
