// Package record builds bounded judge inputs without I/O or verdicts.
package record

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type Object = strictjson.Object
type Array = strictjson.Array

// Fault is an engine-fault value; the caller maps it to exit 70.
type Fault struct{ Rule string }

func (f *Fault) Error() string { return f.Rule + ": record cannot be built" }

// Resolution adds predicate details to the precedence table's result. Invocation
// keys come from the plan, except discovery's actual walk, supplied in Keys.
type Resolution struct {
	Result                                                            decide.Result
	Expected, Observed, FailureKind, TargetSize, PreconditionObserved any
	Keys                                                              []Object
}
type Input struct {
	Plan         plan.Plan
	Results      map[string]Resolution
	Request      string
	Scope        Array
	SubjectFacts Object
	Markers      Array
	Scripted     bool
	Masker       *Masker
}

// EvidenceValues contains display values only; verdict facts stay in the document.
type EvidenceValues struct{ Expected, Observed any }
type EvidenceCopy map[string]EvidenceValues

// Build returns a fresh judge document and masked display values before QE10.
func Build(in Input) (docResult Object, display EvidenceCopy, buildErr error) {
	defer func() {
		if p := recover(); p != nil {
			if f, ok := p.(*Fault); ok {
				docResult = nil
				display = nil
				buildErr = f
			} else {
				panic(p)
			}
		}
	}()
	required, evidence := Array{}, Array{}
	checks := append([]plan.Check(nil), in.Plan.Checks...)
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	for _, ch := range checks {
		r, ok := in.Results[ch.ID]
		if !ok || r.Result.Unbuilt != nil || r.Result.Outcome == "" {
			return nil, nil, &Fault{"QE8"}
		}

		if r.Result.Outcome == decide.NotObserved {
			if !decide.ValidReason(r.Result.Reason) {
				return nil, nil, &Fault{"QE13"}
			}
		} else if (r.Result.Outcome != "held" && r.Result.Outcome != "not_held") || r.Result.Reason != "" {
			return nil, nil, &Fault{"QE13"}
		}
		req := Object{}
		for k, v := range ch.Required {
			req[k] = v
		}
		required = append(required, req)
		keys := ch.Keys
		if r.Keys != nil {
			keys = r.Keys
		}
		// QE8: computations supply expected values, not subject targets.
		subjectKeys := make([]Object, 0, len(keys))
		for _, key := range keys {
			if key["computation"] == nil {
				subjectKeys = append(subjectKeys, key)
			}
		}
		keys = subjectKeys
		targets := Array{}
		tier := "offline"
		n := len(keys)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			t := "live"
			if i < len(keys) {
				args := array(keys[i]["offline_args"])
				env := array(keys[i]["env"])
				if len(args) > 0 || len(env) > 0 {
					t = "offline"
				}
			}
			if in.Scripted {
				t = "scripted"
			}
			if t == "live" {
				tier = "live"
			}
			id := ch.ID
			if n > 1 {
				id = fmt.Sprintf("%s#%d", id, i+1)
			}
			targets = append(targets, Object{"kind": "invocation", "id": id, "tier": t})
		}
		artifact := any(nil)
		if ch.Preimage["probe"] == "store_unchanged" && len(keys) > 0 {
			artifact = ch.Preimage["store"]
		}
		if ch.Preimage["probe"] == "planted_secret" {
			p := object(ch.Preimage["parameters"])
			artifact = p["path"]
		}
		if in.Scripted {
			tier = "scripted"
		}
		if path, ok := artifact.(string); ok {
			targets = append(targets, Object{"kind": "artifact", "id": path, "tier": tier})
		}
		var reason any
		if r.Result.Reason != "" {
			reason = string(r.Result.Reason)
		}
		evidence = append(evidence, Object{"check_id": ch.ID, "outcome": string(r.Result.Outcome), "reason": reason, "failure_kind": r.FailureKind, "target_size": r.TargetSize, "precondition_observed": r.PreconditionObserved, "expected": r.Expected, "observed": r.Observed, "targets": targets, "tier": tier, "evaluator": Object{"kind": "deterministic", "model": nil}})
	}
	tier := "live"
	if in.Scripted {
		tier = "scripted"
	}
	markers := in.Markers
	if markers == nil {
		markers = Array{}
	}
	doc := Object{"request": in.Request, "scope": in.Scope, "claimed_tier": tier, "subject_facts": in.SubjectFacts, "markers": markers, "required": required, "evidence": evidence, "baseline": nil, "retired": Array{}, "accepted_regressions": Array{}, "trials": Array{}}
	maskText(doc, in.Masker)
	display = EvidenceCopy{}
	for i, v := range evidence {
		e := object(v)
		id := checks[i].ID
		display[id] = EvidenceValues{cloneValue(e["expected"]), cloneValue(e["observed"])}
	}
	if err := bound(doc, evidence); err != nil {
		return nil, nil, err
	}
	return doc, display, nil
}

