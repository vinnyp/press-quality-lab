package throwaway

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vinnyp/press-quality-lab/internal/observe"
)

func TestPlantedSnapshotOrder(t *testing.T) {
	// QH27: the supplied planting digest avoids a content read after backdating.
	home := t.TempDir()
	if err := WriteHomeFiles(home, []NamedFile{{Name: "token", Data: []byte("secret-token")}}); err != nil {
		t.Fatal(err)
	}
	digest, err := PlantGivenCanary(home, "token", "secret-token", "pqlcanary11111111111111111111111111111111")
	if err == nil && digest != "2fcbcf78534527eae8ae92604f3c71049f723091a0e0148b457ed6f6ec523ebc" {
		t.Fatalf("planted bytes digest %s", digest)
	}
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	reads := 0
	hooks := defaultHomeHooks(func(root *os.Root, name string) (homeFile, error) {
		reads++
		now = now.Add(time.Second)
		return openHomeFile(root, name)
	})
	hooks.now = func() time.Time { return now }
	snap, err := snapshotHomeKnown(home, hooks, "token", digest)
	if err != nil || snap.Unreadable || len(snap.Entries) != 1 || snap.Entries[0].Digest != digest || reads != 0 {
		t.Fatalf("snapshot %+v reads %d error %v", snap, reads, err)
	}
	before, err := BackdatePlanted(home, "token")
	if err != nil {
		t.Fatal(err)
	}
	// A reader after this point represents the first read that consumes atime.
	hooks.open = func(*os.Root, string) (homeFile, error) { reads++; return nil, errors.New("read after backdate") }
	snap, err = snapshotHomeKnown(home, hooks, "token", digest)
	after, statErr := InspectFile(filepath.Join(home, "token"))
	if err != nil || statErr != nil || snap.Unreadable || reads != 0 || !after.Access.Equal(before.Access) {
		t.Fatalf("after backdate %+v %d %v %v", snap, reads, err, statErr)
	}
	// Twin without a supplied digest proves the fake reader is active.
	snap, err = snapshotHomeWith(home, hooks)
	if err != nil || !snap.Unreadable || reads != 1 {
		t.Fatalf("reader twin %+v %d %v", snap, reads, err)
	}
}
func TestSwappedPlantedFile(t *testing.T) {
	// QH27: a byte-identical replacement fails identity, beside its unchanged twin.
	home := t.TempDir()
	if err := WriteHomeFiles(home, []NamedFile{{Name: "token", Data: []byte("same")}, {Name: "replacement", Data: []byte("same")}}); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(home, "token")
	before, err := InspectFile(name)
	if err != nil {
		t.Fatal(err)
	}
	after, err := InspectFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if got := observe.PlantedFacts(before, after); !got.SameFile || got.AtimeMoved {
		t.Fatal(got)
	}
	if err := os.Chtimes(filepath.Join(home, "replacement"), before.Access, before.Modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(home, "replacement"), name); err != nil {
		t.Fatal(err)
	}
	after, err = InspectFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if got := observe.PlantedFacts(before, after); got.SameFile || got.AtimeMoved {
		t.Fatal(got)
	}
}
func TestStoreCopyControl(t *testing.T) {
	// QP9: comparator control detects the append and rejects a no-append twin.
	for _, appendByte := range []bool{true, false} {
		home := t.TempDir()
		if err := WriteHomeFiles(home, []NamedFile{{Name: "store", Data: []byte("x")}}); err != nil {
			t.Fatal(err)
		}
		before, err := SnapshotHome(home)
		if err != nil {
			t.Fatal(err)
		}
		if appendByte {
			if err := AppendStoreControl(home, "store"); err != nil {
				t.Fatal(err)
			}
		}
		after, err := SnapshotHome(home)
		if err != nil {
			t.Fatal(err)
		}
		changes, unreadable := CompareHomes(before, after)
		want := 0
		if appendByte {
			want = 1
		}
		if unreadable || len(changes) != want {
			t.Fatalf("%+v %v", changes, unreadable)
		}
		if appendByte && (changes[0].Path != "store" || changes[0].Kind != "file" || changes[0].Change != "modified") {
			t.Fatal(changes)
		}
	}
}
