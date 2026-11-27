//go:build scripted

// Package scripted reads sealed recordings as raw invocation facts.
package scripted

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

// Error carries the exit funnel's classification without exiting the process.
type Error struct {
	Code         int
	Rule, Detail string
}

func (e *Error) RuleID() string        { return e.Rule }
func (e *Error) Error() string         { return fmt.Sprintf("%s: %s", e.Rule, e.Detail) }
func reject(rule, detail string) error { return &Error{Code: 2, Rule: rule, Detail: detail} }

// Source owns validated inputs. The source never launches a process.
type Source struct {
	host      object
	runs      map[string]observe.Run
	overrides map[string]string
	trace     *Trace
}

// Read validates shape and QT4/QT5 relations before serving any facts.
func Read(host, recording []byte, trace io.Writer) (*Source, error) {
	bounds := strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}
	h, ok := strictjson.Read(host, bounds, nil)
	if !ok {
		return nil, reject("QT1", "host JSON refused")
	}
	b, ok := strictjson.Read(recording, bounds, nil)
	if !ok {
		return nil, reject("QT1", "recording JSON refused")
	}
	if !hostShape(h) {
		return nil, reject("QT4", "host shape refused")
	}
	if !recordingShape(b) {
		return nil, reject("QT5", "recording shape refused")
	}
	s := &Source{host: obj(h), runs: map[string]observe.Run{}, overrides: map[string]string{}, trace: NewTrace(trace)}
	for _, value := range arr(obj(b)["invocations"]) {
		invocation := obj(value)
		key := keyID(invocation["key"])
		if _, exists := s.runs[key]; exists {
			return nil, reject("QT5", "duplicate invocation key")
		}
		violating := obj(b)["violating"] == true
		if violating != (invocation["unconfined"] != nil) {
			return nil, reject("QT5", "unconfined control presence differs")
		}
		confined := arr(invocation["confined"])
		all := append(array{}, confined...)
		if invocation["unconfined"] != nil {
			all = append(all, invocation["unconfined"])
		}
		for _, value := range all {
			run := obj(value)
			by := obj(run["end"])["by"]
			marker := obj(run["markers"])["confinement"] == true
			if by == "before_marker" && marker || by == "confinement" && !marker {
				return nil, reject("QT5", "marker contradicts attribution")
			}
		}
		first := obj(confined[0])
		for _, run := range confined[1:] {
			if !agreement(first, obj(run)) {
				return nil, reject("QT5", "confined runs disagree")
			}
		}
		s.runs[key] = readRun(first)
	}
	for _, value := range arr(s.host["kill_attribution"]) {
		entry := obj(value)
		key := keyID(entry["invocation"])
		if _, exists := s.overrides[key]; exists {
			return nil, reject("QT4", "duplicate kill override")
		}
		r, exists := s.runs[key]
		if !exists || r.End.SignalName != "SIGKILL" || r.End.EngineKilled || r.End.MemoryKilled || r.End.CodeSigningKilled || !r.Markers.Confinement {
			return nil, reject("QT4", "override does not name confinement SIGKILL")
		}
		s.overrides[key] = textValue(entry["by"])
	}
	return s, nil
}
func keyID(v any) string { return string(strictjson.Write(v)) }
func agreement(a, b object) bool {
	for _, name := range []string{"markers", "end", "leak_scan", "planted", "decoys", "printed_value"} {
		if !strictjson.EqualJSON(a[name], b[name]) {
			return false
		}
	}
	if !strictjson.EqualJSON(obj(a["home_changes"])["changes"], obj(b["home_changes"])["changes"]) {
		return false
	}
	for _, stream := range []string{"stdout", "stderr"} {
		for _, name := range []string{"bytes", "sha256", "cut"} {
			if !strictjson.EqualJSON(obj(a[stream])[name], obj(b[stream])[name]) {
				return false
			}
		}
	}
	return true
}
func intValue(v any) int64 {
	n, ok := v.(number)
	if !ok {
		return 0
	}
	r, _ := new(big.Rat).SetString(string(n))
	if r.Num().IsInt64() {
		return r.Num().Int64()
	}
	return 0
}
func readEnd(v any) observe.End {
	o := obj(v)
	e := observe.End{Code: int(intValue(o["code"]))}
	if o["kind"] == "signal" {
		e.SignalName = textValue(o["signal"])
		if e.SignalName == "SIGKILL" {
			e.Signal = 9
		} else {
			e.Signal = -1
		}
	}
	switch o["by"] {
	case "budget":
		e.EngineKilled = true
		e.BudgetExpired = true
		e.RunningAtBudget = true
	case "memory":
		e.MemoryKilled = true
	case "code_signing":
		e.CodeSigningKilled = true
	}
	return e
}
func readRun(o object) observe.Run {
	r := observe.Run{End: readEnd(o["end"]), Markers: observe.Markers{Confinement: obj(o["markers"])["confinement"] == true, CouldNotStart: obj(o["markers"])["could_not_start"] == true}}
	for _, stream := range []string{"stdout", "stderr"} {
		v := obj(o[stream])
		d := observe.StreamDetails{Count: json.Number(numberValue(v["bytes"])), Digest: textValue(v["sha256"]), Head: textValue(v["head"]), Kept: v["kept"] != nil}
		if v["kept"] != nil {
			spelling := textValue(v["kept"])
			d.KeptBase64 = &spelling
		}
		b, err := base64.StdEncoding.DecodeString(textValue(v["kept"]))
		if err != nil {
			panic("validated base64: " + err.Error())
		}
		if stream == "stdout" {
			r.Details.Stdout = d
			r.Stdout = b
			r.StdoutSeen = intValue(v["bytes"])
			r.StdoutCut = v["cut"] == true
		} else {
			r.Details.Stderr = d
			r.Stderr = b
			r.StderrSeen = intValue(v["bytes"])
			r.StderrCut = v["cut"] == true
		}
	}
	for _, v := range arr(obj(o["home_changes"])["changes"]) {
		c := obj(v)
		change := observe.HomeChange{Path: textValue(c["path"]), Kind: textValue(c["kind"]), Change: textValue(c["change"])}
		if c["sha256"] != nil {
			d := textValue(c["sha256"])
			change.Digest = &d
		}
		r.Details.Home.Changes = append(r.Details.Home.Changes, change)
	}
	if b, ok := obj(o["home_changes"])["control_detected"].(bool); ok {
		r.Details.Home.ControlDetected = &b
	}
	if p := obj(o["planted"]); p != nil {
		r.Details.Planted = &observe.Planted{AtimeMoved: p["atime_moved"] == true, SameFile: p["same_file"] == true}
		if p["control_end"] != nil {
			e := readEnd(p["control_end"])
			r.Details.Planted.ControlEnd = &observe.InvocationProtocol{End: e, Markers: observe.Markers{Confinement: obj(p["control_end"])["by"] != "before_marker"}}
		}
	}
	l := obj(o["leak_scan"])
	r.Details.LeakScan.BoundReached = l["bound_reached"] == true
	for _, v := range arr(l["leaks"]) {
		o := obj(v)
		r.Details.LeakScan.Leaks = append(r.Details.LeakScan.Leaks, observe.Leak{Place: textValue(o["place"]), Encoding: textValue(o["encoding"])})
	}
	for _, v := range arr(o["decoys"]) {
		d := obj(v)
		r.Details.Decoys = append(r.Details.Decoys, observe.Decoy{Name: textValue(d["name"]), Touched: d["touched"] == true})
	}
	if p := obj(o["printed_value"]); p != nil {
		r.Details.Printed = &observe.PrintedValue{Value: standardValue(p["value"])}
	}
	return r
}

