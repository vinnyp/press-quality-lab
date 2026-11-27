//go:build scripted

package corpusrunner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
)

func TestPlaceholderClassifiers(t *testing.T) {
	// QS5, KTD2.8: the runner independently agrees with the shared prediction oracle.
	var document struct {
		Dependencies map[string]map[string]bool `json:"placeholder_dependencies"`
	}
	read(t, "../../scripts/ratchet/pr10-prediction.json", &document)
	predicted := document.Dependencies
	cases, checks := 0, 0
	for name, predictions := range predicted {
		_, _, s, pres := fixture(t, name)
		if len(pres) != len(predictions) {
			t.Fatal("preimage coverage", name)
		}
		for id := range pres {
			if _, ok := predictions[id]; !ok {
				t.Fatal("missing prediction", name, id)
			}
		}
		cases++
		for id, want := range predictions {
			pre := object(pres[id])
			if pre == nil {
				t.Fatalf("%s missing preimage %s", name, id)
			}
			runner := levelTwoPlaceholder(s, pre)
			if runner != want {
				t.Fatalf("%s %s: runner=%v predictor=%v", name, id, runner, want)
			}
			checks++
		}
	}
	if cases != 121 || checks == 0 {
		t.Fatalf("coverage cases=%d checks=%d", cases, checks)
	}
}

func TestCreateThenGrade(t *testing.T) {
	// QS5, QT6, QE16: create's placeholders decide every check at level 2;
	// QD2, QO7: discovery still walks; no other invocation or home is served.
	binary := filepath.Join(t.TempDir(), "pql")
	if output, err := exec.Command("go", "build", "-tags", "scripted", "-o", binary, "../../cmd/pql").CombinedOutput(); err != nil {
		t.Fatalf("build %s %v", output, err)
	}
	c, _, _, _ := fixture(t, "case-004")
	c.Input.Host["self_test"] = "passed"
	layout := caseassembly.Assemble(t, kit, filepath.Join(kit, "cases", "case-004"), c)
	command := exec.Command(binary, "create", "bin/subject")
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	data, err := command.Output()
	if err != nil {
		t.Fatalf("create %v %s", err, diagnostic.String())
	}
	if err := os.WriteFile(filepath.Join(layout.Subject, c.Input.SubjectFile), data, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := runGrade(binary, c, layout)
	if err != nil {
		t.Fatal(err)
	}
	if r.Record == nil {
		t.Fatalf("no record: %+v", r)
	}
	input := object(r.Record["judge_input"])
	if len(list(input["required"])) != 6 || len(list(input["evidence"])) != 6 {
		t.Fatalf("check set: %v", input)
	}
	for _, v := range list(input["evidence"]) {
		e := object(v)
		if e["outcome"] != "not_observed" || e["reason"] != "placeholder_unfilled" {
			t.Fatal(e)
		}
	}
	for _, v := range object(input["subject_facts"]) {
		if v != nil {
			t.Fatalf("fact not unstated: %v", v)
		}
	}
	if r.Exit != 4 || len(r.Trace.Served) != 6 || len(r.Trace.Homes) != 0 {
		t.Fatalf("exit=%d trace=%+v", r.Exit, r.Trace)
	}
	for _, key := range r.Trace.Served {
		if !discovery(object(key)) {
			t.Fatal("unexpected non-discovery key", key)
		}
	}
	if err := checkRequired(r.Trace, input); err != nil {
		t.Fatal(err)
	}
	if err := rejudge(binary, r.Record); err != nil {
		t.Fatal(err)
	}
}
