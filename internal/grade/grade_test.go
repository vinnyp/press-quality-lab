package grade

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

const validSubject = `{"subject_file_version":"1.0","binary":"bin/subject","facts":{"read_only":false,"has_destructive_ops":false,"store_backed":false,"has_cli":true,"bounding":"declared"},"commands":[{"words":[],"class":"safe","writes":false,"exit_codes":[0],"byte_bound":4096,"budget_seconds":10}],"declared_count":0,"seed":null,"store":null,"offline":[],"bookkeeping":[],"secret":null,"runbooks":[],"markers":[],"prompts":{"version":"none"}}`

func fixture(t *testing.T) (Options, Dependencies, *store.Record) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "subject"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "subject.json")
	if err := os.WriteFile(path, []byte(validSubject), 0600); err != nil {
		t.Fatal(err)
	}
	saved := new(store.Record)
	return Options{SubjectFile: path, Scope: []string{"conformance"}}, Dependencies{Workspace: func() (*throwaway.Workspace, error) {
		w, err := throwaway.BeginAt(filepath.Join(dir, "workspaces"))
		if err == nil && !strings.HasPrefix(w.Path(), dir+string(os.PathSeparator)) {
			t.Fatal("test resolved real workspace parent")
		}
		return w, err
	}, IDs: func() (int, int) { return 1000, 1000 }, Write: func(r store.Record) error { *saved = r; return nil }}, saved
}
func assertFailure(t *testing.T, code int, err error, want int, rule, cause string) {
	t.Helper()
	var e *Error
	if code != want || !errors.As(err, &e) || e.Rule != rule || e.Cause != cause {
		t.Fatalf("code=%d error=%v; want %d %s %s", code, err, want, rule, cause)
	}
}

