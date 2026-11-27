//go:build scripted

package grade

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func helpRecording(t *testing.T, text string, cut bool) []byte {
	t.Helper()
	key := map[string]any{"words": []any{"--help"}, "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil, "home": "empty"}
	var kept any = base64.StdEncoding.EncodeToString([]byte(text))
	if cut {
		kept = nil
	}
	stream := map[string]any{"bytes": len(text), "head": text, "kept": kept, "cut": cut}
	empty := map[string]any{"bytes": 0, "head": "", "kept": "", "cut": false}
	rows := []any{map[string]any{"key": key, "confined": []any{map[string]any{"markers": map[string]any{"confinement": true}, "stdout": stream, "stderr": empty}}}}
	for _, name := range []string{"a", "b"} {
		child := map[string]any{"words": []any{name, "--help"}, "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil, "home": "empty"}
		rows = append(rows, map[string]any{"key": child, "confined": []any{map[string]any{"markers": map[string]any{"confinement": true}, "stdout": map[string]any{"bytes": 4, "kept": base64.StdEncoding.EncodeToString([]byte("leaf")), "head": "leaf", "cut": false}, "stderr": empty}}})
	}
	return mustJSON(t, map[string]any{"invocations": rows})
}

func TestIndependentHelpErrata(t *testing.T) {
	// QD2, E-32, E-33, E-27: the independent trace reader refuses cut streams and deduplicates greedily parsed listings.
	for _, row := range []struct {
		name, text string
		cut        bool
		words      [][]any
	}{
		{"Cut", "Commands:\n  a  desc", true, [][]any{{"--help"}}},
		{"Duplicate", "Commands:\n  a  desc\n  b  desc\n  a  again", false, [][]any{{"--help"}, {"a", "--help"}, {"b", "--help"}}},
		{"GreedyBlanks", "Commands:\n  a  \t(hidden)\n  b\t  HIDDEN", false, [][]any{{"--help"}}},
	} {
		t.Run(row.name, func(t *testing.T) {
			order := independentHelpOrder(t, helpRecording(t, row.text, row.cut))
			words := [][]any{}
			for _, key := range order {
				w, ok := key["words"].([]any)
				if !ok {
					t.Fatal("trace words missing")
				}
				words = append(words, w)
			}
			if !reflect.DeepEqual(words, row.words) {
				t.Fatal(words, row.words)
			}
		})
	}
}

func TestTraceOwnInvocationQO1(t *testing.T) {
	// QO1, QT5: a check decided on its own unconfined invocation has served that key.
	recording := helpRecording(t, "leaf", false)
	discovery := map[string]any{"words": []any{"--help"}, "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil, "home": "empty"}
	own := map[string]any{"words": []any{"a"}, "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil, "home": "seed"}
	trace := append(mustJSON(t, map[string]any{"kind": "served", "value": discovery}), '\n')
	trace = append(trace, mustJSON(t, map[string]any{"kind": "served", "value": own})...)
	host := map[string]any{"confinement": map[string]any{"applied": true}, "self_test": "passed", "private_copy": "unchanged"}
	pre := map[string]any{"check": map[string]any{"probe": "bare", "words": []any{"a"}, "offline_args": []any{}, "runbook": nil, "step": nil}}
	predictions := map[string]struct {
		Level  int
		Ending predictedEnding
	}{"check": {Level: 1, Ending: predictedEnding{Phase: "deciding", Rules: []string{"QO1"}}}}
	compareTrace(t, trace, recording, host, map[string]any{}, pre, predictions)
}
