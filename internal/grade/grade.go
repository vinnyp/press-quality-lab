// Package grade owns the ordered collection pipeline and its exit funnel.
package grade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/judge"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/record"
	"github.com/vinnyp/press-quality-lab/internal/scorecard"
	"github.com/vinnyp/press-quality-lab/internal/store"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"github.com/vinnyp/press-quality-lab/internal/subject"
	"github.com/vinnyp/press-quality-lab/internal/throwaway"
)

// ComputationWarning is shown before grading and in command documentation (QA5).
const ComputationWarning = "Warning: grading another person's subject file runs their computations on this machine."

type Options struct {
	SubjectFile               string
	Scope                     []string
	Baseline, Retired, Accept string
	Anchor, Scripted, JSON    bool
}

// Error carries a classified failure without exiting the process.
type Error struct {
	Code        int
	Rule, Cause string
}

func (e *Error) RuleID() string                   { return e.Rule }
func (e *Error) Error() string                    { return "rejected: " + e.Rule + ": " + e.Cause }
func failure(code int, rule, cause string) *Error { return &Error{code, rule, cause} }

// Dependencies exposes effects, including identity before any store resolution.
// Source is constructed inside the funnel. Its trace closes only after groups
// end and the collector stops; otherwise a stuck collector may still hold it.
type Dependencies struct {
	CollectorWait time.Duration // Zero uses the five-second production bound.
	EndGroups     func(*throwaway.Workspace, io.Writer) error
	Remove        func(*throwaway.Workspace) error
	// Workspace is constructed after root refusal, inside the exit funnel.
	Workspace         func() (*throwaway.Workspace, error)
	SourceInWorkspace func(*throwaway.Workspace) (observe.Source, func() error, error)
	Stderr            io.Writer
	Masker            *record.Masker
	// Signals permits deterministic tests; nil subscribes to the four signals.
	Signals           <-chan os.Signal
	OpenSystemFile    func(string) (*os.File, error) // Nil opens absolute computation executables nonblocking.
	ResolveSystemFile func(string) (string, error)
	StatSystemFile    func(string) (os.FileInfo, error)
	CopySystemFile    func(io.Writer, io.Reader) (int64, error)
	StatInput         func(string) (os.FileInfo, error)
	SystemPath        func(string) bool
	AccountHome       func() (string, error) // Nil uses the system account record, never an environment fallback.
	IDs               func() (real, effective int)
	Open              func(string) (io.ReadCloser, error)
	Source            func() (observe.Source, func() error, error)
	Judge             func([]byte) ([]byte, int)
	StoreRoot         func() (string, error)
	Write             func(store.Record) error
	Collect           func(plan.Plan, observe.Source) ([]collect.Check, error)
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
	if d.CollectorWait <= 0 {
		d.CollectorWait = 5 * time.Second
	}
	if d.EndGroups == nil {
		d.EndGroups = (*throwaway.Workspace).EndGroups
	}
	if d.Remove == nil {
		d.Remove = (*throwaway.Workspace).Remove
	}
	if d.Workspace == nil {
		d.Workspace = throwaway.Begin
	}
	if d.Stderr == nil {
		d.Stderr = os.Stderr
	}
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
	var waitCollector func() error
	groupsEnded := true
	var workspace *throwaway.Workspace
	var decided strictjson.Object
	var display record.EvidenceCopy
	var postJudgeFaults []string
	d = defaults(d)
	if d.Masker == nil {
		d.Masker = record.NewMasker(nil, nil)
	}
	maskedOut, maskedErr := d.Masker.Writer(out), d.Masker.Writer(d.Stderr)
	out, d.Stderr = maskedOut, maskedErr
	defer func() {
		if flushErr := errors.Join(maskedOut.Flush(), maskedErr.Flush()); flushErr != nil {
			code = 70
			err = errors.Join(err, flushErr)
		}
		if err != nil {
			err = maskedError{err, d.Masker}
		}
	}()
	signals := d.Signals
	if signals == nil {
		ch := make(chan os.Signal, 4)
		for _, sig := range []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
			if !signal.Ignored(sig) {
				signal.Notify(ch, sig)
			}
		}
		defer signal.Stop(ch)
		signals = ch
	}
	interrupted := false
	defer func() {
		add := func(rule string, cause error) {
			if cause != nil {
				if decided != nil && rule != "QE20" && rule != "QH23" {
					postJudgeFaults = append(postJudgeFaults, cause.Error())
				}
				err = errors.Join(failure(70, rule, cause.Error()), err)
				code = 70
			}
		}
		if p := recover(); p != nil {
			add("QE16", fmt.Errorf("engine panic: %v", p))
		}
		// Cleanup remains interruptible even when a filesystem operation blocks.
		step := func(fn func() error) error {
			done := make(chan error, 1)
			go func() { done <- closeSafely(fn) }()
			for {
				select {
				case e := <-done:
					return e
				case sig := <-signals:
					if interrupted {
						os.Exit(70)
					}
					interrupted = true
					add("QE16", fmt.Errorf("interrupted by %s", sig))
				}
			}
		}
		if workspace != nil {
			if endErr := step(func() error { return d.EndGroups(workspace, d.Stderr) }); endErr != nil {
				groupsEnded = false
				add("QH23", endErr)
			}
		}
		if waitCollector != nil {
			if waitErr := step(waitCollector); waitErr != nil {
				groupsEnded = false
				add("QH23", waitErr)
			}
		}
		if closeSource != nil && groupsEnded {
			add("QE16", step(closeSource))
		}
		if workspace != nil {
			if groupsEnded {
				if removeErr := step(func() error { return d.Remove(workspace) }); removeErr != nil {
					groupsEnded = false
					add("QE20", removeErr)
				}
			}
			if !groupsEnded {
				cleanup := "cleanup failed; directory left behind: " + d.Masker.Mask(workspace.Path())
				if decided != nil {
					postJudgeFaults = append(postJudgeFaults, cleanup)
				}
				var reportErr error
				if decided == nil || o.JSON {
					_, reportErr = io.WriteString(d.Stderr, Escape(cleanup)+"\n")
				}
				add("QE20", reportErr)
			}
		}
		select {
		case sig := <-signals:
			if interrupted {
				os.Exit(70)
			}
			add("QE16", fmt.Errorf("interrupted by %s", sig))
		default:
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
		if decided != nil {
			var printErr error
			if o.JSON {
				_, printErr = record.RawWriter(out).Write(terminalJSON(strictjson.Write(decided["judge_output"])))
			} else {
				printErr = scorecard.Render(record.RawWriter(out), decided, display, Escape, postJudgeFaults)
			}
			if printErr != nil {
				code = 70
				err = errors.Join(err, printErr)
			}
		}
	}()

	_, effective := d.IDs()
	if effective == 0 {
		return 70, failure(70, "QH3", "effective user is root")
	}
	workspace, err = d.Workspace()
	if err != nil {
		return 70, err
	}
	canonical, pathErr := physicalSubjectPath(o.SubjectFile)
	if pathErr != nil {
		return 2, failure(2, "QS2", "subject file unreadable")
	}
	var subjectInfo os.FileInfo
	data, err := readFile(canonical, func(path string) (io.ReadCloser, error) {
		f, e := d.Open(path)
		if e != nil {
			return nil, e
		}
		if descriptor, ok := f.(interface{ Stat() (os.FileInfo, error) }); ok {
			subjectInfo, e = descriptor.Stat()
			if e != nil {
				return nil, errors.Join(e, f.Close())
			}
		} else {
			return nil, errors.Join(errors.New("subject reader cannot be statted"), f.Close())
		}
		return f, nil
	})
	if err != nil {
		return 2, failure(2, "QS2", "subject file unreadable: "+err.Error())
	}
	var in snapshot
	var inputErr error
	s, parseErr := subject.ParseWithFiles(data, func(s subject.File) []string {
		in, inputErr = planningInputWith(o, s, inputAccess{resolveAbsolute: d.ResolveSystemFile, statAbsolute: d.StatSystemFile, copyAbsolute: d.CopySystemFile, canonical: canonical, subjectInfo: subjectInfo, stat: d.StatInput, systemPath: d.SystemPath, home: d.AccountHome, openAbsolute: d.OpenSystemFile})
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
	for _, setting := range subjectArray(s["offline"]) {
		for _, v := range subjectObject(subjectObject(setting)["env"]) {
			if value, ok := v.(string); ok {
				d.Masker.AddDeclared(value)
			}
		}
	}
	fixed := in.Input
	fixed.Baseline, fixed.Anchor = false, false
	p, err := plan.Build(fixed)
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
	rawFacts, factsOK := s["facts"].(strictjson.Object)
	if !factsOK {
		return 70, failure(70, "QE16", "parsed subject facts lost")
	}
	facts := strictjson.Object{}
	for key, value := range rawFacts {
		if _, ok := value.(strictjson.Object); ok {
			facts[key] = nil
		} else {
			facts[key] = value
		}
	}
	if in.Baseline || in.Anchor {
		source, closer, sourceErr := d.Source()
		closeSource = closer
		if sourceErr != nil {
			return 70, sourceErr
		}
		if traced, ok := source.(interface{ Required(any) error }); ok {
			if err := traced.Required(required); err != nil {
				return 70, err
			}
		}
		part := ""
		if in.Anchor {
			part = "anchor"
		}
		return 70, &plan.Gap{Rule: "QE4", Part: part}
	}
	tier := "live"
	if o.Scripted {
		tier = "scripted"
	}
	pre := strictjson.Object{"request": request, "scope": scopes, "claimed_tier": tier, "subject_facts": facts, "markers": s["markers"], "required": required, "evidence": strictjson.Array{}, "baseline": nil, "retired": strictjson.Array{}, "accepted_regressions": strictjson.Array{}, "trials": strictjson.Array{}}
	preOutput, _, err := callJudge(d.Judge, strictjson.Write(pre))
	if err != nil {
		return 70, err
	}
	if preOutput["outcome"] == "rejected" {
		rule := stringValue(preOutput["rejected_by"])
		return precheck(rule)
	}
	var source observe.Source
	var closer func() error
	if d.SourceInWorkspace != nil {
		source, closer, err = d.SourceInWorkspace(workspace)
	} else {
		source, closer, err = d.Source()
	}
	closeSource = closer
	if err != nil {
		return 70, err
	}
	if traced, ok := source.(interface{ Required(any) error }); ok {
		if err := traced.Required(required); err != nil {
			return 70, err
		}
	}
	checks, wait, collectErr := gatherSignals(d.Collect, p, source, signals, d.CollectorWait)
	waitCollector = wait
	interrupted = wait != nil
	err = collectErr
	if err != nil {
		return 70, err
	}
	results := map[string]record.Resolution{}
	for _, ch := range checks {
		if ch.Result.Unbuilt != nil {
			return 70, failure(70, ch.Result.Unbuilt.Rule, "collection level not built")
		}
		resolution, resolutionErr := checkResolution(ch, d.Masker)
		if resolutionErr != nil {
			return 70, resolutionErr
		}
		results[ch.Plan.ID] = resolution
	}
	markers, markersOK := s["markers"].(strictjson.Array)
	if !markersOK {
		return 70, failure(70, "QE16", "parsed subject shape lost")
	}
	input, names, err := record.Build(record.Input{Plan: p, Results: results, Request: request, Scope: scopes, SubjectFacts: facts, Markers: markers, Scripted: o.Scripted, Masker: d.Masker})
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
	r.NextActions, r.OfflineSettings, r.BookkeepingExcluded = record.Interim(s, output, d.Masker)
	r.BinaryDigest = in.BinaryDigest

	displayPath := d.Masker.Mask(canonical)
	r.DisplaySubjectFilePath = &displayPath
	if !rejected {
		decided = r.Document()
		display = names
	}
	storeErr := persist(d, r)
	if storeErr != nil {
		if decided != nil {
			postJudgeFaults = append(postJudgeFaults, "record was not stored")
		}
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

func gatherSignals(fn func(plan.Plan, observe.Source) ([]collect.Check, error), p plan.Plan, source observe.Source, signals <-chan os.Signal, bound time.Duration) ([]collect.Check, func() error, error) {
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
	select {
	case r := <-done:
		return r.checks, nil, r.err
	case sig := <-signals:
		return nil, func() error {
			timer := time.NewTimer(bound)
			defer timer.Stop()
			select {
			case r := <-done:
				return r.err
			case <-timer.C:
				return errors.New("collector did not stop after signal")
			}
		}, failure(70, "QE16", "interrupted by "+sig.String())
	}
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
		if terminalUnsafe(r) {
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

type maskedError struct {
	err    error
	masker *record.Masker
}

func (e maskedError) Error() string      { return e.masker.Mask(e.err.Error()) }
func (e maskedError) MaskedText() string { return e.Error() }
func (e maskedError) Unwrap() error      { return e.err }
func subjectArray(v any) strictjson.Array {
	a, ok := v.(strictjson.Array)
	if !ok {
		return nil
	}
	return a
}
func subjectObject(v any) strictjson.Object {
	o, ok := v.(strictjson.Object)
	if !ok {
		return nil
	}
	return o
}

func terminalUnsafe(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' || r >= 0xe0000 && r <= 0xe007f
}

// terminalJSON preserves JSON values while spelling terminal controls as escapes.
func terminalJSON(raw []byte) []byte {
	var b strings.Builder
	for _, r := range string(raw) {
		if terminalUnsafe(r) && r >= 0x20 {
			if r > 0xffff {
				a, z := utf16.EncodeRune(r)
				fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
			} else {
				fmt.Fprintf(&b, "\\u%04x", r)
			}
		} else {
			b.WriteRune(r)
		}
	}
	return []byte(b.String())
}
