//go:build scripted

package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/caseassembly"
	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

func read(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func canonical(t *testing.T, v any) string {
	t.Helper()
	b, e := jcs.Canonical(normalize(v))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func oracle(t *testing.T, p string) any {
	t.Helper()
	v, ok := strictjson.Read(read(t, p), strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	if !ok {
		t.Fatal("oracle JSON")
	}
	return v
}
func TestCorpus(t *testing.T) {
	// QE2: independent sealed preimages.
	// QE3: fixed scoped required set.
	// QE7: complete required fields.
	// QP1: probe placement.
	// QA3: crediting.
	// QA9: empty_satisfiable.
	// QS7: offline settings.
	// QT5: subject keys.
	kit := "../../docs/specs/conformance-collection-1.0"
	var prediction struct {
		Cases []struct {
			Case   string
			Ending struct {
				Phase string
				Rules []string
			}
		}
	}
	if e := json.Unmarshal(read(t, "../../scripts/ratchet/pr10-prediction.json"), &prediction); e != nil {
		t.Fatal(e)
	}
	checked := 0
	for _, pr := range prediction.Cases {
		source := filepath.Join(kit, "cases", pr.Case)
		var c caseassembly.Case
		if e := caseassembly.Read(filepath.Join(source, "case.json"), &c); e != nil {
			t.Fatal(e)
		}
		if c.Kind != "judge-checked" {
			continue
		}
		checked++
		t.Run(strings.ReplaceAll(c.ID, "-", "_"), func(t *testing.T) {
			l := caseassembly.Assemble(t, kit, source, c)
			s, e := subject.Parse(read(t, filepath.Join(l.Subject, c.Input.SubjectFile)))
			if e != nil {
				t.Fatal(e)
			}
			// QE4 baseline reconciliation remains unbuilt; planning identities do not depend on it.
			in := Input{Subject: s, Files: map[string][]byte{}}
			for _, scope := range c.Input.Scope {
				in.Scopes = append(in.Scopes, rulebook.Scope(scope))
			}
			if seed, ok := s["seed"].(string); ok {
				root := filepath.Join(l.Subject, seed)
				err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if p == root {
						return nil
					}
					rel, err := filepath.Rel(root, p)
					if err != nil {
						return err
					}
					entry := Entry{Path: filepath.ToSlash(rel), Directory: d.IsDir()}
					if !d.IsDir() {
						entry.Data = read(t, p)
					}
					in.Seed = append(in.Seed, entry)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			err := filepath.WalkDir(filepath.Join(l.Subject, "computations"), func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					rel, e := filepath.Rel(l.Subject, p)
					if e != nil {
						return e
					}
					in.Files[filepath.ToSlash(rel)] = read(t, p)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := Build(in)
			if err != nil {
				t.Fatal(err)
			}
			pres := map[string]any{}
			required := []any{}
			for _, ch := range got.Checks {
				pres[ch.ID] = ch.Preimage
				required = append(required, ch.Required)
			}
			want := oracle(t, filepath.Join(source, "preimages.json"))
			if len(pres) != len(got.Checks) {
				t.Fatal("duplicate planned check id")
			}
			if canonical(t, pres) != canonical(t, want) {
				wm := obj(want)
				for id, pre := range pres {
					if _, ok := wm[id]; !ok {
						t.Errorf("unexpected %s %s", id, canonical(t, pre))
					}
				}
				for id, pre := range wm {
					if _, ok := pres[id]; !ok {
						t.Errorf("missing %s %s", id, canonical(t, pre))
					}
				}
				t.Fatal("preimages differ")
			}
			input := obj(oracle(t, filepath.Join(source, "input.json")))
			expected := arr(input["required"])
			byID := map[string]any{}
			for _, v := range expected {
				o := obj(v)
				byID[text(o["id"])] = o
			}
			actual := map[string]any{}
			for _, v := range required {
				o := obj(v)
				actual[text(o["id"])] = o
			}
			if len(required) != len(expected) || canonical(t, actual) != canonical(t, byID) {
				t.Fatalf("required differ\ngot %s\nwant %s", canonical(t, actual), canonical(t, byID))
			}
			// QT6 excludes computations whose captures were cut at observation time.
			// Subject keys and computations without captures must be present.
			keys := map[string]bool{}
			for _, k := range c.Invocations {
				keys[canonical(t, k)] = true
			}
			for _, ch := range got.Checks {
				for _, key := range ch.Keys {
					if key["computation"] != nil && len(arr(obj(obj(ch.Preimage["expected"])["computation"])["captures"])) > 0 {
						continue
					}
					if !keys[canonical(t, key)] {
						t.Fatalf("unexpected key %s", canonical(t, key))
					}
				}
			}
		})
	}
	if checked != 121 {
		t.Fatalf("judge-checked cases %d; want 121", checked)
	}
}

func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		o := Object{}
		for k, v := range x {
			o[k] = normalize(v)
		}
		return o
	case []any:
		a := Array{}
		for _, v := range x {
			a = append(a, normalize(v))
		}
		return a
	case Object:
		o := Object{}
		for k, v := range x {
			o[k] = normalize(v)
		}
		return o
	case Array:
		a := Array{}
		for _, v := range x {
			a = append(a, normalize(v))
		}
		return a
	}
	return v
}
