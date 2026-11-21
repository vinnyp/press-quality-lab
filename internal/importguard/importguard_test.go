package importguard

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoCommandFrameworks(t *testing.T) {
	mod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("find module: %v", err)
	}
	path := strings.TrimSpace(string(mod))
	if path == "" || path == "/dev/null" {
		t.Fatal("no module found")
	}
	cmd := exec.Command("go", "list", "-deps", "-test", "-f", "{{.ImportPath}}", "./internal/...")
	cmd.Dir = filepath.Dir(path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list dependencies: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) == "" {
		t.Fatal("dependency list is empty")
	}
	for line := range strings.Lines(string(output)) {
		if strings.HasPrefix(strings.TrimSpace(line), "github.com/spf13/") {
			t.Errorf("forbidden framework dependency: %s", strings.TrimSpace(line))
		}
	}
}
