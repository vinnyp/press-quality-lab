//go:build scripted

package corpusrunner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
)

// Literal wire words from rulebook section 8 and judge contract section 4.
var runWords = map[string]map[int]string{
	"grade":  {0: "green", 1: "red", 3: "known_defect", 4: "not_measured"},
	"anchor": {0: "anchored", 5: "refused"},
}

func agreement(c caseassembly.Case, textRun, jsonRun result) error {
	if c.Kind == "derivation-only" {
		if len(textRun.Stdout) != 0 || len(jsonRun.Stdout) != 0 {
			return errors.New("derivation_stdout")
		}
		for _, r := range []result{textRun, jsonRun} {
			if r.Record != nil {
				return errors.New("derivation_record")
			}
			if fmt.Sprint(r.Exit) != fmt.Sprint(c.Expect["exit_code"]) {
				return errors.New("derivation_exit")
			}
			if r.Rule != c.Expect["rule"] {
				return errors.New("derivation_rule")
			}
		}
		return nil
	}
	doc := textRun.Record
	output := object(doc["judge_output"])
	if fmt.Sprint(textRun.Exit) != fmt.Sprint(c.Expect["exit_code"]) || textRun.Exit != jsonRun.Exit || fmt.Sprint(textRun.Exit) != fmt.Sprint(doc["exit_code"]) || fmt.Sprint(textRun.Exit) != fmt.Sprint(output["exit_code"]) {
		return errors.New("exit_equality")
	}
	if !bytes.Equal(textRun.RecordBytes, jsonRun.RecordBytes) {
		return errors.New("record_bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(jsonRun.Stdout))
	decoder.UseNumber()
	var got any
	if err := decoder.Decode(&got); err != nil || !reflect.DeepEqual(got, output) {
		return errors.New("json_document")
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return errors.New("json_document")
	}
	lines := strings.Split(strings.TrimSuffix(string(textRun.Stdout), "\n"), "\n")
	verdict := object(output["verdict"])
	word := verdict["plain"]
	if c.Input.Request == "anchor" {
		word = object(output["anchor"])["result"]
	}
	base := fmt.Sprintf("pql %s: %v (exit %d)", c.Input.Request, word, textRun.Exit)
	first := base
	if verdict["sub_label"] != nil {
		first += fmt.Sprintf(" (%v)", verdict["sub_label"])
	}
	if len(lines) == 0 || lines[0] != first {
		if len(lines) > 0 && lines[0] == base && first != base {
			return errors.New("sub_label")
		}
		return errors.New("run_line")
	}
	if runWords[c.Input.Request][textRun.Exit] != word {
		return errors.New("word_table")
	}
	reason := func(m map[string]any) string {
		if m["reason"] == nil {
			return ""
		}
		return fmt.Sprintf(" reason=%v", m["reason"])
	}
	wantDimensions, wantChecks := []string{}, []string{}
	for _, v := range list(output["dimensions"]) {
		d := object(v)
		wantDimensions = append(wantDimensions, fmt.Sprintf("dimension %v: %v%s", d["dimension"], d["state"], reason(d)))
	}
	held := 0
	for _, v := range list(output["checks"]) {
		ch := object(v)
		switch ch["result"] {
		case "unobserved", "failed", "failed_expected":
			wantChecks = append(wantChecks, fmt.Sprintf("  check %v: %v%s", ch["id"], ch["result"], reason(ch)))
		case "held_crediting", "held_non_crediting":
			held++
		}
	}
	dims, checks, heldLines, countLines := []string{}, []string{}, []string{}, []string{}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "dimension "):
			dims = append(dims, line)
		case strings.HasPrefix(line, "  check "):
			checks = append(checks, line)
		case strings.HasPrefix(line, "held checks: "):
			heldLines = append(heldLines, line)
		case strings.HasPrefix(line, "counts: "):
			countLines = append(countLines, line)
		}
	}
	if !reflect.DeepEqual(dims, wantDimensions) {
		return errors.New("dimensions")
	}
	// Check lines are grouped under dimensions; pair by id, retaining multiplicity.
	if !reflect.DeepEqual(unorderedStrings(checks), unorderedStrings(wantChecks)) {
		return errors.New("listed_checks")
	}
	if !reflect.DeepEqual(heldLines, []string{fmt.Sprintf("held checks: %d", held)}) {
		return errors.New("held_count")
	}
	counts := object(output["counts"])
	if !reflect.DeepEqual(countLines, []string{fmt.Sprintf("counts: xfail=%v xpass=%v", counts["xfail"], counts["xpass"])}) {
		return errors.New("counts")
	}
	return nil
}
func unorderedStrings(a []string) any {
	v := make([]any, len(a))
	for i, s := range a {
		v[i] = s
	}
	return unordered(v)
}

