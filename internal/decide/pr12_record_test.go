package decide_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestComputedDepthInBuiltRecord(t *testing.T) {
	// QA6, E-12, QE8: derive the allowable depth from the production evidence builder.
	build := func(depth int) strictjson.Object {
		var value any = nil
		for i := 0; i < depth; i++ {
			value = strictjson.Array{value}
		}
		input, err := record.Build(record.Input{Plan: plan.Plan{Checks: []plan.Check{{ID: "synthetic", Preimage: plan.Object{}, Required: plan.Object{}}}}, Results: map[string]record.Resolution{"synthetic": {Result: decide.Result{Outcome: "held", Level: 8}, Expected: value}}, Masker: record.NewMasker(nil, nil)})
		if err != nil {
			t.Fatal(err)
		}
		return input
	}
	bounds := strictjson.Bounds{Bytes: 1 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}
	depth := 0
	for ; depth <= 64; depth++ {
		if _, ok := strictjson.Read(strictjson.Write(build(depth)), bounds, nil); !ok {
			break
		}
	}
	if depth-1 != decide.ComputedValueDepth {
		t.Fatalf("derived %d, constant %d", depth-1, decide.ComputedValueDepth)
	}
	input := build(61)
	temp := t.TempDir()
	s, err := store.New(filepath.Join(temp, "store"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.Write(store.Record{SubjectFilePath: filepath.Join(temp, "subject.json"), JudgeInput: input})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := strictjson.Read(data, bounds, nil); ok {
		t.Fatal("stored depth 65 accepted at depth 64")
	}
	bounds.Depth = 65
	got, ok := strictjson.Read(data, bounds, nil)
	if !ok {
		t.Fatal("stored depth 65 refused")
	}
	doc, ok := got.(strictjson.Object)
	if !ok || !strictjson.EqualJSON(doc["judge_input"], input) {
		t.Fatal("stored input changed")
	}
}
