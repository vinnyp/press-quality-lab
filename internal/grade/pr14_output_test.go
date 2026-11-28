package grade

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

func TestPR14JudgeAuthority(t *testing.T) {
	// QE14: the scorecard reads a well-formed judge document even when its facts contradict recomputation.
	o, d, saved := fixture(t)
	calls := 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		raw, code := judge.Judge(b)
		if calls == 1 {
			return raw, code
		}
		v, ok := strictjson.Read(raw, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
		if !ok {
			t.Fatal("judge document")
		}
		doc := subjectObject(v)
		doc["exit_code"] = strictjson.Number("0")
		doc["verdict"] = strictjson.Object{"token": "FAILED", "plain": "red", "sub_label": nil}
		subjectObject(subjectArray(doc["dimensions"])[0])["state"] = "XPASS"
		doc["counts"] = strictjson.Object{"xfail": strictjson.Number("17"), "xpass": strictjson.Number("19")}
		return strictjson.Write(doc), 0
	}
	var out bytes.Buffer
	code, err := Run(o, d, &out)
	if err != nil || code != 0 {
		t.Fatal(code, err)
	}
	dim, ok := subjectObject(subjectArray(subjectObject(saved.JudgeOutput)["dimensions"])[0])["dimension"].(string)
	if !ok {
		t.Fatal("missing dimension")
	}
	for _, line := range []string{"pql grade: red (exit 0)", "dimension " + dim + ": XPASS", "counts: xfail=17 xpass=19"} {
		if !strings.Contains(out.String(), line) {
			t.Fatal(line, out.String())
		}
	}
}

func TestPR14TwoModes(t *testing.T) {
	// QE14, QE19: changing presentation leaves the durable bytes and exit unchanged.
	o, d, _ := fixture(t)
	d.Write = nil
	roots := []string{filepath.Join(t.TempDir(), "text"), filepath.Join(t.TempDir(), "json")}
	var records [][]byte
	var text, json bytes.Buffer
	var codes []int
	for i, root := range roots {
		d.StoreRoot = func() (string, error) { return root, nil }
		o.JSON = i == 1
		var out *bytes.Buffer
		if i == 0 {
			out = &text
		} else {
			out = &json
		}
		code, e := Run(o, d, out)
		if e != nil {
			t.Fatal(e)
		}
		codes = append(codes, code)
		paths, e := filepath.Glob(filepath.Join(root, "*", "*.json"))
		if e != nil || len(paths) != 1 {
			t.Fatal(paths, e)
		}
		raw, e := os.ReadFile(paths[0])
		if e != nil {
			t.Fatal(e)
		}
		records = append(records, raw)
	}
	if codes[0] != codes[1] || !bytes.Equal(records[0], records[1]) {
		t.Fatal(codes, "records differ")
	}

	if !strings.HasPrefix(text.String(), "pql grade:") || !strings.HasPrefix(json.String(), "{") {
		t.Fatal(text.String(), json.String())
	}
}

