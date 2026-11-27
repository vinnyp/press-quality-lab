package observe

import (
	"encoding/base64"
	"strconv"

	"encoding/json"
)

// StreamDetails retains the count, digest, display head and kept-byte presence.
// Count retains integers larger than a machine word without rounding.
type StreamDetails struct {
	Count        json.Number
	Digest, Head string
	Kept         bool
	KeptBase64   *string
}
type HomeChange struct {
	Path, Kind, Change string
	Digest             *string
}
type HomeChanges struct {
	Changes         []HomeChange
	ControlDetected *bool
}
type Planted struct {
	AtimeMoved, SameFile bool
	ControlEnd           *InvocationProtocol
}
type Leak struct{ Place, Encoding string }
type LeakScan struct {
	BoundReached bool
	Leaks        []Leak
}
type Decoy struct {
	Name    string
	Touched bool
}

// PrintedValue distinguishes a printed JSON null from an invalid output.
type PrintedValue struct{ Value any }

// InvocationProtocol retains the invocation itself before a control replaces its result.
type InvocationProtocol struct {
	End     End
	Markers Markers
}
type RunDetails struct {
	BeforeControls *InvocationProtocol
	Stdout, Stderr StreamDetails
	Home           HomeChanges
	Planted        *Planted
	LeakScan       LeakScan
	Decoys         []Decoy
	Printed        *PrintedValue
}

// ProjectEnd derives QT5's spelling from raw termination facts.
func ProjectEnd(e End, markers Markers) map[string]any {
	if e.Signal == 0 && e.SignalName == "" {
		return map[string]any{"kind": "exit", "code": e.Code}
	}
	name := e.SignalName
	if name == "" && e.Signal == 9 {
		name = "SIGKILL"
	}
	var by any
	if name == "SIGKILL" {

		memory, signing := DecodeExitDetail(e.ExitDetail)
		switch {
		case !markers.Confinement:
			by = "before_marker"
		case e.EngineKilled:
			if e.BudgetExpired && e.RunningAtBudget {
				by = "budget"
			}
		case e.MemoryKilled || memory:
			by = "memory"
		case e.CodeSigningKilled || signing:
			by = "code_signing"
		case !e.BudgetExpired:
			by = "confinement"

		}
	}
	return map[string]any{"kind": "signal", "signal": name, "by": by}
}
func projectStream(d StreamDetails, b []byte, seen int64, cut bool) map[string]any {
	count := d.Count
	if count == "" {
		count = json.Number(strconv.FormatInt(seen, 10))
	}
	var kept any
	if d.Kept {
		kept = base64.StdEncoding.EncodeToString(b)
		if d.KeptBase64 != nil {
			kept = *d.KeptBase64
		}
	}
	return map[string]any{"bytes": count, "sha256": d.Digest, "head": d.Head, "cut": cut, "kept": kept}
}

// ProjectRun reconstructs QT5's run shape from facts, before host overrides.
func ProjectRun(r Run) map[string]any {
	d := r.Details
	if d.BeforeControls != nil {
		r.End = d.BeforeControls.End
		r.Markers = d.BeforeControls.Markers
	}
	changes := []any{}
	for _, c := range d.Home.Changes {
		var digest any
		if c.Digest != nil {
			digest = *c.Digest
		}
		changes = append(changes, map[string]any{"path": c.Path, "kind": c.Kind, "change": c.Change, "sha256": digest})
	}
	var control any
	if d.Home.ControlDetected != nil {
		control = *d.Home.ControlDetected
	}
	var planted any
	if p := d.Planted; p != nil {
		var end any
		if p.ControlEnd != nil {
			end = ProjectEnd(p.ControlEnd.End, p.ControlEnd.Markers)
		}
		planted = map[string]any{"atime_moved": p.AtimeMoved, "same_file": p.SameFile, "control_end": end}
	}
	leaks := []any{}
	for _, l := range d.LeakScan.Leaks {
		leaks = append(leaks, map[string]any{"place": l.Place, "encoding": l.Encoding})
	}
	decoys := []any{}
	for _, d := range d.Decoys {
		decoys = append(decoys, map[string]any{"name": d.Name, "touched": d.Touched})
	}
	var printed any
	if d.Printed != nil {
		printed = map[string]any{"value": d.Printed.Value}
	}
	end := ProjectEnd(r.End, r.Markers)
	return map[string]any{
		"markers":       map[string]any{"confinement": r.Markers.Confinement, "could_not_start": r.Markers.CouldNotStart},
		"end":           end,
		"stdout":        projectStream(d.Stdout, r.Stdout, r.StdoutSeen, r.StdoutCut),
		"stderr":        projectStream(d.Stderr, r.Stderr, r.StderrSeen, r.StderrCut),
		"home_changes":  map[string]any{"changes": changes, "control_detected": control},
		"planted":       planted,
		"leak_scan":     map[string]any{"bound_reached": d.LeakScan.BoundReached, "leaks": leaks},
		"decoys":        decoys,
		"printed_value": printed,
	}
}

// ProjectHost projects every host control, including per-invocation controls.
func ProjectHost(c Controls) map[string]any {
	p := ProjectControls(c)
	o := map[string]any{}
	for k, v := range p {
		o[k] = v
	}
	var exit any
	if c.SandboxExecExitExact != "" {
		exit = c.SandboxExecExitExact
	} else if c.SandboxExecExit != nil {
		exit = *c.SandboxExecExit
	}
	o["confinement"] = map[string]any{"applied": c.ConfinementApplied, "sandbox_exec_exit": exit}
	kills := []any{}
	for _, k := range c.KillControls {
		by := "engine"
		if k.Memory {
			by = "memory"
		} else if k.CodeSigning {
			by = "code_signing"
		}
		kills = append(kills, map[string]any{"invocation": map[string]any(k.Key), "by": by})
	}
	o["kill_attribution"] = kills
	return o
}
