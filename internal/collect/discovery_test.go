package collect

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

type helpSource struct {
	controls observe.Controls
	serve    func(plan.Array) observe.Run
	keys     []plan.Array
}

func (s *helpSource) Controls() (observe.Controls, error) { return s.controls, nil }
func (s *helpSource) Observe(k observe.Key, spec observe.Spec) (observe.Run, error) {
	w := discoveryArray(k["words"])
	s.keys = append(s.keys, w)
	if k["home"] != "empty" || len(discoveryArray(k["offline_args"])) != 0 || len(discoveryArray(k["env"])) != 0 || spec.BudgetSeconds != 10 {
		panic("discovery settings")
	}
	return s.serve(w[:len(w)-1]), nil
}
func goodControls() observe.Controls {
	return observe.Controls{ConfinementApplied: true, SelfTestPassed: true, SystemNamed: true, PrivateCopyUnchanged: true, KnownFastWithin: true}
}
func help(text string) observe.Run {
	return observe.Run{Markers: observe.Markers{Confinement: true}, Stdout: []byte(text)}
}
func discoveryCheck(commands ...string) plan.Check {
	cs := plan.Array{}
	for _, s := range commands {
		w := plan.Array{}
		for _, word := range strings.Fields(s) {
			w = append(w, word)
		}
		cs = append(cs, plan.Object{"words": w})
	}
	return plan.Check{ID: "discovery", Subject: plan.Object{"commands": cs, "declared_count": strictjson.Number(fmt.Sprint(len(commands)))}, Preimage: plan.Object{"probe": "discovery", "dimension": "discoverability", "expected": nil, "earlier": plan.Array{}}, Keys: []plan.Object{plan.DiscoveryKey(plan.Array{})}}
}
func resolveWalk(t *testing.T, ch plan.Check, serve func(plan.Array) observe.Run) (decide.Result, *decide.Discovery, *helpSource) {
	t.Helper()
	s := &helpSource{controls: goodControls(), serve: serve}
	d, _, err := walk(ch, s.controls, s)
	if err != nil {
		t.Fatal(err)
	}
	return decide.Resolve(ch, s.controls, decide.Evidence{Discovery: d, Invocations: d.Invocations}), d, s
}

func TestDiscoveryUnreadable(t *testing.T) {
	// QD5, QD2, QO7: all non-readable ends remain QD5 while their named children are still walked.
	ends := []struct {
		name  string
		end   observe.End
		empty bool
	}{
		{"Nonzero", observe.End{Code: 2}, false}, {"Empty", observe.End{}, true}, {"Crash", observe.End{Signal: 11}, false}, {"Memory", observe.End{Signal: 9, MemoryKilled: true}, false}, {"Signing", observe.End{Signal: 9, CodeSigningKilled: true}, false}, {"Engine", observe.End{Signal: 9, EngineKilled: true}, false},
	}
	for _, below := range []bool{false, true} {
		for _, row := range ends {
			t.Run(fmt.Sprintf("%sBelow%t", row.name, below), func(t *testing.T) {
				ch := discoveryCheck("a", "a b")
				r, d, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
					run := help("leaf")
					if len(w) == 0 {
						run = help("Commands:\n  a  desc")
					}
					if len(w) == 1 {
						run = help("Commands:\n  b  desc")
					}
					if (below && len(w) == 1) || (!below && len(w) == 0) {
						run.End = row.end
						if row.empty {
							run.Stdout = nil
						}
					}
					return run
				})
				if r != (decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "precondition_unmet"}) || !d.Unreadable {
					t.Fatal(r, d)
				}
				if !row.empty && len(s.keys) != 3 {
					t.Fatal("did not descend", s.keys)
				}
			})
		}
	}
	for _, declared := range []bool{false, true} {
		t.Run(fmt.Sprintf("NoSectionDeclared%t", declared), func(t *testing.T) {
			ch := discoveryCheck()
			if declared {
				ch = discoveryCheck("missing")
			}
			r, _, _ := resolveWalk(t, ch, func(plan.Array) observe.Run { return help("leaf") })
			want := decide.Result{Level: 8, Outcome: "held"}
			if declared {
				want = decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "precondition_unmet"}
			}
			if r != want {
				t.Fatal(r, want)
			}
		})
	}
}

