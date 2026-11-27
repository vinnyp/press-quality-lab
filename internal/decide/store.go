package decide

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func below(parent, name string) bool { return name == parent || strings.HasPrefix(name, parent+"/") }
func storeExcluded(ch plan.Check, name, kind, change string) bool {
	if kind == "link" || kind == "special" {
		return false
	}
	if below(".pql-tmp", name) {
		return true
	}
	for _, v := range array(ch.Preimage["bookkeeping"]) {
		book, ok := v.(string)
		if !ok {
			panic("QP7: invalid bookkeeping path")
		}
		if below(book, name) || kind == "directory" && change == "created" && strings.HasPrefix(book, name+"/") {
			return true
		}
	}
	store, ok := ch.Preimage["store"].(string)
	if !ok {
		store = ""
	}
	return kind == "file" && store != "" && name == store+"-shm"
}
func emptyDigest() string { return fmt.Sprintf("%x", sha256.Sum256(nil)) }
func storeUnchanged(ch plan.Check, e Evidence) Result {
	empty := Result{Level: 8, Outcome: NotObserved, Reason: "empty_target"}
	if len(ch.Keys) == 0 || ch.Preimage["store"] == nil {
		return empty
	}
	target := false
	for _, entry := range ch.Seed {
		if entry.Directory || storeExcluded(ch, entry.Path, "file", "") {
			continue
		}
		digest := entry.Digest
		if digest == "" {
			digest = fmt.Sprintf("%x", sha256.Sum256(entry.Data))
		}
		if digest != emptyDigest() {
			target = true
		}
	}
	if !target {
		return empty
	}
	fault := Result{Level: 8, Outcome: NotObserved, Reason: "harness_fault"}
	if len(e.Invocations) != 1 {
		panic("QP6: missing store invocation")
	}
	home := e.Invocations[0].Run.Details.Home
	if home.ControlDetected == nil || !*home.ControlDetected {
		return fault
	}
	store, ok := ch.Preimage["store"].(string)
	if !ok {
		store = ""
	}
	for _, change := range home.Changes {
		if storeExcluded(ch, change.Path, change.Kind, change.Change) {
			continue
		}
		if change.Kind == "file" && (change.Path == store+"-wal" || change.Path == store+"-journal") && (change.Change == "removed" || change.Digest != nil && *change.Digest == emptyDigest()) {
			continue
		}
		return Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}
	}
	if home.Unreadable {
		return fault
	}
	return Result{Level: 8, Outcome: "held"}
}
