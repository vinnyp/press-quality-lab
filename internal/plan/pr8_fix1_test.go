package plan

import (
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestPR8EarlierScope(t *testing.T) {
	// QE2: earlier contains every earlier step check, even outside the scope.
	s := Object{"commands": Array{}, "runbooks": Array{Object{"name": "book", "steps": Array{
		Object{"name": "first", "words": Array{}, "budget_seconds": strictjson.Number("1"), "checks": Array{Object{"dimension": "destructive-guard", "predicate": "stdout_equals", "expected": Object{"literal": "x"}}}},
		Object{"name": "second", "words": Array{}, "budget_seconds": strictjson.Number("1"), "checks": Array{Object{"dimension": "response-correctness", "predicate": "stdout_equals", "expected": Object{"literal": "x"}}}},
	}}}}
	var ids []string
	for _, scopes := range [][]rulebook.Scope{{rulebook.ConformanceScope}, {rulebook.ConformanceScope, rulebook.DestructiveScope}} {
		p, e := Build(Input{Subject: s, Scopes: scopes})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		first := false
		for _, ch := range p.Checks {
			if ch.Preimage["step"] == "first" {
				first = true
			}
			if ch.Preimage["step"] == "second" {
				found = true
				ids = append(ids, ch.ID)
				if len(arr(ch.Preimage["earlier"])) != 1 {
					t.Fatal("earlier missing")
				}
			}
		}
		if first != (len(scopes) == 2) {
			t.Fatal("scope membership", scopes, first)
		}
		if !found {
			t.Fatal("later check missing")
		}
	}
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatal(ids)
	}
}

func TestPR8QA3Predicates(t *testing.T) {
	// QA3: all three output-conformance predicates can credit; captures cannot.
	for _, pred := range []string{"json_path_equals", "row_count"} {
		t.Run(pred, func(t *testing.T) {
			ch := Object{"dimension": "output-conformance", "predicate": pred, "path": "/items", "expected": Object{"literal": strictjson.Number("1")}}
			if !credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("positive does not credit")
			}
			ch["expected"] = Object{"computation": Object{"captures": Array{"prior"}}}
			if credits(Object{}, Object{}, Object{}, ch) {
				t.Fatal("self-agreement credits")
			}
		})
	}
}

func TestPR8PlaceholderShape(t *testing.T) {
	// QS5: only an object with exactly one key is a placeholder.
	if !placeholder(Object{"placeholder": "Supply seed."}) {
		t.Fatal("placeholder missed")
	}
	for _, v := range []any{Object{"placeholder": "x", "k": 1}, Object{"literal": Object{"placeholder": "x"}}} {
		if placeholder(v) {
			t.Fatal("data is placeholder")
		}
	}
}
