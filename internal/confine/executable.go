package confine

import (
	"path"
	"strings"
)

// systemExecutable implements E-55's in-place computation exception.
func systemExecutable(name string) bool {
	return path.IsAbs(name) && path.Clean(name) == name && (strings.HasPrefix(name, "/bin/") || strings.HasPrefix(name, "/usr/bin/"))
}
