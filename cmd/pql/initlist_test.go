package main

import (
	"debug/elf"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Cross-check the Linux linker's ordered init tasks on any host. The Linux
// runtime test separately checks what inittrace actually reports.
func TestLinuxInitialisationTasks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pql")
	cmd := exec.Command("go", "build", "-o", path, ".")
	cmd.Env = append(builderEnvironment(), "GOOS=linux", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Linux build: %v %s", err, b)
	}
	f, err := elf.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	symbols, err := f.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	names := map[uint64]string{}
	tasks := map[string]elf.Symbol{}
	for _, sym := range symbols {
		names[sym.Value] = sym.Name
		if sym.Name == "go:runtime.inittasks" || sym.Name == "go:main.inittasks" {
			tasks[sym.Name] = sym
		}
	}
	var packages []string
	seen := map[string]bool{}
	for _, name := range []string{"go:runtime.inittasks", "go:main.inittasks"} {
		sym, ok := tasks[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		section := f.Sections[sym.Section]
		b, err := section.Data()
		if err != nil {
			t.Fatal(err)
		}
		b = b[sym.Value-section.Addr : sym.Value-section.Addr+sym.Size]
		for len(b) > 0 {
			address := binary.LittleEndian.Uint64(b)
			b = b[8:]
			task := strings.TrimSuffix(names[address], "..inittask")
			if task == "" {
				t.Fatal("unnamed init task")
			}
			if !seen[task] {
				packages = append(packages, task)
				seen[task] = true
			}
		}
	}
	want, err := os.ReadFile("testdata/launcher-linux-init.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != strings.Join(packages, "\n")+"\n" {
		t.Fatalf("Linux init task list changed: %q", packages)
	}
}
