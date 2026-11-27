package collect

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestFixedHelpArgumentLimit(t *testing.T) {
	// E-34, QD5: count UTF-8 bytes and one byte for every request word, including --help.
	for _, row := range []struct {
		name                    string
		ancestor, parent, child string
		calls                   int
		unreadable              bool
	}{
		{"Exact", "", "", strings.Repeat("a", 131064), 2, false},
		{"PlusOne", "", "", strings.Repeat("a", 131065), 1, true},
		{"Ancestor", strings.Repeat("a", 131060), "", "tiny", 2, true},
		// 70,001 + 61,061 + 5 + 7 (--help) = 131,074 bytes: tiny is never requested.
		{"TwoAncestors", strings.Repeat("a", 70000), strings.Repeat("b", 61060), "tiny", 3, true},
		{"UTF8", "", "", strings.Repeat("é", 65532), 2, false},
		{"UTF8PlusOne", "", "", strings.Repeat("é", 65532) + "a", 1, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, d, s := resolveWalk(t, discoveryCheck(), func(w plan.Array) observe.Run {
				if len(w) == 0 && row.ancestor != "" {
					return help("Commands:\n  " + row.ancestor + "  desc")
				}
				if len(w) == 1 && row.parent != "" {
					return help("Commands:\n  " + row.parent + "  desc")
				}
				if len(w) == 2 && row.parent != "" {
					return help("Commands:\n  " + row.child + "  desc")
				}
				if len(w) == 0 || row.ancestor != "" && len(w) == 1 {
					return help("Commands:\n  " + row.child + "  desc")
				}
				return help("leaf")
			})
			if len(s.keys) != row.calls || d.Unreadable != row.unreadable {
				t.Fatalf("calls %d unreadable %t", len(s.keys), d.Unreadable)
			}
		})
	}
}

func TestDeclaredPrefixAllocation(t *testing.T) {
	// QD2: undeclared child lookup must not encode its entire ancestor path per child.
	ancestor := strings.Repeat("a", 120000)
	var b strings.Builder
	b.WriteString("Commands:\n")
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "  c%d  desc\n", i)
	}
	text := b.String()
	ch := discoveryCheck(ancestor + " declared")
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, d, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
		if len(w) == 0 {
			return help("Commands:\n  " + ancestor + "  desc")
		}
		if len(w) == 1 {
			return help(text)
		}
		return help("leaf")
	})
	runtime.ReadMemStats(&after)
	// At most 100,000 allocation operations; the fixed walk measured 78,914.
	if n := after.Mallocs - before.Mallocs; n > 100000 {
		t.Fatalf("child lookup used %d allocation operations, limit 100000", n)
	}
	if d.FoundCount != 5001 || !d.Missing || !d.Undeclared {
		t.Fatal("incorrect declaration lookup")
	}
	if n := after.TotalAlloc - before.TotalAlloc; n > 128<<20 {
		t.Fatalf("child lookup allocated %d bytes, limit 128 MiB", n)
	}
}

func TestDiscoveryDeepWideMemory(t *testing.T) {
	// QD2, QE10, E-34: this input retains about 260 KB of valid pending sibling names
	// per level, to depth 64. Measured on darwin/arm64: peak HeapInuse
	// 18,112,512 bytes, baseline 991,232. The 32 MiB ceiling covers only this input.
	// The stated worst case is about 1 MiB of names per level, each under 128 KiB;
	// the reviewer measured about 66 MiB live for that input.
	runtime.GC()
	var before, live runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak uint64
	_, d, source := resolveWalk(t, discoveryCheck(), func(w plan.Array) observe.Run {
		runtime.GC()
		runtime.ReadMemStats(&live)
		if live.HeapInuse > peak {
			peak = live.HeapInuse
		}
		if live.HeapInuse > before.HeapInuse+32<<20 {
			t.Fatalf("live heap %d exceeds baseline %d plus 32 MiB", live.HeapInuse, before.HeapInuse)
		}
		var b strings.Builder
		b.WriteString("Commands:\n  a  desc\n")
		for i := 0; i < 4; i++ {
			fmt.Fprintf(&b, "  %d_%d_%s  desc\n", len(w), i, strings.Repeat("x", 65000))
		}
		return help(b.String())
	})
	t.Logf("peak HeapInuse=%d baseline=%d", peak, before.HeapInuse)
	if !d.Bound || d.Unreadable || len(source.keys) != 65 {
		t.Fatal(d.Bound, d.Unreadable, len(source.keys))
	}
}
