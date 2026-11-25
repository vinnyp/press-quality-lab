//go:build darwin && macos_evidence

package hostilehome_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/vinnyp/press-quality-lab/internal/confine"
	"github.com/vinnyp/press-quality-lab/internal/spike/hostilehome"
)

// Register after TempDir's cleanup, so hardened entries are undone first.
func tempTree(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var log bytes.Buffer
		if err := hostilehome.CleanupParent(&log, filepath.Dir(path), filepath.Base(path)); err != nil {
			t.Errorf("cleanup tree: %v", err)
		}
		t.Log(log.String())
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("temporary tree remains: %v", err)
		}
	})
	return path
}

func build(t *testing.T, target string) string {
	t.Helper()
	binary := filepath.Join(tempTree(t), "program")
	cmd := exec.Command("go", "build", "-o", binary, target)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	return binary
}

func TestHostileHomeConfined(t *testing.T)   { evidence(t, true) }
func TestHostileHomeUnconfined(t *testing.T) { evidence(t, false) }

func evidence(t *testing.T, confined bool) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("evidence requires an ordinary user")
	}
	program := build(t, "./testdata/hostile")
	var copies *confine.Copies
	if confined {
		launcher := build(t, "../../../cmd/pql")
		var err error
		copies, err = confine.NewCopies(tempTree(t), launcher, program)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Independent literal inventory: dropping a production probe is observable.
	for _, item := range []string{"file-uchg", "dir-uchg", "file-uappnd", "dir-uappnd", "file-acl", "dir-acl", "file-xattr", "dir-xattr", "file-mode", "dir-mode", "fifo", "socket", "file-combined", "dir-combined", "symlink"} {
		for _, strategy := range []string{"descriptor", "parent"} {
			t.Run(item+"/"+strategy, func(t *testing.T) {
				base := tempTree(t)
				home := filepath.Join(base, "home")
				err := os.Mkdir(home, 0700)
				if err != nil {
					t.Fatal(err)
				}
				var outside *os.File
				var before string
				if item == "symlink" {
					path := filepath.Join(base, "outside")
					outside, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := outside.Close(); err != nil {
							t.Error(err)
						}
					})
					if err := hostilehome.SetDenyACL(path); err != nil {
						t.Fatal(err)
					}
					if err := unix.Chflags(path, unix.UF_IMMUTABLE); err != nil {
						t.Fatal(err)
					}
					before, err = hostilehome.Snapshot(int(outside.Fd()))
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("outside before %s", before)
				}
				env := []string{"HOME=" + home, "TMPDIR=" + home, fmt.Sprintf("__CF_USER_TEXT_ENCODING=0x%X:0:0", os.Getuid())}
				var out []byte
				killed := false
				if confined {
					values := map[string]string{"ROOT": "/", "RUNTIME": "/System/Volumes/Preboot/Cryptexes", "VOLUME": "/System/Volumes/Data", "HOME": home, "TRACE_DEVICE": "/dev/dtracehelper", "NULL_DEVICE": "/dev/null", "RANDOM_DEVICE": "/dev/urandom", "ZERO_DEVICE": "/dev/zero", "STDOUT_DEVICE": "/dev/fd/1", "STDERR_DEVICE": "/dev/fd/2", "ZONEFILE": "/usr/share/zoneinfo/UTC", "ZONE_RULES": "/usr/share/zoneinfo/posixrules", "SERVICES": "/etc/services", "PROTOCOLS": "/etc/protocols", "PLANTED": filepath.Join(home, "planted")}
					p, e := copies.Start(confine.Invocation{Paths: confine.Paths{Values: values}, Words: []string{item}, Env: env, Directory: home})
					if e != nil {
						t.Fatal(e)
					}
					timer := time.AfterFunc(10*time.Second, func() {
						if e := p.Kill(); e != nil {
							t.Errorf("timeout kill: %v", e)
						}
					})
					run, e := p.Wait()
					timer.Stop()
					t.Logf("run %s marker=%+v end=%+v error=%v stderr=%q", item, run.Markers, run.End, e, run.Stderr)
					out = run.Stdout
					killed = run.End.Signal != 0
					if e != nil {
						t.Error(e)
					}
					if !run.Markers.Confinement {
						t.Error("confinement not applied; no behavioural evidence")
					}
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, program, item)
					cmd.Dir = home
					cmd.Env = env
					out, err = cmd.CombinedOutput()
					t.Logf("unconfined end=%v", err)
					if err != nil {
						t.Error(err)
					}
				}
				t.Logf("%s", out)
				// A killed syscall has no return value: retain the end and partial output,
				// never invent an errno or pretend that missing output was a refusal.
				if !bytes.Contains(out, []byte("attempt "+item+" ")) {
					t.Error("missing attempt line")
				}
				if !killed {
					steps := map[string][]string{
						"file-uchg": {"chflags-uchg"}, "dir-uchg": {"chflags-uchg"},
						"file-uappnd": {"chflags-uappnd"}, "dir-uappnd": {"chflags-uappnd"},
						"file-acl": {"setattrlist-deny"}, "dir-acl": {"setattrlist-deny"},
						"file-xattr": {"setxattr"}, "dir-xattr": {"setxattr"},
						"file-mode": {"chmod-0000"}, "dir-mode": {"chmod-0000"},
						"symlink": {"create", "replace-remove", "symlink"},
						"fifo":    {"mkfifo"}, "socket": {"socket", "bind", "close"},
						"file-combined": {"setattrlist-deny", "chmod-0000", "chflags-uchg"},
						"dir-combined":  {"setattrlist-deny", "chmod-0000", "chflags-uchg"},
					}
					for _, step := range steps[item] {
						if !bytes.Contains(out, []byte("attempt "+item+" "+step+"=")) {
							t.Errorf("missing syscall result: %s", step)
						}
					}
				}
				if item == "symlink" {
					info, err := os.Lstat(filepath.Join(home, item))
					linked := err == nil && info.Mode()&os.ModeSymlink != 0
					t.Logf("link-control lstat=%v symlink=%t", err, linked)
					if !confined && !linked {
						t.Fatal("unconfined link control did not create a symlink")
					}
				}
				var cleanup bytes.Buffer
				prefix := "cleanup "
				if strategy == "descriptor" {
					if err := hostilehome.Cleanup(&cleanup, home, item); err != nil {
						t.Error(err)
					}
				} else {
					prefix = "cleanup-parent "
					// Refusals are probe results, not infrastructure failures.
					err = hostilehome.CleanupParent(&cleanup, home, item)
					t.Logf("parent strategy result=%v", err)
				}
				t.Log(cleanup.String())
				if !strings.Contains(cleanup.String(), prefix+item+" ") {
					t.Error("missing cleanup line")
				}
				if strategy == "parent" {
					for _, step := range []string{"setattrlistat-clear-flags=", "setattrlistat-clear-acl=", "fchmodat-nofollow=", "fstatat-nofollow=", "unlinkat=", "gone="} {
						if !strings.Contains(cleanup.String(), step) {
							t.Errorf("missing parent result: %s", step)
						}
					}
				}
				if outside != nil {
					after, err := hostilehome.Snapshot(int(outside.Fd()))
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("outside after %s unchanged=%t", after, before == after)
					if before != after {
						t.Error("outside ACL, flags or mode changed")
					}
				}
			})
		}
	}
}

func TestHostileHomeSocketOptions(t *testing.T) {
	var b bytes.Buffer
	if err := hostilehome.SocketOptions(&b); err != nil {
		t.Fatal(err)
	}
	t.Log(b.String())
	for _, kind := range []string{"listen", "dial"} {
		for _, name := range []string{"SO_REUSEADDR", "SO_REUSEPORT", "SO_KEEPALIVE", "SO_NOSIGPIPE", "SO_BROADCAST", "SO_LINGER", "TCP_NODELAY", "TCP_KEEPALIVE", "TCP_KEEPINTVL", "TCP_KEEPCNT"} {
			if !strings.Contains(b.String(), "socket "+kind+" "+name+" ") {
				t.Errorf("missing %s %s", kind, name)
			}
		}
	}
}
