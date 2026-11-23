//go:build confinementspike && darwin

package confinement

import (
	"debug/macho"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestCodeSigningExitDetail(t *testing.T) {
	f := newFixture(t)
	// Keep the trusted launcher alive long enough to register the exit observer
	// before an invalid image can be rejected during exec.
	launcherSource, err := os.ReadFile("testdata/programs/launcher.c")
	if err != nil {
		t.Fatal("read trusted launcher")
	}
	delayedSource := filepath.Join(f.root, "delayed-launcher.c")
	delayed := strings.Replace(string(launcherSource), "pid_t pid;", "usleep(100000); pid_t pid;", 1)
	if os.WriteFile(delayedSource, []byte(delayed), 0600) != nil {
		t.Fatal("write delayed trusted launcher")
	}
	f.launcher = filepath.Join(f.root, "delayed-launcher")
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-o", f.launcher, delayedSource}, f.env, f.root); r.err != nil {
		t.Fatal("compile delayed trusted launcher")
	}

	f.probe = filepath.Join(f.root, "signed")
	source, err := filepath.Abs("testdata/programs/signed.c")
	if err != nil {
		t.Fatal("resolve signed program source")
	}
	if r := run("/usr/bin/clang", []string{"-Wall", "-Wextra", "-Werror", "-O0", "-o", f.probe, source}, f.env, f.root); r.err != nil {
		t.Fatal("compile signed clean program")
	}
	if r := run("/usr/bin/codesign", []string{"-s", "-", "-o", "kill", "--force", f.probe}, f.env, f.root); r.err != nil {
		t.Fatal("sign clean program with kill flag")
	}
	runScenario(t, "signed-clean", func(t *testing.T) {
		repeat(t, func() bool { return correct(f.invokeOn(t, "", "clean", false), "clean\n") })
	})
	image, err := macho.Open(f.probe)
	if err != nil {
		t.Fatal("read signed image layout")
	}
	var offset int64 = -1
	for _, symbol := range image.Symtab.Syms {
		if symbol.Name == "_signed_page" && symbol.Sect > 0 {
			section := image.Sections[int(symbol.Sect)-1]
			offset = int64(uint64(section.Offset) + symbol.Value - section.Addr)
		}
	}
	if err := image.Close(); err != nil || offset < 0 {
		t.Fatal("locate signed code page")
	}
	data, err := os.ReadFile(f.probe)
	if err != nil || offset >= int64(len(data)) {
		t.Fatal("read signed clean image")
	}
	data[offset] ^= 1
	if os.WriteFile(f.probe, data, 0700) != nil {
		t.Fatal("corrupt own signed code page")
	}
	// The system kills a corrupted signed page with SIGKILL. On macOS 27.2 it
	// reports no exit detail for it, so the harness's filter for code-signing
	// kills is unwitnessed: such a kill after a violator's readiness marker would
	// look like a refusal kill. This scenario asserts what the system does and
	// fails if that changes, so the limit is revisited.
	var reported atomic.Int64
	runScenario(t, "corrupted-code-page", func(t *testing.T) {
		repeat(t, func() bool {
			r := f.invokeOn(t, "", "clean", false)
			if r.exitDetail&syscall.NOTE_EXIT_CSERROR != 0 {
				reported.Add(1)
			}
			return r.marked && r.diagnostic == "confined\n" && !r.ready && r.exitDetail == 0 && !r.timedOut && !r.harnessKilled && r.status.Signaled() && r.status.Signal() == syscall.SIGKILL && r.output == ""
		})
		t.Logf("code-signing kills reporting NOTE_EXIT_CSERROR=%d", reported.Load())
		t.Log("open limit: a code-signing kill reports no exit detail, so the exit-detail filter is unwitnessed")
	})
}

func TestImageCheckDuringRun(t *testing.T) {
	if os.Getenv("CONFINEMENT_IMAGE_HELPER") == "1" {
		f := newFixture(t)
		t.Run("changed-during-run", func(t *testing.T) {
			prepared, _, err := f.prepareImagesOn(t)
			if err != nil {
				t.Fatal("prepare image set")
			}
			f.firstLine = func(string) error { return os.Chmod(prepared.probe, 0700) }
			f.invokeOn(t, profile(false), "clean", false)
		})
		return
	}
	f := newFixture(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal("locate own test executable")
	}
	runScenario(t, "post-run-image-change", func(t *testing.T) {
		repeat(t, func() bool {
			executed.Add(1)
			r := runBudget(binary, []string{"-test.v", "-test.run=^TestImageCheckDuringRun$"}, append(append([]string{}, f.env...), "CONFINEMENT_IMAGE_HELPER=1"), "", 15*time.Second)
			return !r.timedOut && r.status.Exited() && r.status.ExitStatus() == 1 && strings.Contains(r.output, "private image set changed after run") && strings.Contains(r.output, "--- FAIL: TestImageCheckDuringRun/changed-during-run") && !strings.Contains(r.output+r.diagnostic, "subtest may have called FailNow on a parent test")
		})
	})
}
