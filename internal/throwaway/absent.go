package throwaway

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AbsentPlanted reserves a private parent outside all subject homes.
func AbsentPlanted(parent string) (*Workspace, string, error) {
	w, err := Create(parent)
	if err != nil {
		return nil, "", err
	}
	name := filepath.Join(w.Path(), "absent")
	if _, err := os.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		return nil, "", errors.Join(fmt.Errorf("stand-in planted path exists"), w.Remove())
	}
	return w, name, nil
}
