package grade

import (
	"strings"
	"testing"
)

func TestUnameFields(t *testing.T) {
	// QE9, KTD8: retain complete values, taking only uname version's first token.
	for _, row := range []struct {
		name, release, version, arch string
		overflow                     bool
	}{
		{"Normal", "6.1", "#1 SMP", "x86_64", false},
		{"LongVersion", "6.1", "#1 " + strings.Repeat("long ", 20), "x86_64", false},
		{"ReleaseOverflow", strings.Repeat("r", 33), "#1 SMP", "x86_64", true},
		{"BuildOverflow", "6.1", strings.Repeat("b", 33), "x86_64", true},
		{"ArchOverflow", "6.1", "#1 SMP", strings.Repeat("a", 33), true},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := parseUname(row.release, row.version, row.arch)
			if row.overflow {
				if err == nil {
					t.Fatal("accepted overlong host")
				}
				return
			}
			if err != nil || got["os_version"] != row.release || got["os_build"] != "#1" || got["arch"] != row.arch {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}
