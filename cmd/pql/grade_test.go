package main

import (
	"bytes"
	"fmt"
	"testing"
)

func checkCLI(t *testing.T, args []string, want int, line string) {
	t.Helper()
	var out, diagnostic bytes.Buffer
	code := run(args, bytes.NewReader(nil), &out, &diagnostic)
	if code != want || out.Len() != 0 || (line != "" && diagnostic.String() != line) {
		t.Fatalf("args=%q code=%d stdout=%q stderr=%q", args, code, out.String(), diagnostic.String())
	}
}

func TestGradeMalformed(t *testing.T) {
	// GKD8: syntax errors precede QT1 content refusals; QE16 reserves 64.
	for _, command := range []string{"grade", "anchor"} {
		for i, tail := range [][]string{
			{"--host", "host", "--unknown"},
			{"--behaviour", "recording", "--verifier", "unknown"},
			{"--verifier", "scripted"},
			{"subject", "extra", "--host", "host"},
			{"subject", "--host"},
			{"subject", "--verifier", "scripted", "--unknown"},
		} {
			t.Run(fmt.Sprintf("%s_Row%d", command, i), func(t *testing.T) {
				checkCLI(t, append([]string{command}, tail...), 64, "")
			})
		}
	}
}

func TestGradeCommandSurface(t *testing.T) {
	// GKD8 and errata E-7: scope tokens are passed through, never validated here.
	// QE16: the unbuilt pipeline ends as an engine fault.
	for _, args := range [][]string{
		{"grade", "subject", "--scope", "unknown,,destructive", "--baseline", "base"},
		{"anchor", "subject", "--retired", "old", "--accept", "new"},
	} {
		checkCLI(t, args, 70, "pql: rejected: QE16: grade pipeline not built (U25)\n")
	}
}

func TestGradeParsedRequest(t *testing.T) {
	// GKD8 and E-7: preserve each option and every comma-separated scope token.
	grade, anchor := runGrade, runAnchor
	defer func() { runGrade, runAnchor = grade, anchor }()
	var got commandOptions
	calls := 0
	capture := func(options commandOptions) error { calls++; got = options; return nil }
	runGrade, runAnchor = capture, capture
	checkCLI(t, []string{"grade", "subject", "--scope", "unknown,,destructive", "--baseline", "base"}, 0, "")
	if calls != 1 || got.SubjectFile != "subject" || got.Baseline != "base" ||
		len(got.Scope) != 3 || got.Scope[0] != "unknown" || got.Scope[1] != "" || got.Scope[2] != "destructive" {
		t.Fatalf("grade calls=%d request=%+v", calls, got)
	}
	checkCLI(t, []string{"anchor", "subject", "--retired", "old", "--accept", "new"}, 0, "")
	if calls != 2 || got.SubjectFile != "subject" || got.Retired != "old" || got.Accept != "new" {
		t.Fatalf("anchor calls=%d request=%+v", calls, got)
	}
}
