package scorecard

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/record"
)

func fixture() object {
	return object{"request": "grade", "exit_code": 0, "scope": array{"conformance"}, "judge_input": object{"required": array{object{"id": "check", "dimension": "execution"}}, "evidence": array{object{"check_id": "check", "observed": object{"probe": "store_unchanged", "store": "data/store", "streams": array{}}}}}, "judge_output": object{"verdict": object{"plain": "green", "sub_label": nil}, "dimensions": array{object{"dimension": "execution", "state": "GREEN", "reason": nil}}, "checks": array{object{"id": "check", "result": "held_crediting", "reason": nil}}, "counts": object{"xfail": 0, "xpass": 0}, "comparison": object{"rule": "B1"}}, "next_actions": array{}, "offline_settings": array{}, "bookkeeping_excluded": array{}}
}
func render(t *testing.T, doc object, copy record.EvidenceCopy) string {
	t.Helper()
	var b bytes.Buffer
	if e := Render(&b, doc, copy, func(s string) string { return s }, nil); e != nil {
		t.Fatal(e)
	}
	return b.String()
}
func observed(doc object) object {
	return obj(obj(arr(obj(doc["judge_input"])["evidence"])[0])["observed"])
}
func output(doc object) object { return obj(doc["judge_output"]) }
func check(doc object) object  { return obj(arr(output(doc)["checks"])[0]) }
func TestDisplayRows(t *testing.T) {
	// QE14, QE12, QP7, QS5, QS7, QE21, QD1, QD2, QD3, QD4: each display has a twin differing only in its triggering value.
	for _, row := range []struct {
		name, want string
		set        func(object)
	}{
		{"Run", "pql grade: red (exit 0)", func(d object) { obj(output(d)["verdict"])["plain"] = "red" }},
		{"Label", "pql grade: green (exit 0) (known-defects)", func(d object) { obj(output(d)["verdict"])["sub_label"] = "known-defects" }},
		{"Dimension", "dimension execution: HARNESS reason=harness_fault", func(d object) {
			output(d)["dimensions"] = array{object{"dimension": "execution", "state": "HARNESS", "reason": "harness_fault"}}
		}},
		{"Failed", "  check check: failed", func(d object) { check(d)["result"] = "failed" }},
		{"Expected", "  check check: failed_expected", func(d object) { check(d)["result"] = "failed_expected" }},
		{"Probe", "    probe: store_unchanged", func(d object) { check(d)["result"] = "unobserved" }},
		{"Action", "  next action: synthetic action", func(d object) {
			d["next_actions"] = array{object{"check_id": nil, "dimension": "execution", "action": "synthetic action"}}
		}},
		{"CheckAction", "  next action: retry check", func(d object) {
			check(d)["result"] = "failed"
			d["next_actions"] = array{object{"check_id": "check", "dimension": nil, "action": "retry check"}}
		}},
		{"Bookkeeping", "bookkeeping excluded: state/path", func(d object) { d["bookkeeping_excluded"] = array{"state/path"} }},
		{"Offline", "offline setting 1: applies_to=[[\"show\"]] args=[\"--offline\"] env_names=[\"MODE\"]", func(d object) {
			d["offline_settings"] = array{object{"applies_to": array{array{"show"}}, "args": array{"--offline"}, "env_names": array{"MODE"}}}
		}},
		{"Placeholder", "placeholder: /commands/0/class", func(d object) { observed(d)["placeholders"] = array{"/commands/0/class"} }},
		{"Short", "MODE: not masked: shorter than 8 bytes", func(d object) { observed(d)["short_env_names"] = array{"MODE"} }},
		{"Found", "  found command: [\"found\"]", func(d object) { observed(d)["discovery"] = object{"ran": true, "found_names": array{array{"found"}}} }},
		{"Missed", "  missing command: [\"missed\"]", func(d object) {
			observed(d)["discovery"] = object{"ran": true, "missing_names": array{array{"missed"}}}
		}},
		{"Undeclared", "  undeclared command: [\"extra\"]", func(d object) {
			observed(d)["discovery"] = object{"ran": true, "undeclared_names": array{array{"extra"}}}
		}},
		{"Counts", "discovery: found=2 declared=3", func(d object) { observed(d)["discovery"] = object{"ran": true, "found_count": 2, "declared_count": 3} }},
		{"RequestBound", "discovery request bound reached", func(d object) { observed(d)["discovery"] = object{"ran": true, "request_bound_reached": true} }},
		{"DepthBound", "discovery depth bound reached", func(d object) { observed(d)["discovery"] = object{"ran": true, "depth_bound_reached": true} }},
		{"NotRun", "discovery did not run", func(d object) { observed(d)["discovery"] = object{"ran": false} }},
		{"FoundCut", "found names preview was cut", func(d object) { observed(d)["discovery"] = object{"ran": true, "found_cut": true} }},
		{"UndeclaredCut", "undeclared names shown: 0 of 5000; remaining names counted but not named", func(d object) {
			observed(d)["discovery"] = object{"ran": true, "undeclared_cut": true, "undeclared_count": 5000}
		}},
		{"Comparison", "  comparison entry: old missing E1", func(d object) {
			output(d)["comparison"] = object{"result": "not_measured", "entries": array{object{"check_id": "old", "class": "missing", "rule": "E1"}}, "next_action": "re_anchor"}
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			d := fixture()
			plain := render(t, d, nil)
			if strings.Contains(plain, row.want) {
				t.Fatal("twin already contains trigger", row.want)
			}
			row.set(d)
			if got := render(t, d, nil); !strings.Contains(got, row.want) {
				t.Fatal(row.want, got)
			}
		})
	}
	d := fixture()
	text := render(t, d, nil)
	for _, want := range []string{`scope: ["conformance"]`, "bookkeeping excluded: .pql-tmp", "bookkeeping excluded: data/store-shm", "bookkeeping excluded: data/store-wal (when empty)", "bookkeeping excluded: data/store-journal (when empty)", "Directories on the way to bookkeeping paths are also left out.", "no baseline for this scope", "held checks: 1"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
	observed(d)["store"] = nil
	if text = render(t, d, nil); strings.Contains(text, "data/store-") {
		t.Fatal(text)
	}
}
func TestAnchorRows(t *testing.T) {
	// QE21: anchor words and own verdicts occupy different lines; a refused anchor names its cause.
	for _, row := range []struct {
		word, plain, label, reason string
		exit                       int
	}{{"anchored", "green", "", "", 0}, {"anchored", "known_defect", "known-defects", "", 0}, {"refused", "red", "", "red", 5}} {
		d := fixture()
		d["request"] = "anchor"
		d["exit_code"] = row.exit
		output(d)["verdict"] = nil
		var label any
		if row.label != "" {
			label = row.label
		}
		output(d)["own_verdict"] = object{"plain": row.plain, "sub_label": label}
		a := object{"result": row.word, "reason": nil, "checks": array{}}
		output(d)["anchor"] = a
		if row.reason != "" {
			a["reason"] = row.reason
			a["checks"] = array{"check"}
		}
		text := render(t, d, nil)
		first := strings.Split(text, "\n")[0]
		ownLine := "own verdict: " + row.plain
		if row.label != "" {
			ownLine += " (" + row.label + ")"
		}
		if first != fmt.Sprintf("pql anchor: %s (exit %d)", row.word, row.exit) || !strings.Contains(text, ownLine+"\n") {
			t.Fatal(text)
		}
		if row.label == "" && strings.Contains(text, "known-defects") {
			t.Fatal(text)
		}
		if row.reason != "" && (!strings.Contains(text, "anchor reason: red") || !strings.Contains(text, "anchor check: check")) {
			t.Fatal(text)
		}
	}
}
func TestCheckSelection(t *testing.T) {
	// QE14: out_of_scope is neither listed nor counted, and held checks are counted only.
	for _, result := range []string{"held_crediting", "held_non_crediting", "out_of_scope", "failed", "failed_expected", "unobserved"} {
		d := fixture()
		check(d)["result"] = result
		text := render(t, d, nil)
		listed := result == "failed" || result == "failed_expected" || result == "unobserved"
		if strings.Contains(text, "  check check:") != listed {
			t.Fatal(result, text)
		}
		held := 0
		if strings.HasPrefix(result, "held_") {
			held = 1
		}
		if !strings.Contains(text, fmt.Sprintf("held checks: %d\n", held)) {
			t.Fatal(text)
		}
		if result == "out_of_scope" && strings.Contains(text, "out_of_scope") {
			t.Fatal(text)
		}
	}
}
func TestComparisonWords(t *testing.T) {
	// QE14: the tool-description-efficacy dimension carries every judge comparison word.
	for _, word := range []string{"improved", "no_material_change"} {
		d := fixture()
		check(d)["detail"] = word
		plain := render(t, d, nil)
		if !strings.Contains(plain, "dimension execution:") || strings.Contains(plain, "comparison word") {
			t.Fatal(plain)
		}
		obj(arr(obj(d["judge_input"])["required"])[0])["dimension"] = "tool-description-efficacy"
		obj(arr(output(d)["dimensions"])[0])["dimension"] = "tool-description-efficacy"
		check(d)["detail"] = word
		if text := render(t, d, nil); !strings.Contains(text, "  comparison word check: "+word) {
			t.Fatal(text)
		}
	}
}
func TestPreBoundNames(t *testing.T) {
	// QE14, QE10, QS5, QD3, QA4: names survive the bound while decisions remain those in the stored document.
	d := fixture()
	check(d)["result"] = "unobserved"
	observed(d)["probe"] = "discovery"
	names := object{"probe": nil, "placeholders": array{"/binary"}, "discovery": object{"ran": true, "undeclared_names": array{array{"extra"}}}, "invocations": array{object{"computation": array{"compute"}}}, "streams": array{object{"stderr": object{"head": "failure", "cut": true}}}}
	obj(arr(obj(d["judge_input"])["evidence"])[0])["observed"] = object{"digest": "synthetic"}
	copy := record.EvidenceCopy{"check": record.EvidenceValues{Observed: names}}
	text := render(t, d, copy)
	for _, want := range []string{"placeholder: /binary", "  undeclared command: [\"extra\"]", "computation=[\"compute\"]", "computation stderr: failure", "computation stderr was cut", "pql grade: green (exit 0)"} {
		if !strings.Contains(text, want) {
			t.Fatal(want, text)
		}
	}
	if !reflect.DeepEqual(obj(arr(obj(d["judge_input"])["evidence"])[0])["observed"], object{"digest": "synthetic"}) {
		t.Fatal("record changed")
	}
}

func TestPlaceholderPlaces(t *testing.T) {
	// QS5: all eight allowed placeholder locations print as JSON Pointers, with an empty twin.
	for i, location := range []string{"/binary", "/declared_count", "/seed", "/store", "/secret", "/facts/a~1b~0", "/commands/0/class", "/commands/0/writes"} {
		t.Run(fmt.Sprintf("Place%d", i), func(t *testing.T) {
			d := fixture()
			plain := render(t, d, nil)
			if strings.Contains(plain, "placeholder:") {
				t.Fatal("plain control")
			}
			observed(d)["placeholders"] = array{location}
			if text := render(t, d, nil); !strings.Contains(text, "placeholder: "+location+"\n") {
				t.Fatal(text)
			}
		})
	}
}
func TestDiscoveryDidNotRun(t *testing.T) {
	// QD4, QD1: missed names and counts describe a walk only when it ran.
	for _, ran := range []bool{false, true} {
		d := fixture()
		observed(d)["discovery"] = object{"ran": ran, "found_count": 0, "declared_count": 3, "missing_names": array{array{"missed"}}}
		text := render(t, d, nil)
		if strings.Contains(text, "missing command:") != ran || strings.Contains(text, "discovery: found=0 declared=3") != ran || strings.Contains(text, "discovery did not run") == ran {
			t.Fatal(text)
		}
	}
}
func TestComparisonEntries(t *testing.T) {
	// QE14: baseline entries, new checks and R2's action are the judge's own values.
	d := fixture()
	output(d)["comparison"] = object{"result": "not_measured", "entries": array{object{"check_id": "old", "class": "missing", "rule": "E1"}}, "new": array{"new"}, "next_action": "re_anchor"}
	text := render(t, d, nil)
	for _, line := range []string{"comparison: not_measured", "  comparison entry: old missing E1", "  new check: new", "  next action: re_anchor"} {
		if !strings.Contains(text, line) {
			t.Fatal(text)
		}
	}
	if strings.Contains(text, "no baseline for this scope") {
		t.Fatal(text)
	}
}

func TestComputationCut(t *testing.T) {
	// QA4: the masked standard-error head is always shown, and cut status is explicit only when cut.
	for _, cut := range []bool{false, true} {
		d := fixture()
		check(d)["result"] = "unobserved"
		observed(d)["invocations"] = array{object{"computation": array{"compute"}}}
		observed(d)["streams"] = array{object{"stderr": object{"head": "synthetic stderr", "cut": cut}}}
		text := render(t, d, nil)
		if !strings.Contains(text, "computation stderr: synthetic stderr") || strings.Contains(text, "computation stderr was cut") != cut {
			t.Fatal(text)
		}
	}
}

func TestMalformedLookupValues(t *testing.T) {
	// QE14: malformed display metadata is rendered without interface-comparison panics.
	d := fixture()
	dim := array{"execution"}
	obj(arr(obj(d["judge_input"])["required"])[0])["dimension"] = dim
	obj(arr(output(d)["dimensions"])[0])["dimension"] = dim
	d["next_actions"] = array{object{"check_id": nil, "dimension": dim, "action": "retry"}}
	if got := render(t, d, nil); !strings.Contains(got, `dimension ["execution"]: GREEN`) {
		t.Fatal(got)
	}
}
func TestFaultOrder(t *testing.T) {
	// E-4, E-11, QE21: process faults follow the run line before anchor and scope details.
	d := fixture()
	d["request"] = "anchor"
	output(d)["anchor"] = object{"result": "anchored"}
	output(d)["own_verdict"] = object{"plain": "green"}
	var b bytes.Buffer
	if e := Render(&b, d, nil, func(s string) string { return s }, []string{"store failed", "cleanup failed"}); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(b.String(), "pql anchor: anchored (exit 0)\nexits 70: store failed\nexits 70: cleanup failed\nown verdict: green\n") {
		t.Fatal(b.String())
	}
}
