package observe

import (
	"testing"
	"time"
)

func TestSystemNamed(t *testing.T) {
	// QH1, QH2: U35's prefix cases must not widen the named product versions.
	for _, row := range []struct {
		name    string
		product string
		want    bool
	}{
		{"NamedFirst", "27.0.1", true},
		{"NamedSecond", "27.2", true},
		{"FirstPrefix", "27.0.10", false},
		{"SecondPrefix", "27.20", false},
		{"Patch", "27.2.1", false},
		{"Earlier", "27.0", false},
		{"Later", "27.3", false},
		{"Build", "26B123", false},
		{"Empty", "", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if SystemNamed(row.product) != row.want {
				t.Fatal(row.product)
			}
		})
	}
}
func TestPlantedFacts(t *testing.T) {
	// QH27: identity includes both device and inode, independently of access time.
	before := FileFacts{Access: time.Unix(1, 0), Device: 1, Inode: 2}
	for _, row := range []struct {
		name  string
		after FileFacts
		want  Planted
	}{
		{"Unchanged", before, Planted{SameFile: true}},
		{"Read", FileFacts{Access: time.Unix(2, 0), Device: 1, Inode: 2}, Planted{AtimeMoved: true, SameFile: true}},
		{"Device", FileFacts{Access: time.Unix(2, 0), Device: 3, Inode: 2}, Planted{AtimeMoved: true}},
		{"Inode", FileFacts{Access: time.Unix(2, 0), Device: 1, Inode: 3}, Planted{AtimeMoved: true}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := PlantedFacts(before, row.after); got != row.want {
				t.Fatalf("%+v", got)
			}
		})
	}
}
