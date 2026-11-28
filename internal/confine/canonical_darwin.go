package confine

import (
	"fmt"
	"path/filepath"
)

// canonicalExecutable refuses a private-copy source that is not absolute and canonical.
func canonicalExecutable(name string) error {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return fmt.Errorf("executable path must be absolute and canonical")
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil {
		return err
	}
	if resolved != name {
		return fmt.Errorf("executable path must be absolute and canonical")
	}
	return nil
}
