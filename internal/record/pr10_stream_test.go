package record

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPR10StreamObjects(t *testing.T) {
	// QE10, QE18: exact keys, raw digest and counts, masked bytes before head.
	for _, row := range []struct {
		name string
		raw  []byte
	}{
		{"BigNumber", []byte("123456789012345678901234567890123")},
		{"InvalidUTF8", bytes.Repeat([]byte{0xff}, 64<<10)},
		{"SeventeenMB", bytes.Repeat([]byte("a"), 17<<20)},
		{"HeadEdge", []byte(strings.Repeat("!", (64<<10)-4) + "éééé" + "tail")},
		{"DigestEdge", []byte(strings.Repeat("!", (1<<20)-4) + "éééé" + "tail")},
	} {
		t.Run(row.name, func(t *testing.T) {
			raw := append([]byte(nil), row.raw...)
			m := NewMasker(nil, []string{"éééé"})
			kept := raw[:min(len(raw), 1<<20)]
			masked := strings.ReplaceAll(string(raw), "éééé", "****")
			head := masked[:min(len(masked), 64<<10)]
			var decoded strings.Builder
			for len(head) > 0 {
				r, n := utf8.DecodeRuneInString(head)
				decoded.WriteRune(r)
				head = head[n:]
			}
			want := Object{"bytes": int64(len(raw)), "sha256": fmt.Sprintf("%x", sha256.Sum256(kept)), "cut": len(raw) > 1<<20, "head": decoded.String()}
			got := Stream(raw, int64(len(raw)), len(raw) > 1<<20, observe.StreamDetails{}, m)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("stream differs: got keys %d bytes %v head length %d", len(got), got["bytes"], len(fmt.Sprint(got["head"])))
			}
			if !bytes.Equal(raw, row.raw) {
				t.Fatal("raw bytes changed")
			}
		})
	}
}

func TestPR10RecordedHead(t *testing.T) {
	// QE10, QT5: a cut recording's head is already decoded; do not cut it again.
	head := strings.Repeat("\ufffd", 64<<10)
	d := observe.StreamDetails{Count: "17825792", Digest: strings.Repeat("a", 64), Head: head}
	got := Stream(nil, 17<<20, true, d, nil)
	want := Object{"bytes": int64(17 << 20), "sha256": strings.Repeat("a", 64), "cut": true, "head": head}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("decoded recording head was changed")
	}
}
