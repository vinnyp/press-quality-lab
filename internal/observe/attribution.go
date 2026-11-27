package observe

import "fmt"

// Attribution describes an invocation's end, without deciding its check.
type Attribution uint8

const (
	Unconfined Attribution = iota
	CouldNotStart
	Exited
	ConfinementKill
	BudgetKill
	EngineKill
	Crashed
)

// DecodeExitDetail reads Darwin's NOTE_EXIT_MEMORY and NOTE_EXIT_CSERROR bits.
// The raw detail remains in End for the evidence record.
func DecodeExitDetail(detail uint32) (memory, codeSigning bool) {
	return detail&0x20000 != 0, detail&0x40000 != 0
}

// AfterMarker has exactly one source: the launcher's protocol facts.
func (r Run) AfterMarker() bool { return r.Markers.Confinement }

// Attribute applies QH6, QH7, QH21 and QO2 to QO5 to raw facts. Unsupported
// combinations are engine faults rather than successful observations.
func Attribute(r Run) (Attribution, error) {
	e := r.End
	if !r.AfterMarker() { // QH6, E-28: precedes even a could-not-start marker.
		return Unconfined, nil
	}
	memory, signing := DecodeExitDetail(e.ExitDetail)
	memory = memory || e.MemoryKilled
	signing = signing || e.CodeSigningKilled
	signal := e.Signal != 0 || e.SignalName != ""
	kill := e.Signal == 9 || e.SignalName == "SIGKILL"
	if e.Signal != 0 && e.SignalName != "" && (e.Signal == 9) != (e.SignalName == "SIGKILL") || e.RunningAtBudget && !e.BudgetExpired || (!kill && (memory || signing)) || (!signal && e.ExitDetail != 0) {
		return 0, fmt.Errorf("QH21: inconsistent invocation end")
	}
	if r.Markers.CouldNotStart { // QH7, QO2: never infer this from the exit code.
		return CouldNotStart, nil
	}
	// Darwin can send a group kill after the leader exits between its state
	// lookup and the signal. Wait4's exit status is still the program's (QO5).
	if !signal {
		return Exited, nil
	}
	if !kill { // QO5.
		return Crashed, nil
	}
	if e.EngineKilled {
		if e.BudgetExpired { // QO4 also covers no longer running at the budget.
			return BudgetKill, nil
		}
		return EngineKill, nil // QO5.
	}
	if memory || signing { // QO5.
		return Crashed, nil
	}
	if !e.BudgetExpired { // QH21.
		return ConfinementKill, nil
	}
	return Crashed, nil // QO5: QH21 excludes a kill at the budget.
}
