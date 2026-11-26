package decide

import (
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/plan"
)

func TestMissingPreimageKeys(t *testing.T) {
	// QE2: a fixed preimage always contains probe and earlier.
	for _, row := range []struct {
		name, fault string
		run         level
		pre         plan.Object
	}{
		{"Probe", "QE2: preimage lacks probe", level3, plan.Object{"earlier": plan.Array{}}},
		{"Earlier", "QE2: preimage lacks earlier array", level4, plan.Object{"probe": nil}},
	} {
		t.Run(row.name, func(t *testing.T) {
			complete := plan.Check{Preimage: plan.Object{"probe": nil, "earlier": plan.Array{}}}
			if got := row.run(complete, passing(), Evidence{}); got != (Result{}) {
				t.Fatalf("positive: %+v", got)
			}
			defer func() {
				if got := recover(); got != row.fault {
					t.Fatalf("fault %v; want %s", got, row.fault)
				}
			}()
			row.run(plan.Check{Preimage: row.pre}, passing(), Evidence{})
		})
	}
}
