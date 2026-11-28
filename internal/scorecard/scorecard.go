// Package scorecard displays decided records without making decisions.
package scorecard

import (
	"fmt"
	"io"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type object = strictjson.Object
type array = strictjson.Array

// printer is the only writer path. Escape is supplied by grade to keep one
// terminal policy without a dependency from presentation back to the pipeline.
type printer struct {
	writer io.Writer
	escape func(string) string
	err    error
}

func (p *printer) line(s string) {
	if p.err == nil {
		_, p.err = io.WriteString(p.writer, p.escape(s)+"\n")
	}
}
func obj(v any) object {
	o, ok := v.(object)
	if !ok {
		return nil
	}
	return o
}
func arr(v any) array {
	a, ok := v.(array)
	if !ok {
		return nil
	}
	return a
}
func text(v any) string {
	if v == nil {
		return "missing"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSuffix(string(strictjson.AppendValue(nil, v)), "\n")
}
func reason(o object) string {
	if o["reason"] == nil {
		return ""
	}
	return " reason=" + text(o["reason"])
}
func label(o object) string {
	if o["sub_label"] == nil {
		return ""
	}
	return " (" + text(o["sub_label"]) + ")"
}

// Render reads measurement facts only from doc. Display contains masked names
// copied before the evidence bound; faults are post-decision funnel events.
func Render(w io.Writer, doc object, display record.EvidenceCopy, escape func(string) string, faults []string) error {
	p := &printer{writer: w, escape: escape}
	output := obj(doc["judge_output"])
	verdict := obj(output["verdict"])
	word := verdict["plain"]
	if doc["request"] == "anchor" {
		word = obj(output["anchor"])["result"]
	}
	p.line("pql " + text(doc["request"]) + ": " + text(word) + " (exit " + text(doc["exit_code"]) + ")" + label(verdict))
	for _, fault := range faults {
		p.line("exits 70: " + fault)
	}

	if own := obj(output["own_verdict"]); own != nil {
		p.line("own verdict: " + text(own["plain"]) + label(own))
	}
	if anchor := obj(output["anchor"]); anchor != nil && anchor["reason"] != nil {
		p.line("anchor reason: " + text(anchor["reason"]))
		for _, id := range arr(anchor["checks"]) {
			p.line("anchor check: " + text(id))
		}
	}
	p.line("scope: " + text(doc["scope"]))
	input := obj(doc["judge_input"])
	required := map[string]object{}
	evidence := map[string]object{}
	for _, v := range arr(input["required"]) {
		r := obj(v)
		required[text(r["id"])] = r
	}
	for _, v := range arr(input["evidence"]) {
		e := obj(v)
		evidence[text(e["check_id"])] = obj(e["observed"])
	}
	for id, values := range display {
		evidence[id] = obj(values.Observed)
	}
	action := func(id any, dimension any) {
		for _, v := range arr(doc["next_actions"]) {
			a := obj(v)
			if text(a["check_id"]) == text(id) && text(a["dimension"]) == text(dimension) {
				p.line("  next action: " + text(a["action"]))
			}
		}
	}
	listed := func(c object) bool {
		return c["result"] == "unobserved" || c["result"] == "failed" || c["result"] == "failed_expected"
	}
	showCheck := func(c object) {
		id := text(c["id"])
		p.line("  check " + id + ": " + text(c["result"]) + reason(c))
		action(c["id"], nil)
		observed := evidence[id]
		invocations := arr(observed["invocations"])
		if len(invocations) == 0 && observed["discovery"] == nil {
			p.line("    probe: " + text(observed["probe"]))
		}
		for i, v := range invocations {
			inv := obj(v)
			p.line("    invocation: words=" + text(inv["words"]) + " runbook=" + text(inv["runbook"]) + " step=" + text(inv["step"]) + " computation=" + text(inv["computation"]))
			if inv["computation"] != nil {
				streams := arr(observed["streams"])
				if i < len(streams) {
					stderr := obj(obj(streams[i])["stderr"])
					p.line("    computation stderr: " + text(stderr["head"]))
					if stderr["cut"] == true {
						p.line("    computation stderr was cut")
					}
				} else {
					p.line("    computation stderr: missing")
				}
			}
		}
	}
	shown := map[int]bool{}
	for _, v := range arr(output["dimensions"]) {
		d := obj(v)
		p.line("dimension " + text(d["dimension"]) + ": " + text(d["state"]) + reason(d))
		action(nil, d["dimension"])
		for i, v := range arr(output["checks"]) {
			c := obj(v)
			if text(required[text(c["id"])]["dimension"]) != text(d["dimension"]) {
				continue
			}
			if d["dimension"] == "tool-description-efficacy" && c["detail"] != nil {
				p.line("  comparison word " + text(c["id"]) + ": " + text(c["detail"]))
			}
			if listed(c) {
				showCheck(c)
				shown[i] = true
			}
		}
	}
	// Missing metadata cannot hide a failed check.
	for i, v := range arr(output["checks"]) {
		c := obj(v)
		if listed(c) && !shown[i] {
			showCheck(c)
		}
	}
	counts := map[string]int{}
	for _, v := range arr(output["checks"]) {
		c := obj(v)
		if c["result"] != "out_of_scope" {
			counts[text(c["result"])]++
		}
	}
	p.line(fmt.Sprintf("held checks: %d", counts["held_crediting"]+counts["held_non_crediting"]))
	p.line(fmt.Sprintf("check results: unobserved=%d failed=%d failed_expected=%d held_crediting=%d held_non_crediting=%d", counts["unobserved"], counts["failed"], counts["failed_expected"], counts["held_crediting"], counts["held_non_crediting"]))
	p.line("counts: xfail=" + text(obj(output["counts"])["xfail"]) + " xpass=" + text(obj(output["counts"])["xpass"]))
	// Iterate stored evidence order, never the display map's iteration order.
	for _, v := range arr(input["evidence"]) {
		observed := evidence[text(obj(v)["check_id"])]
		if discovery := obj(observed["discovery"]); discovery != nil {
			if discovery["ran"] == false {
				p.line("discovery did not run")
			} else {
				p.line("discovery: found=" + text(discovery["found_count"]) + " declared=" + text(discovery["declared_count"]))
				for _, kind := range []string{"found", "missing", "undeclared"} {
					for _, name := range arr(discovery[kind+"_names"]) {
						p.line("  " + kind + " command: " + text(name))
					}
				}
				p.line("undeclared count: " + text(discovery["undeclared_count"]))
				if discovery["found_cut"] == true {
					p.line("found names preview was cut")
				}
				if discovery["undeclared_cut"] == true {
					p.line(fmt.Sprintf("undeclared names shown: %d of %s; remaining names counted but not named", len(arr(discovery["undeclared_names"])), text(discovery["undeclared_count"])))
				}
				if discovery["request_bound_reached"] == true {
					p.line("discovery request bound reached")
				}
				if discovery["depth_bound_reached"] == true {
					p.line("discovery depth bound reached")
				}
			}
		}
		for _, location := range arr(observed["placeholders"]) {
			p.line("placeholder: " + text(location))
		}
	}
	for i, v := range arr(doc["offline_settings"]) {
		setting := obj(v)
		p.line(fmt.Sprintf("offline setting %d: applies_to=%s args=%s env_names=%s", i+1, text(setting["applies_to"]), text(setting["args"]), text(setting["env_names"])))
	}
	p.line("bookkeeping excluded: .pql-tmp")
	for _, path := range arr(doc["bookkeeping_excluded"]) {
		p.line("bookkeeping excluded: " + text(path))
	}
	for _, v := range arr(input["evidence"]) {
		observed := evidence[text(obj(v)["check_id"])]
		if path, ok := observed["store"].(string); ok {
			for _, suffix := range []string{"-shm", "-wal (when empty)", "-journal (when empty)"} {
				p.line("bookkeeping excluded: " + path + suffix)
			}
		}
	}
	p.line("Directories on the way to bookkeeping paths are also left out.")
	for _, v := range arr(input["evidence"]) {
		for _, name := range arr(evidence[text(obj(v)["check_id"])]["short_env_names"]) {
			p.line(text(name) + ": not masked: shorter than 8 bytes")
		}
	}
	comparison := obj(output["comparison"])
	if comparison["rule"] == "B1" {
		p.line("no baseline for this scope")
	} else if comparison != nil {
		p.line("comparison: " + text(comparison["result"]))
		for _, v := range arr(comparison["entries"]) {
			e := obj(v)
			p.line("  comparison entry: " + text(e["check_id"]) + " " + text(e["class"]) + " " + text(e["rule"]))
		}
		for _, v := range arr(comparison["new"]) {
			p.line("  new check: " + text(v))
		}
		if comparison["next_action"] != nil {
			p.line("  next action: " + text(comparison["next_action"]))
		}
	} else if doc["request"] != "anchor" {
		p.line("comparison: missing")
	}
	return p.err
}
