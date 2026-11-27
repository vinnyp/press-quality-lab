package grade

import (
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"io"
	"strings"
	"testing"
)

func TestPR10TargetSize(t *testing.T) {
	// QE8, QP3: target_size counts drained bytes only after predicate evaluation.
	ch := collect.Check{Plan: plan.Check{Preimage: plan.Object{"probe": "byte_bound", "parameters": plan.Object{"bound": strictjson.Number("8")}}}, Invocations: []observe.Invocation{{Run: observe.Run{StdoutSeen: 4, StderrSeen: 5}}}}
	for _, row := range []struct {
		outcome decide.Outcome
		failure string
		size    any
	}{{"held", "", int64(9)}, {"not_held", "predicate", int64(9)}, {"not_held", "wrong_exit", nil}, {decide.NotObserved, "", nil}} {
		ch.Result = decide.Result{Outcome: row.outcome, FailureKind: row.failure}
		r, err := checkResolution(ch, nil)
		if err != nil {
			t.Fatal(err)
		}
		if r.TargetSize != row.size {
			t.Fatalf("%s %s: target_size %v want %v", row.outcome, row.failure, r.TargetSize, row.size)
		}
	}
	ch.Result = decide.Result{Outcome: "held"}
	ch.Invocations[0].Run.StdoutSeen = observe.MaxCount
	if _, err := checkResolution(ch, nil); err == nil {
		t.Fatal("sum overflow accepted")
	}
}

func TestPR10MaskedEnvelope(t *testing.T) {
	// QE18: the stored display path masks without changing storage identity.
	o, d, saved := fixture(t)
	d.Masker = record.NewMasker(nil, []string{"subject.json"})
	if !strings.Contains(o.SubjectFile, "subject.json") {
		t.Fatal("missing path injection")
	}
	_, err := Run(o, d, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(saved.SubjectFilePath, "/subject.json") {
		t.Fatal("storage identity changed")
	}
	path, ok := saved.Document()["subject_file_path"].(string)
	if !ok || !strings.HasSuffix(path, "/************") {
		t.Fatalf("display path %q", path)
	}
}
