package subject

import (
	"strconv"
	"strings"
	"testing"

	"github.com/vinnyp/press-quality-lab/internal/jcs"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
)

func checkFixture(t *testing.T, o File, want string) {
	t.Helper()
	_, err := Parse(strictjson.Write(o))
	assertRule(t, err, want)
}
func TestDeclarationClauses(t *testing.T) {
	// QS3, QS5, QS6, QS7, QS9, QS10, QS11, QP10, QA4, QA9:
	// Every row starts with its own accepted control and changes one condition.
	rows := []struct {
		name, rule string
		change     func(File)
	}{
		{"RequiredCommands", "QS3", func(o File) { delete(o, "commands") }},
		{"BareDestructive", "QS6", func(o File) { testObject(testArray(o["commands"])[0])["class"] = "destructive" }},
		{"DuplicateCommands", "QS6", func(o File) { o["commands"] = append(testArray(o["commands"]), testArray(o["commands"])[0]) }},
		{"UndeclaredStep", "QS6", func(o File) {
			testObject(testArray(testObject(testArray(o["runbooks"])[0])["steps"])[0])["words"] = strictjson.Array{"undeclared"}
		}},
		{"UndeclaredSecret", "QS6", func(o File) { testObject(o["secret"])["commands"] = strictjson.Array{strictjson.Array{"undeclared"}} }},
		{"OfflineUnknown", "QS7", func(o File) {
			testObject(testArray(o["offline"])[0])["applies_to"] = strictjson.Array{strictjson.Array{"undeclared"}}
		}},
		{"OfflineDuplicateEnv", "QS7", func(o File) {
			s := testObject(testArray(o["offline"])[0])
			s["env"] = strictjson.Object{"SYNTHETIC": "one"}
			o["offline"] = append(testArray(o["offline"]), s)
		}},
		{"StoreDot", "QS9", func(o File) { o["store"] = "./data.json" }},
		{"BookkeepingEmptyComponent", "QS9", func(o File) { o["bookkeeping"] = strictjson.Array{"cache//journal"} }},
		{"SecretParent", "QS9", func(o File) { testObject(o["secret"])["path"] = "../token" }},
		{"DuplicateBooks", "QS10", func(o File) { o["runbooks"] = append(testArray(o["runbooks"]), testArray(o["runbooks"])[0]) }},
		{"DuplicateSteps", "QS10", func(o File) {
			b := testObject(testArray(o["runbooks"])[0])
			b["steps"] = append(testArray(b["steps"]), testArray(b["steps"])[0])
		}},
		{"BookControlCharacter", "QS11", func(o File) { testObject(testArray(o["runbooks"])[0])["name"] = "bad\nname" }},
		{"StepControlCharacter", "QS11", func(o File) {
			testObject(testArray(testObject(testArray(o["runbooks"])[0])["steps"])[0])["name"] = "bad\nname"
		}},
		{"SecretUnsafe", "QP10", func(o File) {
			testObject(testArray(o["commands"])[1])["class"] = "destructive"
			testObject(o["secret"])["commands"] = strictjson.Array{testObject(testArray(o["commands"])[1])["words"]}
		}},
		{"ExecutableOutsideSystem", "QA4", func(o File) {
			firstCheck(o)["expected"] = strictjson.Object{"computation": testComputation("/opt/synthetic")}
		}},
		{"CaptureLater", "QA4", func(o File) {
			c := testComputation("/bin/echo")
			c["captures"] = strictjson.Array{"later"}
			firstCheck(o)["expected"] = strictjson.Object{"computation": c}
		}},
		{"CaptureUnlisted", "QA4", func(o File) {
			c := testComputation("/bin/echo")
			c["argv"] = strictjson.Array{"/bin/echo", "{CAPTURE:missing}"}
			firstCheck(o)["expected"] = strictjson.Object{"computation": c}
		}},
		{"PointerRequired", "QA9", func(o File) { c := firstCheck(o); c["predicate"] = "json_path_equals"; c["path"] = nil }},
		{"PointerEscape", "QA9", func(o File) { c := firstCheck(o); c["predicate"] = "json_path_equals"; c["path"] = "/bad~2" }},
		{"UnexpectedPath", "QA9", func(o File) { c := firstCheck(o); c["predicate"] = "stdout_json_equals"; c["path"] = "/x" }},
		{"StringExpected", "QA9", func(o File) {
			c := firstCheck(o)
			c["predicate"] = "stdout_equals"
			c["expected"] = strictjson.Object{"literal": true}
		}},
		{"WholeExpected", "QA9", func(o File) {
			c := firstCheck(o)
			c["predicate"] = "stdout_bytes_at_most"
			c["expected"] = strictjson.Object{"literal": strictjson.Number("1.0")}
		}},
		{"ExitCodesForbidden", "QA9", func(o File) {
			c := firstCheck(o)
			c["predicate"] = "exit_code"
			c["expected"] = strictjson.Object{"literal": strictjson.Number("0")}
			c["exit_codes"] = strictjson.Array{strictjson.Number("0")}
		}},
		{"EmptyPreconditionRequired", "QA9", func(o File) {
			c := firstCheck(o)
			c["predicate"] = "stdout_empty"
			c["expected"] = strictjson.Object{"literal": nil}
		}},
		{"UnwantedPrecondition", "QA9", func(o File) {
			firstCheck(o)["precondition"] = strictjson.Object{"computation": testComputation("/bin/echo")}
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			o := parsedFixture(t)
			checkFixture(t, o, "")
			row.change(o)
			checkFixture(t, o, row.rule)
		})
	}
	for _, token := range []string{
		"--help",
		"-h",
		"help",
		"--version",
		"-v",
		"version",
	} {
		t.Run("ForbiddenArgument_"+strconv.Itoa(len(token))+"_"+strings.ReplaceAll(token, "-", "x"), func(t *testing.T) {
			o := parsedFixture(t)
			checkFixture(t, o, "")
			testObject(testArray(o["offline"])[0])["args"] = strictjson.Array{token}
			checkFixture(t, o, "QS7")
		})
	}
	for _, name := range []string{"HOME", "TMPDIR", "PATH", "LANG", "LC_ALL", "TZ", "__CF_USER_TEXT_ENCODING"} {
		t.Run("ReservedEnv_"+name, func(t *testing.T) {
			o := parsedFixture(t)
			checkFixture(t, o, "")
			testObject(testArray(o["offline"])[0])["env"] = strictjson.Object{name: "synthetic"}
			checkFixture(t, o, "QS7")
		})
	}
}
func testComputation(executable string) strictjson.Object {
	return strictjson.Object{"argv": strictjson.Array{executable}, "budget_seconds": strictjson.Number("1"), "declared_files": strictjson.Array{}, "captures": strictjson.Array{}}
}

