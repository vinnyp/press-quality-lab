package plan

import (
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"reflect"
	"testing"
)

func TestPlanScopesVerbatim(t *testing.T) {
	// E-7, QE3, QA8: the guard receives requested tokens, including invalid ones.
	scopes := []rulebook.Scope{"conformance", "unknown", "conformance", "destructive"}
	want := append([]rulebook.Scope(nil), scopes...)
	p, err := Build(Input{Subject: Object{}, Scopes: scopes})
	if err != nil || !reflect.DeepEqual(p.Scopes, want) {
		t.Fatal(p.Scopes, err)
	}
	scopes[0] = "changed"
	if !reflect.DeepEqual(p.Scopes, want) {
		t.Fatal("scope aliases caller", p.Scopes)
	}
}