func TestPrecheckRows(t *testing.T) {
	// QE5, QE6: every X1 to X12 row's literal cause and exit code.
	rows := []struct {
		rule  string
		code  int
		cause string
	}{
		{"X1", 2, "user"},
		{"X2", 70, "engine"},
		{"X3", 70, "engine"},
		{"X4", 2, "user"},
		{"X5", 70, "engine"},
		{"X6", 70, "engine"},
		{"X7", 70, "engine"},
		{"X8", 70, "engine"},
		{"X9", 70, "engine"},
		{"X10", 70, "engine"},
		{"X11", 2, "user"},
		{"X12", 70, "engine"},
	}
	for _, row := range rows {
		t.Run(row.rule, func(t *testing.T) {
			o, d, _ := fixture(t)
			calls := 0
			d.Judge = func(input []byte) ([]byte, int) {
				calls++
				v, ok := strictjson.Read(input, strictjson.Bounds{Bytes: 16 << 20, Depth: 64}, nil)
				doc, object := v.(strictjson.Object)
				if !ok || !object {
					t.Fatal("invalid precheck")
				}
				ev, ok := doc["evidence"].(strictjson.Array)
				req, reqOK := doc["required"].(strictjson.Array)
				if !ok || len(ev) != 0 || !reqOK || len(req) == 0 || doc["request"] != "grade" {
					t.Fatal("wrong precheck", doc)
				}
				return []byte(fmt.Sprintf(`{"exit_code":2,"outcome":"rejected","rejected_by":%q}`, row.rule)), 2
			}
			d.Source = func() (observe.Source, func() error, error) {
				t.Fatal("collection after rejection")
				return nil, nil, nil
			}
			d.StoreRoot = func() (string, error) { t.Fatal("store after rejection"); return "", nil }
			code, err := Run(o, d, io.Discard)
			assertFailure(t, code, err, row.code, "QE6", row.rule+": "+row.cause+" caused pre-check rejection")
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func TestScopeOrdering(t *testing.T) {
	// QE5, QE6, X1, E-7: unknown scope reaches the judge unless planning has a gap.
	o, d, _ := fixture(t)
	o.Scope = []string{"unknown"}
	code, err := Run(o, d, io.Discard)
	assertFailure(t, code, err, 2, "QE6", "X1: user caused pre-check rejection")
	if err := os.WriteFile(o.SubjectFile, []byte(strings.Replace(validSubject, `"seed":null`, `"seed":{"placeholder":"seed"}`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	d.Judge = func([]byte) ([]byte, int) { t.Fatal("judge before planning gap"); return nil, 0 }
	code, err = Run(o, d, io.Discard)
	assertFailure(t, code, err, 70, "QE2", "planning rule not built")
}

func TestJudgeAuthority(t *testing.T) {
	// QE14, QE16, KTD16: output, stored document and exit follow the judge.
	o, d, saved := fixture(t)
	calls := 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		if calls == 1 {
			return judge.Judge(b)
		}
		return []byte(`{"exit_code":3,"outcome":"graded","verdict":"DOCTORED"}`), 3
	}
	var out bytes.Buffer
	code, err := Run(o, d, &out)
	if err != nil || code != 3 || saved.ExitCode != 3 || out.String() != "{\"exit_code\":3,\"outcome\":\"graded\",\"verdict\":\"DOCTORED\"}\n" {
		t.Fatalf("code=%d err=%v stored=%+v out=%q", code, err, saved, out.String())
	}
	doc, ok := saved.JudgeOutput.(strictjson.Object)
	if !ok || doc["verdict"] != "DOCTORED" {
		t.Fatal(saved.JudgeOutput)
	}
	calls = 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		if calls == 1 {
			return judge.Judge(b)
		}
		return []byte(`{"exit_code":3}`), 4
	}
	code, err = Run(o, d, io.Discard)
	assertFailure(t, code, err, 70, "QE16", "judge exit code disagreement")
}

func TestRejectedRecord(t *testing.T) {
	// QE15, E-5: preserve a schema-invalid input rejected after collection; no verdict.
	o, d, _ := fixture(t)
	root := filepath.Join(t.TempDir(), "store")
	d.Write = nil
	d.StoreRoot = func() (string, error) { return root, nil }
	calls := 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		if calls == 1 {
			return judge.Judge(b)
		}
		return []byte(`{"exit_code":2,"outcome":"rejected","rejected_by":"X12","verdict":"MUST_NOT_PRINT"}`), 2
	}
	var out bytes.Buffer
	code, err := Run(o, d, &out)
	assertFailure(t, code, err, 70, "X12", "judge rejected collected record")
	paths, globErr := filepath.Glob(filepath.Join(root, "*", "*.json"))
	if globErr != nil || len(paths) != 1 {
		t.Fatal(paths, globErr)
	}
	data, readErr := os.ReadFile(paths[0])
	if readErr != nil {
		t.Fatal(readErr)
	}
	if out.Len() != 0 || !bytes.Contains(data, []byte(`"judge_rejected":true`)) || !bytes.Contains(data, []byte(`"exit_code":70`)) || !bytes.Contains(data, []byte(`"judge_input":{`)) {
		t.Fatalf("out=%q record=%s", out.String(), data)
	}
}

func TestEffectiveRoot(t *testing.T) {
	// QH3: effective identity wins before store resolution, reading or collection.
	o, d, _ := fixture(t)
	d.IDs = func() (int, int) { return 1000, 0 }
	d.Open = func(string) (io.ReadCloser, error) { t.Fatal("read as root"); return nil, nil }
	d.StoreRoot = func() (string, error) { t.Fatal("resolved store as root"); return "", nil }
	code, err := Run(o, d, io.Discard)
	assertFailure(t, code, err, 70, "QH3", "effective user is root")
}

func TestProductionEffectiveID(t *testing.T) {
	// QH3: pin the production seam to the effective-id syscall, not the real id.
	data, err := os.ReadFile("grade.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("return os.Getuid(), os.Geteuid()")) {
		t.Fatal("production identity seam lost Geteuid")
	}
}

type source struct {
	panicControls bool
	err           error
	observed      *bool
}

func (s source) Controls() (observe.Controls, error) {
	if s.panicControls {
		panic("collector")
	}
	return observe.Controls{}, s.err
}
func (s source) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	if s.observed != nil {
		*s.observed = true
	}
	panic("Observe must not run")
}

func TestFunnel(t *testing.T) {
	// KTD17, QE16: recover on both goroutines; close the trace on every acquired path.
	for _, row := range []string{"Success", "Fault", "MainPanic", "CollectorPanic", "CloseError"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			closed := 0
			d.Source = func() (observe.Source, func() error, error) {
				close := func() error {
					closed++
					if row == "CloseError" {
						return errors.New("close")
					}
					return nil
				}
				if row == "Fault" {
					return nil, close, failure(70, "QT5", "missing invocation")
				}
				return source{panicControls: row == "CollectorPanic"}, close, nil
			}
			if row == "MainPanic" {
				d.Write = func(store.Record) error { panic("main") }
			}
			code, err := Run(o, d, io.Discard)
			if closed != 1 {
				t.Fatalf("close count %d", closed)
			}
			if row == "Success" {
				if err != nil || code != 4 {
					t.Fatalf("%d %v", code, err)
				}
			} else {
				var classified *Error
				wantRule := "QE16"
				if row == "Fault" {
					wantRule = "QT5"
				}
				if code != 70 || !errors.As(err, &classified) || classified.Rule != wantRule {
					t.Fatalf("%d %v want %s", code, err, wantRule)
				}
			}
		})
	}
}

func TestReleasedNoObserve(t *testing.T) {
	// QO1, E-15, KTD21: every required check is unverifiable; no invocation or home.
	o, d, saved := fixture(t)
	observed := false
	d.Source = func() (observe.Source, func() error, error) {
		return source{observed: &observed}, func() error { return nil }, nil
	}
	code, err := Run(o, d, io.Discard)
	if code != 4 || err != nil || observed {
		t.Fatalf("code=%d err=%v observed=%v", code, err, observed)
	}
	doc, ok := saved.JudgeInput.(strictjson.Object)
	if !ok {
		t.Fatal("no stored input")
	}
	evidence, ok := doc["evidence"].(strictjson.Array)
	if !ok || len(evidence) == 0 {
		t.Fatal("no evidence")
	}
	for _, v := range evidence {
		e, ok := v.(strictjson.Object)
		if !ok || e["outcome"] != "not_observed" || e["reason"] != "network_unverifiable" {
			t.Fatal(v)
		}
	}
}

type badReader struct{ readErr, closeErr error }

type closeReader struct {
	io.Reader
	err error
}

func (r closeReader) Close() error { return r.err }

func (b badReader) Read([]byte) (int, error) { return 0, b.readErr }
func (b badReader) Close() error             { return b.closeErr }
func TestSubjectReader(t *testing.T) {
	// QS2, QE16: strict parse and all open/read/close failures use the file path.
	for _, row := range []string{"Valid", "Duplicate", "Oversized", "Open", "Read", "Close"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			switch row {
			case "Duplicate":
				if err := os.WriteFile(o.SubjectFile, []byte(strings.Replace(validSubject, `"subject_file_version":"1.0"`, `"subject_file_version":"1.0","subject_file_version":"1.0"`, 1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "Oversized":
				data := append([]byte(validSubject), bytes.Repeat([]byte(" "), (16<<20)-len(validSubject)+1)...)
				if err := os.WriteFile(o.SubjectFile, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "Open":
				o.SubjectFile = filepath.Join(t.TempDir(), "missing")
			case "Read":
				d.Open = func(string) (io.ReadCloser, error) { return badReader{readErr: errors.New("read")}, nil }
			case "Close":
				d.Open = func(string) (io.ReadCloser, error) {
					return closeReader{Reader: strings.NewReader(validSubject), err: errors.New("close")}, nil
				}
			}
			code, err := Run(o, d, io.Discard)
			if row == "Valid" {
				if code != 4 || err != nil {
					t.Fatalf("%d %v", code, err)
				}
			} else if code != 2 || err == nil {
				t.Fatalf("%d %v", code, err)
			}
		})
	}
}

func TestEscaping(t *testing.T) {
	// KTD16: every specified terminal-control family is escaped in the funnel.
	o, d, _ := fixture(t)
	d.Collect = func(plan.Plan, observe.Source) ([]collect.Check, error) {
		return nil, errors.New("bad\x1b[31m\x00\x7f\u0085\u202e\u200d\u2028\u2029\U000e0061")
	}
	code, err := Run(o, d, io.Discard)
	assertFailure(t, code, err, 70, "QE16", "bad\x1b[31m\x00\x7f\u0085\u202e\u200d\u2028\u2029\U000e0061")
	if got := err.Error(); got != `rejected: QE16: bad\u{001b}[31m\u{0000}\u{007f}\u{0085}\u{202e}\u{200d}\u{2028}\u{2029}\u{e0061}` {
		t.Fatal(got)
	}
}

func TestReadFailurePropagation(t *testing.T) {
	// QS2, QE16: both errors survive, including a close error after a valid read.
	readErr, closeErr := errors.New("read"), errors.New("close")
	open := func(string) (io.ReadCloser, error) { return badReader{readErr: readErr, closeErr: closeErr}, nil }
	_, err := readFile("subject", open)
	if !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Fatal(err)
	}
	_, err = readFile("subject", func(string) (io.ReadCloser, error) { return nil, readErr })
	if !errors.Is(err, readErr) {
		t.Fatal(err)
	}
}

func TestStoreFailureOutput(t *testing.T) {
	// QE14, QE16, E-4: a failed store still prints the judge output and exits 70.
	for _, row := range []string{"Write", "Resolve"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			if row == "Write" {
				d.Write = func(store.Record) error { return errors.New("write") }
			} else {
				d.Write = nil
				d.StoreRoot = func() (string, error) { return "", errors.New("resolve") }
			}
			var out bytes.Buffer
			code, err := Run(o, d, &out)
			assertFailure(t, code, err, 70, "QE17", "record was not stored")
			if !strings.Contains(out.String(), `"exit_code":4`) || !strings.Contains(out.String(), `"verdict":`) {
				t.Fatal(out.String())
			}
		})
	}
}