func TestAgreementControls(t *testing.T) {
	// QE14, GR3, AE21: each planted defect breaks exactly its named agreement leg.
	makeFixture := func() (caseassembly.Case, result, result) {
		var c caseassembly.Case
		c.Kind = "judge-checked"
		c.Input.Request = "grade"
		c.Expect = map[string]any{"exit_code": json.Number("4")}
		output := map[string]any{"exit_code": json.Number("4"), "verdict": map[string]any{"plain": "not_measured", "sub_label": "gaps"}, "dimensions": []any{map[string]any{"dimension": "execution", "state": "HARNESS", "reason": "live_not_run"}}, "checks": []any{map[string]any{"id": "a", "result": "unobserved", "reason": "live_not_run"}, map[string]any{"id": "b", "result": "held_crediting", "reason": nil}}, "counts": map[string]any{"xfail": json.Number("0"), "xpass": json.Number("0")}}
		raw, err := json.Marshal(output)
		if err != nil {
			t.Fatal(err)
		}
		r := result{Exit: 4, Record: map[string]any{"exit_code": json.Number("4"), "judge_output": output}, RecordBytes: []byte("same"), Stdout: []byte("pql grade: not_measured (exit 4) (gaps)\ndimension execution: HARNESS reason=live_not_run\n  check a: unobserved reason=live_not_run\nheld checks: 1\ncounts: xfail=0 xpass=0\n")}
		j := r
		j.Stdout = raw
		return c, r, j
	}
	for _, row := range []struct {
		name, leg string
		mutate    func(*caseassembly.Case, *result, *result)
	}{
		{"Run", "run_line", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte("not_measured"), []byte("red"), 1)
		}},
		{"Table", "word_table", func(c *caseassembly.Case, r, j *result) {
			c.Expect["exit_code"] = json.Number("0")
			r.Exit = 0
			j.Exit = 0
			r.Record["exit_code"] = json.Number("0")
			object(r.Record["judge_output"])["exit_code"] = json.Number("0")
			r.Stdout = bytes.Replace(r.Stdout, []byte("exit 4"), []byte("exit 0"), 1)
			raw, e := json.Marshal(r.Record["judge_output"])
			if e != nil {
				t.Fatal(e)
			}
			j.Stdout = raw
		}},
		{"JSONExit", "exit_equality", func(_ *caseassembly.Case, _, j *result) { j.Exit = 5 }},
		{"OutputExit", "exit_equality", func(_ *caseassembly.Case, r, _ *result) {
			object(r.Record["judge_output"])["exit_code"] = json.Number("5")
		}},
		{"ExpectedExit", "exit_equality", func(c *caseassembly.Case, _, _ *result) { c.Expect["exit_code"] = json.Number("5") }},
		{"SecondDocument", "json_document", func(_ *caseassembly.Case, _, j *result) { j.Stdout = append(j.Stdout, []byte("\n{}")...) }},
		{"Exit", "exit_equality", func(_ *caseassembly.Case, r, _ *result) { r.Record["exit_code"] = json.Number("5") }},
		{"Dimension", "dimensions", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte("HARNESS"), []byte("GREEN"), 1)
		}},
		{"Held", "held_count", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte("held checks: 1"), []byte("held checks: 2"), 1)
		}},
		{"Check", "listed_checks", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte("  check a: unobserved reason=live_not_run\n"), nil, 1)
		}},
		{"Label", "sub_label", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte(" (gaps)"), nil, 1)
		}},
		{"Counts", "counts", func(_ *caseassembly.Case, r, _ *result) {
			r.Stdout = bytes.Replace(r.Stdout, []byte("xfail=0"), []byte("xfail=1"), 1)
		}},
		{"JSON", "json_document", func(_ *caseassembly.Case, _, j *result) {
			j.Stdout = bytes.Replace(j.Stdout, []byte("HARNESS"), []byte("GREEN"), 1)
		}},
		{"Bytes", "record_bytes", func(_ *caseassembly.Case, _, j *result) { j.RecordBytes = []byte("changed") }},
	} {
		t.Run(row.name, func(t *testing.T) {
			c, r, j := makeFixture()
			if e := agreement(c, r, j); e != nil {
				t.Fatal("control", e)
			}
			row.mutate(&c, &r, &j)
			if e := agreement(c, r, j); e == nil || e.Error() != row.leg {
				t.Fatalf("want %s got %v", row.leg, e)
			}
		})
	}
	c := caseassembly.Case{Kind: "derivation-only", Expect: map[string]any{"exit_code": json.Number("2"), "rule": "QS5"}}
	r := result{Exit: 2, Rule: "QS5"}
	if e := agreement(c, r, r); e != nil {
		t.Fatal(e)
	}
	for _, row := range []struct {
		name, leg string
		mutate    func(*result)
	}{
		{"Stored", "derivation_record", func(r *result) { r.Record = map[string]any{} }},
		{"Rule", "derivation_rule", func(r *result) { r.Rule = "QS2" }},
		{"Exit", "derivation_exit", func(r *result) { r.Exit = 70 }},
		{"TextStdout", "derivation_stdout", func(r *result) { r.Stdout = []byte("unexpected\n") }},
		{"JSONStdout", "derivation_stdout", func(r *result) { r.Stdout = []byte("unexpected\n") }},
	} {
		t.Run("Derivation"+row.name, func(t *testing.T) {
			text, jsonRun := r, r
			if row.name == "JSONStdout" {
				row.mutate(&jsonRun)
			} else {
				row.mutate(&text)
			}
			if e := agreement(c, text, jsonRun); e == nil || e.Error() != row.leg {
				t.Fatalf("want %s got %v", row.leg, e)
			}
		})
	}
}
