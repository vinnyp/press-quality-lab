package grade

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/collect"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/store"
)

func TestScopeRefusals(t *testing.T) {
	// E-7, X1: invalid scope lists reach the pre-check and never reach collection.
	for _, row := range []struct {
		name  string
		scope []string
	}{
		{"DestructiveAlone", []string{"destructive"}},
		{"Unknown", []string{"conformance", "unknown"}},
		{"Repeated", []string{"conformance", "conformance"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			o, d, _ := fixture(t)
			o.Scope = row.scope
			d.Source = func() (observe.Source, func() error, error) { t.Fatal("source reached"); return nil, nil, nil }
			code, err := Run(o, d, io.Discard)
			assertFailure(t, code, err, 2, "QE6", "X1: user caused pre-check rejection")
		})
	}
}

// safetySource changes source facts after the collector's startup preflight.
type safetySource struct {
	observe.Source
	change func()
	served int
	probes int
}

func (s *safetySource) Controls() (observe.Controls, error) {
	s.change()
	return s.Source.Controls()
}
func (s *safetySource) Observe(key observe.Key, spec observe.Spec) (observe.Run, error) {
	if key["runbook"] != nil {
		s.served++
	} else {
		s.probes++
	}
	return s.Source.Observe(key, spec)
}

type safetyFixtureSource struct{}

func (safetyFixtureSource) Controls() (observe.Controls, error) {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}, nil
}
func (safetyFixtureSource) Observe(observe.Key, observe.Spec) (observe.Run, error) {
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte("value"), StdoutSeen: 5}, nil
}

func TestDestructiveServeRefusal(t *testing.T) {
	// QS6, QA7, QE3: a post-planning destructive invocation is an engine fault before storage.
	for _, name := range []string{"Allowed", "Refused"} {
		t.Run(name, func(t *testing.T) {
			o, d, _ := fixture(t)
			input := strings.Replace(validSubject, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"first","words":["remove"],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"literal":"value"},"precondition":null,"exit_codes":null}]}]}]`, 1)
			input = strings.Replace(input, `"commands":[`, `"commands":[{"words":["remove"],"class":"safe","writes":true,"exit_codes":null,"byte_bound":null,"budget_seconds":10},`, 1)
			if err := os.WriteFile(o.SubjectFile, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			d.Source = func() (observe.Source, func() error, error) {
				return safetyFixtureSource{}, func() error { return nil }, nil
			}
			var wrapped *safetySource
			var collectErr error
			expectedProbes := 0
			d.Collect = func(p plan.Plan, source observe.Source) ([]collect.Check, error) {
				if len(p.Runbooks) != 1 || len(p.Runbooks[0].Steps) != 1 {
					t.Fatal("invalid fixture runbook")
				}
				key := p.Runbooks[0].Steps[0].Key
				if key["runbook"] != "book" || key["step"] != "first" || !reflect.DeepEqual(key["words"], plan.Array{"remove"}) {
					t.Fatalf("wrong runbook key: %#v", key)
				}
				for _, ch := range p.Checks {
					if ch.Preimage["probe"] != nil {
						for _, probeKey := range ch.Keys {
							expectedProbes++
							if reflect.DeepEqual(probeKey["words"], plan.Array{"remove"}) {
								t.Fatal("runbook command selected for probe")
							}
						}
					}
				}
				wrapped = &safetySource{Source: source, change: func() {
					if name == "Refused" {
						commands, ok := p.Checks[0].Subject["commands"].(plan.Array)
						if !ok || len(commands) != 2 {
							t.Fatal("invalid fixture commands")
						}
						command, ok := commands[0].(plan.Object)
						if !ok {
							t.Fatal("invalid fixture command")
						}
						command["class"] = "destructive"
					}
				}}
				checks, err := collect.Collect(p, wrapped)
				collectErr = err
				return checks, err
			}
			writes := 0
			d.Write = func(store.Record) error { writes++; return nil }
			code, err := Run(o, d, io.Discard)
			if name == "Allowed" {
				if err != nil || code != 4 || writes != 1 || wrapped == nil || wrapped.served != 1 {
					t.Fatalf("code=%d err=%v writes=%d source=%+v", code, err, writes, wrapped)
				}
				return
			}
			var fault *plan.Fault
			if !errors.As(collectErr, &fault) || fault.Rule != "QS6" || fault.Cause != "destructive invocation outside allowed runbook scope" {
				t.Fatalf("collection error: %v", collectErr)
			}
			// Every probe completed and only the sole runbook command changed:
			// the refusal must be that step's key, before Observe can receive it.
			var gradeErr *Error
			if code != 70 || !errors.As(err, &gradeErr) || gradeErr.Rule != "QS6" || writes != 0 || wrapped == nil || wrapped.served != 0 || expectedProbes == 0 || wrapped.probes != expectedProbes {
				t.Fatalf("code=%d err=%v writes=%d source=%+v", code, err, writes, wrapped)
			}
		})
	}
}
