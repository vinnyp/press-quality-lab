package record

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR14Interim(t *testing.T) {
	// QE12, QS7, QP7: preserve judge and declaration order, full action text, names only, and declared paths only.
	subject := Object{"offline": Array{Object{"applies_to": Array{Array{"second"}}, "args": Array{"--offline"}, "env": Object{"Z": "value_z", "A": "value_a"}}, Object{"applies_to": Array{Array{}}, "args": Array{}, "env": Object{}}}, "bookkeeping": Array{"z", "a"}}
	output := Object{"checks": Array{Object{"id": "b", "reason": "could_not_start"}, Object{"id": "a", "reason": "live_not_run"}, Object{"id": "c", "reason": "empty_target"}}, "dimensions": Array{Object{"dimension": "second", "reason": "scope_not_requested"}, Object{"dimension": "first", "reason": "placeholder_unfilled"}, Object{"dimension": "skip", "reason": "live_not_run"}}}
	actions, offline, book := Interim(subject, output, nil)
	want := Array{Object{"check_id": "b", "dimension": nil, "reason": "could_not_start", "action": "Check that the binary the subject file names is an executable for this machine."}, Object{"check_id": "c", "dimension": nil, "reason": "empty_target", "action": "Seed data the check can evaluate, or declare the bound, the exit codes or the free-of-writes command the probe needs."}, Object{"check_id": nil, "dimension": "second", "reason": "scope_not_requested", "action": "Request a scope that includes the dimension. For `cli-mcp-parity`, no scope includes it in this version."}, Object{"check_id": nil, "dimension": "first", "reason": "placeholder_unfilled", "action": "Fill each placeholder the scorecard names, and classify each discovered command the file does not declare."}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("actions: %#v", actions)
	}
	wantOffline := Array{Object{"applies_to": Array{Array{"second"}}, "args": Array{"--offline"}, "env_names": Array{"A", "Z"}}, Object{"applies_to": Array{Array{}}, "args": Array{}, "env_names": Array{}}}
	if !reflect.DeepEqual(offline, wantOffline) || !reflect.DeepEqual(book, Array{"z", "a"}) {
		t.Fatal(offline, book)
	}
	doc := store.Record{NextActions: actions, OfflineSettings: offline, BookkeepingExcluded: book}.Document()
	for key, want := range map[string]any{"next_actions": want, "offline_settings": wantOffline, "bookkeeping_excluded": Array{"z", "a"}} {
		if !reflect.DeepEqual(doc[key], want) {
			t.Fatal(key, doc[key])
		}
	}
	emptyA, emptyO, emptyB := Interim(Object{}, Object{}, nil)
	if !reflect.DeepEqual(emptyA, Array{}) || !reflect.DeepEqual(emptyO, Array{}) || !reflect.DeepEqual(emptyB, Array{}) {
		t.Fatal("empty control")
	}
}

func TestPR14BeforeBound(t *testing.T) {
	// QE10, QE14: only display values are copied before the bound; all measurement fields remain unchanged.
	observed := Object{"discovery": Object{"found_names": Array{Array{"found"}}, "undeclared_names": Array{Array{"extra"}}}, "placeholders": Array{"/binary"}, "invocations": Array{Object{"computation": Array{"failed_computation"}}}, "padding": strings.Repeat("n", 1024)}
	for _, large := range []bool{false, true} {
		padding := "small"
		if large {
			padding = strings.Repeat("x", (4<<20)-256)
		}
		in := Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "a"}, {ID: "discovery"}}}, Results: map[string]Resolution{"a": {Result: decide.Result{Outcome: "held"}, Expected: padding}, "discovery": {Result: decide.Result{Outcome: decide.NotObserved, Reason: "precondition_unmet"}, Observed: observed}}}
		doc, copy, err := Build(in)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(copy["discovery"].Observed, observed) {
			t.Fatal("copy lost names")
		}
		ev := object(array(doc["evidence"])[1])
		if large {
			raw := strictjson.AppendValue(nil, observed)
			want := Object{"bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
			if !reflect.DeepEqual(ev["observed"], want) {
				t.Fatal(ev["observed"])
			}
		} else if !reflect.DeepEqual(ev["observed"], observed) {
			t.Fatal(ev)
		}
		want := Object{"check_id": "discovery", "outcome": "not_observed", "reason": "precondition_unmet", "failure_kind": nil, "target_size": nil, "precondition_observed": nil, "expected": ev["expected"], "observed": ev["observed"], "targets": Array{Object{"kind": "invocation", "id": "discovery", "tier": "live"}}, "tier": "live", "evaluator": Object{"kind": "deterministic", "model": nil}}
		if !reflect.DeepEqual(ev, want) {
			t.Fatalf("measurement changed: %#v", ev)
		}
		object(copy["discovery"].Observed)["placeholders"] = Array{"changed"}
		if !large && !reflect.DeepEqual(ev["observed"], observed) {
			t.Fatal("copy aliases record")
		}
	}
}

