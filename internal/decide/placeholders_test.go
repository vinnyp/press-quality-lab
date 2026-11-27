package decide

import (
	"encoding/json"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestPlaceholderPositions(t *testing.T) {
	// QS5: each allowed location independently; facts affect only the final override.
	for _, position := range []string{"binary", "seed", "store", "declared_count", "secret", "read_only", "has_cli", "store_backed", "has_destructive_ops", "bounding", "class", "writes"} {
		t.Run(position, func(t *testing.T) {
			facts := plan.Object{}
			command := plan.Object{"words": plan.Array{"read"}, "class": "safe", "writes": false}
			s := plan.Object{"binary": "bin/subject", "seed": nil, "store": nil, "declared_count": 0, "secret": plan.Object{"commands": plan.Array{plan.Array{"read"}}}, "facts": facts, "commands": plan.Array{command}}
			checks := []struct {
				name  string
				probe any
				words any
			}{{"Discovery", "discovery", nil}, {"Bare", "bare", plan.Array{}}, {"Probe", "byte_bound", plan.Array{"read"}}, {"Secret", "planted_secret", nil}, {"Authored", nil, plan.Array{"read", "item"}}, {"Unrelated", nil, plan.Array{"other"}}}
			for _, check := range checks {
				ch := plan.Check{Subject: s, Required: plan.Object{}, Preimage: plan.Object{"probe": check.probe, "words": check.words}}
				if got := level2(ch, passing(), Evidence{}); got != (Result{}) {
					t.Fatalf("control %s: %+v", check.name, got)
				}
			}
			ph := plan.Object{"placeholder": "Supply this value so the grade can measure it."}
			switch position {
			case "class", "writes":
				command[position] = ph
			case "read_only", "has_cli", "store_backed", "has_destructive_ops", "bounding":
				facts[position] = ph
			default:
				s[position] = ph
			}
			held := Result{Outcome: "held", Level: 8}
			override := FinalOverride(plan.Check{Subject: s, Preimage: plan.Object{"dimension": "availability"}}, held)
			if override != (Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: 8}) {
				t.Fatalf("override position %s: %+v", position, override)
			}
			for _, check := range checks {
				want := position == "binary" || (position == "seed" || position == "store") && check.name != "Discovery" || position == "declared_count" && check.name == "Discovery" || position == "secret" && check.name == "Secret" || (position == "class" || position == "writes") && (check.name == "Probe" || check.name == "Secret" || check.name == "Authored")
				got := level2(plan.Check{Subject: s, Required: plan.Object{}, Preimage: plan.Object{"probe": check.probe, "words": check.words}}, passing(), Evidence{})
				expected := Result{}
				if want {
					expected = Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: 2}
				}
				if got != expected {
					t.Fatalf("%s: %+v want %+v", check.name, got, expected)
				}
			}
		})
	}
}

func TestPlaceholderOverride(t *testing.T) {
	// QS5, QA7: final override changes held always-applicable checks only, after runbooks.
	for _, dim := range []string{"availability", "execution", "graceful-failure", "robustness", "response-correctness", "determinism", "provenance-honesty", "completeness-signaling", "output-conformance", "discoverability", "error-message-quality", "token-economy", "pii-secret-hygiene", "bounding", "idempotency", "destructive-guard", "side-effect-integrity", "flag-conformance"} {
		t.Run(strings.ReplaceAll(dim, "-", "_"), func(t *testing.T) {
			for _, outcome := range []Outcome{"held", "not_held", NotObserved} {
				for _, ph := range []bool{false, true} {
					// Observed preconditions belong to record.Resolution; FinalOverride never reads them.
					s := plan.Object{"facts": plan.Object{"has_cli": true}}
					if ph {
						s["facts"] = plan.Object{"has_cli": plan.Object{"placeholder": "State whether a CLI exists."}}
					}
					ch := plan.Check{Subject: s, Preimage: plan.Object{"dimension": dim}, Required: plan.Object{}}
					before := Result{Outcome: outcome, Level: 8}
					want := before
					excluded := dim == "bounding" || dim == "idempotency" || dim == "destructive-guard" || dim == "side-effect-integrity" || dim == "flag-conformance"
					if ph && outcome == "held" && !excluded {
						want = Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: 8}
					}
					if got := FinalOverride(ch, before); got != want {
						t.Fatalf("%s ph=%v: %+v want %+v", outcome, ph, got, want)
					}
				}
			}
		})
	}
}

func TestPlaceholderClassifiers(t *testing.T) {
	// QS5, KTD2.8: level 2 independently agrees with the predictor for every preimage.
	read := func(path string, target any) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	var document struct {
		Dependencies map[string]map[string]bool `json:"placeholder_dependencies"`
	}
	read("../../scripts/ratchet/pr13-prediction.json", &document)
	checks := 0
	for name, predictions := range document.Dependencies {
		directory := filepath.Join("../../docs/specs/conformance-collection-1.0/cases", name)
		decode := func(path string) plan.Object {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			value, ok := strictjson.Read(data, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
			if !ok {
				t.Fatal("invalid fixture", path)
			}
			result, ok := value.(strictjson.Object)
			if !ok {
				t.Fatal("fixture not object", path)
			}
			return result
		}
		subject := decode(filepath.Join(directory, "subject.json"))
		scopes := []rulebook.Scope{}
		for _, scope := range array(object(decode(filepath.Join(directory, "case.json"))["input"])["scope"]) {
			if token, ok := scope.(string); ok {
				scopes = append(scopes, rulebook.Scope(token))
			}
		}
		preimages := decode(filepath.Join(directory, "preimages.json"))
		if len(preimages) != len(predictions) {
			t.Fatal("preimage coverage", name)
		}
		for id, value := range preimages {
			want, present := predictions[id]
			if !present {
				t.Fatal("missing prediction", name, id)
			}
			pre, ok := value.(strictjson.Object)
			if !ok {
				t.Fatal("invalid preimage", name, id)
			}
			got := level2(plan.Check{Scopes: scopes, Subject: subject, Preimage: pre}, passing(), Evidence{})
			expected := Result{}
			if want {
				expected = Result{Outcome: NotObserved, Reason: "placeholder_unfilled", Level: 2}
			}
			if got != expected {
				t.Fatalf("%s %s: got %+v want %+v", name, id, got, expected)
			}
			checks++
		}
	}
	if len(document.Dependencies) != 121 || checks == 0 {
		t.Fatalf("coverage cases=%d checks=%d", len(document.Dependencies), checks)
	}
}
