package confine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// plantedDirectories lists only the ancestors below HOME, excluding the file.
func plantedDirectories(home, planted string) ([]string, error) {
	if planted == "" {
		return nil, nil
	}
	relative, err := filepath.Rel(home, planted)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return nil, fmt.Errorf("planted file outside home")
	}
	var dirs []string
	current := home
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		dirs = append(dirs, current)
	}
	return dirs, nil
}
