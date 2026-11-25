package record

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func decoded(t *testing.T, b []byte) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// multisets applies the judge contract's input-order rule to every list.
func multisets(t *testing.T, v any) any {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			x[k] = multisets(t, child)
		}
	case []any:
		keys := make(map[string]any, len(x))
		order := make([]string, len(x))
		for i, child := range x {
			child = multisets(t, child)
			b, err := json.Marshal(child)
			if err != nil {
				t.Fatal(err)
			}
			order[i] = string(b)
			keys[string(b)] = child
		}
		sort.Strings(order)
		for i, key := range order {
			x[i] = keys[key]
		}
	}
	return v
}
func TestCase004(t *testing.T) {
	// QE7: required entries are preserved. QE8: level-1 evidence and targets.
	// QE9: claimed tier. QT2: all scripted tiers. QS4: facts as given.
	// QE11: markers as given. QE19: corpus comparison excludes value content.
	root := "../../docs/specs/conformance-collection-1.0"
	read := func(p string) []byte {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	s, e := subject.Parse(read(root + "/cases/case-004/subject.json"))
	if e != nil {
		t.Fatal(e)
	}
	in := plan.Input{Subject: s, Scopes: []rulebook.Scope{rulebook.ConformanceScope}, Files: map[string][]byte{}}
	for _, dir := range []string{"seed", "computations"} {
		base := root + "/subjects/known-good/" + dir
		err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == base {
				return nil
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			if dir == "seed" {
				var data []byte
				if !d.IsDir() {
					data = read(p)
				}
				in.Seed = append(in.Seed, plan.Entry{Path: filepath.ToSlash(rel), Directory: d.IsDir(), Data: data})
			} else if !d.IsDir() {
				in.Files["computations/"+filepath.ToSlash(rel)] = read(p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := plan.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]Resolution{}
	for _, ch := range p.Checks {
		results[ch.ID] = Resolution{Result: decide.Resolve(ch, observe.Controls{ConfinementApplied: true, SystemNamed: true}, decide.Evidence{})}
	}
	doc, err := Build(Input{Plan: p, Results: results, Request: "grade", Scope: Array{"conformance"}, SubjectFacts: assertType[Object](t, s["facts"]), Markers: assertType[Array](t, s["markers"]), Scripted: true})
	if err != nil {
		t.Fatal(err)
	}
	got := assertType[map[string]any](t, decoded(t, strictjson.Write(doc)))
	want := assertType[map[string]any](t, decoded(t, read(root+"/cases/case-004/input.json")))
	for _, v := range []map[string]any{got, want} {
		for _, name := range []string{"required", "evidence"} {
			a := assertType[[]any](t, v[name])
			key := "id"
			if name == "evidence" {
				key = "check_id"
				for _, x := range a {
					delete(assertType[map[string]any](t, x), "expected")
					delete(assertType[map[string]any](t, x), "observed")
				}
			}
			sort.Slice(a, func(i, j int) bool {
				return assertType[string](t, assertType[map[string]any](t, a[i])[key]) < assertType[string](t, assertType[map[string]any](t, a[j])[key])
			})
		}
	}
	if !reflect.DeepEqual(multisets(t, got), multisets(t, want)) {
		t.Fatalf("case-004 mismatch\ngot %v\nwant %v", got, want)
	}
}

func TestTargets(t *testing.T) {
	// QE8: ordered invocation and artifact targets, including E-1's no-run store.
	// QE9: mixed, offline and no-invocation tiers. QT2: overrides every tier.
	for _, row := range []struct {
		name, probe string
		keys        []Object
		path        any
		want        Array
		tier        string
	}{
		{"NoInvocation", "store_unchanged", nil, "data", Array{Object{"kind": "invocation", "id": "a", "tier": "live"}}, "live"},
		{"OfflineStore", "store_unchanged", []Object{{"offline_args": Array{"--offline"}}}, "data", Array{Object{"kind": "invocation", "id": "a", "tier": "offline"}, Object{"kind": "artifact", "id": "data", "tier": "offline"}}, "offline"},
		{"MixedSecret", "planted_secret", []Object{{"env": Array{"OFFLINE"}}, {}}, "token", Array{Object{"kind": "invocation", "id": "a#1", "tier": "offline"}, Object{"kind": "invocation", "id": "a#2", "tier": "live"}, Object{"kind": "artifact", "id": "token", "tier": "live"}}, "live"},
		{"Discovery", "discovery", []Object{{}, {}}, nil, Array{Object{"kind": "invocation", "id": "a#1", "tier": "live"}, Object{"kind": "invocation", "id": "a#2", "tier": "live"}}, "live"},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := plan.Check{ID: "a", Required: Object{}, Preimage: Object{"probe": row.probe, "store": row.path, "parameters": Object{"path": row.path}}, Keys: row.keys}
			for _, scripted := range []bool{false, true} {
				doc, err := Build(Input{Plan: plan.Plan{Checks: []plan.Check{ch}}, Results: map[string]Resolution{"a": {Result: decide.Result{Outcome: decide.NotObserved, Reason: decide.NetworkUnverifiable}}}, Scripted: scripted})
				if err != nil {
					t.Fatal(err)
				}
				claimed := "live"
				if scripted {
					claimed = "scripted"
				}
				if doc["claimed_tier"] != claimed {
					t.Fatal("claimed tier", doc["claimed_tier"])
				}
				ev := assertType[Object](t, assertType[Array](t, doc["evidence"])[0])
				want := row.want
				tier := row.tier
				if scripted {
					tier = "scripted"
					want = Array{}
					for _, v := range row.want {
						o := assertType[Object](t, v)
						want = append(want, Object{"kind": o["kind"], "id": o["id"], "tier": "scripted"})
					}
				}
				if !reflect.DeepEqual(ev["targets"], want) || ev["tier"] != tier {
					t.Fatalf("%v", ev)
				}
			}
		})
	}
}

func TestBounds(t *testing.T) {
	// QE10: exact 4 MiB and 15 MiB boundaries, chosen digest and fault.
	value := strings.Repeat("x", (4<<20)-6) // string encoding + null = exactly 4 MiB.
	ev := Object{"expected": value, "observed": nil}
	doc := Object{"evidence": Array{ev}}
	if err := bound(doc, Array{ev}); err != nil || ev["expected"] != value {
		t.Fatal("exact value cap", err)
	}
	ev = Object{"expected": value + "x", "observed": nil}
	doc = Object{"evidence": Array{ev}}
	// Keeping expected would overflow once observed's digest is included.
	if err := bound(doc, Array{ev}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`"` + value + `x"`)
	wantDigest := Object{"bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
	if !reflect.DeepEqual(ev["expected"], wantDigest) {
		t.Fatalf("digest %v", ev["expected"])
	}
	for _, extra := range []int{0, 1} {
		a := Object{"expected": strings.Repeat("a", 1000), "observed": strings.Repeat("b", 1000)}
		b := Object{"expected": strings.Repeat("c", 1000), "observed": strings.Repeat("d", 1000)}
		evidence := Array{a, b}
		d := Object{"evidence": evidence, "padding": ""}
		d["padding"] = strings.Repeat("p", (15<<20)-strictjson.Length(d)+extra)
		if err := bound(d, evidence); err != nil {
			t.Fatal(err)
		}
		if strictjson.Length(d) > 15<<20 {
			t.Fatal("record too large")
		}
		_, changed := b["expected"].(Object)
		if changed != (extra == 1) {
			t.Fatal("reverse pass boundary")
		}
		if extra == 1 {
			raw := []byte(`"` + strings.Repeat("c", 1000) + `"`)
			want := Object{"bytes": 1002, "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
			if !reflect.DeepEqual(b["expected"], want) || b["observed"] != strings.Repeat("d", 1000) {
				t.Fatal("reverse digest order")
			}
		}
	}
	d := Object{"padding": strings.Repeat("p", 15<<20)}
	if err := bound(d, Array{}); err == nil {
		t.Fatal("unfittable accepted")
	} else if f, ok := err.(*Fault); !ok || f.Rule != "QE10" {
		t.Fatal(err)
	}
}

func TestUnbuilt(t *testing.T) {
	// QE8: absent or unbuilt resolutions cannot produce evidence.
	ch := plan.Check{ID: "a"}
	for _, r := range []map[string]Resolution{nil, {"a": {Result: decide.Result{Unbuilt: &decide.Unbuilt{Rule: "QH8"}}}}} {
		d, err := Build(Input{Plan: plan.Plan{Checks: []plan.Check{ch}}, Results: r})
		if d != nil || err == nil {
			t.Fatal("built unresolved evidence")
		}
	}
}

func TestValueTotalBoundary(t *testing.T) {
	// QE10: replacing a value past the cap must still satisfy the total cap.
	e := Object{"expected": strings.Repeat("x", (4<<20)-5), "observed": nil}
	if err := bound(Object{"evidence": Array{e}}, Array{e}); err != nil {
		t.Fatal(err)
	}
	size := strictjson.Length(e["expected"]) - 1 + strictjson.Length(e["observed"]) - 1
	if size > 4<<20 {
		t.Fatalf("values occupy %d bytes; cap is %d", size, 4<<20)
	}
}

func TestFactsAndMarkers(t *testing.T) {
	// QS4: every fact passes through as given. QE11: markers pass through.
	for _, b := range []bool{false, true} {
		for _, bounding := range []string{"none", "declared"} {
			facts := Object{"read_only": b, "has_destructive_ops": b, "store_backed": b, "has_cli": b, "bounding": bounding}
			markers := Array{Object{"record": "example", "dimension": "execution"}}
			doc, err := Build(Input{SubjectFacts: facts, Markers: markers})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(doc["subject_facts"], facts) || !reflect.DeepEqual(doc["markers"], markers) {
				t.Fatal("changed subject declarations")
			}
		}
	}
}

func TestEvidenceFieldsAndOrder(t *testing.T) {
	// QE7: required entries are unchanged. QE8: every evidence field is carried.
	// QE10: the value cap follows check-id order, independent of plan order.
	p := plan.Plan{Checks: []plan.Check{{ID: "b", Required: Object{"id": "b"}}, {ID: "a", Required: Object{"id": "a"}}}}
	r := Resolution{Result: decide.Result{Outcome: "not_held"}, FailureKind: "predicate", TargetSize: 3, PreconditionObserved: true, Expected: "wanted", Observed: "actual"}
	doc, err := Build(Input{Plan: p, Results: map[string]Resolution{"a": r, "b": r}})
	if err != nil {
		t.Fatal(err)
	}
	want := Object{"check_id": "a", "outcome": "not_held", "reason": nil, "failure_kind": "predicate", "target_size": 3, "precondition_observed": true, "expected": "wanted", "observed": "actual", "targets": Array{Object{"kind": "invocation", "id": "a", "tier": "live"}}, "tier": "live", "evaluator": Object{"kind": "deterministic", "model": nil}}
	if !reflect.DeepEqual(assertType[Array](t, doc["evidence"])[0], want) || !reflect.DeepEqual(doc["required"], Array{Object{"id": "a"}, Object{"id": "b"}}) {
		t.Fatal("changed fields or order", doc)
	}
	if p.Checks[0].ID != "b" {
		t.Fatal("mutated caller's plan")
	}
}

func TestValuePrefixBoundary(t *testing.T) {
	// QE10: include suffix digests in the exact 4 MiB boundary; expected
	// precedes observed. The next byte forces both original values out.
	for _, extra := range []int{0, 1} {
		// A digest of this 1002-byte string is 90 serialized bytes.
		expected := strings.Repeat("x", (4<<20)-90-2+extra)
		observed := strings.Repeat("y", 1000)
		e := Object{"expected": expected, "observed": observed}
		if err := bound(Object{"evidence": Array{e}}, Array{e}); err != nil {
			t.Fatal(err)
		}
		raw := []byte(`"` + observed + `"`)
		want := Object{"bytes": 1002, "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
		if !reflect.DeepEqual(e["observed"], want) {
			t.Fatal("observed digest", e["observed"])
		}
		if extra == 0 {
			if e["expected"] != expected || strictjson.Length(e["expected"])-1+strictjson.Length(e["observed"])-1 != 4<<20 {
				t.Fatal("longest prefix not kept at exact cap")
			}
		} else {
			raw = []byte(`"` + expected + `"`)
			want = Object{"bytes": len(raw), "sha256": fmt.Sprintf("%x", sha256.Sum256(raw))}
			if !reflect.DeepEqual(e["expected"], want) {
				t.Fatal("expected digest", e["expected"])
			}
		}
	}
}

func assertType[T any](t *testing.T, v any) T {
	t.Helper()
	x, ok := v.(T)
	if !ok {
		t.Fatalf("unexpected type %T", v)
	}
	return x
}

func TestBuildFault(t *testing.T) {
	// QE10: the public builder returns no partial record on an unfittable input.
	doc, err := Build(Input{Markers: Array{strings.Repeat("x", 15<<20)}})
	f, ok := err.(*Fault)
	if doc != nil || !ok || f.Rule != "QE10" {
		t.Fatalf("document returned: %t; fault: %v", doc != nil, err)
	}
}
