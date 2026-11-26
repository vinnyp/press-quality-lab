package confine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInvocationEnvironment(t *testing.T) {
	// QH12: only the fixed table and literal declarations reach the program.
	for _, name := range []string{"PQL_SENTINEL", "DYLD_X", "XDG_CONFIG_HOME"} {
		t.Setenv(name, "foreign")
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := Environment(home, 0xABC, []string{"FIRST=one", "SECOND=two"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"HOME=" + home, "TMPDIR=" + home + "/.pql-tmp/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", "__CF_USER_TEXT_ENCODING=0xABC:0x0:0x0", "FIRST=one", "SECOND=two"}
	if !slices.Equal(got, want) {
		t.Fatalf("environment = %q, want %q", got, want)
	}
	for _, declaration := range []string{"HOME=x", "TMPDIR=x", "PATH=x", "LANG=x", "LC_ALL=x", "TZ=x", "__CF_USER_TEXT_ENCODING=x", "DYLD_X=x", "__CF_OTHER=x"} {
		if _, err := Environment(home, os.Getuid(), []string{declaration}); err == nil {
			t.Fatalf("accepted %s", declaration)
		}
	}
	for _, uid := range []int{0, 1, 0xABC} {
		env, err := Environment(home, uid, nil)
		if err != nil {
			t.Fatal(err)
		}
		if env[6] != fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", uid) {
			t.Fatal(env)
		}
	}
}

func TestLaunchPlan(t *testing.T) {
	// QH13: standard input is null and a new session has no controlling terminal.
	got := planLaunch("synthetic", []string{"ONLY=value"})
	if got.stdin != "/dev/null" || got.attributes.Dir != "synthetic" || !slices.Equal(got.attributes.Env, []string{"ONLY=value"}) || !got.newSession {
		t.Fatalf("launch plan = %+v attributes=%+v", got, got.attributes)
	}
}

func TestEnvironmentCanonicalHome(t *testing.T) {
	// QH11, QH12: resolve all links before constructing invocation paths.
	home := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	env, err := Environment(link, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if env[0] != "HOME="+real || env[1] != "TMPDIR="+real+"/.pql-tmp/" {
		t.Fatal(env)
	}
}

func TestLegacyEnvironmentCanonicalHome(t *testing.T) {
	// QH11, QH12: rebuild older callers' full tables with canonical home paths.
	home := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	supplied := []string{"HOME=" + link, "TMPDIR=" + link + "/.pql-tmp/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", "__CF_USER_TEXT_ENCODING=0xABC:0x0:0x0", "DECLARED=value"}
	got, err := invocationEnvironment(link, 0xABC, supplied)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"HOME=" + real, "TMPDIR=" + real + "/.pql-tmp/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", "__CF_USER_TEXT_ENCODING=0xABC:0x0:0x0", "DECLARED=value"}
	if !slices.Equal(got, want) {
		t.Fatalf("environment=%q want=%q", got, want)
	}
	supplied[0] = "HOME=/wrong-home"
	if _, err := invocationEnvironment(link, 0xABC, supplied); err == nil {
		t.Fatal("accepted a fixed variable override")
	}
}
