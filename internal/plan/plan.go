// Package plan fixes check identities and required entries without performing I/O.
package plan

import (
	"crypto/sha256"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

type Object = strictjson.Object
type Array = strictjson.Array

// Gap names an unbuilt planning rule and its exact ratchet part.
type Gap struct{ Rule, Part string }

func (g *Gap) Error() string {
	if g.Part == "" {
		return g.Rule
	}
	return g.Rule + "." + g.Part
}

// Fault names a planning defect, distinct from an unbuilt rule.
type Fault struct{ Rule, Cause string }

func (f *Fault) Error() string { return f.Rule + ": " + f.Cause }

// RuleID preserves the planning rule through caller classification.
func (f *Fault) RuleID() string { return f.Rule }

// Entry describes one caller-snapshotted seed entry. Digest, when supplied,
// replaces Data; directories have neither.
type Entry struct {
	Path      string
	Directory bool
	Data      []byte
	Digest    string
}

// Input contains parsed values and caller-supplied file contents or digests.
type Input struct {
	Subject     subject.File
	Scopes      []rulebook.Scope
	Seed        []Entry
	Files       map[string][]byte
	FileDigests map[string]string
	Baseline    bool
	Anchor      bool
}

// Check holds a required entry, its preimage and subject invocation keys.
// Keys include subject and computation invocations.
type Check struct {
	Seed []Entry
	// Subject supplies QS5 dependency context; it is not part of the check id.
	Subject  subject.File
	ID       string
	Preimage Object
	Required Object
	Keys     []Object
}

// Step retains scheduling context even when every check is outside the scope.
type Step struct {
	BudgetSeconds any
	OfflineArgs   Array
	Env           Object
	Key           Object
	Required      []int
	Outside       []string
}

// Runbook retains the authored step order.
type Runbook struct{ Steps []Step }
type Plan struct {
	Checks   []Check
	Runbooks []Runbook
}

func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

// SeedDigest implements QE2's path-sorted content tree digest.
func SeedDigest(entries []Entry) (string, error) {
	entries = append([]Entry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	a := Array{}
	for _, e := range entries {
		kind := "file"
		var d any = e.Digest
		if e.Digest == "" {
			d = hash(e.Data)
		}
		if e.Directory {
			kind = "directory"
			d = nil
		}
		a = append(a, Array{e.Path, kind, d})
	}
	return jcs.Digest(a)
}
func obj(v any) Object {
	o, ok := v.(Object)
	if !ok {
		return nil
	}
	return o
}
func arr(v any) Array {
	a, ok := v.(Array)
	if !ok {
		return nil
	}
	return a
}
func text(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
func same(a, b any) bool { return reflect.DeepEqual(a, b) }
func placeholder(v any) bool {
	o := obj(v)
	_, ok := o["placeholder"]
	return len(o) == 1 && ok
}
func (in Input) includes(d string) bool {
	for _, s := range in.Scopes {
		// E-7: unknown tokens include no dimension; the judge owns X1 rejection.
		switch s {
		case rulebook.ConformanceScope, rulebook.DestructiveScope, rulebook.SteeringScope, rulebook.CompareScope:
		default:
			continue
		}
		if rulebook.ScopeIncludes(s, rulebook.Dimension(d)) {
			return true
		}
	}
	return false
}
func command(s Object, words Array) Object {
	var best Object
	n := -1
	for _, v := range arr(s["commands"]) {
		c := obj(v)
		w := arr(c["words"])
		if len(w) <= len(words) && len(w) > n && same(w, words[:len(w)]) {
			best = c
			n = len(w)
		}
	}
	return best
}
func offline(s Object, words Array) (Array, Object, Array) {
	args := Array{}
	env := Object{}
	names := Array{}
	c := command(s, words)
	cw := Array{}
	if c != nil {
		cw = arr(c["words"])
	}
	for _, v := range arr(s["offline"]) {
		o := obj(v)
		for _, w := range arr(o["applies_to"]) {
			if same(w, cw) {
				args = append(args, arr(o["args"])...)
				for k, v := range obj(o["env"]) {
					env[k] = hash([]byte(text(v)))
				}
				break
			}
		}
	}
	for k := range env {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool { return text(names[i]) < text(names[j]) })
	return args, env, names
}

// SubjectKey constructs QT5 subject keys, excluding offline settings for discovery.
func SubjectKey(s subject.File, words Array, home string, runbook, step any) Object {
	args, _, names := offline(s, words)
	if home == "empty" {
		args = Array{}
		names = Array{}
	}
	return Object{"words": words, "offline_args": args, "env": names, "home": home, "runbook": runbook, "step": step, "computation": nil}
}

// Build returns no partial plan on a planning gap or malformed digest input.
func Build(in Input) (Plan, error) {
	s := in.Subject
	if in.Anchor {
		return Plan{}, &Gap{"QE4", "anchor"}
	}
	if in.Baseline {
		return Plan{}, &Gap{"QE4", ""}
	}
	var seed any
	if placeholder(s["seed"]) {
		seed = s["seed"]
	} else if s["seed"] != nil {
		d, err := SeedDigest(in.Seed)
		if err != nil {
			return Plan{}, err
		}
		seed = d
	}
	book := append(Array{}, arr(s["bookkeeping"])...)
	sort.Slice(book, func(i, j int) bool { return text(book[i]) < text(book[j]) })
	p := Plan{Checks: []Check{}}
	base := func(probe any, dimension string, words any, budget any, parameters Object) Object {
		args := Array{}
		env := Object{}
		if words != nil {
			args, env, _ = offline(s, arr(words))
		}
		return Object{"scheme": "pql-check-id-1", "probe": probe, "runbook": nil, "step": nil, "index": nil, "earlier": Array{}, "dimension": dimension, "predicate": nil, "path": nil, "words": words, "offline_args": args, "env": env, "budget_seconds": budget, "seed": seed, "store": s["store"], "bookkeeping": book, "parameters": parameters, "expected": nil}
	}
	add := func(pre Object, credit, empty bool, keys []Object) (string, error) {
		id, err := CheckID(pre)
		if err != nil {
			return "", err
		}
		r := Object{"id": id, "kind": "plain", "dimension": pre["dimension"], "record": pre["runbook"], "crediting": credit, "empty_satisfiable": empty, "asserts_emptiness": false, "precondition": nil, "declared_count": nil, "condition": nil, "condition_a": nil}
		p.Checks = append(p.Checks, Check{ID: id, Preimage: pre, Required: r, Keys: keys, Subject: s, Seed: in.Seed})
		return id, nil
	}
	commands := append(Array{}, arr(s["commands"])...)
	sort.SliceStable(commands, func(i, j int) bool {
		return len(arr(obj(commands[i])["words"])) == 0 && len(arr(obj(commands[j])["words"])) != 0
	})
	for _, spec := range []struct{ probe, dim string }{{"discovery", "discoverability"}, {"bare", "availability"}, {"byte_bound", "bounding"}, {"exit_codes", "execution"}, {"planted_secret", "pii-secret-hygiene"}, {"store_unchanged", "side-effect-integrity"}} {
		if !in.includes(spec.dim) {
			continue
		}
		candidates := Array{}
		switch spec.probe {
		case "discovery", "planted_secret":
			candidates = append(candidates, nil)
		case "bare":
			c := command(s, Array{})
			if c == nil {
				c = Object{"words": Array{}, "exit_codes": nil, "budget_seconds": strictjson.Number("10")}
			}
			candidates = append(candidates, c)
		default:
			for _, v := range commands {
				c := obj(v)
				if c["class"] != "safe" && !placeholder(c["class"]) {
					continue
				}
				if spec.probe == "byte_bound" && c["byte_bound"] != nil || spec.probe == "exit_codes" && c["exit_codes"] != nil || spec.probe == "store_unchanged" && (c["writes"] == false || placeholder(c["writes"])) {
					candidates = append(candidates, c)
				}
			}
			if len(candidates) == 0 {
				candidates = append(candidates, nil)
			}
		}
		for _, v := range candidates {
			c := obj(v)
			var words, budget any
			params := Object{}
			keys := []Object{}
			if c != nil {
				words = c["words"]
				budget = c["budget_seconds"]
				keys = append(keys, SubjectKey(s, arr(words), "seed", nil, nil))
			}
			switch spec.probe {
			case "discovery":
				keys = append(keys, DiscoveryKey(Array{}))
				cs := Array{}
				for _, v := range commands {
					w := obj(v)["words"]
					if len(arr(w)) > 0 {
						cs = append(cs, w)
					}
				}
				encoded := make([]string, len(cs))
				byEncoding := map[string]any{}
				for i, w := range cs {
					b, err := jcs.Canonical(w)
					if err != nil {
						return Plan{}, err
					}
					encoded[i] = string(b)
					byEncoding[string(b)] = w
				}
				sort.Strings(encoded)
				for i, key := range encoded {
					cs[i] = byEncoding[key]
				}
				params = Object{"declared_count": s["declared_count"], "commands": cs}
			case "bare", "exit_codes":
				if c != nil {
					params["exit_codes"] = c["exit_codes"]
				}
			case "byte_bound":
				if c != nil {
					params["bound"] = c["byte_bound"]
				}
			case "planted_secret":
				if placeholder(s["secret"]) {
					params = Object{"secret": s["secret"]}
				} else if sec := obj(s["secret"]); sec != nil {
					cs := Array{}
					for _, w := range arr(sec["commands"]) {
						cmd := command(s, arr(w))
						b := any(strictjson.Number("10"))
						if cmd != nil {
							b = cmd["budget_seconds"]
						}
						a, e, _ := offline(s, arr(w))
						cs = append(cs, Object{"words": w, "budget_seconds": b, "offline_args": a, "env": e})
						keys = append(keys, SubjectKey(s, arr(w), "planted", nil, nil))
					}
					params = Object{"commands": cs, "path": sec["path"], "kind": sec["kind"], "text_sha256": hash([]byte(text(sec["token"])))}
				}
			}
			if _, err := add(base(spec.probe, spec.dim, words, budget, params), true, spec.probe == "byte_bound", keys); err != nil {
				return Plan{}, err
			}
		}
	}
	for _, rv := range arr(s["runbooks"]) {
		r := obj(rv)
		steps := arr(r["steps"])
		allowed := true
		for _, sv := range steps {
			c := command(s, arr(obj(sv)["words"]))
			if c != nil && c["class"] == "destructive" && !in.includes("destructive-guard") {
				allowed = false
			}
		}
		if !allowed {
			continue
		}
		earlier := Array{}
		bookPlan := Runbook{}
		required := false
		for _, sv := range steps {
			args, env, _ := offline(s, arr(obj(sv)["words"]))
			stepPlan := Step{BudgetSeconds: obj(sv)["budget_seconds"], OfflineArgs: args, Env: env, Key: SubjectKey(s, arr(obj(sv)["words"]), "runbook", r["name"], obj(sv)["name"])}
			st := obj(sv)
			ids := Array{}
			for i, cv := range arr(st["checks"]) {
				ch := obj(cv)
				dim := text(ch["dimension"])
				pred := text(ch["predicate"])
				expected := obj(ch["expected"])
				asserts := pred == "stdout_empty" || pred == "row_count" && zero(expected["literal"])
				keys := []Object{SubjectKey(s, arr(st["words"]), "runbook", r["name"], st["name"])}
				ex := Object{"precondition": nil}
				for _, source := range []struct {
					name  string
					value Object
				}{
					{"expected", expected}, {"precondition", obj(ch["precondition"])},
				} {
					if source.value == nil {
						continue
					}
					comp := obj(source.value["computation"])
					if comp == nil {
						if source.name == "expected" {
							ex["literal"] = expected["literal"]
						}
						continue
					}
					fixed, err := in.computation(comp)
					if err != nil {
						return Plan{}, err
					}
					if source.name == "expected" {
						ex["computation"] = fixed
					} else {
						ex["precondition"] = Object{"computation": fixed}
					}
					keys = append(keys, Object{"words": nil, "offline_args": Array{}, "env": Array{}, "home": "scratch", "runbook": r["name"], "step": st["name"], "computation": comp["argv"]})
				}
				pre := base(nil, dim, st["words"], st["budget_seconds"], Object{"exit_codes": ch["exit_codes"]})
				pre["runbook"] = r["name"]
				pre["step"] = st["name"]
				pre["index"] = strictjson.Number(strconv.Itoa(i))
				pre["earlier"] = append(Array{}, earlier...)
				pre["predicate"] = pred
				pre["path"] = ch["path"]
				pre["expected"] = ex
				empty := pred == "stdout_not_contains" || pred == "stdout_bytes_at_most" || pred == "row_count" && expected["computation"] != nil
				if !in.includes(dim) {
					id, err := CheckID(pre)
					if err != nil {
						return Plan{}, err
					}
					ids = append(ids, id)
					stepPlan.Outside = append(stepPlan.Outside, id)
					continue
				}
				stepPlan.Required = append(stepPlan.Required, len(p.Checks))
				required = true
				credit := credits(s, r, st, ch)
				id, err := add(pre, credit, empty, keys)
				if err != nil {
					return Plan{}, err
				}
				if asserts {
					p.Checks[len(p.Checks)-1].Required["asserts_emptiness"] = true
					p.Checks[len(p.Checks)-1].Required["precondition"] = "precondition computation printed true"
				}
				ids = append(ids, id)
			}
			earlier = append(earlier, ids...)
			bookPlan.Steps = append(bookPlan.Steps, stepPlan)
		}
		if required {
			p.Runbooks = append(p.Runbooks, bookPlan)
		}
	}
	return p, nil
}
func credits(s, r, st, ch Object) bool {
	dim := text(ch["dimension"])
	pred := text(ch["predicate"])
	ex := obj(ch["expected"])
	for _, source := range []Object{ex, obj(ch["precondition"])} {
		if len(arr(obj(source["computation"])["captures"])) > 0 {
			return false
		}
	}
	switch dim {
	case "pii-secret-hygiene", "side-effect-integrity":
		return false
	case "graceful-failure":
		if pred == "exit_code" && ex["literal"] != nil && !zero(ex["literal"]) {
			return true
		}
		codes := arr(ch["exit_codes"])
		if len(codes) == 0 {
			return false
		}
		for _, c := range codes {
			if zero(c) {
				return false
			}
		}
		return true
	case "output-conformance":
		return pred == "stdout_json_equals" || pred == "json_path_equals" || pred == "row_count"
	case "error-message-quality":
		return pred == "stderr_contains"
	case "token-economy":
		return pred == "stdout_bytes_at_most"
	case "destructive-guard":
		return command(s, arr(st["words"]))["class"] == "destructive"
	case "determinism":
		for _, v := range arr(r["steps"]) {
			other := obj(v)
			if other["name"] == st["name"] || !same(other["words"], st["words"]) {
				continue
			}
			for _, v := range arr(other["checks"]) {
				c := obj(v)
				if c["dimension"] == dim && c["predicate"] == ch["predicate"] && same(c["path"], ch["path"]) && same(c["expected"], ch["expected"]) {
					return true
				}
			}
		}
		return false
	case "idempotency":
		if ex["computation"] == nil {
			return false
		}
		count := 0
		for _, v := range arr(r["steps"]) {
			c := command(s, arr(obj(v)["words"]))
			if c != nil && (c["writes"] == true || c["class"] == "destructive") {
				for _, w := range arr(r["steps"]) {
					if same(command(s, arr(obj(w)["words"])), c) {
						count++
					}
				}
				if count >= 2 {
					return true
				}
				count = 0
			}
		}
		return false
	}
	return true
}

func zero(v any) bool {
	n, ok := v.(strictjson.Number)
	return ok && (string(n) == "0" || string(n) == "-0")
}

// DiscoveryKey supplies a key for any node reached by discovery's help walk.
func DiscoveryKey(words Array) Object {
	help := append(Array{}, words...)
	help = append(help, "--help")
	return SubjectKey(nil, help, "empty", nil, nil)
}

// CheckID returns QE2's canonical preimage digest.
func CheckID(preimage Object) (string, error) { return jcs.Digest(preimage) }

// computation fixes the executable and declared-file digests for QE2.
func (in Input) computation(comp Object) (Object, error) {
	files := Object{}
	names := append(Array{}, arr(comp["declared_files"])...)
	argv := arr(comp["argv"])
	if len(argv) > 0 && !strings.HasPrefix(text(argv[0]), "/") {
		names = append(names, argv[0])
	}
	for _, n := range names {
		key := path.Clean(text(n))
		digest, ok := in.FileDigests[key]
		if !ok {
			b, found := in.Files[key]
			if !found {
				return nil, &Fault{"QE2", "missing computation file digest"}
			}
			digest = hash(b)
		}
		files[text(n)] = digest
	}
	return Object{"argv": comp["argv"], "budget_seconds": comp["budget_seconds"], "captures": comp["captures"], "files": files}, nil
}
