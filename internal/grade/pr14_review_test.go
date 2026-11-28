package grade

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestPR14TextRun(t *testing.T) {
	// KTD16, QS5, QS7, QP7, QE18, E-11: text-mode wiring masks and escapes every display source after cleanup.
	o, d, _ := fixture(t)
	v, ok := strictjson.Read([]byte(validSubject), strictjson.Bounds{Bytes: 1 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		t.Fatal("fixture")
	}
	s := subjectObject(v)
	word := "show\x1b\u200d"
	path := "state\x1b\u200d"
	subjectObject(subjectArray(s["commands"])[0])["words"] = plan.Array{word}
	subjectObject(subjectArray(s["commands"])[0])["class"] = plan.Object{"placeholder": "supply"}
	s["offline"] = plan.Array{plan.Object{"applies_to": plan.Array{plan.Array{word}}, "args": plan.Array{"--offline"}, "env": plan.Object{"MODE": "synthetic_value"}}}
	s["bookkeeping"] = plan.Array{path}
	if e := os.WriteFile(o.SubjectFile, strictjson.Write(s), 0600); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(t.TempDir(), "synthetic_value\x1b\u200d")
	d.Workspace = func() (*throwaway.Workspace, error) { return throwaway.BeginAt(root) }
	var cleanupPath string
	d.Remove = func(w *throwaway.Workspace) error {
		cleanupPath = w.Path()
		return errors.New("synthetic cleanup failure")
	}
	var out bytes.Buffer
	code, e := Run(o, d, &out)
	if code != 70 || e == nil {
		t.Fatal(code, e)
	}
	got := out.String()
	for _, r := range got {
		if r != '\n' && terminalUnsafe(r) {
			t.Fatalf("unsafe %U", r)
		}
	}
	maskedPath := strings.ReplaceAll(cleanupPath, "synthetic_value", "***************")
	maskedPath = strings.ReplaceAll(strings.ReplaceAll(maskedPath, "\x1b", `\u{001b}`), "\u200d", `\u{200d}`)
	for _, line := range []string{
		`scope: ["conformance"]`,
		`invocation: words=["show\u001b\u{200d}"]`,
		`placeholder: /commands/0/class`,
		`offline setting 1: applies_to=[["show\u001b\u{200d}"]] args=["--offline"] env_names=["MODE"]`,
		`bookkeeping excluded: state\u{001b}\u{200d}`,
		"exits 70: cleanup failed; directory left behind: " + maskedPath,
	} {
		if !strings.Contains(got, line) {
			t.Fatal("missing", line, got)
		}
	}
	lines := strings.Split(got, "\n")
	if lines[1] != "exits 70: cleanup failed; directory left behind: "+maskedPath {
		t.Fatal("fault order", got)
	}
}

func TestPR14RunBeforeBound(t *testing.T) {
	// QE10, QE14, QD3: grade passes the masked copy from before the evidence bound to the renderer.
	for _, large := range []bool{false, true} {
		o, d, saved := fixture(t)
		d.Collect = func(p plan.Plan, _ observe.Source) ([]collect.Check, error) {
			checks := make([]collect.Check, 0, len(p.Checks))
			for _, ch := range p.Checks {
				c := collect.Check{Plan: ch, Result: decide.Result{Outcome: decide.NotObserved, Reason: "could_not_start"}}
				if ch.Preimage["probe"] == "discovery" {
					c.Discovery = &decide.Discovery{Ran: true, UndeclaredNames: []plan.Array{{"before_bound_only"}}, UndeclaredCount: 1}
					if large {
						for i := 0; i < 70; i++ {
							c.Invocations = append(c.Invocations, observe.Invocation{Run: observe.Run{Stdout: []byte(strings.Repeat("x", 64<<10)), StdoutSeen: 64 << 10}})
						}
					}
				}
				checks = append(checks, c)
			}
			return checks, nil
		}
		var out bytes.Buffer
		code, e := Run(o, d, &out)
		if e != nil || code != 4 {
			t.Fatal(code, e)
		}
		found := false
		for _, v := range subjectArray(subjectObject(saved.JudgeInput)["evidence"]) {
			ob := subjectObject(subjectObject(v)["observed"])
			if ob["sha256"] != nil {
				found = true
			}
		}
		if found != large {
			t.Fatal("bound", large, found)
		}
		if bytes.Contains(strictjson.Write(saved.JudgeInput), []byte("before_bound_only")) == large {
			t.Fatal("stored name did not follow the evidence bound", large)
		}
		if !strings.Contains(out.String(), `undeclared command: ["before_bound_only"]`) {
			t.Fatal(out.String())
		}
	}
}

func TestPR14EngineKeys(t *testing.T) {
	// QE18, QD3: a declared value cannot erase engine-owned observed keys or the undeclared list.
	o, d, saved := fixture(t)
	data := strings.Replace(validSubject, `"offline":[]`, `"offline":[{"applies_to":[[]],"args":[],"env":{"VALUE":"undeclared_names"}}]`, 1)
	if e := os.WriteFile(o.SubjectFile, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	d.Collect = func(p plan.Plan, _ observe.Source) ([]collect.Check, error) {
		var checks []collect.Check
		for _, ch := range p.Checks {
			c := collect.Check{Plan: ch, Result: decide.Result{Outcome: decide.NotObserved, Reason: "could_not_start"}}
			if ch.Preimage["probe"] == "discovery" {
				c.Discovery = &decide.Discovery{Ran: true, UndeclaredNames: []plan.Array{{"extra"}}, UndeclaredCount: 1}
			}
			checks = append(checks, c)
		}
		return checks, nil
	}
	var out bytes.Buffer
	code, e := Run(o, d, &out)
	if code != 4 || e != nil {
		t.Fatal(code, e)
	}
	if !bytes.Contains(strictjson.Write(saved.JudgeInput), []byte(`"undeclared_names":[["extra"]]`)) || !strings.Contains(out.String(), `undeclared command: ["extra"]`) {
		t.Fatal(out.String())
	}
}
