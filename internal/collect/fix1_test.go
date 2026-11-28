package collect

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestDiscoveryWideMemory(t *testing.T) {
	// QD2, QE10: 1 MiB per help must not accumulate retained streams or unbounded pending paths.
	var builder strings.Builder
	builder.WriteString("Commands:\n")
	for i := 0; builder.Len()+32 < 1<<20; i++ {
		fmt.Fprintf(&builder, "  c%d  description\n", i)
	}
	text := builder.String() + strings.Repeat(" ", (1<<20)-builder.Len())
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
	runtime.GC()
	var before, peak runtime.MemStats
	runtime.ReadMemStats(&before)
	_, d, s := resolveWalk(t, discoveryCheck(), func(plan.Array) observe.Run {
		runtime.GC()
		runtime.ReadMemStats(&peak)
		if peak.HeapInuse > before.HeapInuse+128<<20 {
			t.Fatal("walk exceeded 128 MiB live heap ceiling")
		}
		run := help(text)
		run.Stderr = []byte(text)
		run.StderrSeen = int64(len(text))
		run.Details.Stdout.Digest = digest
		run.Details.Stderr.Digest = digest
		return run
	})
	if !d.Bound || len(s.keys) != 65 {
		t.Fatal(d.Bound, len(s.keys))
	}
	for _, inv := range d.Invocations {
		if !inv.Run.BytesDropped || len(inv.Run.Stdout)+len(inv.Run.Stderr) != 0 {
			t.Fatal("help bytes retained")
		}
		if inv.Run.StdoutSeen != 1<<20 || inv.Run.StderrSeen != 1<<20 || inv.Run.Details.Stdout.Digest != digest || inv.Run.Details.Stderr.Digest != digest {
			t.Fatal("lost stream count", inv.Run.StdoutSeen)
		}
	}
}

func TestDiscoveryCutStreams(t *testing.T) {
	// QD2, QD5, E-32: choose by bytes written, refuse a cut chosen stream.
	for _, name := range []string{"CutStdoutWithStderr", "CutStdoutNoDeclarations", "CutStderr"} {
		t.Run(name, func(t *testing.T) {
			ch := discoveryCheck()
			if name != "CutStdoutNoDeclarations" {
				ch = discoveryCheck("a")
			}
			r, _, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
				if len(w) > 0 {
					return help("leaf")
				}
				run := help("Commands:\n  a  desc")
				run.Stderr = []byte("Commands:\n  wrong  desc")
				run.StderrSeen = int64(len(run.Stderr))
				if name == "CutStderr" {
					run.StderrCut = true
				} else {
					run.Stdout = nil
					if name == "CutStdoutWithStderr" {
						run.Stdout = []byte("Commands:\n  unsafe  desc")
					}
					run.StdoutSeen = 1 << 21
					run.StdoutCut = true
				}
				return run
			})
			want := decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "precondition_unmet"}
			requests := 1
			if name == "CutStderr" {
				want = decide.Result{Level: 8, Outcome: "held"}
				requests = 2
			}
			if r != want || len(s.keys) != requests {
				t.Fatal(r, s.keys)
			}
		})
	}
}

func TestDiscoveryDuplicateListing(t *testing.T) {
	// QD2, QD3, E-33: duplicate names count and request once, in first-listing order.
	r, _, s := resolveWalk(t, discoveryCheck("a", "b"), func(w plan.Array) observe.Run {
		if len(w) == 0 {
			return help("Commands:\n  a  desc\n  b  desc\n  a  again")
		}
		return help("leaf")
	})
	if r != (decide.Result{Level: 8, Outcome: "held"}) {
		t.Fatal(r)
	}
	if !reflect.DeepEqual(s.keys, []plan.Array{{"--help"}, {"a", "--help"}, {"b", "--help"}}) {
		t.Fatal(s.keys)
	}
}

func TestDiscoveryInvalidArguments(t *testing.T) {
	// QD2, QD5, E-34: invalid names are not requested; valid siblings still are.
	for _, row := range []struct{ name, word string }{
		{"NUL", "bad\x00name"}, {"UTF8", "bad\xffname"}, {"ArgumentLimit", strings.Repeat("a", 2<<20)},
	} {
		t.Run(row.name, func(t *testing.T) {
			r, d, s := resolveWalk(t, discoveryCheck("good"), func(w plan.Array) observe.Run {
				if len(w) == 0 {
					return help("Commands:\n  " + row.word + "  desc\n  good  desc")
				}
				return help("leaf")
			})
			if d.FoundCount != 2 || !d.Undeclared || !d.DeclaredFound[0] || r != (decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "precondition_unmet"}) || !reflect.DeepEqual(s.keys, []plan.Array{{"--help"}, {"good", "--help"}}) {
				t.Fatal(r, len(s.keys))
			}
		})
	}
}