func TestDiscoveryOutcomeOrder(t *testing.T) {
	// QD5, QO2, QO3, QO4, QO7, QO1: invocation rules beat QD5 in either order; E-29 lifts every QO1 to level 1.
	for _, row := range []struct {
		name string
		run  observe.Run
		want decide.Result
	}{
		{"Start", observe.Run{Markers: observe.Markers{Confinement: true, CouldNotStart: true}}, decide.Result{Level: 7, Outcome: decide.NotObserved, Reason: "could_not_start"}},
		{"Confinement", observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9}}, decide.Result{Level: 7, Outcome: decide.NotObserved, Reason: "network_unverifiable"}},
		{"Budget", observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}}, decide.Result{Level: 7, Outcome: "not_held", FailureKind: "over_budget"}},
	} {
		for _, first := range []bool{false, true} {
			t.Run(fmt.Sprintf("%sFirst%t", row.name, first), func(t *testing.T) {
				ch := discoveryCheck("missing")
				r, _, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
					if len(w) == 0 {
						return help("Commands:\n  a  desc\n  b  desc")
					}
					if (w[0] == "a") == first {
						return row.run
					}
					run := help("leaf")
					run.End.Code = 2
					return run
				})
				if r != row.want {
					t.Fatal(r, row.want)
				}
			})
		}
	}
	t.Run("LaterUnconfined", func(t *testing.T) {
		ch := discoveryCheck("missing")
		ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
		r, _, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
			if len(w) == 0 {
				return help("Commands:\n  a  desc\n  b  desc")
			}
			if w[0] == "a" {
				return observe.Run{Markers: observe.Markers{Confinement: true, CouldNotStart: true}}
			}
			return observe.Run{}
		})
		if r.Level != 1 || r.Reason != "network_unverifiable" {
			t.Fatal(r)
		}
	})
}

func TestDiscoveryMissingAndOverride(t *testing.T) {
	// QD3, QD4, QD5, QS5: E-26 and E-30 govern missing names; the final trigger depends only on finding every declaration.
	for _, row := range []struct {
		name                                            string
		missing, placeholder, unreadable, bound, level7 bool
	}{
		{name: "Extra"}, {name: "Missing", missing: true}, {name: "Unreadable", unreadable: true}, {name: "Bound", bound: true}, {name: "Level7", level7: true}, {name: "PlaceholderMissing", missing: true, placeholder: true}, {name: "PlaceholderUnreadableMissing", missing: true, placeholder: true, unreadable: true}, {name: "PlaceholderBoundMissing", missing: true, placeholder: true, bound: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := discoveryCheck("a")
			if row.missing {
				ch = discoveryCheck("a missing")
			}
			if row.placeholder {
				ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
			}
			r, d, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
				if len(w) == 0 {
					return help("Commands:\n  a  desc\n  extra  desc")
				}
				if row.bound {
					return help("Commands:\n  again  desc")
				}
				run := help("leaf")
				if row.unreadable {
					run.End.Code = 2
				}
				if row.level7 {
					run.Markers.CouldNotStart = true
				}
				return run
			})
			generated := ch
			generated.Preimage = plan.Object{"probe": "bare", "words": plan.Array{}, "expected": nil, "earlier": plan.Array{}}
			before := decide.BeforeInvocationsWithEvidence(generated, goodControls(), decide.Evidence{Discovery: d})
			missingDecides := row.missing && !row.unreadable && !row.bound && !row.level7
			if missingDecides {
				if before.Level != 3 || before.Reason != "precondition_unmet" {
					t.Fatal(before)
				}
			} else if before.Level != 0 {
				t.Fatal(before)
			}
			held := decide.Result{Level: 8, Outcome: "held"}
			got := decide.DiscoveryOverride(ch, held, d)
			if !row.missing {
				if got.Reason != "placeholder_unfilled" {
					t.Fatal(got)
				}
			} else if got != held {
				t.Fatal(got)
			}
			if row.placeholder && r.Level != 2 {
				t.Fatal(r)
			}
		})
	}
	t.Run("WalkOrder", func(t *testing.T) {
		ch := discoveryCheck("a", "a b", "c")
		r, _, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
			if len(w) == 0 {
				return help("Commands:\n  a  desc\n  help  desc\n  completion  desc\n  hidden  (hidden)\n  c  desc")
			}
			if reflect.DeepEqual(w, plan.Array{"a"}) {
				return help("Commands:\n  b  desc")
			}
			return help("leaf")
		})
		want := []plan.Array{{"--help"}, {"a", "--help"}, {"a", "b", "--help"}, {"c", "--help"}}
		if r.Outcome != "held" || !reflect.DeepEqual(s.keys, want) {
			t.Fatal(r, s.keys)
		}
	})
}

