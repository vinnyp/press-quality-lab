package grade

import (
	"bytes"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR14InvocationEvidence(t *testing.T) {
	// QT6, QA4: declared invocation names survive even when their streams were not observed.
	key := plan.Object{"words": plan.Array{"show"}, "runbook": "book", "step": "step", "computation": nil}
	comp := plan.Object{"words": nil, "runbook": "book", "step": "step", "computation": plan.Array{"compute", "{SEED}"}}
	for _, ran := range []bool{false, true} {
		ch := collect.Check{Plan: plan.Check{Keys: []plan.Object{key, comp}}, Result: decide.Result{Outcome: decide.NotObserved, Reason: "computation_failed"}}
		if ran {
			ch.Invocations = []observe.Invocation{{Key: map[string]any(key), Run: observe.Run{Stderr: []byte("problem"), StderrSeen: 7}}}
		}
		r, err := checkResolution(ch, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := subjectObject(r.Observed)
		want := strictjson.Array{strictjson.Object{"words": plan.Array{"show"}, "runbook": "book", "step": "step", "computation": nil}, strictjson.Object{"words": nil, "runbook": "book", "step": "step", "computation": plan.Array{"compute", "{SEED}"}}}
		if !reflect.DeepEqual(got["invocations"], want) {
			t.Fatalf("invocations: %#v", got)
		}
		streams := subjectArray(got["streams"])
		if len(streams) != 2 || streams[1] != nil || (streams[0] != nil) != ran {
			t.Fatalf("streams: %#v", streams)
		}
	}
	r, err := checkResolution(collect.Check{}, nil)
	if err != nil || !reflect.DeepEqual(r.Observed, strictjson.Object{"probe": nil, "streams": strictjson.Array{}}) {
		t.Fatal(r, err)
	}
}

func TestPR14DiscoveryEvidence(t *testing.T) {
	// QD1, QD4: found and declared counts, found names, and missing names are independent facts.
	for _, ran := range []bool{true, false} {
		ch := collect.Check{Plan: plan.Check{Preimage: plan.Object{"probe": "discovery"}, Subject: plan.Object{"declared_count": strictjson.Number("3"), "commands": plan.Array{plan.Object{"words": plan.Array{"found"}}, plan.Object{"words": plan.Array{"missed"}}}}}, Discovery: &decide.Discovery{Ran: ran, FoundCount: 2, DeclaredFound: []bool{true, false}, Names: []plan.Array{{"found"}, {"extra"}}, UndeclaredNames: []plan.Array{{"extra"}}, UndeclaredCount: 1}}
		r, err := checkResolution(ch, nil)
		if err != nil {
			t.Fatal(err)
		}
		o := subjectObject(r.Observed)
		d := subjectObject(o["discovery"])
		if d["ran"] != ran {
			t.Fatal(d)
		}
		if ran {
			want := plan.Object{"ran": true, "found_count": 2, "declared_count": strictjson.Number("3"), "found_names": plan.Array{plan.Array{"found"}, plan.Array{"extra"}}, "missing_names": plan.Array{plan.Array{"missed"}}, "undeclared_names": plan.Array{plan.Array{"extra"}}, "undeclared_count": 1, "request_bound_reached": false, "depth_bound_reached": false, "found_cut": false, "undeclared_cut": false}
			if !reflect.DeepEqual(d, want) {
				t.Fatal(d)
			}
		} else {
			for _, key := range []string{"found_count", "declared_count", "found_names", "missing_names", "undeclared_names", "undeclared_count"} {
				if d[key] != nil {
					t.Fatal(key, d)
				}
			}
		}
		if !ran {
			ch.Plan.Keys = []plan.Object{plan.DiscoveryKey(plan.Array{})}
			notRun, err := checkResolution(ch, nil)
			if err != nil || !reflect.DeepEqual(subjectObject(notRun.Observed)["streams"], plan.Array{nil}) {
				t.Fatal(notRun, err)
			}
		}
		if _, ok := o["invocations"]; ok {
			t.Fatal("discovery has invocation names")
		}
	}
	ch := collect.Check{Plan: plan.Check{Preimage: plan.Object{"probe": "discovery"}, Subject: plan.Object{"declared_count": plan.Object{"placeholder": "count"}}}, Discovery: &decide.Discovery{Ran: true}}
	r, err := checkResolution(ch, nil)
	if err != nil || subjectObject(subjectObject(r.Observed)["discovery"])["declared_count"] != nil {
		t.Fatal(r, err)
	}
}

func TestPR14PlaceholderLocations(t *testing.T) {
	// QS5: each of the eight allowed places has a pointer, in fixed order with RFC 6901 escaping.
	s := plan.Object{"binary": "bin", "declared_count": strictjson.Number("0"), "seed": nil, "store": nil, "secret": nil, "facts": plan.Object{"a/b~": true, "z": false}, "commands": plan.Array{plan.Object{"class": "safe", "writes": false}}}
	setters := []func(any){func(v any) { s["binary"] = v }, func(v any) { s["declared_count"] = v }, func(v any) { s["seed"] = v }, func(v any) { s["store"] = v }, func(v any) { s["secret"] = v }, func(v any) { subjectObject(s["facts"])["a/b~"] = v }, func(v any) { subjectObject(subjectArray(s["commands"])[0])["class"] = v }, func(v any) { subjectObject(subjectArray(s["commands"])[0])["writes"] = v }}
	pointers := []string{"/binary", "/declared_count", "/seed", "/store", "/secret", "/facts/a~1b~0", "/commands/0/class", "/commands/0/writes"}
	if len(placeholderLocations(s)) != 0 {
		t.Fatal("plain control")
	}
	for i, set := range setters {
		set(plan.Object{"placeholder": "supply value"})
		want := plan.Array{}
		for _, p := range pointers[:i+1] {
			want = append(want, p)
		}
		if !reflect.DeepEqual(placeholderLocations(s), want) {
			t.Fatal(i, placeholderLocations(s), want)
		}
	}
	subjectObject(s["facts"])["z"] = plan.Object{"placeholder": "supply fact"}
	want := plan.Array{"/binary", "/declared_count", "/seed", "/store", "/secret", "/facts/a~1b~0", "/facts/z", "/commands/0/class", "/commands/0/writes"}
	if !reflect.DeepEqual(placeholderLocations(s), want) {
		t.Fatal(placeholderLocations(s))
	}
}

func TestPR14ShortEnvNames(t *testing.T) {
	// QE18: the threshold is eight bytes and names are sorted and deduplicated.
	for _, v := range []string{"1234567", "12345678", "éééé"} {
		s := plan.Object{"offline": plan.Array{plan.Object{"env": plan.Object{"Z": v, "A": v}}, plan.Object{"env": plan.Object{"Z": v}}}}
		want := plan.Array{}
		if len(v) < 8 {
			want = plan.Array{"A", "Z"}
		}
		if got := shortEnvNames(s); !reflect.DeepEqual(got, want) {
			t.Fatal(got, want)
		}
	}
}

func TestPR14MaskingData(t *testing.T) {
	// QE18, QS7, QP7, QA4: mask raw subject text before copying or cutting; short declared values remain visible.
	for _, row := range []struct {
		name, value string
		masked      bool
	}{{"C0", "synthetic\x01value", true}, {"Cf", "synthetic\u200dvalue", true}, {"Short", "short", false}} {
		t.Run(row.name, func(t *testing.T) {
			s, refusal := subject.Parse([]byte(validSubject))
			if refusal != nil {
				t.Fatal(refusal)
			}
			v := row.value
			s["commands"] = plan.Array{plan.Object{"words": plan.Array{v}, "class": "safe", "writes": false, "exit_codes": plan.Array{strictjson.Number("0")}, "byte_bound": strictjson.Number("4096"), "budget_seconds": strictjson.Number("10")}}
			s["offline"] = plan.Array{plan.Object{"applies_to": plan.Array{plan.Array{v}}, "args": plan.Array{v}, "env": plan.Object{"SYNTHETIC": v}}}
			s["bookkeeping"] = plan.Array{v}
			parsed, refusal := subject.Parse(strictjson.Write(s))
			if refusal != nil {
				t.Fatal(refusal)
			}
			masker := record.NewMasker(nil, []string{v})
			key := plan.Object{"words": plan.Array{v}, "runbook": "book", "step": "step", "computation": plan.Array{"compute", v, "{SEED}"}}
			ch := collect.Check{Plan: plan.Check{ID: "comp", Subject: parsed, Keys: []plan.Object{key}}, Result: decide.Result{Outcome: decide.NotObserved, Reason: "computation_failed"}, Invocations: []observe.Invocation{{Key: map[string]any(key), Run: observe.Run{Stderr: []byte(v), StderrSeen: int64(len(v))}}}}
			r, err := checkResolution(ch, masker)
			if err != nil {
				t.Fatal(err)
			}
			disc := collect.Check{Plan: plan.Check{ID: "discovery", Subject: parsed, Preimage: plan.Object{"probe": "discovery"}}, Discovery: &decide.Discovery{Ran: true, Names: []plan.Array{{v}}}, Result: decide.Result{Outcome: "held"}}
			dr, err := checkResolution(disc, masker)
			if err != nil {
				t.Fatal(err)
			}
			doc, copy, err := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{ch.Plan, disc.Plan}}, Results: map[string]record.Resolution{"comp": r, "discovery": dr}, Masker: masker})
			if err != nil {
				t.Fatal(err)
			}
			a, o, b := record.Interim(parsed, nil, masker)
			stored := store.Record{JudgeInput: doc, NextActions: a, OfflineSettings: o, BookkeepingExcluded: b}.Document()
			want := v
			if row.masked {
				want = strings.Repeat("*", utf8.RuneCountInString(v))
			}
			setting := subjectObject(subjectArray(stored["offline_settings"])[0])
			if !reflect.DeepEqual(setting, plan.Object{"applies_to": plan.Array{plan.Array{want}}, "args": plan.Array{want}, "env_names": plan.Array{"SYNTHETIC"}}) || !reflect.DeepEqual(stored["bookkeeping_excluded"], plan.Array{want}) {
				t.Fatal(stored)
			}
			for _, observed := range []any{subjectObject(subjectArray(doc["evidence"])[0])["observed"], copy["comp"].Observed} {
				observed := subjectObject(observed)
				inv := subjectObject(subjectArray(observed["invocations"])[0])
				if !reflect.DeepEqual(inv["words"], plan.Array{want}) || !reflect.DeepEqual(inv["computation"], plan.Array{"compute", want, "{SEED}"}) {
					t.Fatal(inv)
				}
				head := subjectObject(subjectObject(subjectArray(observed["streams"])[0])["stderr"])["head"]
				if head != want {
					t.Fatal(head, want)
				}
			}
			names := subjectObject(copy["discovery"].Observed)["short_env_names"]
			wantNames := plan.Array{}
			if !row.masked {
				wantNames = plan.Array{"SYNTHETIC"}
			}
			if !reflect.DeepEqual(names, wantNames) {
				t.Fatal(names)
			}
			if row.masked {
				for _, value := range []any{stored, copy["comp"].Observed, copy["discovery"].Observed} {
					if bytes.Contains(strictjson.Write(value), strictjson.AppendValue(nil, v)) {
						t.Fatal("raw declared value retained")
					}
				}
			}
		})
	}
}