func digest(v any) Object {
	b := strictjson.AppendValue(nil, v)
	return Object{"bytes": len(b), "sha256": fmt.Sprintf("%x", sha256.Sum256(b))}
}
func bound(doc Object, evidence Array) error {
	// QE19 (collection-contract.md line 237) leaves content to the collector;
	// judge-contract.md line 26 says no rule reads these values. Choose expected
	// before observed within a check in both passes, including reverse-id order.
	type value struct {
		evidence    Object
		key         string
		size        int
		replacement Object
		replaced    bool
	}
	values := make([]value, 0, 2*len(evidence))
	total := 0
	for _, v := range evidence {
		e := object(v)
		for _, key := range []string{"expected", "observed"} {
			d := digest(e[key])
			values = append(values, value{e, key, strictjson.Length(e[key]) - 1, d, false})
			total += strictjson.Length(d) - 1
		}
	}
	// Start with every value digested, then find the longest fitting prefix.
	// A digest may be larger than its original value, so candidate totals need
	// not increase monotonically. Include all suffix digests in each candidate.
	keep := -1
	if total <= 4<<20 {
		keep = 0
	}
	for i, v := range values {
		total += v.size - (strictjson.Length(v.replacement) - 1)
		if total <= 4<<20 {
			keep = i + 1
		}
	}
	if keep < 0 {
		return &Fault{"QE10"}
	}
	for i := keep; i < len(values); i++ {
		v := &values[i]
		v.evidence[v.key] = v.replacement
		v.replaced = true
	}
	size := strictjson.Length(doc)
	for i := len(evidence) - 1; i >= 0 && size > 15<<20; i-- {
		for _, j := range []int{2 * i, 2*i + 1} {
			if size <= 15<<20 {
				break
			}
			v := &values[j]
			if !v.replaced {
				v.evidence[v.key] = v.replacement
				size += strictjson.Length(v.replacement) - 1 - v.size
				v.replaced = true
			}
		}
	}
	if size > 15<<20 {
		return &Fault{"QE10"}
	}
	return nil
}

func object(v any) Object {
	o, ok := v.(Object)
	if !ok {
		return nil
	}
	return o
}
func array(v any) Array {
	a, ok := v.(Array)
	if !ok {
		return nil
	}
	return a
}

// maskText masks only values displayed as evidence and artifact paths. Required
// entries, outcomes, reasons, tiers and digests remain measurement facts.
func maskText(doc Object, m *Masker) {
	for _, v := range array(doc["evidence"]) {
		e := object(v)
		for _, k := range []string{"expected", "observed"} {
			e[k] = maskValue(e[k], m)
		}
		for _, v := range array(e["targets"]) {
			target := object(v)
			if target["kind"] == "artifact" {
				if path, ok := target["id"].(string); ok {
					target["id"] = m.Mask(path)
				}
			}
		}
	}
}
func maskValue(v any, m *Masker) any {
	switch x := v.(type) {
	case string:
		return m.Mask(x)
	case Object:
		masked := Object{}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := m.Mask(k)
			if _, exists := masked[key]; exists {
				panic(&Fault{"QE10"})
			}
			masked[key] = maskValue(x[k], m)
		}
		return masked
	case streamObject:
		return Object(x)
	case Array:
		masked := make(Array, len(x))
		for i, v := range x {
			masked[i] = maskValue(v, m)
		}
		return masked
	}
	return v
}

// cloneValue detaches nested evidence objects, including stream objects.
func cloneValue(v any) any {
	switch x := v.(type) {
	case Object:
		out := Object{}
		for k, v := range x {
			out[k] = cloneValue(v)
		}
		return out
	case streamObject:
		return cloneValue(Object(x))
	case Array:
		out := make(Array, len(x))
		for i, v := range x {
			out[i] = cloneValue(v)
		}
		return out
	default:
		return v
	}
}
