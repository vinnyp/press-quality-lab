package observe

import "encoding/json"

// Key is QT5's invocation key. A source must not interpret it as a decision.
type Key = map[string]any

// Spec supplies the immutable planned context for one invocation.
type Spec struct {
	// BudgetSeconds supplies discovery's fixed QD2 budget; zero uses planned context.
	BudgetSeconds int
	CheckID       string
	Preimage      map[string]any
}

// Controls are grade-wide facts. False SystemNamed includes an unverified
// system, not just a system detected and found unsupported.
type Controls struct {
	SandboxExecExitExact json.Number
	KillControls         []KillControl
	ConfinementApplied   bool
	SandboxExecExit      *int
	SelfTestPassed       bool
	SystemNamed          bool
	KnownFastWithin      bool
	ReadCalibrationMoved bool
	StoreControlDetected bool
	ReadControlKilled    bool
	PrivateCopyUnchanged bool
}

// Invocation pairs raw facts with the key of a run that actually happened.
// An absent invocation is not a failed invocation.
type Invocation struct {
	Key Key
	Run Run
}

// Source supplies facts only. Errors denote engine faults, never observations.
// Observe is called only when sequencing reaches the invocation it serves.
type Source interface {
	Controls() (Controls, error)
	Observe(Key, Spec) (Run, error)
}

// ProjectControls projects the control shapes defined here into QT4's spelling.
// Kill overrides and the run-end attribution projection belong to U24/U6.
func ProjectControls(c Controls) map[string]any {
	var exit any
	if c.SandboxExecExit != nil {
		exit = *c.SandboxExecExit
	}
	choose := func(b bool, yes, no string) string {
		if b {
			return yes
		}
		return no
	}
	return map[string]any{
		"confinement":      map[string]any{"applied": c.ConfinementApplied, "sandbox_exec_exit": exit},
		"self_test":        choose(c.SelfTestPassed, "passed", "failed"),
		"known_fast":       choose(c.KnownFastWithin, "within", "over"),
		"read_calibration": choose(c.ReadCalibrationMoved, "moved", "not_moved"),
		"store_control":    choose(c.StoreControlDetected, "detected", "not_detected"),
		"read_control":     choose(c.ReadControlKilled, "killed", "not_killed"),
		"private_copy":     choose(c.PrivateCopyUnchanged, "unchanged", "changed"),
	}
}

// ProjectMarkers projects the protocol facts after removal of protocol bytes.
func ProjectMarkers(m Markers) map[string]any {
	return map[string]any{"confinement": m.Confinement, "could_not_start": m.CouldNotStart}
}

// KillControl holds QT4's per-invocation control result without attribution values.
type KillControl struct {
	Key                         Key
	Memory, CodeSigning, Engine bool
}
