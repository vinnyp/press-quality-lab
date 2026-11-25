package jcs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func parse(t *testing.T, b []byte) any {
	t.Helper()
	v, ok := strictjson.Read(b, strictjson.Bounds{Bytes: len(b), Depth: 100, NumberChars: 10000, ExponentDigits: 10000}, nil)
	if !ok {
		t.Fatalf("invalid fixture: %s", b)
	}
	return v
}
func digest(t *testing.T, v any, want string) {
	t.Helper()
	got, err := jcs.Digest(v)
	if err != nil || got != want {
		t.Fatalf("digest = %s, %v; want %s", got, err, want)
	}
}
func TestVectors(t *testing.T) {
	var vectors []struct{ Name, Input, Canonical, Digest string }
	if err := json.Unmarshal(read(t, "testdata/vectors.json"), &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 35 {
		t.Fatalf("vector count %d", len(vectors))
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			value := parse(t, []byte(v.Input))
			got, err := jcs.Canonical(value)
			if err != nil || string(got) != v.Canonical {
				t.Fatalf("canonical = %q, %v; want %q", got, err, v.Canonical)
			}
			digest(t, value, v.Digest)
		})
	}
}
func TestErrors(t *testing.T) {
	cycle := strictjson.Object{}
	cycle["self"] = cycle
	array := make(strictjson.Array, 1)
	array[0] = array
	values := []any{strictjson.Number("9007199254740993"), strictjson.Number("1e400"), strictjson.Number("1e-400"), strictjson.Number("0.123456789012345678901"), strictjson.Number("NaN"), strictjson.Number("01"), strictjson.Number(""), strictjson.Number("true"), strictjson.Number("0x1p0"), "\xff", strictjson.Object{"\xff": nil}, 1, cycle, array}
	names := []string{"integer_loss", "overflow", "underflow", "decimal_loss", "nan", "leading_zero", "empty", "boolean", "hex", "invalid_string", "invalid_key", "unsupported", "object_cycle", "array_cycle"}
	for i, v := range values {
		t.Run(names[i], func(t *testing.T) {
			if b, err := jcs.Canonical(v); err == nil || b != nil {
				t.Fatalf("Canonical returned %q, %v", b, err)
			}
			if names[i] == "overflow" {
				_, err := jcs.Canonical(v)
				if err == nil || err.Error() != `number outside binary64: "1e400"` {
					t.Fatalf("overflow error: %v", err)
				}
			}
			if s, err := jcs.Digest(v); err == nil || s != "" {
				t.Fatalf("Digest returned %q, %v", s, err)
			}
		})
	}
}

const kit = "../../docs/specs/conformance-collection-1.0"

func TestWorkedIDs(t *testing.T) {
	// Collection contract QE2: the four preimages exactly as written in the sealed text.
	text := read(t, filepath.Join(kit, "collection-contract.md"))
	preimages := regexp.MustCompile(`(?m)^\s*(\{"bookkeeping"[^\n]+)\s*$`).FindAllSubmatch(text, -1)
	wants := []string{"46c4884c4195f646969ad14aca889bb614643f7c5e7e88bdec87d1032c0344d7", "c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7", "40b08dbef70438992c53f14fee9f5e1297277bc94bb35cb5e2b435758df22993", "181431d07ae5e61a4a5fde89d6a606708476067a3dcdfad263b473f33adb98ba"}
	if len(preimages) != len(wants) {
		t.Fatalf("worked preimages: %d", len(preimages))
	}
	for i, p := range preimages {
		digest(t, parse(t, p[1]), wants[i])
	}
	// QE2's seed digest hashes canonical entries, not the seed's filesystem metadata.
	digest(t, parse(t, []byte(`[["data.json","file","3890b1a054e609ad21a23184fc351862f3aed55501fdf682bc1d2cec95dacff2"]]`)), "d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee")
	digest(t, parse(t, []byte(`[[".config","directory",null],[".config/widgets","directory",null],[".config/widgets/token","file","b7a2426b2cad538728ed95180a5323b463b166d48bd1d3270ba46e72fd958d7f"],["data.json","file","3a8faa48b8f9b4a4ee9325929bebc6d41692a782586147918ae4b995fba81b8b"]]`)), "c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a")
}
func TestCorpusPreimages(t *testing.T) {
	cases, err := filepath.Glob(filepath.Join(kit, "cases/*/case.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no corpus cases")
	}
	files, entries := 0, 0
	for _, path := range cases {
		meta := object(t, parse(t, read(t, path)))
		preimagePath := filepath.Join(filepath.Dir(path), "preimages.json")
		b, err := os.ReadFile(preimagePath)
		if os.IsNotExist(err) && meta["kind"] != "judge-checked" {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		files++
		t.Run(strings.ReplaceAll(filepath.Base(filepath.Dir(path)), "-", "_"), func(t *testing.T) {
			preimages := object(t, parse(t, b))
			if len(preimages) == 0 {
				t.Fatal("empty preimages")
			}
			for want, v := range preimages {
				entries++
				digest(t, v, want)
			}
			// Every check referenced by the case's input must have a preimage.
			input := object(t, parse(t, read(t, filepath.Join(filepath.Dir(path), "input.json"))))
			for _, check := range array(t, input["required"]) {
				id, ok := object(t, check)["id"].(string)
				if !ok {
					t.Fatal("check id is not a string")
				}
				if _, ok := preimages[id]; !ok {
					t.Fatalf("missing preimage for %s", id)
				}
			}
		})
	}
	t.Logf("checked %d preimage files and %d ids across %d cases", files, entries, len(cases))
}
func TestEquivalentNumbers(t *testing.T) {
	for _, text := range []string{"0.1", "0.10", "1e-1"} {
		b, err := jcs.Canonical(strictjson.Number(text))
		if err != nil || strings.TrimSpace(string(b)) != "0.1" {
			t.Fatalf("%s: %q %v", text, b, err)
		}
	}
}

func object(t *testing.T, v any) strictjson.Object {
	t.Helper()
	x, ok := v.(strictjson.Object)
	if !ok {
		t.Fatalf("want object, got %T", v)
	}
	return x
}
func array(t *testing.T, v any) strictjson.Array {
	t.Helper()
	x, ok := v.(strictjson.Array)
	if !ok {
		t.Fatalf("want array, got %T", v)
	}
	return x
}

func TestAliasedSlices(t *testing.T) {
	x := make(strictjson.Array, 2)
	x[1] = x[0:1]
	b, err := jcs.Canonical(x)
	if err != nil || string(b) != "[null,[null]]" {
		t.Fatalf("canonical=%s error=%v", b, err)
	}
}
