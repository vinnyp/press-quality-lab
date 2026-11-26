//go:build darwin && !macos_evidence

package throwaway

import "testing"

func clearRaceACL(t *testing.T, path string) { t.Helper() }
