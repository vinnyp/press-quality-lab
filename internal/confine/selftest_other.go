//go:build !darwin

package confine

// SelfTestHidden never attempts a platform operation on an unsupported system.
func SelfTestHidden(_, _, _ string) int { return LauncherFailed }

func DecoyHidden() int { return LauncherFailed }