func TestPR14InterimStored(t *testing.T) {
	// QS7, QP7, QE12, QE18: grade attaches masked interim fields to both accepted and rejected records.
	for _, rejected := range []bool{false, true} {
		o, d, saved := fixture(t)
		data := strings.Replace(validSubject, `"offline":[]`, `"offline":[{"applies_to":[[]],"args":["synthetic_value"],"env":{"VALUE":"synthetic_value"}}]`, 1)
		data = strings.Replace(data, `"bookkeeping":[]`, `"bookkeeping":["synthetic_value"]`, 1)
		if err := os.WriteFile(o.SubjectFile, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		d.Source = func() (observe.Source, func() error, error) { return source{}, func() error { return nil }, nil }
		d.Collect = func(p plan.Plan, _ observe.Source) ([]collect.Check, error) {
			checks := []collect.Check{}
			for _, ch := range p.Checks {
				checks = append(checks, collect.Check{Plan: ch, Result: decide.Result{Outcome: decide.NotObserved, Reason: "could_not_start"}})
			}
			return checks, nil
		}
		calls := 0
		d.Judge = func(b []byte) ([]byte, int) {
			calls++
			if calls == 1 {
				return judge.Judge(b)
			}
			if rejected {
				return strictjson.Write(plan.Object{"outcome": "rejected", "rejected_by": "X1", "exit_code": 2}), 2
			}
			return strictjson.Write(plan.Object{"outcome": "graded", "exit_code": 4, "checks": plan.Array{plan.Object{"id": "synthetic_check", "reason": "could_not_start"}}}), 4
		}
		var out bytes.Buffer
		code, err := Run(o, d, &out)
		if rejected {
			if code != 70 || err == nil {
				t.Fatal(code, err)
			}
		} else if code != 4 || err != nil {
			t.Fatal(code, err)
		}
		doc := saved.Document()
		want := strings.Repeat("*", 15)
		if !reflect.DeepEqual(doc["offline_settings"], plan.Array{plan.Object{"applies_to": plan.Array{plan.Array{}}, "args": plan.Array{want}, "env_names": plan.Array{"VALUE"}}}) || !reflect.DeepEqual(doc["bookkeeping_excluded"], plan.Array{want}) {
			t.Fatal(doc)
		}
		actions := subjectArray(doc["next_actions"])
		if rejected {
			if len(actions) != 0 {
				t.Fatal(actions)
			}
		} else if len(actions) != 1 || subjectObject(actions[0])["check_id"] != "synthetic_check" {
			t.Fatal(actions)
		}
	}
}

func TestPR14ComputationOrder(t *testing.T) {
	// QA4, QA6, QT6: preserve today's precondition-then-expected stream order, including identical computations.
	for _, same := range []bool{false, true} {
		subjectKey := plan.Object{"words": plan.Array{"show"}, "computation": nil}
		expected := plan.Object{"words": nil, "computation": plan.Array{"expected", "{SEED}", "{SUBJECT_DIR}"}}
		precondition := plan.Object{"words": nil, "computation": plan.Array{"precondition"}}
		if same {
			precondition = expected
		}
		ch := collect.Check{Plan: plan.Check{Keys: []plan.Object{subjectKey, expected, precondition}, Preimage: plan.Object{"expected": plan.Object{"computation": plan.Object{"argv": expected["computation"]}, "precondition": plan.Object{"computation": plan.Object{"argv": precondition["computation"]}}}}}}
		ch.Invocations = []observe.Invocation{{Key: map[string]any(subjectKey), Run: observe.Run{Stdout: []byte("subject"), StdoutSeen: 7}}, {Key: map[string]any(precondition), Run: observe.Run{Stdout: []byte("precondition"), StdoutSeen: 12}}}
		if !same {
			ch.Invocations = append(ch.Invocations, observe.Invocation{Key: map[string]any(expected), Run: observe.Run{Stdout: []byte("expected"), StdoutSeen: 8}})
		}
		r, err := checkResolution(ch, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Result.Outcome = "held"
		_, display, err := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{ch.Plan}}, Results: map[string]record.Resolution{ch.Plan.ID: r}})
		if err != nil {
			t.Fatal(err)
		}
		observed := subjectObject(display[ch.Plan.ID].Observed)
		streams := subjectArray(observed["streams"])
		names := subjectArray(observed["invocations"])
		wantHeads := []string{"subject", "precondition"}
		if !same {
			wantHeads = append(wantHeads, "expected")
		}
		if len(streams) != len(wantHeads) || len(names) != len(wantHeads) {
			t.Fatal(streams, names)
		}
		for i, want := range wantHeads {
			if subjectObject(subjectObject(streams[i])["stdout"])["head"] != want {
				t.Fatal(i, streams)
			}
		}
		if !reflect.DeepEqual(subjectObject(names[1])["computation"], precondition["computation"]) {
			t.Fatal(names)
		}
		if !same && !reflect.DeepEqual(subjectObject(names[2])["computation"], plan.Array{"expected", "{SEED}", "{SUBJECT_DIR}"}) {
			t.Fatal(names)
		}
	}
}

func TestPR14ProbeStoreEvidence(t *testing.T) {
	// QE14, QP7, QE18: every check carries its probe; only discovery carries the declared store.
	for _, probe := range []any{nil, "discovery", "byte_bound"} {
		for _, path := range []any{nil, "synthetic_store"} {
			ch := collect.Check{Plan: plan.Check{Preimage: plan.Object{"probe": probe}, Subject: plan.Object{"store": path}}}
			r, err := checkResolution(ch, nil)
			if err != nil {
				t.Fatal(err)
			}
			o, ok := r.Observed.(strictjson.Object)
			if !ok {
				t.Fatalf("observed is not an object: %#v", r.Observed)
			}
			if v, present := o["probe"]; !present || v != probe {
				t.Fatalf("probe: %#v", o)
			}
			if v, present := o["store"]; present != (probe == "discovery") || (present && v != path) {
				t.Fatalf("store: %#v", o)
			}
		}
	}
}
