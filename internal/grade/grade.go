// Package grade owns the ordered collection pipeline and its exit funnel.
package grade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
)

// ComputationWarning is shown before grading and in command documentation (QA5).
const ComputationWarning = "Warning: grading another person's subject file runs their computations on this machine."

type Options struct {
	SubjectFile               string
	Scope                     []string
	Baseline, Retired, Accept string
	Anchor, Scripted          bool
}

// Error carries a classified failure without exiting the process.
type Error struct {
	Code        int
	Rule, Cause string
}

func (e *Error) RuleID() string                   { return e.Rule }
func (e *Error) Error() string                    { return "rejected: " + e.Rule + ": " + Escape(e.Cause) }
func failure(code int, rule, cause string) *Error { return &Error{code, rule, cause} }

// Dependencies exposes effects, including identity before any store resolution.
// Source is constructed inside the funnel, so its trace is closed on all exits.
type Dependencies struct {
	IDs       func() (real, effective int)
	Open      func(string) (io.ReadCloser, error)
	Source    func() (observe.Source, func() error, error)
	Judge     func([]byte) ([]byte, int)
	StoreRoot func() (string, error)
	Write     func(store.Record) error
	Collect   func(plan.Plan, observe.Source) ([]collect.Check, error)
}

func UserIDs() (int, int) { return os.Getuid(), os.Geteuid() }

// ReleasedSource deliberately supplies no verified system. No observation may
// be served until the real verifier replaces this interim path (KTD21).
type ReleasedSource struct{}

func (ReleasedSource) Controls() (observe.Controls, error) { return observe.Controls{}, nil }
func (ReleasedSource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	return observe.Run{}, failure(70, "QO1", "released source must never observe")
}

func defaults(d Dependencies) Dependencies {
	if d.IDs == nil {
		d.IDs = UserIDs
	}
	if d.Open == nil {
		d.Open = openInput
	}
	if d.Source == nil {
		d.Source = func() (observe.Source, func() error, error) {
			return ReleasedSource{}, func() error { return nil }, nil
		}
	}
	if d.Judge == nil {
		d.Judge = judge.Judge
	}
	if d.StoreRoot == nil {
		d.StoreRoot = store.ApplicationSupportRoot
	}
	if d.Collect == nil {
		d.Collect = collect.Collect
	}
	return d
}

// Run has one exit funnel. No collection goroutine can escape its recovery.
func Run(o Options, d Dependencies, out io.Writer) (code int, err error) {
	code = 70
	var closeSource func() error
	defer func() {
		if p := recover(); p != nil {
			err = failure(70, "QE16", fmt.Sprintf("engine panic: %v", p))
			code = 70
		}
		if closeSource != nil {
			if closeErr := closeSafely(closeSource); closeErr != nil {
				err = failure(70, "QE16", "trace close failed: "+closeErr.Error())
				code = 70
			}
		}
		if err != nil {
			var classified *Error
			if errors.As(err, &classified) {
				code = classified.Code
			} else {
				classified = classify(err)
				err = classified
				code = classified.Code
			}
		}
	}()
	d = defaults(d)
	_, effective := d.IDs()
	if effective == 0 {
		return 70, failure(70, "QH3", "effective user is root")
	}
	data, err := readFile(o.SubjectFile, d.Open)
	if err != nil {
		return 2, failure(2, "QS2", "subject file unreadable: "+err.Error())
	}
	var in snapshot
	var inputErr error
	s, parseErr := subject.ParseWithFiles(data, func(s subject.File) []string {
		in, inputErr = planningInput(o, s)
		var refusal *Error
		if errors.As(inputErr, &refusal) && refusal.Code == 2 {
			return []string{refusal.Rule}
		}
		return nil
	})
	if parseErr != nil {
		return 2, failure(2, parseErr.Rule, "invalid subject file")
	}
	if inputErr != nil {
		return 70, inputErr
	}
	p, err := plan.Build(in.Input)
	if err != nil {
		return 70, err
	}
	request := "grade"
	if o.Anchor {
		request = "anchor"
	}
	scopes := strictjson.Array{}
	for _, scope := range o.Scope {
		scopes = append(scopes, scope)
	}
	required := strictjson.Array{}
	for _, ch := range p.Checks {
		required = append(required, ch.Required)
	}
	tier := "live"
	if o.Scripted {
		tier = "scripted"
	}
	pre := strictjson.Object{"request": request, "scope": scopes, "claimed_tier": tier, "subject_facts": s["facts"], "markers": s["markers"], "required": required, "evidence": strictjson.Array{}, "baseline": nil, "retired": strictjson.Array{}, "accepted_regressions": strictjson.Array{}, "trials": strictjson.Array{}}
	preOutput, _, err := callJudge(d.Judge, strictjson.Write(pre))
	if err != nil {
		return 70, err
	}
	if preOutput["outcome"] == "rejected" {
		rule := stringValue(preOutput["rejected_by"])
		return precheck(rule)
	}
	source, closer, err := d.Source()
	closeSource = closer
	if err != nil {
		return 70, err
	}
	if traced, ok := source.(interface{ Required(any) error }); ok {
		if err := traced.Required(required); err != nil {
			return 70, err
		}
	}
	checks, err := gather(d.Collect, p, source)
	if err != nil {
		return 70, err
	}
	results := map[string]record.Resolution{}
	for _, ch := range checks {
		if ch.Result.Unbuilt != nil {
			return 70, failure(70, ch.Result.Unbuilt.Rule, "collection level not built")
		}
		results[ch.Plan.ID] = record.Resolution{Result: ch.Result}
	}
	facts, factsOK := s["facts"].(strictjson.Object)
	markers, markersOK := s["markers"].(strictjson.Array)
	if !factsOK || !markersOK {
		return 70, failure(70, "QE16", "parsed subject shape lost")
	}
	input, err := record.Build(record.Input{Plan: p, Results: results, Request: request, Scope: scopes, SubjectFacts: facts, Markers: markers, Scripted: o.Scripted})
	if err != nil {
		return 70, err
	}
	inputBytes := strictjson.Write(input)
	output, judgedCode, err := callJudge(d.Judge, inputBytes)
	if err != nil {
		return 70, err
	}
	rejected := output["outcome"] == "rejected"
	code = judgedCode
	if rejected {
		code = 70
	}
	canonical, err := filepath.EvalSymlinks(o.SubjectFile)
	if err != nil {
		return 70, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return 70, err
	}
	host, hostErr := hostFields()
	if hostErr != nil {
		return 70, hostErr
	}
	r := store.Record{Host: host, SubjectFilePath: canonical, SubjectFileDigest: fmt.Sprintf("%x", sha256.Sum256(data)), Request: request, Scope: scopes, ExitCode: code, JudgeInput: input, JudgeOutput: output, JudgeRejected: rejected}
	if s["seed"] != nil {
		digest, digestErr := plan.SeedDigest(in.Seed)
		if digestErr != nil {
			return 70, digestErr
		}
		r.SeedDigest = &digest
	}
	r.BinaryDigest = in.BinaryDigest

	storeErr := persist(d, r)
	if !rejected {
		if _, err := io.WriteString(out, Escape(strings.TrimSuffix(string(strictjson.Write(output)), "\n"))+"\n"); err != nil {
			return 70, err
		}
	}
	if storeErr != nil {
		return 70, failure(70, "QE17", "record was not stored")
	}
	if rejected {
		rule := stringValue(output["rejected_by"])
		return 70, failure(70, rule, "judge rejected collected record")
	}
	return code, nil
}

