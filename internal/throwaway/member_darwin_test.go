package throwaway

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"testing"
)

func assertMemberExecuting(t *testing.T, pid int) {
	t.Helper()
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		t.Fatal(err)
	}
	if p.Proc.P_stat == 5 {
		t.Fatal("surviving member is a zombie")
	}
}

// raceRestoreMode exercises the actual Darwin metadata operation after the
// test has replaced the entry observed by Lstat.
func raceRestoreMode(root *os.Root, name string) (err error) {
	parent, err := root.OpenFile(".", os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, parent.Close()) }()
	return prepareAt(int(parent.Fd()), name)
}

func assertGroupEmpty(t *testing.T, pid int) {
	t.Helper()
	members, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range members {
		if member.Proc.P_stat != 5 {
			t.Fatalf("live member %d", member.Proc.P_pid)
		}
	}
}
