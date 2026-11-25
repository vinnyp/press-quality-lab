//go:build !darwin

package confine

// Hidden refuses unsupported systems (QH1) without starting a program.
func Hidden(_, _ string, _ func() (string, []string, []string)) int { return LauncherFailed }
