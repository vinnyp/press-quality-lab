package confine

// SelfTestHidden fails closed because the native witnesses require arm64.
func SelfTestHidden(_, _, _ string) int { return LauncherFailed }
