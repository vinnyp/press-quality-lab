package observe

import "testing"

func TestPR10ExactCountBounds(t *testing.T) {
	// QE8, QE10: counts and byte-bound sums fit the judge's exact integer range.
	for _, r := range []Run{{StdoutSeen: MaxCount}, {StdoutSeen: MaxCount - 1, StderrSeen: 1}} {
		if err := ValidateCounts(r, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Run{{StdoutSeen: MaxCount + 1}, {StderrSeen: MaxCount + 1}, {StdoutSeen: MaxCount, StderrSeen: 1}} {
		if err := ValidateCounts(r, r.StdoutSeen == MaxCount); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
	if err := ValidateCounts(Run{StdoutSeen: MaxCount, StderrSeen: 1}, false); err != nil {
		t.Fatal(err)
	}
}
