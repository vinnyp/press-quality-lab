package confine

import (
	"encoding/json"
	"fmt"
	"github.com/vinnyp/press-quality-lab/internal/observe"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRawBudgetTimer(t *testing.T) {
	// QO4, QH21: report the timer, its running sample, and the engine's kill.
	p := shellProcess(t, "sleep 60")
	p.armBudget(10 * time.Millisecond)
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !run.End.BudgetExpired || !run.End.RunningAtBudget || !run.End.EngineKilled || run.End.Signal != int(syscall.SIGKILL) {
		t.Fatalf("budget facts=%+v", run.End)
	}
}

func TestRawBudgetAlreadyEnded(t *testing.T) {
	// QO4: an already ended leader is not running when the timer samples it.
	p := shellProcess(t, "exit 0")
	waitZombie(t, p.pid)
	p.expireBudget()
	run, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if run.End.BudgetExpired || run.End.RunningAtBudget || run.End.EngineKilled || run.End.Code != 0 {
		t.Fatalf("late budget facts=%+v", run.End)
	}
}

func TestRawProtocolCaptureBoundary(t *testing.T) {
	// QE10, QH7: remove protocol before counting and applying the keep bound.
	for _, n := range []int{1048576, 1048577} {
		t.Run(fmt.Sprintf("Bytes_%d", n), func(t *testing.T) {
			p := shellProcess(t, fmt.Sprintf("printf 'pql-launch:test\\n' >&2; head -c %d /dev/zero >&2", n))
			run, err := p.Wait()
			if err != nil {
				t.Fatal(err)
			}
			if !run.Markers.Confinement || run.End.Code != 0 || run.StderrSeen != int64(n) || len(run.Stderr) != 1048576 || run.StderrCut != (n > 1048576) || strings.Count(string(run.Stderr), "\x00") != 1048576 {
				t.Fatalf("n=%d end=%+v seen=%d kept=%d cut=%t", n, run.End, run.StderrSeen, len(run.Stderr), run.StderrCut)
			}
		})
	}
}

func TestStartBuildsEnvironment(t *testing.T) {
	// QH12: the real Start path builds the table before encoding launcher arguments.
	for _, name := range []string{"PQL_SENTINEL", "DYLD_X", "XDG_CONFIG_HOME"} {
		t.Setenv(name, "foreign")
	}
	c, err := NewCopies(t.TempDir(), "/usr/bin/true", "/usr/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, name := range commonNames {
		values[name] = home
	}
	called := false
	_, err = c.startWith(Invocation{Workspace: fakeWorkspace{}, Paths: Paths{Values: values}, Env: []string{"DECLARED=value"}}, func(_ string, args []string, _ string, env []string, _ string) (Process, error) {
		called = true
		if len(env) != 0 {
			t.Fatalf("launcher environment=%q", env)
		}
		var got []string
		if err := json.Unmarshal([]byte(args[len(args)-1]), &got); err != nil {
			t.Fatal(err)
		}
		want := []string{"HOME=" + home, "TMPDIR=" + home + "/.pql-tmp/", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0x0:0x0", os.Getuid()), "DECLARED=value"}
		if !slices.Equal(got, want) {
			t.Fatalf("subject environment=%q want=%q", got, want)
		}
		return &process{changed: true, pid: os.Getpid()}, nil
	})
	if err != nil || !called {
		t.Fatalf("start reached=%t err=%v", called, err)
	}
}

func TestRawConfinementKillBeforeBudget(t *testing.T) {
	// QH21, QO4, QT5: a late timer cannot reclassify the completed kill.
	p := shellProcess(t, "printf 'pql-launch:test\\n' >&2; kill -9 $$")
	waitZombie(t, p.pid)
	p.expireBudget()
	r, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	a, err := observe.Attribute(r)
	if err != nil || a != observe.ConfinementKill {
		t.Fatalf("attribution=%v err=%v run=%+v", a, err, r)
	}
	if _, err := observe.ProjectEnd(r.End, r.Markers); err != nil {
		t.Fatal(err)
	}
}