func TestPR14BoundJudgeAuthority(t *testing.T) {
	// QE10, QE14, QE19: bounding display values cannot change any field of the judge's output.
	data, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/cases/case-020/input.json")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := strictjson.Read(data, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		t.Fatal("fixture parse")
	}
	source := object(v)
	in := Input{Request: "grade", Scope: array(source["scope"]), SubjectFacts: object(source["subject_facts"]), Markers: array(source["markers"]), Results: map[string]Resolution{}, Scripted: true}
	for _, v := range array(source["required"]) {
		r := object(v)
		id, ok := r["id"].(string)
		if !ok {
			t.Fatal("id")
		}
		in.Plan.Checks = append(in.Plan.Checks, plan.Check{ID: id, Required: r})
	}
	for _, v := range array(source["evidence"]) {
		e := object(v)
		id, ok := e["check_id"].(string)
		if !ok {
			t.Fatal("check id")
		}
		outcome, ok := e["outcome"].(string)
		if !ok {
			t.Fatal("outcome")
		}
		reason := ""
		if value, ok := e["reason"].(string); ok {
			reason = value
		}
		in.Results[id] = Resolution{Result: decide.Result{Outcome: decide.Outcome(outcome), Reason: decide.Reason(reason)}, FailureKind: e["failure_kind"], TargetSize: e["target_size"], PreconditionObserved: e["precondition_observed"], Expected: e["expected"], Observed: e["observed"]}
	}
	ids := make([]string, 0, len(in.Results))
	for id := range in.Results {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) < 2 {
		t.Fatal("fixture needs two checks")
	}
	baseline, _, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	baselineOutput, baselineCode := judge.Judge(strictjson.Write(baseline))
	if baselineCode == 2 {
		t.Fatalf("baseline rejected: %s", baselineOutput)
	}
	first := in.Results[ids[0]]
	first.Expected = strings.Repeat("x", (4<<20)-(16<<10))
	in.Results[ids[0]] = first
	last := in.Results[ids[len(ids)-1]]
	last.Observed = Object{"discovery": Object{"undeclared_names": Array{Array{strings.Repeat("name", 8192)}}}, "placeholders": Array{"/binary"}}
	in.Results[ids[len(ids)-1]] = last
	doc, copy, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if object(array(doc["evidence"])[0])["expected"] != first.Expected {
		t.Fatal("earlier value did not consume the budget")
	}
	if object(object(array(doc["evidence"])[len(ids)-1])["observed"])["sha256"] == nil {
		t.Fatal("later observed value was not digested")
	}
	before := cloneValue(doc)
	beforeDoc := object(before)
	for _, v := range array(beforeDoc["evidence"]) {
		e := object(v)
		id, ok := e["check_id"].(string)
		if !ok {
			t.Fatal("check id")
		}
		e["expected"] = copy[id].Expected
		e["observed"] = copy[id].Observed
	}
	raw, rawCode := judge.Judge(strictjson.Write(beforeDoc))
	stored, storedCode := judge.Judge(strictjson.Write(doc))
	if rawCode == 2 || storedCode == 2 {
		t.Fatalf("judge rejected control: %s %s", raw, stored)
	}
	if rawCode != storedCode || !bytes.Equal(raw, stored) || !bytes.Equal(baselineOutput, stored) || baselineCode != storedCode {
		t.Fatalf("verdict differs: %s %s", raw, stored)
	}
}
