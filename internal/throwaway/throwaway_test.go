package throwaway

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestRemoveHostileHome(t *testing.T) {
	// QE20, KTD20: cleanup cannot follow a link or open a special file.
	parent := t.TempDir()
	w, err := Create(parent)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(outside, "kept")
	if err := os.WriteFile(file, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(outside, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(outside, 0700); err != nil {
			t.Error(err)
		}
	})
	if err := os.Symlink(outside, filepath.Join(w.Path(), "link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the no-follow mode operation itself, including its refusal.
	if err := restoreMode(root, "link"); err == nil {
		t.Error("outside link mode change succeeded")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(w.Path(), "locked")
	if err := os.Mkdir(locked, 0700); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(locked, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	// Keep a second name so the blocked reader can be released after unlink.
	alias := filepath.Join(parent, "fifo_control")
	if err := os.Link(fifo, alias); err != nil {
		t.Fatal(err)
	}
	opened := make(chan error, 1)
	go func() {
		fd, err := syscall.Open(alias, syscall.O_RDONLY, 0)
		if err == nil {
			err = syscall.Close(fd)
		}
		opened <- err
	}()
	// A writer opened by cleanup would unblock this reader.
	select {
	case err := <-opened:
		t.Fatalf("FIFO reader returned before cleanup: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	t.Cleanup(func() {
		select {
		case err := <-opened:
			if err != nil {
				t.Error(err)
			}
			return
		default:
		}
		var fd int
		var err error
		for {
			fd, err = syscall.Open(alias, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
			if !errors.Is(err, syscall.EINTR) {
				break
			}
		}
		if err != nil {
			t.Error(err)
			return
		}
		if err := syscall.Close(fd); err != nil {
			t.Error(err)
		}
		select {
		case err := <-opened:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("FIFO control reader did not finish")
		}
	})
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-opened:
		t.Fatalf("cleanup opened FIFO for writing: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if _, err := os.Lstat(w.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace remains: %v", err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0555 {
		t.Fatalf("outside mode: %v", info.Mode())
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged" {
		t.Fatalf("outside content: %q", data)
	}
}

func TestRemovalBounds(t *testing.T) {
	// QE20, KTD20, E-11: exhaustion is an error, never successful cleanup.
	for _, name := range []string{"Entries", "Time"} {
		t.Run(name, func(t *testing.T) {
			w, err := Create(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(w.Path(), "file"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			b := budget{remaining: MaxWalkEntries, deadline: time.Now().Add(MaxWalkDuration), now: time.Now}
			want := ErrEntryLimit
			if name == "Entries" {
				b.remaining = 1
			} else {
				b.deadline = time.Now().Add(-time.Second)
				want = ErrTimeLimit
			}
			if err := w.remove(&b); !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
			if _, err := os.Stat(w.Path()); err != nil {
				t.Fatal(err)
			}
			if err := w.Remove(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