func closeSafely(close func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("trace close panic: %v", p)
		}
	}()
	return close()
}

func persist(d Dependencies, r store.Record) error {
	if d.Write != nil {
		return d.Write(r)
	}
	root, err := d.StoreRoot()
	if err != nil {
		return err
	}
	st, err := store.New(root)
	if err != nil {
		return err
	}
	_, err = st.Write(r)
	return err
}

func precheck(rule string) (int, error) {
	code, cause := 70, "engine"
	switch rule {
	case "X1", "X4", "X11":
		code, cause = 2, "user"
	}
	return code, failure(code, "QE6", rule+": "+cause+" caused pre-check rejection")
}

func callJudge(fn func([]byte) ([]byte, int), input []byte) (strictjson.Object, int, error) {
	output, code, defect := judge.Safe(fn, input)
	if defect != nil {
		return nil, 70, failure(70, "QE16", defect.Error())
	}
	v, ok := strictjson.Read(output, strictjson.Bounds{Bytes: 16 << 20, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
	doc, object := v.(strictjson.Object)
	if !ok || !object {
		return nil, 70, failure(70, "QE16", "invalid judge output")
	}
	n, numeric := doc["exit_code"].(strictjson.Number)
	if !numeric || string(n) != strconv.Itoa(code) {
		return nil, 70, failure(70, "QE16", "judge exit code disagreement")
	}
	return doc, code, nil
}

func gather(fn func(plan.Plan, observe.Source) ([]collect.Check, error), p plan.Plan, source observe.Source) ([]collect.Check, error) {
	type result struct {
		checks []collect.Check
		err    error
	}
	done := make(chan result, 1)
	go func() {
		r := result{}
		defer func() {
			if v := recover(); v != nil {
				r.err = failure(70, "QE16", fmt.Sprintf("collector panic: %v", v))
			}
			done <- r
		}()
		r.checks, r.err = fn(p, source)
	}()
	r := <-done
	return r.checks, r.err
}

func classify(err error) *Error {
	var gap *plan.Gap
	if errors.As(err, &gap) {
		return failure(70, gap.Rule, "planning rule not built")
	}
	var fault *record.Fault
	if errors.As(err, &fault) {
		return failure(70, fault.Rule, "record cannot be built")
	}
	var sourceFault interface{ RuleID() string }
	if errors.As(err, &sourceFault) {
		return failure(70, sourceFault.RuleID(), "engine source fault")
	}

	return failure(70, "QE16", err.Error())
}

// Escape is the sole terminal escaping path, independent of terminal detection.
func Escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' || r >= 0xe0000 && r <= 0xe007f {
			fmt.Fprintf(&b, "\\u{%04x}", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func readFile(path string, open func(string) (io.ReadCloser, error)) ([]byte, error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	return data, errors.Join(readErr, f.Close())
}

func stringValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
