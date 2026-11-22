package judge

import (
	"fmt"
	"testing"
)

// X10 requires exactly one comparison in compare scopes and none in every
// other scope. Distinct ids and valid routing isolate the quantity clauses.
func TestX10ComparisonQuantityByScope(t *testing.T) {
	for _, mask := range []int{1, 3, 4, 12, 5, 7, 13, 15} {
		for _, count := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("scope=%d_comparisons=%d", mask, count), func(t *testing.T) {
				f := allScopeFixture(mask)
				f.in["required"] = without(arr(f.in["required"]), "id", "compare")
				for i := 0; i < count; i++ {
					id := "compare"
					if i != 0 {
						id = "second-comparison"
					}
					f.in["required"] = append(arr(f.in["required"]), required(id, "comparison", "tool-description-efficacy"))
				}
				if mask&8 != 0 && count == 1 || mask&8 == 0 && count == 0 {
					assertFixture(t, f)
				} else {
					assertBytes(t, write(f.in), rejectedExpected(X10))
				}
			})
		}
	}
}
