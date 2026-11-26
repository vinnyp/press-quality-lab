package confine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Environment builds QH12's table in its specified order. Declarations retain
// subject-file order. The caller creates the home and its empty .pql-tmp.
func Environment(home string, uid int, declarations []string) ([]string, error) {
	home, err := canonical(home, false)
	if err != nil {
		return nil, err
	}
	env := []string{
		"HOME=" + home,
		"TMPDIR=" + filepath.Join(home, ".pql-tmp") + "/",
		"PATH=/usr/bin:/bin",
		"LANG=C",
		"LC_ALL=C",
		"TZ=UTC",
		fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", uid),
	}
	for _, declaration := range declarations {
		name, _, ok := strings.Cut(declaration, "=")
		if !ok || name == "" || strings.ContainsRune(declaration, 0) {
			return nil, fmt.Errorf("invalid environment declaration")
		}
		if strings.HasPrefix(name, "DYLD_") || strings.HasPrefix(name, "__CF_") {
			return nil, fmt.Errorf("reserved environment declaration")
		}
		for _, fixed := range env[:7] {
			fixedName, _, _ := strings.Cut(fixed, "=")
			if name == fixedName {
				return nil, fmt.Errorf("reserved environment declaration")
			}
		}
		env = append(env, declaration)
	}
	return env, nil
}

// invocationEnvironment also accepts the complete environment supplied by older
// callers, provided its fixed entries exactly match this invocation's table.
func invocationEnvironment(home string, uid int, supplied []string) ([]string, error) {
	fixed, err := Environment(home, uid, nil)
	if err != nil {
		return nil, err
	}
	var declarations []string
	for _, entry := range supplied {
		matched := entry == "HOME="+home || entry == "TMPDIR="+filepath.Join(home, ".pql-tmp")+"/"
		for _, expected := range fixed {
			if entry == expected {
				matched = true
				break
			}
		}
		if !matched {
			declarations = append(declarations, entry)
		}
	}
	return Environment(home, uid, declarations)
}