// Controls returns the host's grade-wide control results.
func (s *Source) Controls() (observe.Controls, error) {
	h := s.host
	confinement := obj(h["confinement"])
	c := observe.Controls{ConfinementApplied: confinement["applied"] == true, SystemNamed: true, SelfTestPassed: h["self_test"] == "passed", KnownFastWithin: h["known_fast"] == "within", ReadCalibrationMoved: h["read_calibration"] == "moved", StoreControlDetected: h["store_control"] == "detected", ReadControlKilled: h["read_control"] == "killed", PrivateCopyUnchanged: h["private_copy"] == "unchanged"}
	for _, value := range arr(h["kill_attribution"]) {
		e := obj(value)
		c.KillControls = append(c.KillControls, observe.KillControl{Key: map[string]any(obj(e["invocation"])), Memory: e["by"] == "memory", CodeSigning: e["by"] == "code_signing", Engine: e["by"] == "engine"})
	}
	if confinement["sandbox_exec_exit"] != nil {
		c.SandboxExecExitExact = json.Number(numberValue(confinement["sandbox_exec_exit"]))
		n := int(intValue(confinement["sandbox_exec_exit"]))
		c.SandboxExecExit = &n
	}
	return c, nil
}

// Observe serves exactly the first confined run. Missing keys are engine faults.
func (s *Source) Observe(key observe.Key, _ observe.Spec) (observe.Run, error) {
	id, err := externalKeyID(key)
	if err != nil {
		return observe.Run{}, err
	}
	r, ok := s.runs[id]
	if !ok {
		return observe.Run{}, &Error{Code: 70, Rule: "QT5", Detail: "recording lacks invocation"}
	}
	r.Details.BeforeControls = &observe.InvocationProtocol{End: r.End, Markers: r.Markers}
	switch s.overrides[id] {
	case "memory":
		r.End.MemoryKilled = true
	case "code_signing":
		r.End.CodeSigningKilled = true
	case "engine":
		r.End.EngineKilled = true
	}
	c, err := s.Controls()
	if err != nil {
		return observe.Run{}, err
	}
	r.CopyChanged = !c.PrivateCopyUnchanged
	if !c.ConfinementApplied {
		r.Markers = observe.Markers{}
		r.End = observe.End{Code: *c.SandboxExecExit}
	}
	if err := s.trace.Served(key); err != nil {
		return observe.Run{}, err
	}
	return r, nil
}

func standardValue(v any) any {
	switch x := v.(type) {
	case number:
		return json.Number(x)
	case object:
		o := map[string]any{}
		for k, v := range x {
			o[k] = standardValue(v)
		}
		return o
	case array:
		a := []any{}
		for _, v := range x {
			a = append(a, standardValue(v))
		}
		return a
	default:
		return v
	}
}

func endShape(v any) bool {
	o := obj(v)
	if o["kind"] == "exit" {
		return fields(v, map[string]predicate{"kind": enum("exit"), "code": func(v any) bool { return boundedInteger(v, 0, 255) }})
	}
	if !fields(v, map[string]predicate{"kind": enum("signal"), "signal": pattern(`^SIG[A-Z0-9]+$`), "by": nullable(stringValue)}) {
		return false
	}
	if o["signal"] == "SIGKILL" {
		return enum("confinement", "budget", "memory", "code_signing", "before_marker")(o["by"])
	}
	return o["by"] == nil
}