func TestDiscoveryBounds(t *testing.T) {
	// QD2: stop the whole walk immediately before the next request would exceed either bound.
	for _, row := range []struct {
		name                      string
		depth, children, requests int
		bound                     bool
	}{
		{"DepthAt", 64, 0, 65, false}, {"DepthPast", 65, 0, 65, true}, {"RequestsAt", 0, 4095, 4096, false}, {"RequestsPast", 0, 4096, 4096, true}, {"SelfListing", 100, 0, 65, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := discoveryCheck()
			_, d, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
				if row.children > 0 && len(w) == 0 {
					var b strings.Builder
					b.WriteString("Commands:\n")
					for i := 0; i < row.children; i++ {
						fmt.Fprintf(&b, "  c%d  desc\n", i)
					}
					return help(b.String())
				}
				if len(w) < row.depth {
					return help("Commands:\n  self  desc")
				}
				return help("leaf")
			})
			if d.Bound != row.bound || len(s.keys) != row.requests {
				t.Fatal(d.Bound, len(s.keys))
			}
		})
	}
}

func TestDiscoveryCountAndPriority(t *testing.T) {
	// QD3, QO7: names and count are separate comparisons, and competing level-7 ends use walk order.
	t.Run("Count", func(t *testing.T) {
		ch := discoveryCheck("a")
		ch.Subject["declared_count"] = strictjson.Number("2")
		r, _, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
			if len(w) == 0 {
				return help("Commands:\n  a  desc")
			}
			return help("leaf")
		})
		if r != (decide.Result{Level: 8, Outcome: "not_held", FailureKind: "predicate"}) {
			t.Fatal(r)
		}
	})
	for _, firstStart := range []bool{false, true} {
		t.Run(fmt.Sprintf("FirstStart%t", firstStart), func(t *testing.T) {
			r, _, _ := resolveWalk(t, discoveryCheck("a", "b"), func(w plan.Array) observe.Run {
				if len(w) == 0 {
					return help("Commands:\n  a  desc\n  b  desc")
				}
				if (w[0] == "a") == firstStart {
					return observe.Run{Markers: observe.Markers{Confinement: true, CouldNotStart: true}}
				}
				return observe.Run{Markers: observe.Markers{Confinement: true}, End: observe.End{Signal: 9, EngineKilled: true, BudgetExpired: true, RunningAtBudget: true}}
			})
			if r.Level != 7 || firstStart && r.Reason != "could_not_start" || !firstStart && r.FailureKind != "over_budget" {
				t.Fatal(r)
			}
		})
	}
	t.Run("PlaceholderRootUnconfined", func(t *testing.T) {
		ch := discoveryCheck()
		ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
		r, _, _ := resolveWalk(t, ch, func(plan.Array) observe.Run { return observe.Run{} })
		if r.Level != 1 || r.Reason != "network_unverifiable" {
			t.Fatal(r)
		}
	})
	t.Run("PlaceholderNonzeroListsDescendant", func(t *testing.T) {
		ch := discoveryCheck("a", "missing")
		ch.Subject["declared_count"] = plan.Object{"placeholder": "count"}
		_, d, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
			if len(w) == 0 {
				return help("Commands:\n  a  desc")
			}
			if len(w) == 1 {
				r := help("Commands:\n  child  desc")
				r.End.Code = 2
				return r
			}
			return help("leaf")
		})
		other := ch
		other.Preimage = plan.Object{"probe": "bare", "words": plan.Array{}, "expected": nil, "earlier": plan.Array{}}
		if len(s.keys) != 3 || decide.BeforeInvocationsWithEvidence(other, goodControls(), decide.Evidence{Discovery: d}).Level != 0 {
			t.Fatal(s.keys, d)
		}
	})
}