func TestCollectPlanAndDiscoveryFacts(t *testing.T) {
	// QE8, QO1, QH8, E-29: discovery facts reach precedence through Collect, without mutating the plan.
	for _, name := range []string{"LaterUnconfined", "PlaceholderRootUnconfined", "CopyChanged", "PlanUnchanged"} {
		t.Run(name, func(t *testing.T) {
			ch := discoveryCheck("a", "b")
			if name == "LaterUnconfined" || name == "PlaceholderRootUnconfined" {
				ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
			}
			p := plan.Plan{Checks: []plan.Check{ch}}
			old := append([]plan.Object{}, ch.Keys...)
			s := &helpSource{controls: goodControls(), serve: func(w plan.Array) observe.Run {
				if name == "PlaceholderRootUnconfined" {
					return observe.Run{}
				}
				if len(w) == 0 {
					return help("Commands:\n  a  desc\n  b  desc")
				}
				if name == "LaterUnconfined" {
					if w[0] == "a" {
						return observe.Run{Markers: observe.Markers{Confinement: true, CouldNotStart: true}}
					}
					return observe.Run{}
				}
				run := help("leaf")
				if name == "CopyChanged" {
					return observe.Run{CopyChanged: true}
				}
				return run
			}}
			got, err := Collect(p, s)
			if err != nil || len(got) != 1 {
				t.Fatal(got, err)
			}
			want := decide.Result{Level: 8, Outcome: "held"}
			if name == "LaterUnconfined" || name == "PlaceholderRootUnconfined" {
				want = decide.Result{Level: 1, Outcome: decide.NotObserved, Reason: "network_unverifiable"}
			}
			if name == "CopyChanged" {
				want = decide.Result{Level: 5, Outcome: decide.NotObserved, Reason: "harness_fault"}
			}
			if got[0].Result != want {
				t.Fatal(got[0].Result, want)
			}
			if !reflect.DeepEqual(p.Checks[0].Keys, old) {
				t.Fatal("caller plan changed")
			}
		})
	}
}

func TestDiscoveryBareDeclarationLeaf(t *testing.T) {
	// QD5: declaring only the bare command does not make a headingless help unreadable.
	ch := discoveryCheck("")
	ch.Subject["declared_count"] = strictjson.Number("0")
	r, _, _ := resolveWalk(t, ch, func(plan.Array) observe.Run { return help("leaf") })
	if r != (decide.Result{Level: 8, Outcome: "held"}) {
		t.Fatal(r)
	}
}

func TestDiscoveryWrittenOutput(t *testing.T) {
	// QD5, E-32: output is present when written bytes were not retained.
	r, _, _ := resolveWalk(t, discoveryCheck(), func(plan.Array) observe.Run {
		return observe.Run{Markers: observe.Markers{Confinement: true}, StdoutSeen: 1}
	})
	if r != (decide.Result{Level: 8, Outcome: "held"}) {
		t.Fatal(r)
	}
}

type ownInvocationSource struct {
	*helpSource
	own int
}

func (s *ownInvocationSource) Observe(key observe.Key, spec observe.Spec) (observe.Run, error) {
	if spec.BudgetSeconds != 0 {
		return s.helpSource.Observe(key, spec)
	}
	s.own++
	return observe.Run{Markers: observe.Markers{Confinement: true}}, nil
}

func TestCollectUnconfinedHelpLeavesOtherInvocation(t *testing.T) {
	// QO1, QD4, E-29: an unconfined help blocks QD4, while another generated check serves its own invocation.
	ch := discoveryCheck("missing")
	ch.Subject["runbooks"] = plan.Array{}
	ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
	other := ch
	other.ID = "bare"
	other.Preimage = plan.Object{"probe": "bare", "dimension": "availability", "words": plan.Array{}, "expected": nil, "earlier": plan.Array{}}
	other.Keys = []plan.Object{{"home": "seed", "words": plan.Array{}, "computation": nil}}
	source := &ownInvocationSource{helpSource: &helpSource{controls: goodControls(), serve: func(plan.Array) observe.Run { return observe.Run{} }}}
	got, err := Collect(plan.Plan{Checks: []plan.Check{ch, other}}, source)
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if got[0].Result != (decide.Result{Level: 1, Outcome: decide.NotObserved, Reason: "network_unverifiable"}) || source.own != 1 || got[1].Result.Level != 8 || got[1].Result.Reason != "placeholder_unfilled" {
		t.Fatal(got, source.own)
	}
}
