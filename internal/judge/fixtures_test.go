package judge

import (
	"bytes"
	"fmt"
	"testing"
)

var fixtureDimensions = []string{"availability", "execution", "graceful-failure", "robustness", "response-correctness", "determinism", "cli-mcp-parity", "provenance-honesty", "completeness-signaling", "output-conformance", "flag-conformance", "bounding", "discoverability", "error-message-quality", "token-economy", "side-effect-integrity", "idempotency", "destructive-guard", "pii-secret-hygiene", "steering", "tool-description-efficacy"}

type fixture struct{ in, want object }

func baseInput() object {
	return object{"request": "grade", "scope": array{"conformance"}, "claimed_tier": "live", "subject_facts": object{"read_only": false, "has_destructive_ops": true, "store_backed": true, "has_cli": true, "bounding": "declared"}, "required": array{}, "evidence": array{}, "trials": array{}, "markers": array{}, "baseline": nil, "retired": array{}, "accepted_regressions": array{}}
}
func required(id, kind, d string) object {
	return object{"id": id, "kind": kind, "dimension": d, "record": id, "crediting": true, "empty_satisfiable": false, "asserts_emptiness": false, "precondition": nil, "declared_count": nil, "condition": nil, "condition_a": nil}
}
func evidence(id string) object {
	return object{"check_id": id, "outcome": "held", "reason": nil, "failure_kind": nil, "target_size": number("1"), "precondition_observed": true, "tier": "offline", "expected": nil, "observed": nil, "targets": array{object{"kind": "invocation", "id": "sample", "tier": "offline"}}, "evaluator": object{"kind": "deterministic", "model": nil}}
}
func trial(id, arm, condition string, index int) object {
	var a any
	if arm != "" {
		a = arm
	}
	return object{"check_id": id, "arm": a, "condition": condition, "trial_index": number(fmt.Sprint(index)), "declared_count": number("16"), "outcome": "held", "errored": false, "tier": "live", "model": "sample-model", "sampling": object{}, "framing": "sample-frame", "evaluator": object{"kind": "deterministic", "model": nil}, "grade": nil}
}
func baselineEntry(id, kind, d, result string) object {
	e := object{"check_id": id, "kind": kind, "dimension": d, "result": result, "rulebook_version": "1.0", "tier": "offline", "condition": nil, "k": nil, "n": nil}
	if kind != "plain" {
		e["tier"] = "live"
	}
	if kind == "prompt" {
		e["condition"] = "current"
		e["k"] = number("16")
		e["n"] = number("16")
	}
	return e
}
func expectedBase(steering bool) object {
	o := object{"rulebook_version": "1.0", "outcome": "verdict", "rejected_by": nil, "exit_code": 0, "scope": array{"conformance"}, "verdict": object{"token": "PASSED", "sub_label": nil, "plain": "green"}, "own_verdict": nil, "counts": object{"xfail": 0, "xpass": 0}, "stale_markers": array{}, "checks": array{}, "dimensions": array{}, "groups": array{}, "comparison": object{"result": "none", "rule": B1, "next_action": nil, "entries": array{}, "new": array{}}, "set_aside": array{}, "earlier_pools": array{}, "anchor": nil}
	if steering {
		o["scope"] = array{"steering"}
	}
	for _, d := range fixtureDimensions {
		state, rule, reason := "GREEN", D7, any(nil)
		out := d == "cli-mcp-parity" || d == "destructive-guard" || d == "steering" || d == "tool-description-efficacy"
		if steering {
			out = d != "steering"
		}
		if out {
			state, rule, reason = "OUT-OF-SCOPE", D0, "scope_not_requested"
			if d == "steering" || d == "tool-description-efficacy" {
				reason = "live_not_run"
			}
		}
		o["dimensions"] = append(arr(o["dimensions"]), object{"dimension": d, "state": state, "rule": rule, "reason": reason})
	}
	for i, g := range []string{"Operability", "Correctness", "Conformance", "Usability", "Safety", "Steering"} {
		s, r := "GREEN", G5
		if steering && i != 5 || !steering && i == 5 {
			s, r = "OUT-OF-SCOPE", G1
		}
		o["groups"] = append(arr(o["groups"]), object{"group": g, "state": s, "rule": r})
	}
	return o
}
func expectedCheck(id, result string, rule Rule, reason any) object {
	return object{"id": id, "result": result, "rule": rule, "reason": reason, "detail": nil, "k": nil, "n": nil, "to_run": nil, "lower": nil, "upper": nil, "d_lo": nil, "d_hi": nil}
}
func conformanceFixture() fixture {
	f := fixture{baseInput(), expectedBase(false)}
	for _, d := range fixtureDimensions {
		if d == "cli-mcp-parity" || d == "destructive-guard" || d == "steering" || d == "tool-description-efficacy" {
			continue
		}
		id := d
		f.in["required"] = append(arr(f.in["required"]), required(id, "plain", d))
		f.in["evidence"] = append(arr(f.in["evidence"]), evidence(id))
		f.want["checks"] = append(arr(f.want["checks"]), expectedCheck(id, "held_crediting", C7, nil))
	}
	sortObjects(arr(f.want["checks"]), "id")
	return f
}
func steeringFixture() fixture {
	f := fixture{baseInput(), expectedBase(true)}
	f.in["scope"] = array{"steering"}
	c := required("prompt", "prompt", "steering")
	c["declared_count"] = number("16")
	c["condition"] = "current"
	f.in["required"] = array{c}
	for i := 0; i < 16; i++ {
		f.in["trials"] = append(arr(f.in["trials"]), trial("prompt", "", "current", i))
	}
	o := expectedCheck("prompt", "held_crediting", P3, nil)
	o["k"] = int64(16)
	o["n"] = int64(16)
	o["lower"] = "0.806392"
	o["upper"] = "1.000000"
	f.want["checks"] = array{o}
	return f
}
func find(a array, key, value string) object {
	for _, v := range a {
		o := obj(v)
		if str(o[key]) == value {
			return o
		}
	}
	panic("missing fixture entry " + value)
}
func (f fixture) c(id string) object   { return find(arr(f.in["required"]), "id", id) }
func (f fixture) e(id string) object   { return find(arr(f.in["evidence"]), "check_id", id) }
func (f fixture) out(id string) object { return find(arr(f.want["checks"]), "id", id) }
func (f fixture) dim(d, state string, rule Rule, reason any) {
	o := find(arr(f.want["dimensions"]), "dimension", d)
	o["state"] = state
	o["rule"] = rule
	o["reason"] = reason
}
func (f fixture) group(g, state string, rule Rule) {
	o := find(arr(f.want["groups"]), "group", g)
	o["state"] = state
	o["rule"] = rule
}
func (f fixture) runVerdict(token string, sub any, plain string, code int) {
	f.want["verdict"] = object{"token": token, "sub_label": sub, "plain": plain}
	f.want["exit_code"] = code
}
func (f fixture) gap(d, g string, rule Rule, reason any) {
	f.dim(d, "HARNESS", rule, reason)
	f.group(g, "GAPPED", G3)
	f.runVerdict("PROVISIONAL", "gaps", "not_measured", 4)
}
func (f fixture) red(d, g string) {
	f.dim(d, "RED", D1, nil)
	f.group(g, "RED", G2)
	f.runVerdict("FAILED", nil, "red", 1)
}
func (f fixture) mark(id, d string) {
	f.in["markers"] = append(arr(f.in["markers"]), object{"record": id, "dimension": d})
}
func assertFixture(t *testing.T, f fixture) { t.Helper(); assertBytes(t, write(f.in), f.want) }
func assertBytes(t *testing.T, input []byte, want object) {
	t.Helper()
	output, code, defect := Safe(Judge, input)
	if defect != nil {
		t.Fatal(defect)
	}
	if !bytes.Equal(output, write(want)) {
		t.Fatalf("document differs\n got %s\nwant %s", output, write(want))
	}
	wantCode, ok := want["exit_code"].(int)
	if !ok || code != wantCode {
		t.Fatalf("exit=%d want=%v", code, want["exit_code"])
	}
}
func rejectedExpected(rule Rule) object {
	return object{"rulebook_version": "1.0", "outcome": "rejected", "rejected_by": rule, "exit_code": 2, "scope": nil, "verdict": nil, "own_verdict": nil, "counts": nil, "stale_markers": array{}, "checks": array{}, "dimensions": array{}, "groups": array{}, "comparison": nil, "set_aside": array{}, "earlier_pools": array{}, "anchor": nil}
}
func sortObjects(a array, key string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && str(obj(a[j])[key]) < str(obj(a[j-1])[key]); j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
