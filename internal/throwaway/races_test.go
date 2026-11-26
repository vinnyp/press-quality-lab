package throwaway

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestHomeSwapDuringCleanup(t *testing.T) {
	// QE20, KTD20: a home replaced after its parent is opened stays inside root.
	parent := t.TempDir()
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0555); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "kept")
	if err := os.Chmod(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(outside, 0700); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(outside, 0555); err != nil {
		t.Fatal(err)
	}
	verifyMetadata := protectRaceOutside(t, outside)
	defer verifyMetadata()
	home := filepath.Join(w.Path(), "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Positive control: same parent, directory and operation before substitution.
	if err := raceRestoreMode(root, "home"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(home, filepath.Join(w.Path(), "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, home); err != nil {
		t.Fatal(err)
	}
	if err := raceRestoreMode(root, "home"); err != nil {
		t.Logf("replacement refused: %v", err)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	assertOutside(t, outside, sentinel, sha256.Sum256([]byte("kept")), 0555)
}

func TestLockedDirectorySwapBeforeMode(t *testing.T) {
	// KTD20, QH25: the Lstat-to-Chmod race must not change an outside directory.
	parent := t.TempDir()
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "kept")
	if err := os.WriteFile(sentinel, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(outside, 0700); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(outside, 0555); err != nil {
		t.Fatal(err)
	}
	verifyMetadata := protectRaceOutside(t, outside)
	defer verifyMetadata()
	home := filepath.Join(w.Path(), "home")
	if err := os.Mkdir(home, 0000); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := raceRestoreMode(root, "home"); err != nil {
		t.Fatal("positive control", err)
	}
	if err := os.Chmod(home, 0000); err != nil {
		t.Fatal(err)
	}
	info, err := root.Lstat("home")
	if err != nil || !info.IsDir() {
		t.Fatal(info, err)
	}
	if err := os.Rename(home, filepath.Join(w.Path(), "old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, home); err != nil {
		t.Fatal(err)
	}
	if err := raceRestoreMode(root, "home"); err != nil {
		t.Logf("replacement mode change refused: %v", err)
	}
	assertOutside(t, outside, sentinel, sha256.Sum256([]byte("kept")), 0555)
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
}

func assertOutside(t *testing.T, dir, file string, digest [32]byte, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("outside mode %v", info.Mode())
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(b) != digest {
		t.Fatal("outside digest changed")
	}
	info, err = os.Lstat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0400 {
		t.Fatalf("outside file mode %v", info.Mode())
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0].Name() != "kept" {
		t.Fatalf("outside entries %v", names)
	}
}

func TestRacingSwapperHelper(t *testing.T) {
	if os.Getenv("PQL_SWAPPER") != "1" {
		return
	}
	// KTD20: the helper is bounded even if its parent test fails.
	root, outside := os.Getenv("PQL_SWAP_ROOT"), os.Getenv("PQL_SWAP_OUTSIDE")
	ready := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		home, park := filepath.Join(root, "home"), filepath.Join(root, "park")
		if err := os.Mkdir(home, 0700); err != nil && !errors.Is(err, os.ErrExist) && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.EINVAL) {
			if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
				os.Exit(80)
			}
			os.Exit(81)
		}
		if err := os.Rename(home, park); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.ENOTDIR) && !errors.Is(err, syscall.EISDIR) && !errors.Is(err, syscall.EINVAL) {
			os.Exit(82)
		}
		if err := os.Symlink(outside, home); err != nil {
			if !errors.Is(err, os.ErrExist) && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.EINVAL) {
				os.Exit(83)
			}
		} else if !ready {
			if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
				os.Exit(86)
			}
			ready = true
		}
		if err := os.Remove(home); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTDIR) && !errors.Is(err, syscall.EISDIR) {
			os.Exit(84)
		}
		if err := os.Rename(park, home); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTDIR) && !errors.Is(err, syscall.EISDIR) {
			os.Exit(85)
		}
	}
	os.Exit(0)
}

func TestThousandRacingWalks(t *testing.T) {
	// KTD20, QE20: digest, mode and existence of every outside entry survive.
	parent := t.TempDir()
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "kept")
	if err := os.WriteFile(sentinel, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(outside, 0700); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(outside, 0555); err != nil {
		t.Fatal(err)
	}
	verifyMetadata := protectRaceOutside(t, outside)
	defer verifyMetadata()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r, wpipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	pid, err := syscall.ForkExec("/usr/bin/timeout", []string{"timeout", "-k", "1s", "30s", exe, "-test.run=^TestRacingSwapperHelper$"}, &syscall.ProcAttr{Env: append(os.Environ(), "PQL_SWAPPER=1", "PQL_SWAP_ROOT="+w.Path(), "PQL_SWAP_OUTSIDE="+outside), Files: []uintptr{0, wpipe.Fd(), 2}, Sys: &syscall.SysProcAttr{Setpgid: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := wpipe.Close(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Error(err)
		}
		if err := EndGroup(pid); err != nil {
			t.Error(err)
		}
		var status syscall.WaitStatus
		if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
			t.Error(err)
		} else if !status.Signaled() || status.Signal() != syscall.SIGKILL {
			t.Errorf("swapper ended early: %v", status)
		}
	}
	defer stop()
	var ready [6]byte
	if _, err := io.ReadFull(r, ready[:]); err != nil || string(ready[:]) != "ready\n" {
		t.Fatalf("swapper control %q: %v", ready, err)
	}
	refusals := 0
	for range 1000 {
		if err := w.Remove(); err != nil {
			refusals++
		} // Races can refuse cleanup; integrity is always asserted.
		assertOutside(t, outside, sentinel, sha256.Sum256([]byte("kept")), 0555)
		if err := os.Mkdir(w.Path(), 0700); err != nil && !errors.Is(err, os.ErrExist) {
			t.Fatal(err)
		}
	}
	if refusals == 0 {
		t.Fatal("no racing walk was refused")
	}
	t.Logf("racing walks refused: %d of 1000", refusals)
	stop()
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
}

func TestOddNamesAndScanSizeTrees(t *testing.T) {
	// QE20, KTD20, QP13: cleanup handles byte names and homes at scan byte bounds.
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ name, value string }{{"Newline", "line\nbreak"}, {"InvalidUTF8", string([]byte{0xff, 0xfe})}} {
		t.Run(row.name, func(t *testing.T) {
			if err := os.Mkdir(filepath.Join(w.Path(), row.value), 0700); err != nil {
				if errors.Is(err, syscall.EILSEQ) && row.name == "InvalidUTF8" {
					t.Skip("filesystem rejects invalid UTF-8 names")
				}
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(w.Path(), row.value, "leaf\n"), []byte("bytes"), 0000); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Sixteen sparse 16 MiB files total 256 MiB without allocating that space.
	for i := range 16 {
		f, err := os.Create(filepath.Join(w.Path(), fmt.Sprintf("bound%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		if err := errors.Join(f.Truncate(16<<20), f.Close()); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tree remains: %v", err)
	}
}