func TestPR14Funnel(t *testing.T) {
	// QE14, QE20, QE16, E-4, E-11, E-6: presentation waits for all cleanup and reports every post-judge fault.
	for _, row := range []struct {
		name                   string
		store, cleanup, signal bool
	}{{"Clean", false, false, false}, {"Store", true, false, false}, {"Cleanup", false, true, false}, {"Signal", false, false, true}, {"Both", true, true, false}} {
		t.Run(row.name, func(t *testing.T) {
			o, d, _ := fixture(t)
			var diagnostic bytes.Buffer
			d.Stderr = &diagnostic
			signals := make(chan os.Signal, 2)
			d.Signals = signals
			if row.store {
				d.Write = func(store.Record) error { return errors.New("synthetic store failure") }
			}
			if row.cleanup {
				d.Remove = func(*throwaway.Workspace) error { return errors.New("synthetic cleanup failure") }
			}
			if row.signal {
				d.Write = func(store.Record) error { signals <- syscall.SIGTERM; return nil }
			}
			var out bytes.Buffer
			code, err := Run(o, d, &out)
			if row.name == "Clean" {
				if code != 4 || err != nil {
					t.Fatal(code, err)
				}
			} else if code != 70 || err == nil {
				t.Fatal(code, err)
			}
			for _, want := range []struct {
				on   bool
				text string
			}{{row.store, "record was not stored"}, {row.cleanup, "cleanup failed; directory left behind:"}, {row.signal, "interrupted by terminated"}} {
				if strings.Contains(out.String(), "exits 70: "+want.text) != want.on {
					t.Fatal(want, out.String())
				}
			}
			wantFaults := 0
			if row.store {
				wantFaults++
			}
			if row.cleanup {
				wantFaults++
			}
			if row.signal {
				wantFaults++
			}
			if strings.Count(out.String(), "exits 70: ") != wantFaults {
				t.Fatal("fault multiplicity", out.String())
			}
			if !strings.HasPrefix(out.String(), "pql grade: not_measured (exit 4)") {
				t.Fatal(out.String())
			}
		})
	}
}

func TestPR14RejectedModes(t *testing.T) {
	// QE15, QE6: final and precheck judge rejections have empty stdout in both modes.
	for _, jsonMode := range []bool{false, true} {
		for _, precheck := range []bool{false, true} {
			o, d, saved := fixture(t)
			o.JSON = jsonMode
			calls := 0
			d.Judge = func(b []byte) ([]byte, int) {
				calls++
				if calls == 1 && !precheck {
					return judge.Judge(b)
				}
				return []byte(`{"exit_code":2,"outcome":"rejected","rejected_by":"X12"}`), 2
			}
			var out bytes.Buffer
			code, e := Run(o, d, &out)
			if e == nil || code != 70 || out.Len() != 0 {
				t.Fatal(code, e, out.String())
			}
			var classified *Error
			wantRule := "X12"
			if precheck {
				wantRule = "QE6"
			}
			if !errors.As(e, &classified) || classified.Rule != wantRule {
				t.Fatal(e)
			}
			if (saved.JudgeInput == nil) != precheck {
				t.Fatal("wrong storage behavior", precheck)
			}
		}
	}
}

func TestPR14CleanupWithoutScorecard(t *testing.T) {
	// QE20, E-11: cleanup paths use stderr when there is no decided document, in either output mode.
	for _, jsonMode := range []bool{false, true} {
		o, d, _ := fixture(t)
		o.JSON = jsonMode
		d.Open = func(string) (io.ReadCloser, error) { return nil, errors.New("synthetic unreadable subject") }
		d.Remove = func(*throwaway.Workspace) error { return errors.New("synthetic cleanup failure") }
		var out, diagnostic bytes.Buffer
		d.Stderr = &diagnostic
		code, e := Run(o, d, &out)
		if code != 70 || e == nil || out.Len() != 0 || !strings.Contains(diagnostic.String(), "cleanup failed; directory left behind:") {
			t.Fatal(code, e, out.String(), diagnostic.String())
		}
	}
}

func TestPR14PostJudgeMask(t *testing.T) {
	// QE18: post-judge panic text is masked before the raw scorecard writer.
	for _, message := range []string{"synthetic_value", "plain failure"} {
		o, d, _ := fixture(t)
		data := strings.Replace(validSubject, `"offline":[]`, `"offline":[{"applies_to":[[]],"args":[],"env":{"VALUE":"synthetic_value"}}]`, 1)
		if e := os.WriteFile(o.SubjectFile, []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
		d.Write = func(store.Record) error { panic(message) }
		var out bytes.Buffer
		code, e := Run(o, d, &out)
		if code != 70 || e == nil {
			t.Fatal(code, e)
		}
		want := message
		if message == "synthetic_value" {
			want = "***************"
		}
		if !strings.Contains(out.String(), "engine panic: "+want) || strings.Contains(out.String(), "synthetic_value") {
			t.Fatal(out.String())
		}
	}
}
