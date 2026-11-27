package subject

import (
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestEmptyExpectedStrings(t *testing.T) {
	// QA9, E-50: empty strings reject in QA9's E-9 slot, for all four predicates.
	for _, p := range []string{"stdout_equals", "stdout_contains", "stdout_not_contains", "stderr_contains"} {
		t.Run(p, func(t *testing.T) {
			o := parsedFixture(t)
			c := firstCheck(o)
			c["predicate"] = p
			c["expected"] = strictjson.Object{"literal": "x"}
			c["precondition"] = nil
			c["path"] = nil
			checkFixture(t, o, "")
			c["expected"] = strictjson.Object{"literal": ""}
			checkFixture(t, o, "QA9")
			o["commands"] = append(arr(o["commands"]), arr(o["commands"])[0])
			checkFixture(t, o, "QS6")
		})
	}
}
func TestComputationArgumentCollisions(t *testing.T) {
	// QA4, E-52: identical argv with different computation settings reject in E-9 order.
	for _, name := range []string{"Budget", "DeclaredFiles", "Captures", "Precondition"} {
		t.Run(name, func(t *testing.T) {
			o := parsedFixture(t)
			book := obj(arr(o["runbooks"])[0])
			step := obj(arr(book["steps"])[0])
			c := firstCheck(o)
			c["predicate"] = "stdout_json_equals"
			c["expected"] = strictjson.Object{"computation": testComputation("/bin/echo")}
			c["path"] = nil
			c["precondition"] = nil
			// A second step makes captures legal before testing only the collision rule.
			step["name"] = "earlier"
			other := strictjson.Object{"name": "current", "words": step["words"], "budget_seconds": step["budget_seconds"], "checks": strictjson.Array{c}}
			step["checks"] = strictjson.Array{strictjson.Object{"dimension": "availability", "predicate": "exit_code", "path": nil, "expected": strictjson.Object{"literal": strictjson.Number("0")}, "precondition": nil, "exit_codes": nil}}
			book["steps"] = strictjson.Array{step, other}
			duplicate := strictjson.Object{}
			for k, v := range c {
				duplicate[k] = v
			}
			second := testComputation("/bin/echo")
			duplicate["expected"] = strictjson.Object{"computation": second}
			other["checks"] = strictjson.Array{c, duplicate}
			if name == "Precondition" {
				c["predicate"] = "stdout_empty"
				c["expected"] = strictjson.Object{"literal": nil}
				c["precondition"] = strictjson.Object{"computation": testComputation("/bin/echo")}
			}
			checkFixture(t, o, "")
			switch name {
			case "Budget", "Precondition":
				second["budget_seconds"] = strictjson.Number("11")
			case "DeclaredFiles":
				second["declared_files"] = strictjson.Array{"fixture.json"}
			case "Captures":
				second["captures"] = strictjson.Array{"earlier"}
			}
			checkFixture(t, o, "QA4")
			// QA4 precedes a simultaneous QA9 failure.
			duplicate["path"] = "/unexpected"
			checkFixture(t, o, "QA4")
		})
	}
}
