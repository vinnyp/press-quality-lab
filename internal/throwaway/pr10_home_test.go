//go:build darwin || linux

package throwaway

import (
	"errors"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func putHome(t *testing.T, home, name, value string) {
	t.Helper()
	p := filepath.Join(home, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestHomeSnapshots(t *testing.T) {
	// QP5, QH14, QP6, QH25: independent homes copy bytes, and modes do not change content identity.
	seed := t.TempDir()
	putHome(t, seed, "dir/data", "original")
	// E-41: a seed side file left byte-for-byte unchanged is not a delta.
	putHome(t, seed, "dir/data-wal", "committed")
	parent := t.TempDir()
	home := filepath.Join(parent, "home")
	if err := CopySeed(seed, home); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(filepath.Join(home, "dir/data"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != "original" {
		t.Fatalf("seed bytes not copied: %q", copied)
	}
	if err := CopySeed(seed, home); !errors.Is(err, os.ErrExist) {
		t.Fatalf("reused a home: %v", err)
	}
	before, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, "dir/data"), 0); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	changes, unreadable := CompareHomes(before, after)
	if len(changes) != 0 || unreadable {
		t.Fatalf("mode changed identity: %v %v", changes, unreadable)
	}
	putHome(t, home, "dir/data", "changed")
	after, err = SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	changes, unreadable = CompareHomes(before, after)
	if unreadable || len(changes) != 1 || changes[0].Path != "dir/data" || changes[0].Change != "modified" {
		t.Fatalf("changes %v unreadable %v", changes, unreadable)
	}
	original, err := os.ReadFile(filepath.Join(seed, "dir/data"))
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "original" {
		t.Fatal("seed changed")
	}
	if err := os.Remove(filepath.Join(home, "dir/data")); err != nil {
		t.Fatal(err)
	}
	after, err = SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	changes, _ = CompareHomes(before, after)
	if len(changes) != 1 || changes[0].Change != "removed" {
		t.Fatalf("removal %v", changes)
	}
}
func TestHomeUnreadable(t *testing.T) {
	// QH25, QP6: E-10 cannot silently compare an unreadable entry as unchanged.
	home := t.TempDir()
	putHome(t, home, "data", "value")
	before, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(home, "data"), 0); err != nil {
		t.Fatal(err)
	}
	calls := 0
	after, err := snapshotHome(home, func(root *os.Root, name string) (homeFile, error) {
		calls++
		info, e := root.Lstat(name)
		if e != nil {
			t.Fatal(e)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("opener ran before reset")
		}
		return nil, syscall.EACCES
	})
	if err != nil {
		t.Fatal(err)
	}
	changes, unreadable := CompareHomes(before, after)
	if calls != 1 || !unreadable || len(changes) != 0 {
		t.Fatalf("calls=%d unreadable=%v changes=%v", calls, unreadable, changes)
	}
}
func TestHomeKinds(t *testing.T) {
	// QH25, QP9, QP5: links and special files are changes, never opened or copied.
	home := t.TempDir()
	before, err := SnapshotHome(home)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	putHome(t, outside, "decoy", "outside")
	if err := os.Symlink(filepath.Join(outside, "decoy"), filepath.Join(home, "link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(home, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotHome(home, func(*os.Root, string) (homeFile, error) {
		t.Fatal("opened nonregular entry")
		return nil, syscall.EACCES
	})
	if err != nil {
		t.Fatal(err)
	}
	changes, unreadable := CompareHomes(before, after)
	want := []HomeDelta{{Path: "fifo", Kind: "special", Change: "created"}, {Path: "link", Kind: "link", Change: "created"}}
	if unreadable || !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes %v unreadable %v", changes, unreadable)
	}
	if err := CopySeed(home, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("copied hostile seed")
	}
}
func TestCanaryPlanting(t *testing.T) {
	// QP10, QH27: fresh random replacements, one occurrence, access time one day before modification.
	home := t.TempDir()
	putHome(t, home, "credential", "prefix TOKEN123 suffix")
	canary, err := PlantCanary(home, "credential", "TOKEN123")
	if err != nil {
		t.Fatal(err)
	}
	if len(canary) != 41 || !strings.HasPrefix(canary, "pqlcanary") || strings.Trim(canary[9:], "0123456789abcdef") != "" {
		t.Fatal("invalid canary")
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	info, err := root.Stat("credential")
	if err != nil {
		t.Fatal(err)
	}
	// Stat before reading: the verification read itself moves atime.
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("missing stat identity")
	}
	atime := statAccessTime(stat)
	if !atime.Equal(info.ModTime().Add(-24 * time.Hour)) {
		t.Fatalf("atime %v mtime %v", atime, info.ModTime())
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, "credential"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "prefix "+canary+" suffix" {
		t.Fatalf("replacement %q", data)
	}
	for _, value := range []string{"absent", "TOKEN123 TOKEN123"} {
		putHome(t, home, "credential", value)
		if _, err := PlantCanary(home, "credential", "TOKEN123"); err == nil {
			t.Fatal("accepted nonunique token")
		}
	}
	putHome(t, home, "credential", "TOKEN123")
	other, err := PlantCanary(home, "credential", "TOKEN123")
	if err != nil {
		t.Fatal(err)
	}
	if canary == other {
		t.Fatal("reused canary")
	}
}
func TestLeakScan(t *testing.T) {
	// QP12, QP13, QH25: file-only budgets, byte order, names excluded, leaks beat bounds.
	const value = "planted_value_123"
	home := t.TempDir()
	putHome(t, home, "planted", value)
	putHome(t, home, ".pql-tmp/leak", value)
	result, err := ScanHome(home, "planted", homeScanForms(value), nil, []byte(value), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.BoundReached || result.Unreadable || len(result.Leaks) != 2 || result.Leaks[0].Place != "stderr" || result.Leaks[1].Place != "home/.pql-tmp/leak" {
		t.Fatalf("result %+v", result)
	}
	names := t.TempDir()
	putHome(t, names, value, "clean")
	result, err = ScanHome(names, "", homeScanForms(value), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Leaks) != 0 {
		t.Fatal("scanned a name")
	}
	bounds := t.TempDir()
	putHome(t, bounds, "a", "1234")
	putHome(t, bounds, "b", "5678")
	for _, row := range []struct {
		name        string
		file, total int64
		want        bool
	}{{"AtFile", 4, 8, false}, {"PastFile", 3, 8, true}, {"AtTotal", 8, 8, false}, {"PastTotal", 8, 7, true}} {
		t.Run(row.name, func(t *testing.T) {
			r, e := scanHome(bounds, "", homeScanForms(value), nil, nil, false, openHomeFile, row.file, row.total)
			if e != nil {
				t.Fatal(e)
			}
			if r.BoundReached != row.want {
				t.Fatalf("bound %+v", r)
			}
		})
	}
	putHome(t, bounds, "a", value+"tail")
	putHome(t, bounds, "b", value)
	r, err := scanHome(bounds, "", homeScanForms(value), nil, nil, true, openHomeFile, int64(len(value)), int64(len(value)))
	if err != nil {
		t.Fatal(err)
	}
	if !r.BoundReached || len(r.Leaks) != 1 || r.Leaks[0].Place != "home/a" {
		t.Fatalf("leak at bound %+v", r)
	}
	calls := []string{}
	for _, name := range []string{"a", "b"} {
		if err := os.Chmod(filepath.Join(bounds, name), 0); err != nil {
			t.Fatal(err)
		}
	}
	r, err = scanHome(bounds, "", homeScanForms(value), nil, nil, false, func(root *os.Root, name string) (homeFile, error) {
		calls = append(calls, name)
		info, e := root.Lstat(name)
		if e != nil {
			t.Fatal(e)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("scan opener ran before reset")
		}
		return nil, syscall.EACCES
	}, FileScanLimit, TotalScanLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Unreadable || !reflect.DeepEqual(calls, []string{"a", "b"}) {
		t.Fatalf("unreadable %+v calls %v", r, calls)
	}
	if !errors.Is(syscall.EACCES, os.ErrPermission) {
		t.Fatal("fixture error is not permission denied")
	}
}

func TestScanLiteralBounds(t *testing.T) {
	// QP13: the actual 16 MiB and 256 MiB boundaries, with file bytes only.
	home := t.TempDir()
	file := filepath.Join(home, "a")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(16 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := ScanHome(home, "", homeScanForms("unseen_value"), []byte("stream bytes"), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.BoundReached {
		t.Fatal("exact file bound flagged")
	}
	for i := 1; i < 16; i++ {
		name := filepath.Join(home, fmt.Sprintf("file%02d", i))
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(16 << 20); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	r, err = ScanHome(home, "", homeScanForms("unseen_value"), []byte("stream bytes"), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.BoundReached {
		t.Fatal("exact total bound flagged")
	}
	putHome(t, home, "z", "!")
	r, err = ScanHome(home, "", homeScanForms("unseen_value"), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !r.BoundReached {
		t.Fatal("total overflow not flagged")
	}
	if err := os.Truncate(file, (16<<20)+1); err != nil {
		t.Fatal(err)
	}
	r, err = ScanHome(home, "", homeScanForms("unseen_value"), nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !r.BoundReached {
		t.Fatal("file overflow not flagged")
	}
}

func homeScanForms(value string) []ScanForm {
	out := []ScanForm{}
	for _, f := range record.EncodeForms(value) {
		out = append(out, ScanForm{Name: f.Name, Text: f.Text})
	}
	return out
}

func TestStreamScanBounds(t *testing.T) {
	// QP12, QP13: a leak within kept bytes wins; a canary crossing the cut is incomplete.
	home := t.TempDir()
	const value = "canary_boundary_value"
	for _, row := range []struct {
		name        string
		offset      int
		leak, bound bool
	}{{"Within", (1 << 20) - len(value), true, false}, {"Crossing", (1 << 20) - 4, false, true}, {"Past", 1 << 20, false, true}} {
		t.Run(row.name, func(t *testing.T) {
			data := []byte(strings.Repeat("!", row.offset) + value)
			r, err := ScanHome(home, "", homeScanForms(value), nil, data, false)
			if err != nil {
				t.Fatal(err)
			}
			if (len(r.Leaks) > 0) != row.leak || r.BoundReached != row.bound {
				t.Fatalf("got %+v", r)
			}
		})
	}
}
