package confine

import "testing"

func TestSystemExecutable(t *testing.T) {
	// QH8, E-55: only canonical paths below the two system directories run in place.
	for _, row := range []struct {
		name, path string
		want       bool
	}{
		{"Bin", "/bin/echo", true},
		{"UsrBin", "/usr/bin/true", true},
		{"Relative", "computations/value", false},
		{"Other", "/usr/local/bin/value", false},
		{"Escape", "/bin/../other/value", false},
		{"Prefix", "/usr/binary/value", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if systemExecutable(row.path) != row.want {
				t.Fatal(row.path)
			}
		})
	}
}
