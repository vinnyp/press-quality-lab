//go:build !darwin

package confine

import "testing"

func TestUnsupportedCopies(t *testing.T) {
	if copies, err := NewCopies("unused", "unused", "unused"); err == nil || copies != nil {
		t.Fatalf("copies=%v error=%v", copies, err)
	}
	if process, err := (&Copies{}).Start(Invocation{}); err == nil || process != nil {
		t.Fatalf("process=%v error=%v", process, err)
	}
}
