//go:build scripted

package grade

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/scripted"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func TestAE24Unclassified(t *testing.T) {
	// AE24, QS5, QD3, QT5: unclassified commands receive discovery help only.
	for _, row := range []struct{ name, command string }{{"case_022", "export"}, {"case_063", "account"}} {
		t.Run(row.name, func(t *testing.T) {
			const kit = "../../docs/specs/conformance-collection-1.0"
			directory := filepath.Join(kit, "cases", "case-"+row.name[len("case_"):])
			var c caseassembly.Case
			if err := caseassembly.Read(filepath.Join(directory, "case.json"), &c); err != nil {
				t.Fatal(err)
			}
			layout := caseassembly.Assemble(t, kit, directory, c)
			data, err := os.ReadFile(filepath.Join(layout.Subject, c.Input.SubjectFile))
			if err != nil {
				t.Fatal(err)
			}
			sub, refusal := subject.Parse(data)
			if refusal != nil {
				t.Fatal(refusal)
			}
			snapshot, err := planningInput(Options{SubjectFile: filepath.Join(layout.Subject, c.Input.SubjectFile), Scope: c.Input.Scope, Scripted: true}, sub)
			if err != nil {
				t.Fatal(err)
			}
			p, err := plan.Build(snapshot.Input)
			if err != nil {
				t.Fatal(err)
			}
			host, err := json.Marshal(c.Input.Host)
			if err != nil {
				t.Fatal(err)
			}
			recording, err := os.ReadFile(filepath.Join(kit, "behaviour", c.Input.Form+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var trace bytes.Buffer
			raw, err := scripted.Read(host, recording, &trace)
			if err != nil {
				t.Fatal(err)
			}
			if err := raw.Required(plan.Array{}); err != nil {
				t.Fatal(err)
			}
			checks, err := collect.Collect(p, raw)
			if err != nil {
				t.Fatal(err)
			}
			keys, err := traceList(trace.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			var got []map[string]any
			for _, key := range keys {
				words := traceArray(key["words"])
				if len(words) > 0 && words[0] == row.command {
					got = append(got, key)
				}
			}
			want := []map[string]any{{"words": []any{row.command, "--help"}, "home": "empty", "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil}}
			if !reflect.DeepEqual(got, want) {
				t.Fatal(got, want)
			}
			if row.command == "account" {
				found := 0
				for _, ch := range checks {
					words, ok := ch.Plan.Preimage["words"].(plan.Array)
					if ok && len(words) > 0 && words[0] == "account" && ch.Plan.Preimage["probe"] != nil {
						found++
						if ch.Result.Reason != "placeholder_unfilled" || ch.Result.Level != 2 {
							t.Fatal(ch)
						}
					}
				}
				if found == 0 {
					t.Fatal("no account probe checks")
				}
			}
		})
	}
}

func TestTraceDestructiveScopes(t *testing.T) {
	// QA7, QA8, QT5: derive whole-book exclusion independently of required checks.
	help := map[string]any{"words": []any{"--help"}, "offline_args": []any{}, "env": []any{}, "home": "empty", "runbook": nil, "step": nil, "computation": nil}
	first := map[string]any{"words": []any{"read"}, "offline_args": []any{}, "env": []any{}, "home": "runbook", "runbook": "book", "step": "first", "computation": nil}
	last := map[string]any{"words": []any{"remove", "gear"}, "offline_args": []any{}, "env": []any{}, "home": "runbook", "runbook": "book", "step": "last", "computation": nil}
	host := map[string]any{"confinement": map[string]any{"applied": true}, "self_test": "passed"}
	sub := map[string]any{"commands": []any{map[string]any{"words": []any{"read"}, "class": "safe"}, map[string]any{"words": []any{"remove"}, "class": "destructive"}}, "runbooks": []any{map[string]any{"name": "book", "steps": []any{map[string]any{"name": "first", "words": []any{"read"}}, map[string]any{"name": "last", "words": []any{"remove", "gear"}}}}}}
	pres := map[string]any{"check": map[string]any{"runbook": "book", "step": "first", "index": json.Number("0"), "words": []any{"read"}, "offline_args": []any{}, "env": map[string]any{}}}
	predictions := map[string]tracePrediction{"check": {Level: 8}}
	recording := mustJSON(t, map[string]any{"invocations": []any{map[string]any{"key": help, "confined": []any{map[string]any{"markers": map[string]any{"confinement": false}}}}, map[string]any{"key": first, "confined": []any{map[string]any{"markers": map[string]any{"confinement": true}}}}, map[string]any{"key": last, "confined": []any{map[string]any{"markers": map[string]any{"confinement": true}}}}}})
	for _, inside := range []bool{false, true} {
		scope := []any{"conformance"}
		want := []map[string]any{help}
		if inside {
			scope = append(scope, "destructive")
			want = append(want, first, last)
		}
		input := map[string]any{"scope": scope, "evidence": []any{map[string]any{"check_id": "check", "outcome": "held"}}}
		if got := independentTrace(t, recording, host, sub, input, pres, predictions); !reflect.DeepEqual(got, want) {
			t.Fatal(inside, got, want)
		}
	}
}
