//go:build !darwin || !macos_evidence

package throwaway

import "testing"

func protectRaceOutside(t *testing.T, path string) func() {
	t.Helper()
	return func() {}
}

func clearRaceACL(t *testing.T, path string) { t.Helper() }
