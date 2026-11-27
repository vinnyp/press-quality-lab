package importguard

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGateNecessityUnitTests(t *testing.T) {
	// KTD7: CI executes the test-only renderer's pure oracles, rather than only compiling evidence.
	b, err := os.ReadFile("../../gate.sh")
	if err != nil {
		t.Fatal(err)
	}
	source := string(b)
	start := strings.Index(source, "for tags in ")
	if start < 0 {
		t.Fatal("test variants loop missing")
	}
	end := strings.Index(source[start:], "\ndone")
	if end < 0 {
		t.Fatal("test variants loop not terminated")
	}
	// Evaluate only the job declarations with a recording step; never run the gate.
	script := "set -eu\nstep(){ printf '%s\\n' \"$*\"; }\n" + source[start:start+end+len("\ndone")]
	output, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("declarations: %v %s", err, output)
	}
	for _, tags := range []string{"necessity_test", "scripted,necessity_test"} {
		want := "test-" + tags + " env GOFLAGS=-mod=readonly -tags=" + tags + " go test -count=1 ./..."
		found := false
		for _, line := range strings.Split(string(output), "\n") {
			if line == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing unit test job %q; declarations:\n%s", tags, output)
		}
	}
}

func TestGateDarwinAMD64Build(t *testing.T) {
	// QH9: both supported native witness veneers must receive a build check.
	b, err := os.ReadFile("../../gate.sh")
	if err != nil {
		t.Fatal(err)
	}
	var declarations []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "step ") && strings.Contains(line, "GOARCH=amd64") {
			declarations = append(declarations, line)
		}
	}
	script := "set -eu\nstep(){ printf '%s\\n' \"$*\"; }\n" + strings.Join(declarations, "\n")
	output, err := exec.Command("bash", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("declarations: %v %s", err, output)
	}
	found := false
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		darwin, amd64, build := false, false, false
		for _, field := range fields {
			darwin = darwin || field == "GOOS=darwin"
			amd64 = amd64 || field == "GOARCH=amd64"
		}
		if len(fields) >= 3 {
			n := len(fields)
			build = fields[n-3] == "go" && fields[n-2] == "build" && fields[n-1] == "./..."
		}
		found = found || darwin && amd64 && build
	}
	if !found {
		t.Fatalf("missing Darwin/amd64 build: %s", output)
	}
}
