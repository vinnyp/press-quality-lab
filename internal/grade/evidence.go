package grade

import (
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"

	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func checkResolution(ch collect.Check, masker *record.Masker) (record.Resolution, error) {
	var failure any
	if ch.Result.FailureKind != "" {
		failure = ch.Result.FailureKind
	}
	resolution := record.Resolution{Result: ch.Result, FailureKind: failure, Keys: ch.Plan.Keys}
	if ch.Plan.Preimage["probe"] == "byte_bound" && (ch.Result.Outcome == "held" || ch.Result.FailureKind == "predicate") {
		if len(ch.Invocations) != 1 {
			return record.Resolution{}, fmt.Errorf("QE8: missing evaluated byte-bound invocation")
		}
		run := ch.Invocations[0].Run
		if err := observe.ValidateCounts(run, true); err != nil {
			return record.Resolution{}, err
		}
		resolution.TargetSize = run.StdoutSeen + run.StderrSeen
		resolution.Expected = subjectObject(ch.Plan.Preimage["parameters"])["bound"]
	}
	if ch.Plan.Preimage["probe"] == nil {
		resolution.Expected = ch.Result.Expected
		if ch.Result.Outcome != "not_observed" {
			if ch.Result.TargetSize != nil {
				resolution.TargetSize = *ch.Result.TargetSize
			}
			if ch.Result.PreconditionObserved != nil {
				resolution.PreconditionObserved = *ch.Result.PreconditionObserved
			}
		}
	}
	if len(ch.Invocations) > 0 {
		streams := strictjson.Array{}
		for _, inv := range ch.Invocations {
			run := inv.Run
			streams = append(streams, strictjson.Object{"stdout": record.Stream(run.Stdout, run.StdoutSeen, run.StdoutCut, run.Details.Stdout, masker), "stderr": record.Stream(run.Stderr, run.StderrSeen, run.StderrCut, run.Details.Stderr, masker)})
		}
		resolution.Observed = streams
	}
	return resolution, nil
}
