package grade

import (
	"errors"
	"io"
	"os"
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
}

func (s *safetySource) Controls() (observe.Controls, error) {
	s.change()
	return s.Source.Controls()
}
func (s *safetySource) Observe(key observe.Key, spec observe.Spec) (observe.Run, error) {
	if key["runbook"] != nil {
		s.served++
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
			input := strings.Replace(validSubject, `"runbooks":[]`, `"runbooks":[{"name":"book","steps":[{"name":"first","words":[],"budget_seconds":1,"checks":[{"dimension":"response-correctness","predicate":"stdout_equals","path":null,"expected":{"literal":"value"},"precondition":null,"exit_codes":null}]}]}]`, 1)
			if err := os.WriteFile(o.SubjectFile, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			d.Source = func() (observe.Source, func() error, error) {
				return safetyFixtureSource{}, func() error { return nil }, nil
			}
			var wrapped *safetySource
			d.Collect = func(p plan.Plan, source observe.Source) ([]collect.Check, error) {
				wrapped = &safetySource{Source: source, change: func() {
					if name == "Refused" {
						commands, ok := p.Checks[0].Subject["commands"].(plan.Array)
						if !ok || len(commands) != 1 {
							t.Fatal("invalid fixture commands")
						}
						command, ok := commands[0].(plan.Object)
						if !ok {
							t.Fatal("invalid fixture command")
						}
						command["class"] = "destructive"
					}
				}}
				return collect.Collect(p, wrapped)
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
			var fault *Error
			if code != 70 || !errors.As(err, &fault) || fault.Rule != "QS6" || writes != 0 || wrapped == nil || wrapped.served != 0 {
				t.Fatalf("code=%d err=%v writes=%d source=%+v", code, err, writes, wrapped)
			}
		})
	}
}
