package plan

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestNoCommandPreimages(t *testing.T) {
	// QE2: no-command probes have null words and budget and empty parameters.
	// QE3: every probe remains required even without a command.
	// QE7: every required entry has the specified fields.
	for _, secret := range []struct {
		name   string
		value  any
		params Object
	}{
		{"Null", nil, Object{}},
		{"Placeholder", Object{"placeholder": "Supply the secret."}, Object{"secret": Object{"placeholder": "Supply the secret."}}},
	} {
		t.Run(secret.name, func(t *testing.T) {
			p, err := Build(Input{Subject: Object{"commands": Array{}, "bookkeeping": Array{}, "secret": secret.value}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Checks) != 6 {
				t.Fatalf("checks %d", len(p.Checks))
			}
			for _, probe := range []struct {
				name, dim, id string
				empty         bool
			}{
				{"discovery", "discoverability", "f42d924c83f4d342ef835f1d8dcddca0aca74f4548a302f02fffcdc65642f938", false},
				{"bare", "availability", "f6643b2e4961880d061a35c71aa052b23dc8e302d092cb569e7972dce6509026", false},
				{"byte_bound", "bounding", "575ee74f3954922da84db9998fc6aa0837470904656dd717702259ae86e82a2f", true},
				{"exit_codes", "execution", "423a59acd41bd94a5252163b428841ccc36105e9063176a05a9e090716e59139", false},
				{"store_unchanged", "side-effect-integrity", "59f67ab85cd2354eb8d153c9d81b82543d542c441c0340944a10f26c96787cc6", false},
				{"planted_secret", "pii-secret-hygiene", "4e3ea7cd3f7418fe23a7d420bddcb09e4d3e416d91472d89cfa6579bd8e6fda4", false},
			} {
				params := Object{}
				if probe.name == "planted_secret" {
					params = secret.params
				}
				want := Object{"scheme": "pql-check-id-1", "probe": probe.name, "runbook": nil, "step": nil, "index": nil, "earlier": Array{}, "dimension": probe.dim, "predicate": nil, "path": nil, "words": nil, "offline_args": Array{}, "env": Object{}, "budget_seconds": nil, "seed": nil, "store": nil, "bookkeeping": Array{}, "parameters": params, "expected": nil}
				wantKeys := []Object{}
				switch probe.name {
				case "discovery":
					want["parameters"] = Object{"declared_count": nil, "commands": Array{}}
					wantKeys = append(wantKeys, Object{"words": Array{"--help"}, "offline_args": Array{}, "env": Array{}, "home": "empty", "runbook": nil, "step": nil, "computation": nil})
				case "bare":
					want["words"] = Array{}
					want["budget_seconds"] = strictjson.Number("10")
					want["parameters"] = Object{"exit_codes": nil}
					wantKeys = append(wantKeys, Object{"words": Array{}, "offline_args": Array{}, "env": Array{}, "home": "seed", "runbook": nil, "step": nil, "computation": nil})
				}
				found := false
				for _, ch := range p.Checks {
					if ch.Preimage["probe"] != probe.name {
						continue
					}
					found = true
					if !same(ch.Preimage, want) {
						t.Fatalf("%s preimage %#v", probe.name, ch.Preimage)
					}
					id := probe.id
					if probe.name == "planted_secret" && secret.name == "Placeholder" {
						id = "5c8df8573149c1ea51efc0d00b29cb4697644783cdea1c10412132ca9bd53352"
					}
					required := Object{"id": id, "kind": "plain", "dimension": probe.dim, "record": nil, "crediting": true, "empty_satisfiable": probe.empty, "asserts_emptiness": false, "precondition": nil, "declared_count": nil, "condition": nil, "condition_a": nil}
					if !same(ch.Required, required) || !same(ch.Keys, wantKeys) {
						t.Fatalf("%s required %#v keys %#v", probe.name, ch.Required, ch.Keys)
					}
				}
				if !found {
					t.Fatalf("missing %s", probe.name)
				}
			}
		})
	}
}

func TestCreditConditions(t *testing.T) {
	// QA3: each binding condition in credit-and-routes section 2, both sides.
	n := func(s string) strictjson.Number { return strictjson.Number(s) }
	for _, row := range []struct {
		name, dim, pred string
		good, bad       Object
		setup           func(Object, Object, Object)
	}{
		{"GracefulLiteral", "graceful-failure", "exit_code", Object{"literal": n("1")}, Object{"literal": n("0")}, nil},
		{"GracefulCodes", "graceful-failure", "stdout_contains", Object{"literal": "bad"}, Object{"literal": "bad"}, func(s, r, ch Object) { ch["exit_codes"] = Array{n("1")} }},
		{"Output", "output-conformance", "stdout_json_equals", Object{"literal": nil}, Object{"literal": nil}, nil},
		{"Error", "error-message-quality", "stderr_contains", Object{"literal": "bad"}, Object{"literal": "bad"}, nil},
		{"Economy", "token-economy", "stdout_bytes_at_most", Object{"literal": n("2")}, Object{"literal": n("2")}, nil},
		{"Destructive", "destructive-guard", "exit_code", Object{"literal": n("1")}, Object{"literal": n("1")}, func(s, r, ch Object) { obj(arr(s["commands"])[0])["class"] = "destructive" }},
		{"Determinism", "determinism", "stdout_equals", Object{"literal": "same"}, Object{"literal": "same"}, func(s, r, ch Object) {
			r["steps"] = append(arr(r["steps"]), Object{"name": "second", "words": Array{"read"}, "checks": Array{Object{"dimension": "determinism", "predicate": "stdout_equals", "expected": Object{"literal": "same"}}}})
		}},
		{"Idempotency", "idempotency", "stdout_json_equals", Object{"computation": Object{"captures": Array{}}}, Object{"literal": nil}, func(s, r, ch Object) {
			obj(arr(s["commands"])[0])["writes"] = true
			r["steps"] = append(arr(r["steps"]), Object{"name": "second", "words": Array{"read"}})
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			s := Object{"commands": Array{Object{"words": Array{"read"}, "class": "safe", "writes": false}}}
			st := Object{"name": "first", "words": Array{"read"}}
			ch := Object{"dimension": row.dim, "predicate": row.pred, "expected": row.good}
			r := Object{"steps": Array{st}}
			if row.setup != nil {
				row.setup(s, r, ch)
			}
			if !credits(s, r, st, ch) {
				t.Fatal("positive does not credit")
			}
			ch["expected"] = row.bad
			switch row.name {
			case "GracefulCodes":
				ch["exit_codes"] = Array{n("1"), n("0")}
			case "Output", "Error", "Economy":
				ch["predicate"] = "stdout_equals"
			case "Destructive":
				obj(arr(s["commands"])[0])["class"] = "safe"
			case "Determinism":
				obj(arr(r["steps"])[1])["words"] = Array{"other"}
			}
			if credits(s, r, st, ch) {
				t.Fatal("negative credits")
			}
		})
	}
}

func TestSelfAgreement(t *testing.T) {
	// QA3: expected computations and preconditions using captures never credit.
	for _, source := range []string{"expected", "precondition"} {
		t.Run(source, func(t *testing.T) {
			ch := Object{"dimension": "response-correctness", "expected": Object{"literal": "value"}}
			if !credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("positive")
			}
			ch[source] = Object{"computation": Object{"captures": Array{"earlier"}}}
			if credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("self agreement credits")
			}
		})
	}
}