func TestDuplicateDeclaredNameDoesNotOverride(t *testing.T) {
	// QS5, QD3: the trigger requires an undeclared name, not just an excess count.
	ch := discoveryCheck("a")
	_, d, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
		if len(w) == 0 {
			return help("Commands:\n  a  desc\n\nAdditional Commands:\n  a  desc")
		}
		return help("leaf")
	})
	held := decide.Result{Level: 8, Outcome: "held"}
	if got := decide.DiscoveryOverride(ch, held, d); got != held {
		t.Fatal(got)
	}
}

func TestDiscoveryWalkSuppression(t *testing.T) {
	// QD2, QE8, QO1, QH8, QS5: these conditions leave exactly one unserved bare target.
	for _, name := range []string{"Confinement", "SelfTest", "System", "ChangedCopy", "Binary"} {
		t.Run(name, func(t *testing.T) {
			ch := discoveryCheck()
			c := goodControls()
			switch name {
			case "Confinement":
				c.ConfinementApplied = false
			case "SelfTest":
				c.SelfTestPassed = false
			case "System":
				c.SystemNamed = false
			case "ChangedCopy":
				c.PrivateCopyUnchanged = false
			case "Binary":
				ch.Subject["binary"] = plan.Object{"placeholder": "binary"}
			}
			s := &helpSource{controls: c, serve: func(plan.Array) observe.Run { t.Fatal("suppressed walk served help"); return observe.Run{} }}
			d, keys, err := walk(ch, c, s)
			if err != nil || d.Ran || len(keys) != 1 || len(s.keys) != 0 || !reflect.DeepEqual(keys[0], plan.DiscoveryKey(plan.Array{})) {
				t.Fatal(d, keys, err)
			}
		})
	}
}

func TestDiscoveryOverrideAfterAllChecks(t *testing.T) {
	// QS5, QD3: the final discovery trigger runs only after every runbook resolves.
	ch := discoveryCheck("a")
	authored := ch
	authored.Preimage = plan.Object{"probe": nil, "dimension": "availability", "expected": nil, "earlier": plan.Array{}}
	authored.Keys = nil
	s := &helpSource{controls: goodControls(), serve: func(w plan.Array) observe.Run {
		if len(w) == 0 {
			return help("Commands:\n  a  desc\n  extra  desc")
		}
		return help("leaf")
	}}
	calls := 0
	got, err := collect(plan.Plan{Checks: []plan.Check{ch, authored, authored}}, s, func(plan.Check, observe.Controls, decide.Evidence) decide.Result {
		calls++
		return decide.Result{Level: 8, Outcome: "held"}
	})
	if err != nil || calls != 3 || len(got) != 3 {
		t.Fatal(calls, got, err)
	}
	for _, check := range got {
		if check.Result.Reason != "placeholder_unfilled" {
			t.Fatal(check)
		}
	}
}

func TestDiscoveryNumericCount(t *testing.T) {
	// QD3: compare the numeric count for every spelling the integer schema permits.
	for i, count := range []string{"1", "1.0", "1e0"} {
		t.Run(fmt.Sprintf("Spelling%d", i), func(t *testing.T) {
			ch := discoveryCheck("a")
			ch.Subject["declared_count"] = strictjson.Number(count)
			r, _, _ := resolveWalk(t, ch, func(w plan.Array) observe.Run {
				if len(w) == 0 {
					return help("Commands:\n  a  desc")
				}
				return help("leaf")
			})
			if r.Outcome != "held" {
				t.Fatal(r)
			}
		})
	}
}
