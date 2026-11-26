package throwaway

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func assertMemberExecuting(t *testing.T, pid int) {
	t.Helper()
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		t.Fatal(err)
	}
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		t.Fatal("invalid process stat")
	}
	fields := strings.Fields(string(b[i+1:]))
	if len(fields) == 0 || fields[0] == "Z" || fields[0] == "X" {
		t.Fatal("surviving member is not executing")
	}
}

func raceRestoreMode(root *os.Root, name string) error { return restoreMode(root, name) }
