//go:build darwin || linux

package throwaway

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFix1DeepHome(t *testing.T) {
	// QH25, QP13, E-46: descriptor walk is bounded and retains stream leaks.
	home := t.TempDir()
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		if err := root.Mkdir("d", 0700); err != nil {
			t.Fatal(err)
		}
		child, err := root.OpenRoot("d")
		if err != nil {
			t.Fatal(err)
		}
		if err := root.Close(); err != nil {
			t.Fatal(err)
		}
		root = child
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	r, err := ScanHome(home, "", []ScanForm{{"plain", "abcdefgh"}}, []byte("abcdefgh"), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("depth 400 exceeded two seconds")
	}
	if !r.BoundReached || len(r.Leaks) != 1 {
		t.Fatalf("result %+v", r)
	}
	snapshot, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Unreadable {
		t.Fatal("bounded snapshot was readable")
	}
}
func TestFix1Hardlinks(t *testing.T) {
	// QH25, E-10: inspection must not chmod a multiply linked file.
	home := t.TempDir()
	outside := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(outside, []byte("abcdefgh"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, filepath.Join(home, "data")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := ScanHome(home, "", []ScanForm{{"plain", "abcdefgh"}}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Unreadable || !scan.Unreadable || info.Mode().Perm() != 0400 {
		t.Fatalf("snapshot %+v scan %+v mode %o", snapshot, scan, info.Mode().Perm())
	}
}
func TestFix1ScanFormsOrder(t *testing.T) {
	// QP12, QP13: encoded contents and global byte-wise path order.
	home := t.TempDir()
	putHome(t, home, "a-b", "fn5-Pz8_YWI")
	putHome(t, home, "a/b", "%7e%7e%7e%3f%3f%3f%61%62")
	forms := []ScanForm{{"base64url-nopad", "fn5-Pz8_YWI"}, {"percent-lower", "%7e%7e%7e%3f%3f%3f%61%62"}}
	r, err := ScanHome(home, "", forms, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []ScanLeak{{"home/a-b", "base64url-nopad"}, {"home/a/b", "percent-lower"}}
	if !reflect.DeepEqual(r.Leaks, want) {
		t.Fatalf("leaks %+v", r.Leaks)
	}
	r, err = scanHome(home, "", forms, nil, nil, false, openHomeFile, 100, 11)
	if err != nil {
		t.Fatal(err)
	}
	if !r.BoundReached || !reflect.DeepEqual(r.Leaks, want[:1]) {
		t.Fatalf("order %+v", r)
	}
	for _, forms := range [][]ScanForm{nil, {{Name: "plain"}}} {
		if _, err := ScanHome(home, "", forms, nil, nil, false); err == nil {
			t.Fatal("accepted absent form")
		}
	}
}
func TestFix1OverlappingToken(t *testing.T) {
	// QP10, E-20: occurrences count by start position.
	home := t.TempDir()
	putHome(t, home, "data", strings.Repeat("a", 9))
	if _, err := PlantCanary(home, "data", "aaaaaaaa"); err == nil {
		t.Fatal("accepted overlapping token")
	}
}

type failingHomeRead struct{ homeFile }
type deadlineHomeFile struct {
	homeFile
	expire func()
}

func (f deadlineHomeFile) Read(p []byte) (int, error) {
	n, e := f.homeFile.Read(p)
	f.expire()
	return n, e
}

type nonregularHomeInfo struct{ os.FileInfo }

func (i nonregularHomeInfo) Mode() os.FileMode { return os.ModeNamedPipe | 0600 }

type nonregularHomeFile struct{ homeFile }

func (f nonregularHomeFile) Stat() (os.FileInfo, error) {
	i, e := f.homeFile.Stat()
	if e != nil {
		return nil, e
	}
	return nonregularHomeInfo{i}, nil
}

func (f failingHomeRead) Read([]byte) (int, error) { return 0, os.ErrPermission }
func TestFix1InspectionFaults(t *testing.T) {
	// QH25, E-10, E-46: read, chmod, identity and deadline failures cannot compare unchanged.
	for _, row := range []string{"Chmod", "Read", "Deadline", "Identity", "Nonregular", "LastDeadline"} {
		t.Run(row, func(t *testing.T) {
			home := t.TempDir()
			putHome(t, home, "data", "abcdefgh")
			baseline, err := SnapshotHome(home)
			if err != nil || baseline.Unreadable {
				t.Fatalf("positive snapshot %+v %v", baseline, err)
			}
			positive, err := ScanHome(home, "", []ScanForm{{"plain", "abcdefgh"}}, nil, nil, false)
			if err != nil || positive.Unreadable || len(positive.Leaks) != 1 {
				t.Fatalf("positive scan %+v %v", positive, err)
			}
			h := defaultHomeHooks(openHomeFile)
			switch row {
			case "Chmod":
				h.chmod = func(r *os.Root, n string, m os.FileMode) error {
					if n == "data" {
						return os.ErrPermission
					}
					return r.Chmod(n, m)
				}
			case "Read":
				h.open = func(r *os.Root, n string) (homeFile, error) { f, e := openHomeFile(r, n); return failingHomeRead{f}, e }
			case "Identity":
				other := filepath.Join(t.TempDir(), "other")
				if err := os.WriteFile(other, []byte("abcdefgh"), 0600); err != nil {
					t.Fatal(err)
				}
				h.open = func(*os.Root, string) (homeFile, error) { return os.Open(other) }
			case "Nonregular":
				h.open = func(r *os.Root, _ string) (homeFile, error) {
					f, e := openHomeFile(r, "data")
					return nonregularHomeFile{f}, e
				}
			}
			// The opener advances this clock after a successful open of the last file.
			if row == "Deadline" {
				expired := false
				h.now = func() time.Time {
					if expired {
						return time.Now().Add(2 * MaxWalkDuration)
					}
					return time.Now()
				}
				h.open = func(r *os.Root, n string) (homeFile, error) { f, e := openHomeFile(r, n); expired = true; return f, e }
			}

			if row == "LastDeadline" {
				expired := false
				h.now = func() time.Time {
					if expired {
						return time.Now().Add(2 * MaxWalkDuration)
					}
					return time.Now()
				}
				h.open = func(r *os.Root, n string) (homeFile, error) {
					f, e := openHomeFile(r, n)
					return deadlineHomeFile{f, func() { expired = true }}, e
				}
			}
			snapshot, err := snapshotHomeWith(home, h)
			if err != nil {
				t.Fatal(err)
			}
			if !snapshot.Unreadable {
				t.Fatal("snapshot fault lost")
			}
			// Rebuild the deadline clock for the independent scan.
			if row == "Deadline" {
				expired := false
				h.now = func() time.Time {
					if expired {
						return time.Now().Add(2 * MaxWalkDuration)
					}
					return time.Now()
				}
				h.open = func(r *os.Root, n string) (homeFile, error) { f, e := openHomeFile(r, n); expired = true; return f, e }
			}

			if row == "LastDeadline" {
				expired := false
				h.now = func() time.Time {
					if expired {
						return time.Now().Add(2 * MaxWalkDuration)
					}
					return time.Now()
				}
				h.open = func(r *os.Root, n string) (homeFile, error) {
					f, e := openHomeFile(r, n)
					return deadlineHomeFile{f, func() { expired = true }}, e
				}
			}
			scan, err := scanHomeWith(home, "", []ScanForm{{"plain", "abcdefgh"}}, nil, nil, false, h, 8, 8)
			if err != nil {
				t.Fatal(err)
			}
			if !scan.Unreadable {
				t.Fatal("scan fault lost")
			}
			_, unreadable := CompareHomes(snapshot, snapshot)
			if !unreadable {
				t.Fatal("two failed snapshots compared unchanged")
			}
		})
	}
}

func TestFix1WalkFaults(t *testing.T) {
	// E-46, QH25, QP13: directory and walk deadlines are check facts, preserving leaks.
	for _, row := range []string{"Directory", "Deadline", "Entries"} {
		t.Run(row, func(t *testing.T) {
			home := t.TempDir()
			putHome(t, home, "data", "clean")
			putHome(t, home, "other", "clean")
			hooks := func() homeHooks {
				h := defaultHomeHooks(openHomeFile)
				if row == "Entries" {
					h.entryLimit = 1
				} else if row == "Directory" {
					h.chmod = func(*os.Root, string, os.FileMode) error { return os.ErrPermission }
				} else {
					calls := 0
					h.now = func() time.Time {
						calls++
						if calls > 1 {
							return time.Now().Add(2 * MaxWalkDuration)
						}
						return time.Now()
					}
				}
				return h
			}
			snapshot, err := snapshotHomeWith(home, hooks())
			if err != nil || !snapshot.Unreadable {
				t.Fatalf("snapshot %+v error %v", snapshot, err)
			}
			r, err := scanHomeWith(home, "", []ScanForm{{"plain", "abcdefgh"}}, []byte("abcdefgh"), nil, false, hooks(), 100, 100)
			if err != nil || len(r.Leaks) != 1 || row == "Directory" && !r.Unreadable || row != "Directory" && !r.BoundReached {
				t.Fatalf("scan %+v error %v", r, err)
			}
		})
	}
}
