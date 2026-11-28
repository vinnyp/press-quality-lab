package collect

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/decide"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestPR14DiscoveryBounds(t *testing.T) {
	// QD2: trimmed requests survive exhaustion, while a depth stop has only the depth flag.
	for _, row := range []struct {
		name          string
		width, depth  int
		request, deep bool
		requests      int
	}{
		{"AtBudget", 4095, 0, false, false, 4096},
		{"TrimmedExhausted", 4096, 0, true, false, 4096},
		{"DepthOnly", 0, 65, false, true, 65},
		{"DepthAfterTrim", 4200, 65, false, true, 65},
	} {
		t.Run(row.name, func(t *testing.T) {
			ch := discoveryCheck("c0")
			ch.Subject["declared_count"] = strictjson.Number(fmt.Sprint(row.width + row.depth))
			result, d, s := resolveWalk(t, ch, func(w plan.Array) observe.Run {
				if len(w) == 0 && row.width > 0 {
					var b strings.Builder
					b.WriteString("Commands:\n")
					for i := 0; i < row.width; i++ {
						fmt.Fprintf(&b, "  c%d  desc\n", i)
					}
					return help(b.String())
				}
				if len(w) < row.depth {
					return help("Commands:\n  c0  desc")
				}
				return help("leaf")
			})
			if d.RequestBound != row.request || d.DepthBound != row.deep || len(s.keys) != row.requests {
				t.Fatalf("request=%t depth=%t requests=%d", d.RequestBound, d.DepthBound, len(s.keys))
			}
			if row.request || row.deep {
				if result != (decide.Result{Level: 8, Outcome: decide.NotObserved, Reason: "precondition_unmet"}) {
					t.Fatal(result)
				}
			} else if result != (decide.Result{Level: 8, Outcome: "held"}) {
				t.Fatal(result)
			}
		})
	}
}

func TestPR14UndeclaredPreview(t *testing.T) {
	// QD1, QD3: the undeclared list is independent of the found preview and preserves exact counts.
	for _, count := range []int{512, 513, 4096, 4097} {
		t.Run(fmt.Sprintf("Names%d", count), func(t *testing.T) {
			_, d, _ := resolveWalk(t, discoveryCheck(), func(w plan.Array) observe.Run {
				if len(w) > 0 {
					return help("leaf")
				}
				var b strings.Builder
				b.WriteString("Commands:\n")
				for i := 0; i < count; i++ {
					fmt.Fprintf(&b, "  c%d  desc\n", i)
				}
				return help(b.String())
			})
			if d.UndeclaredCount != count || len(d.UndeclaredNames) != min(count, 4096) || d.UndeclaredCut != (count > 4096) || len(d.Names) != min(count, 512) || d.FoundCut != (count > 512) {
				t.Fatalf("found=%d/%t undeclared=%d/%d/%t", len(d.Names), d.FoundCut, len(d.UndeclaredNames), d.UndeclaredCount, d.UndeclaredCut)
			}
			if d.UndeclaredNames[len(d.UndeclaredNames)-1][0] != fmt.Sprintf("c%d", min(count, 4096)-1) {
				t.Fatal("order")
			}
		})
	}
}

func TestPR14UndeclaredByteBound(t *testing.T) {
	// QD3: the byte cap has a held twin at exactly 1 MiB, independently of the entry cap.
	for _, past := range []bool{false, true} {
		_, d, _ := resolveWalk(t, discoveryCheck(), func(w plan.Array) observe.Run {
			if len(w) > 0 {
				return help("leaf")
			}
			var b strings.Builder
			b.WriteString("Commands:\n")
			for i := 0; i < 15; i++ {
				fmt.Fprintf(&b, "  %02d%s\n", i, strings.Repeat("a", 65526))
			}
			length := 65528
			if past {
				length++
			}
			fmt.Fprintf(&b, "  %s\n", strings.Repeat("z", length))
			return help(b.String())
		})
		want := 16
		if past {
			want = 15
		}
		if d.UndeclaredCount != 16 || len(d.UndeclaredNames) != want || d.UndeclaredCut != past {
			t.Fatal(d.UndeclaredCount, len(d.UndeclaredNames), d.UndeclaredCut)
		}
	}
}

func TestPR14DiscoveryCarried(t *testing.T) {
	// QD1, QO1: only discovery carries the walk, including a walk that did not run.
	for _, ran := range []bool{true, false} {
		ch := discoveryCheck()
		other := ch
		other.ID = "bare"
		other.Preimage = plan.Object{"probe": "bare", "dimension": "availability", "earlier": plan.Array{}, "expected": nil, "words": plan.Array{}}
		other.Keys = nil
		controls := goodControls()
		controls.ConfinementApplied = ran
		source := &helpSource{controls: controls, serve: func(plan.Array) observe.Run { return help("leaf") }}
		checks, err := Collect(plan.Plan{Checks: []plan.Check{ch, other}}, source, Options{RunDirectory: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		if checks[0].Discovery == nil || checks[0].Discovery.Ran != ran || checks[1].Discovery != nil {
			t.Fatal("discovery facts were lost or attached to another check")
		}
	}
}
