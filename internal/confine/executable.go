package confine

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// systemExecutable implements E-55's in-place computation exception.
func systemExecutable(name string) bool {
	return path.IsAbs(name) && path.Clean(name) == name && (strings.HasPrefix(name, "/bin/") || strings.HasPrefix(name, "/usr/bin/"))
}

func resolvedSystemExecutable(name string, resolve func(string) (string, error), stat func(string) (os.FileInfo, error)) (string, error) {
	resolved, err := resolve(name)
	if err != nil {
		return "", err
	}
	if !systemExecutable(name) || !systemExecutable(resolved) {
		return "", fmt.Errorf("system executable outside allowed directories")
	}
	info, err := stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("system executable is not regular")
	}
	return resolved, nil
}
