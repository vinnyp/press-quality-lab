package grade

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestStoredDigests(t *testing.T) {
	// QE2, QS4, QE17: digests describe the actual snapshot, including null seed.
	for _, name := range []string{"Seed", "NoSeed"} {
		t.Run(name, func(t *testing.T) {
			o, d, _ := fixture(t)
			data := []byte(validSubject)
			var seed any
			if name == "Seed" {
				dir := filepath.Join(filepath.Dir(o.SubjectFile), "seed")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				content := []byte("seed bytes\n")
				if err := os.WriteFile(filepath.Join(dir, "state"), content, 0600); err != nil {
					t.Fatal(err)
				}
				// QE2: sorted [relative path, kind, content digest] entries, canonical JSON.
				entries := fmt.Sprintf(`[["state","file","%x"]]`, sha256.Sum256(content))
				seed = fmt.Sprintf("%x", sha256.Sum256([]byte(entries)))
				data = []byte(strings.Replace(validSubject, `"seed":null`, `"seed":"seed"`, 1))
				if err := os.WriteFile(o.SubjectFile, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			root := filepath.Join(t.TempDir(), "store")
			d.Write = nil
			d.StoreRoot = func() (string, error) { return root, nil }
			if code, err := Run(o, d, io.Discard); code != 4 || err != nil {
				t.Fatalf("%d %v", code, err)
			}
			paths, err := filepath.Glob(filepath.Join(root, "*", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatal(paths, err)
			}
			b, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Fatal(err)
			}
			for field, want := range map[string]any{
				"subject_file_digest": fmt.Sprintf("%x", sha256.Sum256(data)),
				"binary_digest":       fmt.Sprintf("%x", sha256.Sum256([]byte("synthetic"))),
				"seed_digest":         seed,
			} {
				got, present := doc[field]
				if !present || got != want {
					t.Errorf("%s: %v want %v (present %v)", field, got, want, present)
				}
			}
		})
	}
}

type requiredSource struct {
	ReleasedSource
	err   error
	calls *int
}

func (s requiredSource) Required(any) error { *s.calls++; return s.err }

func TestRequiredFailure(t *testing.T) {
	// QT5, QE16: failed required-set tracing prevents collection and persistence.
	for _, fail := range []bool{false, true} {
		o, d, saved := fixture(t)
		calls, closed, collected := 0, 0, false
		var injected error
		if fail {
			injected = errors.New("trace write failed")
		}
		d.Source = func() (observe.Source, func() error, error) {
			return requiredSource{err: injected, calls: &calls}, func() error { closed++; return nil }, nil
		}
		d.Collect = func(p plan.Plan, s observe.Source) ([]collect.Check, error) {
			collected = true
			return collect.Collect(p, s)
		}
		code, err := Run(o, d, io.Discard)
		if calls != 1 || closed != 1 {
			t.Fatalf("calls=%d closed=%d", calls, closed)
		}
		if fail {
			assertFailure(t, code, err, 70, "QE16", "trace write failed")
			if collected || saved.JudgeInput != nil {
				t.Fatal("continued after trace failure")
			}
		} else if code != 4 || err != nil || !collected || saved.JudgeInput == nil {
			t.Fatalf("control %d %v collected=%v", code, err, collected)
		}
	}
}

func TestSourceRuleErrors(t *testing.T) {
	// QO1, GKD8: rule identity is structured, never inferred from error text.
	_, err := (ReleasedSource{}).Observe(nil, observe.Spec{})
	if err == nil {
		t.Fatal("released Observe succeeded")
	}
	got := classify(err)
	if got.Rule != "QO1" {
		t.Fatal(got)
	}
	got = classify(errors.New("QO1: arbitrary text"))
	if got.Rule != "QE16" || got.Cause != "QO1: arbitrary text" {
		t.Fatal(got)
	}
}