func TestPlaceholderCandidates(t *testing.T) {
	// QS5: class and writes placeholders are candidates as safe and false.
	// QE3: each candidate has its own check in the fixed required set.
	for _, field := range []string{"class", "writes"} {
		t.Run(field, func(t *testing.T) {
			c := Object{"words": Array{"read"}, "class": "safe", "writes": false, "byte_bound": strictjson.Number("8"), "exit_codes": Array{strictjson.Number("0")}, "budget_seconds": strictjson.Number("5")}
			c[field] = Object{"placeholder": "Supply the command setting."}
			p, err := Build(Input{Subject: Object{"commands": Array{c}}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
			if err != nil {
				t.Fatal(err)
			}
			for _, probe := range []string{"byte_bound", "exit_codes", "store_unchanged"} {
				count := 0
				for _, ch := range p.Checks {
					if ch.Preimage["probe"] == probe {
						count++
						if !same(ch.Preimage["words"], Array{"read"}) || len(ch.Keys) != 1 {
							t.Fatalf("%s missing candidate: %+v", probe, ch)
						}
					}
				}
				if count != 1 {
					t.Fatalf("%s count %d", probe, count)
				}
			}
		})
	}
}

func TestDestructiveRunbookScope(t *testing.T) {
	// QA7: one destructive step excludes the whole runbook outside destructive.
	// QE3: included dimensions alone do not admit a forbidden runbook.
	s := Object{"commands": Array{Object{"words": Array{"erase"}, "class": "destructive"}}, "runbooks": Array{Object{"name": "book", "steps": Array{
		Object{"name": "read", "words": Array{"read"}, "checks": Array{Object{"dimension": "availability", "predicate": "exit_code", "expected": Object{"literal": strictjson.Number("0")}}}},
		Object{"name": "erase", "words": Array{"erase"}, "checks": Array{}},
	}}}}
	for _, row := range []struct {
		name   string
		scopes []rulebook.Scope
		count  int
	}{
		{"Excluded", []rulebook.Scope{rulebook.ConformanceScope}, 0},
		{"Included", []rulebook.Scope{rulebook.ConformanceScope, rulebook.DestructiveScope}, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			p, err := Build(Input{Subject: s, Scopes: row.scopes})
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, ch := range p.Checks {
				if ch.Required["record"] == "book" {
					count++
				}
			}
			if count != row.count {
				t.Fatalf("authored count %d", count)
			}
		})
	}
}

func TestComputationKeys(t *testing.T) {
	// QE2: captures are preserved in expected preimages; QA3: they prevent credit.
	// QT5: computations use scratch, with their check's runbook and step.
	// QA9: emptiness preconditions use the same computation form.
	comp := Object{"argv": Array{"/bin/echo", "{CAPTURE:first}"}, "budget_seconds": strictjson.Number("3"), "captures": Array{"first"}, "declared_files": Array{}}
	precomp := Object{"argv": Array{"/bin/echo", "true"}, "budget_seconds": strictjson.Number("2"), "captures": Array{}, "declared_files": Array{}}
	s := Object{"runbooks": Array{Object{"name": "book", "steps": Array{
		Object{"name": "first", "words": Array{"read"}, "checks": Array{}},
		Object{"name": "second", "words": Array{"read"}, "checks": Array{
			Object{"dimension": "response-correctness", "predicate": "stdout_json_equals", "expected": Object{"computation": comp}},
			Object{"dimension": "response-correctness", "predicate": "stdout_empty", "expected": Object{"literal": nil}, "precondition": Object{"computation": precomp}},
		}},
	}}}}
	p, err := Build(Input{Subject: s, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, ch := range p.Checks {
		if ch.Required["record"] != "book" {
			continue
		}
		count++
		wantArgv := Array{"/bin/echo", "{CAPTURE:first}"}
		if ch.Preimage["predicate"] == "stdout_empty" {
			wantArgv = Array{"/bin/echo", "true"}
			if ch.Required["asserts_emptiness"] != true || ch.Required["precondition"] != "precondition computation printed true" {
				t.Fatalf("emptiness %#v", ch.Required)
			}
		} else if ch.Required["crediting"] != false {
			t.Fatal("capture credits")
		}
		want := Object{"words": nil, "offline_args": Array{}, "env": Array{}, "home": "scratch", "runbook": "book", "step": "second", "computation": wantArgv}
		if len(ch.Keys) != 2 || !same(ch.Keys[1], want) {
			t.Fatalf("keys %#v", ch.Keys)
		}
	}
	if count != 2 {
		t.Fatalf("checks %d", count)
	}
}

func TestNegativeZero(t *testing.T) {
	// QA3: -0 is zero, so it cannot credit graceful-failure.
	// QA9: a literal row count of -0 asserts emptiness.
	for _, value := range []string{"0", "-0"} {
		name := "Zero"
		if value == "-0" {
			name = "NegativeZero"
		}
		t.Run(name, func(t *testing.T) {
			ch := Object{"dimension": "graceful-failure", "predicate": "exit_code", "expected": Object{"literal": strictjson.Number(value)}}
			if credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("zero credits")
			}
			ch["predicate"] = "stdout_equals"
			ch["exit_codes"] = Array{strictjson.Number(value)}
			if credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("zero accepted code credits")
			}
			ch["dimension"] = "response-correctness"
			ch["predicate"] = "row_count"
			ch["precondition"] = Object{"computation": Object{"argv": Array{"/bin/echo", "true"}, "budget_seconds": strictjson.Number("1"), "captures": Array{}, "declared_files": Array{}}}
			p, err := Build(Input{Subject: Object{"runbooks": Array{Object{"name": "book", "steps": Array{Object{"name": "step", "words": Array{"read"}, "checks": Array{ch}}}}}}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, c := range p.Checks {
				if c.Required["record"] == "book" {
					found = true
					if c.Required["asserts_emptiness"] != true {
						t.Fatal("zero row count does not assert emptiness")
					}
				}
			}
			if !found {
				t.Fatal("authored check missing")
			}
		})
	}
}

func TestCreditRoutes(t *testing.T) {
	// QA3: none routes credit; probe-only routes never credit authored checks.
	for _, row := range []struct {
		dim  string
		want bool
	}{
		{"availability", true},
		{"execution", true},
		{"robustness", true},
		{"response-correctness", true},
		{"provenance-honesty", true},
		{"completeness-signaling", true},
		{"flag-conformance", true},
		{"bounding", true},
		{"discoverability", true},
		{"side-effect-integrity", false},
		{"pii-secret-hygiene", false},
	} {
		name := ""
		for _, r := range row.dim {
			if r == '-' {
				name += "_"
			} else {
				name += string(r)
			}
		}
		t.Run(name, func(t *testing.T) {
			ch := Object{"dimension": "response-correctness", "predicate": "stdout_equals", "expected": Object{"literal": "value"}}
			if !credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("positive route")
			}
			ch["dimension"] = row.dim
			if got := credits(Object{}, Object{}, Object{}, ch); got != row.want {
				t.Fatalf("credit %v want %v", got, row.want)
			}
			if row.want {
				ch["expected"] = Object{"computation": Object{"captures": Array{"first"}}}
				if credits(Object{}, Object{}, Object{}, ch) {
					t.Fatal("self agreement credits this route")
				}
			}
		})
	}
}

func TestCreditConditionGuards(t *testing.T) {
	// QA3: isolate every conjunct of the determinism and idempotency conditions.
	for _, guard := range []string{"OtherStep", "Words", "Dimension", "Predicate", "Path", "Expected", "Computation", "RepeatedCommand", "WritingCommand", "DestructiveCommand"} {
		t.Run(guard, func(t *testing.T) {
			s := Object{"commands": Array{Object{"words": Array{"write"}, "class": "safe", "writes": true}, Object{"words": Array{"other"}, "class": "safe", "writes": true}}}
			ch := Object{"dimension": "determinism", "predicate": "json_path_equals", "path": "/items", "expected": Object{"literal": "value"}}
			peer := Object{"dimension": "determinism", "predicate": "json_path_equals", "path": "/items", "expected": Object{"literal": "value"}}
			first := Object{"name": "first", "words": Array{"write", "one"}, "checks": Array{ch}}
			second := Object{"name": "second", "words": Array{"write", "one"}, "checks": Array{peer}}
			r := Object{"steps": Array{first, second}}
			switch guard {
			case "Computation", "RepeatedCommand", "WritingCommand", "DestructiveCommand":
				ch["dimension"] = "idempotency"
				ch["expected"] = Object{"computation": Object{"captures": Array{}}}
				// Different invocations still resolve to the same writing command.
				second["words"] = Array{"write", "two"}
				if guard == "DestructiveCommand" {
					obj(arr(s["commands"])[0])["class"] = "destructive"
					obj(arr(s["commands"])[0])["writes"] = false
				}
			}
			if !credits(s, r, first, ch) {
				t.Fatal("positive does not credit")
			}
			switch guard {
			case "OtherStep":
				second["name"] = "first"
			case "Words":
				second["words"] = Array{"other"}
			case "Dimension":
				peer["dimension"] = "response-correctness"
			case "Predicate":
				peer["predicate"] = "stdout_equals"
			case "Path":
				peer["path"] = "/different"
			case "Expected":
				peer["expected"] = Object{"literal": "different"}
			case "Computation":
				ch["expected"] = Object{"literal": "value"}
			case "RepeatedCommand":
				second["words"] = Array{"other"}
			case "WritingCommand":
				obj(arr(s["commands"])[0])["writes"] = false
			case "DestructiveCommand":
				obj(arr(s["commands"])[0])["class"] = "safe"
			}
			if credits(s, r, first, ch) {
				t.Fatal("negative credits")
			}
		})
	}
}
