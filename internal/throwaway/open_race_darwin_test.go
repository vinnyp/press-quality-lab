package throwaway

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestDirectorySwapBeforeOpen(t *testing.T) {
	// QE20, KTD20: O_NOFOLLOW must refuse the exact type-check/open race.
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
	if err := os.Chmod(outside, 0555); err != nil {
		t.Fatal(err)
	}
	verifyMetadata := protectRaceOutside(t, outside)
	defer verifyMetadata()
	t.Cleanup(func() {
		var stat unix.Stat_t
		if err := unix.Lstat(outside, &stat); err != nil {
			t.Error(err)
			return
		}
		if err := unix.Chflags(outside, int(stat.Flags&^(unix.UF_IMMUTABLE|unix.UF_APPEND))); err != nil {
			t.Error(err)
			return
		}
		clearRaceACL(t, outside)
		if err := os.Chmod(outside, 0700); err != nil {
			t.Error(err)
		}
	})
	home := filepath.Join(w.Path(), "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	b := &budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now, beforeDirectoryOpen: func(_ int, name string) {
		if name != "home" {
			return
		}
		called = true
		if err := os.Rename(home, filepath.Join(w.Path(), "park")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, home); err != nil {
			t.Fatal(err)
		}
	}}
	err = w.remove(b)
	if !called || err == nil {
		t.Fatalf("race not refused: called=%v error=%v", called, err)
	}
	assertOutside(t, outside, sentinel, sha256.Sum256([]byte("kept")), 0555)
	if err := w.Remove(); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestHardLinkMetadataUnchanged(t *testing.T) {
	// KTD20, QE20: defense in depth preserves metadata of multiply-linked leaves.
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("kept"), 0400); err != nil {
		t.Fatal(err)
	}
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(w.Path(), "link")); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0400 || string(data) != "kept" {
		t.Fatalf("outside changed: %v %q", info.Mode(), data)
	}
}

func TestDirectoryGoneBeforeOpen(t *testing.T) {
	// QE20, E-19: concurrent removal between stat and directory open is success.
	w, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(w.Path(), "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	b := &budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now, beforeDirectoryOpen: func(_ int, name string) {
		if name == "home" {
			called = true
			if err := os.Remove(home); err != nil {
				t.Fatal(err)
			}
		}
	}}
	if err := w.remove(b); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("directory open not reached")
	}
	if _, err := os.Stat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains: %v", err)
	}
}
