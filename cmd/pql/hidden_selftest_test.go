package main

import "testing"

func TestHiddenSelfTestArgumentCount(t *testing.T) {
	// KTD14: hidden modes have fixed positional layouts before Cobra. Every
	// malformed row returns without attempting a native self-test operation.
	for _, row := range []struct {
		name  string
		words []string
	}{
		{"SelfTestMissing", []string{"__pql_selftest"}},
		{"SelfTestShort", []string{"__pql_selftest", "clean", "target"}},
		{"SelfTestExtra", []string{"__pql_selftest", "clean", "target", "1", "extra"}},
		{"DecoyExtra", []string{"__pql_decoy", "extra"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			code, ok := hidden(row.words)
			if !ok || code != 64 {
				t.Fatalf("got %d %v want 64 true", code, ok)
			}
		})
	}
}
