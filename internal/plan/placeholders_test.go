package plan

import (
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPlaceholderPositionCandidates(t *testing.T) {
	// QS5, QE3: every allowed placeholder retains all six probe candidates,
	// including the safe/false candidate for a command with class or writes missing.
	for _, position := range []string{"binary", "seed", "store", "declared_count", "secret", "read_only", "has_destructive_ops", "store_backed", "has_cli", "bounding", "class", "writes"} {
		t.Run(position, func(t *testing.T) {
			command := Object{"words": Array{"read"}, "class": "safe", "writes": false, "byte_bound": strictjson.Number("8"), "exit_codes": Array{strictjson.Number("0")}, "budget_seconds": strictjson.Number("5")}
			facts := Object{"read_only": false, "has_destructive_ops": false, "store_backed": false, "has_cli": true, "bounding": "declared"}
			s := Object{"binary": "bin/subject", "seed": nil, "store": nil, "secret": nil, "declared_count": strictjson.Number("1"), "facts": facts, "commands": Array{command}}
			check := func() {
				t.Helper()
				p, err := Build(Input{Subject: s, Scopes: []rulebook.Scope{rulebook.ConformanceScope}})
				if err != nil {
					t.Fatal(err)
				}
				counts := map[string]int{}
				for _, ch := range p.Checks {
					probe, ok := ch.Preimage["probe"].(string)
					if !ok {
						t.Fatal("probe absent")
					}
					counts[probe]++
					if probe == "byte_bound" || probe == "exit_codes" || probe == "store_unchanged" {
						if !same(ch.Preimage["words"], Array{"read"}) {
							t.Fatalf("%s missing read candidate", probe)
						}
					}
				}
				if len(p.Checks) != 6 || len(counts) != 6 {
					t.Fatalf("checks %+v", counts)
				}
				for _, probe := range []string{"discovery", "bare", "byte_bound", "exit_codes", "store_unchanged", "planted_secret"} {
					if counts[probe] != 1 {
						t.Fatalf("%s count %d", probe, counts[probe])
					}
				}
			}
			check()
			ph := Object{"placeholder": "Supply the value so the grade can measure it."}
			switch position {
			case "class", "writes":
				command[position] = ph
			case "read_only", "has_destructive_ops", "store_backed", "has_cli", "bounding":
				facts[position] = ph
			default:
				s[position] = ph
			}
			check()
		})
	}
}