func TestEngineFaultRules(t *testing.T) {
	// KTD11, GKD8: an unbuilt decision or unfittable record names its rule.
	for _, row := range []string{"Unbuilt", "Unfittable"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			want := "QE10"
			if row == "Unbuilt" {
				want = "QD4"
				d.Source = func() (observe.Source, func() error, error) { return passingSource{}, func() error { return nil }, nil }
			} else {
				d.Collect = func(p plan.Plan, s observe.Source) ([]collect.Check, error) {
					p.Checks[0].Required["large"] = strings.Repeat("x", 16<<20)
					return collect.Collect(p, s)
				}
			}
			code, err := Run(o, d, io.Discard)
			var e *Error
			if code != 70 || !errors.As(err, &e) || e.Rule != want {
				t.Fatalf("code=%d err=%v want=%s", code, err, want)
			}
		})
	}
}

type passingSource struct{ ReleasedSource }

func (passingSource) Controls() (observe.Controls, error) {
	return observe.Controls{SystemNamed: true, ConfinementApplied: true, SelfTestPassed: true}, nil
}

func TestJudgePanic(t *testing.T) {
	// QE14, QE16: Safe wraps both judge calls, so defects never exit 2.
	for _, row := range []string{"Precheck", "Final"} {
		t.Run(row, func(t *testing.T) {
			o, d, _ := fixture(t)
			calls := 0
			d.Judge = func(b []byte) ([]byte, int) {
				calls++
				if row == "Precheck" || calls == 2 {
					panic("judge")
				}
				return judge.Judge(b)
			}
			code, err := Run(o, d, io.Discard)
			var e *Error
			if code != 70 || !errors.As(err, &e) || e.Rule != "QE16" || !strings.Contains(e.Cause, "judge panic") {
				t.Fatalf("%d %v", code, err)
			}
		})
	}
}

func TestPrecheckRefusalContinues(t *testing.T) {
	// QE5, KTD9: a refusal is not a rejection and collection still runs.
	o, d, saved := fixture(t)
	calls := 0
	d.Judge = func(b []byte) ([]byte, int) {
		calls++
		if calls == 1 {
			return []byte(`{"exit_code":5,"outcome":"refused"}`), 5
		}
		return judge.Judge(b)
	}
	code, err := Run(o, d, io.Discard)
	if code != 4 || err != nil || calls != 2 || saved.JudgeInput == nil {
		t.Fatalf("code=%d err=%v calls=%d", code, err, calls)
	}
}