func TestLiteralBounds(t *testing.T) {
	// QS11: canonical byte size, depth and exact binary64 round trips.
	for _, row := range []struct {
		name      string
		good, bad any
	}{
		{"Size", strings.Repeat("a", (64<<10)-2), strings.Repeat("a", (64<<10)-1)},
		{"EscapedSize", strings.Repeat("\x00", 10922), strings.Repeat("\x00", 10923)},
		{"Depth", nestedLiteral(48), nestedLiteral(49)},
		{"Precision", strictjson.Number("9007199254740992"), strictjson.Number("9007199254740993")},
		{"Infinite", strictjson.Number("1e300"), strictjson.Number("1e400")},
	} {
		t.Run(row.name, func(t *testing.T) {
			o := parsedFixture(t)
			firstCheck(o)["expected"] = strictjson.Object{"literal": row.good}
			checkFixture(t, o, "")
			firstCheck(o)["expected"] = strictjson.Object{"literal": row.bad}
			checkFixture(t, o, "QS11")
		})
	}
	for _, row := range []struct{ raw, want string }{{"0.1", "0.1"}, {"0.10", "0.1"}, {"-0", "0"}} {
		t.Run("Number_"+strings.ReplaceAll(strings.ReplaceAll(row.raw, ".", "_"), "-", "Minus"), func(t *testing.T) {
			o := parsedFixture(t)
			firstCheck(o)["expected"] = strictjson.Object{"literal": strictjson.Number(row.raw)}
			s, e := Parse(strictjson.Write(o))
			if e != nil {
				t.Fatal(e)
			}
			b, e2 := jcs.Canonical(firstCheck(s)["expected"])
			if e2 != nil || string(b) != `{"literal":`+row.want+`}` {
				t.Fatalf("%s %v", b, e2)
			}
		})
	}
}
func nestedLiteral(n int) any {
	var v any = strictjson.Number("0")
	for i := 0; i < n; i++ {
		v = strictjson.Array{v}
	}
	return v
}

