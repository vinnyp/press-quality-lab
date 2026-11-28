package grade

import (
	"fmt"
	"reflect"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/decide"
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
	observed := strictjson.Object{}
	streams := strictjson.Array{}
	invocations := strictjson.Array{}
	keys := ch.Plan.Keys
	if ch.Plan.Preimage["probe"] != "discovery" && len(keys) > 0 {
		// Computations use the collection procedure: precondition, then expected.
		ordered := make([]strictjson.Object, 0, len(keys))
		for _, key := range keys {
			if key["computation"] == nil {
				ordered = append(ordered, key)
			}
		}
		for _, comp := range decide.Computations(ch.Plan) {
			for _, key := range keys {
				if reflect.DeepEqual(key["computation"], comp["argv"]) {
					ordered = append(ordered, key)
				}
			}
		}
		for _, key := range keys {
			found := false
			for _, prior := range ordered {
				found = found || reflect.DeepEqual(prior, key)
			}
			if !found {
				ordered = append(ordered, key)
			}
		}
		keys = ordered
	}
	if ch.Plan.Preimage["probe"] == "discovery" {
		for _, inv := range ch.Invocations {
			streams = append(streams, invocationStreams(inv, masker))
		}
		if len(ch.Invocations) == 0 {
			for range ch.Plan.Keys {
				streams = append(streams, nil)
			}
		}
		observed["discovery"] = discoveryEvidence(ch)
		observed["placeholders"] = placeholderLocations(ch.Plan.Subject)
		observed["short_env_names"] = shortEnvNames(ch.Plan.Subject)
	} else {
		// Synthetic callers may supply captures without a plan; production uses declared keys.
		if len(keys) == 0 {
			for _, inv := range ch.Invocations {
				keys = append(keys, strictjson.Object(inv.Key))
			}
		}
		unique := make([]strictjson.Object, 0, len(keys))
		for _, key := range keys {
			duplicate := false
			for _, prior := range unique {
				duplicate = duplicate || reflect.DeepEqual(key, prior)
			}
			if !duplicate || len(ch.Plan.Keys) == 0 {
				unique = append(unique, key)
			}
		}
		for i, key := range unique {
			invocations = append(invocations, strictjson.Object{"words": key["words"], "runbook": key["runbook"], "step": key["step"], "computation": key["computation"]})
			var pair any
			if len(ch.Plan.Keys) == 0 {
				pair = invocationStreams(ch.Invocations[i], masker)
			}
			for _, inv := range ch.Invocations {
				if len(ch.Plan.Keys) > 0 && reflect.DeepEqual(inv.Key, map[string]any(key)) {
					pair = invocationStreams(inv, masker)
					break
				}
			}
			streams = append(streams, pair)
		}
		if len(invocations) > 0 {
			observed["invocations"] = invocations
		}
	}
	if len(observed) > 0 {
		observed["streams"] = streams
		resolution.Observed = observed
	}
	return resolution, nil
}

func invocationStreams(inv observe.Invocation, masker *record.Masker) strictjson.Object {
	run := inv.Run
	return strictjson.Object{"stdout": record.Stream(run.Stdout, run.StdoutSeen, run.StdoutCut, run.Details.Stdout, masker), "stderr": record.Stream(run.Stderr, run.StderrSeen, run.StderrCut, run.Details.Stderr, masker)}
}
