package decide

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/observe"
	"github.com/vinnyp/press-quality-lab/internal/plan"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func TestDiscoveryLevel8RequiresWalk(t *testing.T) {
	// QD2: reaching discovery's rule without running its walk is an engine fault.
	ch := plan.Check{Subject: plan.Object{"declared_count": strictjson.Number("0")}}
	if got := discoveryLevel8(ch, &Discovery{Ran: true}); got != (Result{Level: 8, Outcome: "held"}) {
		t.Fatal(got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("missing walk did not panic")
		}
	}()
	discoveryLevel8(plan.Check{Subject: plan.Object{"declared_count": strictjson.Number("0")}}, &Discovery{})
}

func TestLevel7RejectsUnconfined(t *testing.T) {
	// QO1: level 1 must already have decided every unconfined invocation.
	if got := InvocationOutcome(plan.Check{}, passing(), observe.Run{Markers: observe.Markers{Confinement: true}}); got != (Result{}) {
		t.Fatal(got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("unconfined invocation passed level 7")
		}
	}()
	InvocationOutcome(plan.Check{}, passing(), observe.Run{})
}
