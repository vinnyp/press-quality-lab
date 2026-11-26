package throwaway

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

func assertGroupEmpty(t *testing.T, pid int) {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		id, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fields, err := processFields(id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if fields[2] == strconv.Itoa(pid) && fields[0] != "Z" && fields[0] != "X" {
			t.Fatalf("live member %d", id)
		}
	}
}
