// Package observe holds invocation facts and untagged derivations, without verdicts.
package observe

import "bytes"

// End records the system's end and the engine's own actions, before attribution.
type End struct {
	Code            int
	Signal          int
	ExitDetail      uint32
	EngineKilled    bool
	BudgetExpired   bool
	RunningAtBudget bool
}

// Run holds invocation facts after protocol removal. Seen counts exclude
// protocol bytes; Cut records whether the interim capture discarded bytes.
type Run struct {
	End                    End
	Stdout, Stderr         []byte
	Markers                Markers
	CopyChanged            bool
	StdoutSeen, StderrSeen int64
	StdoutCut, StderrCut   bool
}

type Markers struct{ Confinement, CouldNotStart bool }

// ParseMarkers accepts only complete, adjacent protocol lines with the expected
// invocation value. Prefixes come from the launcher's shared protocol constants.
func ParseMarkers(stderr []byte, marker, failure string) (Markers, []byte) {
	var seen Markers
	first := []byte(marker + "\n")
	if !bytes.HasPrefix(stderr, first) {
		return seen, stderr
	}
	seen.Confinement = true
	stderr = stderr[len(first):]
	second := []byte(failure + "\n")
	if bytes.HasPrefix(stderr, second) {
		seen.CouldNotStart = true
		stderr = stderr[len(second):]
	}
	return seen, stderr
}

// CopySnapshot is the identity checked before each private-copy use (QH8).
// Times are split into seconds and nanoseconds to preserve system precision.
type CopySnapshot struct {
	Entries                                            []string
	Inode                                              uint64
	Mode                                               uint32
	Size                                               int64
	MtimeSeconds, MtimeNanos, CtimeSeconds, CtimeNanos int64
	Digest                                             [32]byte
}

// SameCopy compares every required property, including the directory entries.
func SameCopy(a, b CopySnapshot) bool {
	if len(a.Entries) != len(b.Entries) {
		return false
	}
	for i := range a.Entries {
		if a.Entries[i] != b.Entries[i] {
			return false
		}
	}
	return a.Inode == b.Inode && a.Mode == b.Mode && a.Size == b.Size && a.MtimeSeconds == b.MtimeSeconds && a.MtimeNanos == b.MtimeNanos && a.CtimeSeconds == b.CtimeSeconds && a.CtimeNanos == b.CtimeNanos && a.Digest == b.Digest
}
