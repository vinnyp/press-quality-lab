package plan

import (
	"github.com/vinnyp/press-quality-lab/internal/rulebook"
	"github.com/vinnyp/press-quality-lab/internal/strictjson"
	"testing"
)

func TestSeedDigest(t *testing.T) {
	// QE2: the worked seed digest is independent of entry order.
	for _, row := range []struct {
		name  string
		entry Entry
	}{
		{"Bytes", Entry{Path: "data.json", Data: []byte("{\"widgets\":[]}\n")}},
		{"Digest", Entry{Path: "data.json", Digest: "3890b1a054e609ad21a23184fc351862f3aed55501fdf682bc1d2cec95dacff2"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := SeedDigest([]Entry{row.entry})
			if err != nil || got != "d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee" {
				t.Fatalf("%s %v", got, err)
			}
		})
	}
}
func TestSubjectKey(t *testing.T) {
	// QS7: command matching appends settings.
	// QT5: env names sort.
	s := Object{"commands": Array{Object{"words": Array{"list"}}}, "offline": Array{Object{"applies_to": Array{Array{"list"}}, "args": Array{"--offline"}, "env": Object{"Z": "secret", "A": "1"}}}}
	got := SubjectKey(s, Array{"list", "--json"}, "runbook", "book", "step")
	want := Object{"words": Array{"list", "--json"}, "offline_args": Array{"--offline"}, "env": Array{"A", "Z"}, "home": "runbook", "runbook": "book", "step": "step", "computation": nil}
	if !same(got, want) {
		t.Fatalf("got %#v", got)
	}
	got = SubjectKey(s, Array{"list", "--help"}, "empty", nil, nil)
	want = Object{"words": Array{"list", "--help"}, "offline_args": Array{}, "env": Array{}, "home": "empty", "runbook": nil, "step": nil, "computation": nil}
	if !same(got, want) {
		t.Fatalf("discovery %#v", got)
	}
}
func TestEmptyAndCredit(t *testing.T) {
	// QA3: exact credit conditions, with literal zero.
	s := Object{"commands": Array{}}
	ch := Object{"dimension": "graceful-failure", "predicate": "exit_code", "expected": Object{"literal": strictjson.Number("0")}}
	if credits(s, Object{}, Object{}, ch) {
		t.Fatal("zero exit credits graceful failure")
	}
}

func TestWorkedIDs(t *testing.T) {
	// QE2: four literal preimages and published digests.
	for _, row := range []struct{ pre, id string }{
		{`{"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"bounding","earlier":[],"env":{},"expected":null,"index":null,"offline_args":["--offline"],"parameters":{"bound":65536},"path":null,"predicate":null,"probe":"byte_bound","runbook":null,"scheme":"pql-check-id-1","seed":"d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee","step":null,"store":"data.json","words":["list"]}`, "46c4884c4195f646969ad14aca889bb614643f7c5e7e88bdec87d1032c0344d7"},
		{`{"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"output-conformance","earlier":[],"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"expected":{"literal":{"widgets":[{"name":"gear"}]},"precondition":null},"index":0,"offline_args":["--offline"],"parameters":{"exit_codes":null},"path":null,"predicate":"stdout_json_equals","probe":null,"runbook":"widgets","scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":"list","store":"data.json","words":["list","--json"]}`, "c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7"},
		{`{"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"response-correctness","earlier":["c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7"],"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"expected":{"literal":null,"precondition":{"computation":{"argv":["computations/has-widgets.sh","{SEED}/data.json"],"budget_seconds":5,"captures":[],"files":{"computations/has-widgets.sh":"a1b660c0d3bd81e132520531ee7d97c5467d5c9f3575fda91abd60daf573dbc8"}}}},"index":0,"offline_args":["--offline"],"parameters":{"exit_codes":null},"path":null,"predicate":"stdout_empty","probe":null,"runbook":"widgets","scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":"filter","store":"data.json","words":["list","--tag","none"]}`, "40b08dbef70438992c53f14fee9f5e1297277bc94bb35cb5e2b435758df22993"},
		{`{"bookkeeping":[".local/state/widgets"],"budget_seconds":null,"dimension":"pii-secret-hygiene","earlier":[],"env":{},"expected":null,"index":null,"offline_args":[],"parameters":{"commands":[{"budget_seconds":30,"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"offline_args":["--offline"],"words":["account"]}],"kind":"credential","path":".config/widgets/token","text_sha256":"d2ea444a8905d8d4d040c5770edcd4a576b858fe4b357ff9e277241dea36261b"},"path":null,"predicate":null,"probe":"planted_secret","runbook":null,"scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":null,"store":"data.json","words":null}`, "181431d07ae5e61a4a5fde89d6a606708476067a3dcdfad263b473f33adb98ba"},
	} {
		v, ok := strictjson.Read([]byte(row.pre), strictjson.Bounds{Bytes: 16384, Depth: 64, NumberChars: 32, ExponentDigits: 4}, nil)
		if !ok {
			t.Fatal("literal preimage")
		}
		got, err := CheckID(obj(v))
		if err != nil || got != row.id {
			t.Fatalf("got %s %v want %s", got, err, row.id)
		}
	}
}

func TestPlanningGaps(t *testing.T) {
	// QE4: baselines and anchors stop before returning any partial plan.
	for _, row := range []struct {
		name       string
		in         Input
		rule, part string
	}{
		{"Baseline", Input{Baseline: true}, "QE4", ""},
		{"Anchor", Input{Anchor: true}, "QE4", "anchor"},
	} {
		t.Run(row.name, func(t *testing.T) {
			p, err := Build(row.in)
			gap, ok := err.(*Gap)
			if !ok || gap.Rule != row.rule || gap.Part != row.part || len(p.Checks) != 0 {
				t.Fatalf("plan %+v error %v", p, err)
			}
		})
	}
}

func TestScope(t *testing.T) {
	// QE3: each requested scope is filtered through the judge's scope definition.
	for _, row := range []struct {
		name      string
		scope     rulebook.Scope
		dimension string
		want      bool
	}{
		{"Conformance", rulebook.ConformanceScope, "availability", true},
		{"DestructiveExcluded", rulebook.ConformanceScope, "destructive-guard", false},
		{"DestructiveIncluded", rulebook.DestructiveScope, "destructive-guard", true},
		{"ConformanceExcluded", rulebook.DestructiveScope, "availability", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			in := Input{Scopes: []rulebook.Scope{row.scope}}
			if got := in.includes(row.dimension); got != row.want {
				t.Fatalf("got %v want %v", got, row.want)
			}
		})
	}
}

func TestCanonicalizationFault(t *testing.T) {
	// QE2: a subject integer outside JCS's exact range cannot produce a check id.
	check := Object{"dimension": "output-conformance", "predicate": "stdout_json_equals", "expected": Object{"literal": strictjson.Number("9007199254740993")}, "path": nil, "precondition": nil, "exit_codes": nil}
	step := Object{"name": "read", "words": Array{"list"}, "budget_seconds": strictjson.Number("10"), "checks": Array{check}}
	in := Input{Subject: Object{"declared_count": strictjson.Number("0"), "runbooks": Array{Object{"name": "book", "steps": Array{step}}}}, Scopes: []rulebook.Scope{rulebook.ConformanceScope}}
	p, err := Build(in)
	_, gap := err.(*Gap)
	if err == nil || gap || len(p.Checks) != 0 {
		t.Fatalf("plan %+v error %v; want canonicalization fault", p, err)
	}
}
