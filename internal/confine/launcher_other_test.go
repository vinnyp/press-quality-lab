//go:build !darwin

package confine

import "testing"

func TestUnsupportedLauncher(t *testing.T) {
	if code := Hidden(LauncherMode, "marker", func() (string, []string, []string) {
		t.Fatal("program selected on unsupported system")
		return "", nil, nil
	}); code != 70 {
		t.Fatalf("unsupported code=%d", code)
	}
}
