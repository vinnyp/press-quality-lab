//go:build scripted

package corpusrunner

import "testing"

func TestPR8LiteralClassifier(t *testing.T) {
	// QS5, E-22: literals and two-key lookalikes are data, while every allowed location still counts.
	for _, literal := range []map[string]any{{"placeholder": "x"}, {"placeholder": "x", "k": 1}} {
		s := map[string]any{"runbooks": []any{map[string]any{"steps": []any{map[string]any{"checks": []any{map[string]any{"expected": map[string]any{"literal": literal}}}}}}}}
		if hasPlaceholder(s) {
			t.Fatal("literal treated as placeholder")
		}
		s["binary"] = map[string]any{"placeholder": "x", "k": 1}
		if hasPlaceholder(s) {
			t.Fatal("lookalike treated as placeholder")
		}
		s["binary"] = map[string]any{"placeholder": "Supply binary."}
		if !hasPlaceholder(s) {
			t.Fatal("binary placeholder missed")
		}
	}
}
