//go:build confinementspike && darwin

package confinement

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func writeDecoy(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("decoy"), 0600); err != nil {
		t.Fatal("write decoy")
	}
}

func TestFileBoundaries(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, pathForm := range []string{"plain", "parent", "data-alias"} {
			for _, mode := range []string{"read", "ignore-home", "map", "write", "truncate", "create", "unlink", "rename", "chmod", "xattr", "link", "clone", "symlink"} {
				for _, form := range []string{"confined", "twin", "positive", "without-kill"} {
					runScenario(t, profileName(computation)+"/"+pathForm+"/"+mode+"/"+form, func(t *testing.T) {
						repeat(t, func() bool {
							root, err := filepath.EvalSymlinks(t.TempDir())
							if err != nil {
								t.Fatal("resolve decoy root")
							}
							home := filepath.Join(root, "home")
							outside := filepath.Join(root, "outside")
							if os.Mkdir(home, 0700) != nil || os.Mkdir(outside, 0700) != nil {
								t.Fatal("create decoy directories")
							}
							bad, good := filepath.Join(outside, "target"), filepath.Join(home, "target")
							if mode != "create" {
								writeDecoy(t, bad)
								writeDecoy(t, good)
							}
							destination := filepath.Join(home, "destination")

							probe := f
							target := bad
							switch pathForm {
							case "parent":
								target = home + "/../outside/target"
							case "data-alias":
								target = "/System/Volumes/Data" + bad
							}
							probe.args = []string{target, good, destination}
							p := profile(computation)
							probe.params = []string{"HOME=" + home, "DECLARED=" + good}
							if form == "positive" {
								p = ""
							}
							if form == "without-kill" {
								p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
							}
							before := decoySnapshot(bad)
							r := probe.invoke(p, "file-"+mode, form == "twin")
							if form == "positive" {
								return correct(r, "performed\nfallback\n")
							}
							unchanged := before == decoySnapshot(bad)
							if form == "confined" {
								return r.killed() && unchanged
							}
							if form == "twin" {
								want := "performed\nfallback\n"
								if mode == "link" || mode == "clone" {
									want = "fallback\n"
								}
								return correct(r, want) && unchanged
							}
							return r.marked && !r.timedOut && !r.killed() && unchanged
						})
					})
				}
			}
		}
	}
}

func TestContentReadProof(t *testing.T) {
	f := newFixture(t)
	secret := filepath.Join(f.root, "planted")
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("create planted canary")
	}
	canary := hex.EncodeToString(random[:])
	if os.WriteFile(secret, []byte(canary), 0600) != nil {
		t.Fatal("write planted canary")
	}
	f.args = []string{secret, secret, filepath.Join(f.root, "unused"), canary}
	f.params = []string{"PLANTED=" + secret}
	for _, computation := range []bool{false, true} {
		ordinary := profile(computation) + "\n(allow file-read-data (literal (param \"PLANTED\")))\n"
		control := ordinary + "\n(deny file-read-data (literal (param \"PLANTED\")) (with send-signal SIGKILL))\n"
		for _, mode := range []string{"read", "map", "stat"} {
			for _, proof := range []bool{false, true} {
				name := "ordinary"
				p := ordinary
				if proof {
					name, p = "control", control
				}
				runScenario(t, profileName(computation)+"/"+mode+"/"+name, func(t *testing.T) {
					repeat(t, func() bool {
						r := f.invoke(p, "file-secret-"+mode, false)
						if proof && mode != "stat" {
							return r.killed()
						}
						return correct(r, "performed\nfallback\n")
					})
				})
			}
		}
	}
}

func decoySnapshot(path string) string {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "absent"
	}
	if err != nil {
		return "unreadable"
	}
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return "unexpected-type"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "unreadable"
	}
	return fmt.Sprintf("%s/%d/%d/%d/%d/%v/%v", data, raw.Ino, raw.Nlink, raw.Mode, raw.Size, raw.Mtimespec, raw.Ctimespec)
}

func TestHomeWrite(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				if !correct(f.invoke(profile(computation), "file-home", false), "home written\n") {
					return false
				}
				data, err := os.ReadFile(filepath.Join(f.root, "home", "state"))
				return err == nil && string(data) == "decoy"
			})
		})
	}
}

func TestHomeLinkInspection(t *testing.T) {
	f := newFixture(t)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("canonical outside directory")
	}
	target := filepath.Join(outside, "target")
	writeDecoy(t, target)
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				home, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal("canonical used home")
				}
				good := filepath.Join(home, "source")
				writeDecoy(t, good)
				probe := f
				probe.params = []string{"HOME=" + home, "DECLARED=" + good}
				probe.args = []string{target, good, filepath.Join(home, "link")}
				before := decoySnapshot(target)
				if !probe.invoke(profile(computation), "file-symlink", false).killed() {
					return false
				}
				links := 0
				err = filepath.WalkDir(home, func(_ string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					// WalkDir does not follow links; no entry is opened for content here.
					if entry.Type()&os.ModeSymlink != 0 {
						links++
					}
					return nil
				})
				return err == nil && links == 1 && before == decoySnapshot(target)
			})
		})
	}
}