func TestCheckCount(t *testing.T) {
	// QS11: checks are counted across all runbooks, including unrequested scopes.
	o := parsedFixture(t)
	check := firstCheck(o)
	o["runbooks"] = strictjson.Array{}
	for i := 0; i < 32; i++ {
		checks := strictjson.Array{}
		for j := 0; j < 64; j++ {
			checks = append(checks, check)
		}
		o["runbooks"] = append(arr(o["runbooks"]), strictjson.Object{"name": "book" + strconv.Itoa(i), "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": checks}}})
	}
	checkFixture(t, o, "")
	o["runbooks"] = append(arr(o["runbooks"]), strictjson.Object{"name": "extra", "steps": strictjson.Array{strictjson.Object{"name": "step", "words": strictjson.Array{}, "budget_seconds": strictjson.Number("1"), "checks": strictjson.Array{check}}}})
	checkFixture(t, o, "QS11")
}

func TestBareSecretCommand(t *testing.T) {
	// QS6, QP10: an undeclared bare binary is safe; undeclared words are refused.
	o := parsedFixture(t)
	o["commands"] = strictjson.Array{}
	o["offline"] = strictjson.Array{}
	o["runbooks"] = strictjson.Array{}
	obj(o["secret"])["commands"] = strictjson.Array{strictjson.Array{}}
	checkFixture(t, o, "")
	obj(o["secret"])["commands"] = strictjson.Array{strictjson.Array{"undeclared"}}
	checkFixture(t, o, "QS6")
}

func TestRequiredFields(t *testing.T) {
	// QS3: each required root field is independently present, then absent.
	for _, field := range []string{"subject_file_version", "binary", "facts", "commands", "declared_count", "seed", "store", "offline", "bookkeeping", "secret", "runbooks", "markers", "prompts"} {
		t.Run(field, func(t *testing.T) {
			o := parsedFixture(t)
			checkFixture(t, o, "")
			delete(o, field)
			want := "QS3"
			if field == "subject_file_version" {
				want = "QS1"
			}
			checkFixture(t, o, want)
		})
	}
}
func TestMarkerCount(t *testing.T) {
	// QS11: exactly 1024 markers are permitted, 1025 are refused.
	o := parsedFixture(t)
	markers := strictjson.Array{}
	for i := 0; i < 1024; i++ {
		markers = append(markers, strictjson.Object{"record": "record" + strconv.Itoa(i), "dimension": "response-correctness"})
	}
	o["markers"] = markers
	checkFixture(t, o, "")
	o["markers"] = append(markers, strictjson.Object{"record": "extra", "dimension": "response-correctness"})
	checkFixture(t, o, "QS11")
}
func TestPreconditionCaptures(t *testing.T) {
	// QA9: an emptiness precondition must have no captures, even valid earlier ones.
	o := parsedFixture(t)
	book := obj(arr(o["runbooks"])[0])
	steps := arr(book["steps"])
	second := obj(steps[1])
	c := obj(arr(second["checks"])[0])
	c["predicate"] = "stdout_empty"
	c["path"] = nil
	c["expected"] = strictjson.Object{"literal": nil}
	comp := testComputation("/bin/echo")
	c["precondition"] = strictjson.Object{"computation": comp}
	checkFixture(t, o, "")
	comp["captures"] = strictjson.Array{obj(steps[0])["name"]}
	checkFixture(t, o, "QA9")
}

func TestRepeatedSecretCommand(t *testing.T) {
	// QP10: E-42 rejects equal words at QP10's position in E-9's order.
	s := parsedFixture(t)
	checkFixture(t, s, "")
	secret := testObject(s["secret"])
	commands := testArray(secret["commands"])
	secret["commands"] = append(commands, commands[0])
	checkFixture(t, s, "QP10")
	s["store"] = "../store"
	checkFixture(t, s, "QS9")
}
