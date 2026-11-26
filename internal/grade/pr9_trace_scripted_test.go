//go:build scripted

package grade

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// independentHelpOrder uses a recursive recording reader, separate from the
// production stream reader and iterative collection walk.
func independentHelpOrder(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var recording struct {
		Invocations []struct {
			Key      map[string]any `json:"key"`
			Confined []struct {
				Markers        map[string]bool `json:"markers"`
				Stdout, Stderr struct {
					Bytes int     `json:"bytes"`
					Kept  *string `json:"kept"`
					Head  string  `json:"head"`
				}
			} `json:"confined"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(data, &recording); err != nil {
		t.Fatal(err)
	}
	headings := regexp.MustCompile(`^(.* Commands:|COMMANDS:|Commands:|SUBCOMMANDS:)$`)
	entry := regexp.MustCompile(`^ +([^ \t]+)(.*)$`)
	description := regexp.MustCompile(`(?: {2,}| *\t[ \t]*)(.*)$`)
	var out []map[string]any
	stopped := false
	var visit func([]any)
	visit = func(words []any) {
		if stopped {
			return
		}
		if len(words) > 64 || len(out) == 4096 {
			stopped = true
			return
		}
		key := map[string]any{"words": append(append([]any{}, words...), "--help"), "offline_args": []any{}, "env": []any{}, "runbook": nil, "step": nil, "computation": nil, "home": "empty"}
		out = append(out, key)
		for _, inv := range recording.Invocations {
			if !reflect.DeepEqual(inv.Key, key) {
				continue
			}
			if len(inv.Confined) == 0 {
				t.Fatal("no confined help")
			}
			run := inv.Confined[0]
			if !run.Markers["confinement"] || run.Markers["could_not_start"] {
				return
			}
			stream := run.Stdout
			if stream.Bytes == 0 {
				stream = run.Stderr
			}
			text := stream.Head
			if stream.Kept != nil {
				b, err := base64.StdEncoding.DecodeString(*stream.Kept)
				if err != nil {
					t.Fatal(err)
				}
				text = string(b)
			}
			section := false
			for _, line := range strings.Split(text, "\n") {
				if !strings.HasPrefix(line, " ") {
					section = headings.MatchString(line)
					continue
				}
				if !section {
					continue
				}
				m := entry.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				name := strings.TrimSuffix(m[1], ",")
				desc := description.FindStringSubmatch(m[2])
				if name == "help" || name == "completion" {
					continue
				}
				if len(desc) > 0 && (strings.HasPrefix(desc[1], "(hidden)") || strings.HasPrefix(desc[1], "[hidden]") || strings.HasPrefix(desc[1], "HIDDEN")) {
					continue
				}
				visit(append(append([]any{}, words...), name))
				if stopped {
					return
				}
			}
			return
		}
		t.Fatal("missing recorded help", key)
	}
	visit([]any{})
	return out
}

func compareTrace(t *testing.T, trace, recording []byte, host map[string]any, subject map[string]any, preimages map[string]any, predictions map[string]struct {
	Level  int
	Ending predictedEnding
}) {
	t.Helper()
	// QT5, QD2, QE8: exact served set and help order come from recorded help and case preimages.
	expected := map[string]bool{}
	helpOrder := []map[string]any{}
	confinement := traceObject(host["confinement"])
	binary := traceObject(subject["binary"])
	blocked := confinement["applied"] != true || host["self_test"] != "passed" || host["private_copy"] == "changed" || binary["placeholder"] != nil
	if !blocked {
		helpOrder = independentHelpOrder(t, recording)
		for _, key := range helpOrder {
			expected[string(mustJSON(t, key))] = true
		}
	}
	for id, pred := range predictions {
		pre := traceObject(preimages[id])
		if pre["probe"] == "discovery" || pre["probe"] == "planted_secret" || blocked {
			continue
		}
		if pred.Level < 7 {
			continue
		}
		if pred.Ending.Phase != "deciding" && (allowedRule(pred.Ending, "QA7") || allowedRule(pred.Ending, "QO6")) {
			continue
		}
		if pre["words"] == nil {
			continue
		}
		env := []any{}
		if e, ok := pre["env"].(map[string]any); ok {
			for name := range e {
				env = append(env, name)
			}
		} // all case environment names are sorted below.
		sortAnyStrings(env)
		home := "seed"
		if pre["runbook"] != nil {
			home = "runbook"
		}
		key := map[string]any{"words": pre["words"], "offline_args": pre["offline_args"], "env": env, "home": home, "runbook": pre["runbook"], "step": pre["step"], "computation": nil}
		expected[string(mustJSON(t, key))] = true
	}
	actual := map[string]bool{}
	actualHelp := []map[string]any{}
	decoder := json.NewDecoder(bytes.NewReader(trace))
	for {
		var event struct {
			Kind  string          `json:"kind"`
			Value json.RawMessage `json:"value"`
		}
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if event.Kind != "served" {
			continue
		}
		var key map[string]any
		if err := json.Unmarshal(event.Value, &key); err != nil {
			t.Fatal(err)
		}
		actual[string(mustJSON(t, key))] = true
		if key["home"] == "empty" {
			actualHelp = append(actualHelp, key)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("served set got %v want %v", actual, expected)
	}
	if !reflect.DeepEqual(actualHelp, helpOrder) {
		t.Errorf("help order got %v want %v", actualHelp, helpOrder)
	}
}
func sortAnyStrings(a []any) {
	for i := range a {
		for j := i + 1; j < len(a); j++ {
			left, lok := a[j].(string)
			right, rok := a[i].(string)
			if !lok || !rok {
				panic("environment name is not a string")
			}
			if left < right {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func traceObject(v any) map[string]any {
	if o, ok := v.(map[string]any); ok {
		return o
	}
	return nil
}
