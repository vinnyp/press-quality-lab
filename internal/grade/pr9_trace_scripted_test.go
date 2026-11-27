//go:build scripted

package grade

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"reflect"
	"regexp"
	"sort"
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
					Cut   bool    `json:"cut"`
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
	description := regexp.MustCompile(`[ \t]*(?: {2}|\t)[ \t]*(.*)$`)
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
			if stream.Cut {
				return
			}
			text := stream.Head
			if stream.Kept != nil {
				b, err := base64.StdEncoding.DecodeString(*stream.Kept)
				if err != nil {
					t.Fatal(err)
				}
				text = string(b)
			}
			seen := map[string]bool{}
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
				if seen[name] {
					continue
				}
				seen[name] = true
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

type tracePrediction = struct {
	Level  int
	Ending predictedEnding
}

func independentTrace(t *testing.T, recording []byte, host, subject, input, preimages map[string]any, predictions map[string]tracePrediction) []map[string]any {
	t.Helper()
	var expected []map[string]any
	confinement := traceObject(host["confinement"])
	binary := traceObject(subject["binary"])
	blocked := confinement["applied"] != true || host["self_test"] != "passed"
	copyChanged := host["private_copy"] == "changed"
	if !blocked && !copyChanged && binary["placeholder"] == nil {
		expected = append(expected, independentHelpOrder(t, recording)...)
	}
	keyFor := func(pre map[string]any, home string) map[string]any {
		env := []any{}
		for name := range traceObject(pre["env"]) {
			env = append(env, name)
		}
		sortAnyStrings(env)
		return map[string]any{"words": pre["words"], "offline_args": pre["offline_args"], "env": env, "home": home, "runbook": pre["runbook"], "step": pre["step"], "computation": nil}
	}
	var recorded struct {
		Invocations []struct {
			Key      map[string]any
			Confined []map[string]any
		}
	}
	if err := json.Unmarshal(recording, &recorded); err != nil {
		t.Fatal(err)
	}
	// A per-invocation QO1 ending still requires serving the subject run.
	before := func(id string) bool {
		pre := traceObject(preimages[id])
		for _, bv := range traceArray(subject["runbooks"]) {
			book := traceObject(bv)
			if book["name"] != pre["runbook"] {
				continue
			}
			for _, sv := range traceArray(book["steps"]) {
				step := traceObject(sv)
				if step["name"] != pre["step"] {
					continue
				}
				index, err := traceIndex(t, pre["index"])
				if err != nil {
					t.Fatal(err)
				}
				checks := traceArray(step["checks"])
				if index < 0 || index >= int64(len(checks)) {
					continue
				}
				check := traceObject(checks[index])
				for _, v := range []any{traceObject(check["precondition"])["computation"], traceObject(check["expected"])["computation"]} {
					for _, name := range traceArray(traceObject(v)["captures"]) {
						for _, inv := range recorded.Invocations {
							if inv.Key["runbook"] == book["name"] && inv.Key["step"] == name && inv.Key["computation"] == nil && len(inv.Confined) > 0 && traceObject(inv.Confined[0]["markers"])["confinement"] != true {
								return false
							}
						}
					}
				}
			}
		}
		pred := predictions[id]
		return !blocked && !copyChanged && binary["placeholder"] == nil && (pred.Level >= 6 || pred.Level == 1 && allowedRule(pred.Ending, "QO1"))
	}
	commands := traceArray(subject["commands"])
	rank := func(words any) int {
		if w, ok := words.([]any); ok && len(w) == 0 {
			return -1
		}
		for i, v := range commands {
			if reflect.DeepEqual(traceObject(v)["words"], words) {
				return i
			}
		}
		return len(commands)
	}
	for _, probe := range []string{"bare", "byte_bound", "exit_codes", "planted_secret", "store_unchanged"} {
		ids := []string{}
		for id := range predictions {
			pre := traceObject(preimages[id])
			if pre["probe"] == probe {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool {
			a, b := rank(traceObject(preimages[ids[i]])["words"]), rank(traceObject(preimages[ids[j]])["words"])
			if a == b {
				return ids[i] < ids[j]
			}
			return a < b
		})
		for _, id := range ids {
			if !before(id) {
				continue
			}
			pre := traceObject(preimages[id])
			if probe == "planted_secret" {
				for _, v := range traceArray(traceObject(pre["parameters"])["commands"]) {
					cmd := traceObject(v)
					expected = append(expected, keyFor(cmd, "planted"))
				}
			} else if pre["words"] != nil {
				expected = append(expected, keyFor(pre, "seed"))
			}
		}
	}
	evidence := map[string]map[string]any{}
	for _, v := range traceArray(input["evidence"]) {
		e := traceObject(v)
		id, ok := e["check_id"].(string)
		if !ok {
			t.Fatal("evidence lacks check id")
		}
		evidence[id] = e
	}
	held := func(id string) bool {
		e := evidence[id]
		return e["outcome"] == "held" || e["reason"] == "placeholder_unfilled" && predictions[id].Level == 8
	}
	runFor := func(key map[string]any) map[string]any {
		for _, inv := range recorded.Invocations {
			if reflect.DeepEqual(inv.Key, key) && len(inv.Confined) > 0 {
				return inv.Confined[0]
			}
		}
		t.Fatal("independent trace missing recorded run", key)
		return nil
	}
	placeholder := func(v any) bool { return traceObject(v)["placeholder"] != nil }
	selected := func(words any) map[string]any {
		var result map[string]any
		n := -1
		w := traceArray(words)
		for _, v := range commands {
			cmd := traceObject(v)
			prefix := traceArray(cmd["words"])
			if len(prefix) > n && len(prefix) <= len(w) && reflect.DeepEqual(prefix, w[:len(prefix)]) {
				result = cmd
				n = len(prefix)
			}
		}
		return result
	}
	for _, bv := range traceArray(subject["runbooks"]) {
		book := traceObject(bv)
		steps := traceArray(book["steps"])
		destructive := false
		included := false
		for _, sv := range steps {
			st := traceObject(sv)
			destructive = destructive || selected(st["words"])["class"] == "destructive"
			for id := range predictions {
				pre := traceObject(preimages[id])
				included = included || pre["runbook"] == book["name"]
			}
		}
		if destructive || !included {
			continue
		}
		stopped := false
		captures := map[any]map[string]any{}
		for _, sv := range steps {
			st := traceObject(sv)
			ids := []string{}
			for id := range predictions {
				pre := traceObject(preimages[id])
				if pre["runbook"] == book["name"] && pre["step"] == st["name"] {
					ids = append(ids, id)
				}
			}
			sort.Slice(ids, func(i, j int) bool {
				left, err := traceIndex(t, traceObject(preimages[ids[i]])["index"])
				if err != nil {
					t.Fatal(err)
				}
				right, err := traceIndex(t, traceObject(preimages[ids[j]])["index"])
				if err != nil {
					t.Fatal(err)
				}
				return left < right
			})
			serve := false
			for _, id := range ids {
				serve = serve || before(id)
			}
			var key map[string]any
			if len(ids) > 0 {
				key = keyFor(traceObject(preimages[ids[0]]), "runbook")
			} else {
				args := []any{}
				env := map[string]any{}
				cmd := selected(st["words"])
				for _, v := range traceArray(subject["offline"]) {
					off := traceObject(v)
					for _, w := range traceArray(off["applies_to"]) {
						if reflect.DeepEqual(w, cmd["words"]) {
							args = append(args, traceArray(off["args"])...)
							for name, value := range traceObject(off["env"]) {
								env[name] = value
							}
							break
						}
					}
				}
				key = keyFor(map[string]any{"words": st["words"], "offline_args": args, "env": env, "runbook": book["name"], "step": st["name"]}, "runbook")
				serve = !blocked && !copyChanged && !placeholder(subject["binary"]) && !placeholder(subject["seed"]) && !placeholder(subject["store"]) && !placeholder(cmd["class"]) && !placeholder(cmd["writes"])
			}
			if stopped {
				continue
			}
			if serve {
				expected = append(expected, key)
				run := runFor(key)
				captures[st["name"]] = run
				// Per-run copy changes belong to the darwin source in PR15.
				shared := map[string]bool{}
				for _, id := range ids {
					if !before(id) || traceObject(run["markers"])["confinement"] != true {
						continue
					}
					ex := traceObject(traceObject(preimages[id])["expected"])
					comps := []any{traceObject(ex["precondition"])["computation"], ex["computation"]}
					cut := false
					for _, v := range comps {
						for _, name := range traceArray(traceObject(v)["captures"]) {
							cut = cut || traceObject(captures[name]["stdout"])["cut"] == true
						}
					}
					if cut {
						continue
					}
					for _, v := range comps {
						comp := traceObject(v)
						if comp == nil {
							continue
						}
						ck := map[string]any{"words": nil, "offline_args": []any{}, "env": []any{}, "home": "scratch", "runbook": book["name"], "step": st["name"], "computation": comp["argv"]}
						encoded := string(mustJSON(t, ck))
						if !shared[encoded] {
							expected = append(expected, ck)
							shared[encoded] = true
						}
					}
				}
			}
			for _, id := range ids {
				stopped = stopped || !held(id)
			}
		}
	}
	return expected
}

func traceList(trace []byte) ([]map[string]any, error) {
	var actual []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(trace))
	for {
		var event struct {
			Kind  string
			Value json.RawMessage
		}
		err := decoder.Decode(&event)
		if err == io.EOF {
			return actual, nil
		}
		if err != nil {
			return nil, err
		}
		if event.Kind == "served" {
			var key map[string]any
			if err := json.Unmarshal(event.Value, &key); err != nil {
				return nil, err
			}
			actual = append(actual, key)
		}
	}
}
func traceMatches(actual, expected []map[string]any) bool { return reflect.DeepEqual(actual, expected) }
func compareTrace(t *testing.T, trace, recording []byte, host, subject, input, preimages map[string]any, predictions map[string]tracePrediction) {
	t.Helper()
	// QT5, QA7: ordered lists catch duplication as well as missing or reordered runs.
	expected := independentTrace(t, recording, host, subject, input, preimages, predictions)
	actual, err := traceList(trace)
	if err != nil {
		t.Fatal(err)
	}
	if !traceMatches(actual, expected) {
		t.Errorf("served order got %v want %v", actual, expected)
	}
}
func traceArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}
func traceIndex(t *testing.T, v any) (int64, error) {
	t.Helper()
	n, ok := v.(json.Number)
	if !ok {
		t.Fatal("preimage lacks integer index")
	}
	return n.Int64()
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

func TestTraceCapturedUnconfined(t *testing.T) {
	// E-53, E-49, QA7: a named unconfined recording prevents serving the capturing step.
	help := map[string]any{"words": []any{"--help"}, "offline_args": []any{}, "env": []any{}, "home": "empty", "runbook": nil, "step": nil, "computation": nil}
	first := map[string]any{"words": []any{"first"}, "offline_args": []any{}, "env": []any{}, "home": "runbook", "runbook": "book", "step": "first", "computation": nil}
	last := map[string]any{"words": []any{"last"}, "offline_args": []any{}, "env": []any{}, "home": "runbook", "runbook": "book", "step": "last", "computation": nil}
	comp := map[string]any{"words": nil, "offline_args": []any{}, "env": []any{}, "home": "scratch", "runbook": "book", "step": "last", "computation": []any{"/bin/echo"}}
	host := map[string]any{"confinement": map[string]any{"applied": true}, "self_test": "passed"}
	for _, named := range []bool{false, true} {
		for _, confined := range []bool{false, true} {
			computation := map[string]any{"argv": []any{"/bin/echo"}, "captures": []any{}}
			if named {
				computation["captures"] = []any{"first"}
			}
			subject := map[string]any{"runbooks": []any{map[string]any{"name": "book", "steps": []any{
				map[string]any{"name": "first", "words": []any{"first"}},
				map[string]any{"name": "last", "words": []any{"last"}, "checks": []any{map[string]any{"expected": map[string]any{"computation": computation}}}},
			}}}}
			preimages := map[string]any{"check": map[string]any{"runbook": "book", "step": "last", "index": json.Number("0"), "words": []any{"last"}, "offline_args": []any{}, "env": map[string]any{}, "expected": map[string]any{"computation": computation}}}
			input := map[string]any{"evidence": []any{map[string]any{"check_id": "check", "outcome": "held"}}}
			predictions := map[string]tracePrediction{"check": {Level: 8}}
			want := []map[string]any{help, first, last, comp}
			if named && !confined {
				predictions["check"] = tracePrediction{Level: 1, Ending: predictedEnding{Rules: []string{"QO1"}}}
				want = want[:2]
			}
			recording := mustJSON(t, map[string]any{"invocations": []any{
				map[string]any{"key": help, "confined": []any{map[string]any{"markers": map[string]any{"confinement": false}}}},
				map[string]any{"key": first, "confined": []any{map[string]any{"markers": map[string]any{"confinement": confined}}}},
				map[string]any{"key": last, "confined": []any{map[string]any{"markers": map[string]any{"confinement": true}}}},
			}})
			if got := independentTrace(t, recording, host, subject, input, preimages, predictions); !reflect.DeepEqual(got, want) {
				t.Fatal(named, confined, got, want)
			}
		}
	}
}
