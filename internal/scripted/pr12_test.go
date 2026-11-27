//go:build scripted

package scripted

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPrintedValueTypes(t *testing.T) {
	// QA2, QA9, QT5: replay retains exact number spellings and strict JSON containers for predicates.
	for _, row := range []struct {
		name  string
		value any
	}{
		{"LargeInteger", number("9007199254740993")},
		{"LargeExponent", number("1e9999")},
		{"NegativeZero", number("-0")},
		{"Containers", object{"array": array{number("9007199254740993"), true, nil}}},
		{"Null", nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			b := decode(t, readFile(t, kit+"/examples/behaviour-01.json"))
			inv := obj(arr(b["invocations"])[0])
			for _, v := range arr(inv["confined"]) {
				obj(v)["printed_value"] = object{"value": row.value}
			}
			source, err := Read([]byte(passing), strictjson.Write(b), nil)
			if err != nil {
				t.Fatal(err)
			}
			run, err := source.Observe(map[string]any(obj(inv["key"])), observe.Spec{})
			if err != nil || run.Details.Printed == nil {
				t.Fatal(run, err)
			}
			if !strictjson.EqualJSON(run.Details.Printed.Value, row.value) {
				t.Fatalf("printed type/value got %T %v want %T %v", run.Details.Printed.Value, run.Details.Printed.Value, row.value, row.value)
			}
		})
	}
}
