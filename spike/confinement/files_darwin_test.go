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
	"time"
)

func writeDecoy(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("decoy"), 0600); err != nil {
		t.Fatal("write decoy")
	}
}

func TestFileBoundaries(t *testing.T) {
	t.Parallel()
	for _, computation := range []bool{false, true} {
		for _, pathForm := range []string{"plain", "parent", "data-alias"} {
			for _, mode := range []string{"read", "ignore-home", "map", "write", "truncate", "create", "unlink", "rename", "chmod", "xattr", "link", "clone", "symlink"} {
				for _, form := range []string{"confined", "twin", "positive", "attribution", "without-kill"} {
					if form == "attribution" && (mode == "link" || mode == "clone") {
						continue
					}
					name := mode
					if mode == "link" || mode == "clone" {
						name += "-smoke"
					}
					runScenario(t, profileName(computation)+"/"+pathForm+"/"+name+"/"+form, func(t *testing.T) {
						t.Parallel()
						f := newFixture(t)
						if mode == "link" || mode == "clone" {
							t.Log("smoke test: this boundary attempt does not isolate link or clone; the operation has a separate behavioral witness")
						}
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
							if form == "attribution" {
								probe.params = append(probe.params, "TARGET="+bad, "DESTINATION="+destination)
								operations := map[string]string{
									"read": "file-read-data", "ignore-home": "file-read-data", "map": "file-read-data",
									"write": "file-write-data", "truncate": "file-write-data", "create": "file-write-create file-write-data",
									"unlink": "file-write-unlink", "rename": "file-write-unlink file-write-create file-write-data",
									"chmod": "file-write-mode", "xattr": "file-write-xattr", "link": "file-link", "clone": "file-clone file-read-data file-write-create file-write-data", "symlink": "file-read-data",
								}
								p += "\n(allow " + operations[mode] + " (literal (param \"TARGET\")) (literal (param \"DESTINATION\")))\n"
							}
							if form == "positive" {
								p = ""
							}
							if form == "without-kill" {
								p = strings.ReplaceAll(p, "(with send-signal SIGKILL)", "")
							}
							before := decoySnapshot(bad)
							r := probe.invokeOn(t, p, "file-"+mode, form == "twin")
							if form == "positive" || form == "attribution" {
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
							return correct(r, "fallback\n") && unchanged
						})
					})
				}
			}
		}
	}
}

// Access time alone is insufficient; both the ordinary read and control kill
// are required. A volume which does not record reads yields no measurement.
func readMeasured(calibrated, moved, killed bool) bool { return calibrated && moved && killed }

func accessTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	raw := info.Sys().(*syscall.Stat_t)
	return time.Unix(raw.Atimespec.Sec, raw.Atimespec.Nsec), nil
}

func TestContentReadProof(t *testing.T) {
	t.Parallel()
	t.Log("read-proof calibration: access time is planted one day before modification time; equal timestamps do not prove a plain read")
	f := newFixture(t)
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("create planted canary")
	}
	canary := hex.EncodeToString(random[:])
	old := time.Unix(946684800, 0)
	calibrated := true
	runScenario(t, "calibration", func(t *testing.T) {
		notMeasured := 0
		repeat(t, func() bool {
			path := filepath.Join(f.home(), "calibration")
			if os.WriteFile(path, []byte(canary), 0600) != nil || os.Chtimes(path, old, old.Add(24*time.Hour)) != nil {
				t.Fatal("prepare calibration")
			}
			probe := f
			probe.args = []string{path, path, filepath.Join(f.home(), "unused"), canary}
			r := probe.invokeOn(t, "", "file-secret-read", false)
			at, err := accessTime(path)
			if err != nil {
				t.Fatal("inspect calibration")
			}
			observed := !at.Equal(old)
			calibrated = calibrated && observed
			if !observed {
				notMeasured++
			}
			return correct(r, "performed\nfallback\n")
		})
		t.Logf("not-measured=%d", notMeasured)
	})
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"read", "pread", "map", "copy", "open", "stat", "times", "setattr", "fsetattr", "replace", "rename-times", "recreate", "recreate-without-denial", "uncalibrated-read"} {
			runScenario(t, profileName(computation)+"/"+mode, func(t *testing.T) {
				repeat(t, func() bool {
					home, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatal("resolve proof home")
					}
					secret := filepath.Join(home, "planted")
					if os.WriteFile(secret, []byte(canary+strings.Repeat(" ", 8192)), 0600) != nil {
						t.Fatal("plant canary")
					}
					probe := f
					probe.params = []string{"HOME=" + home, "PLANTED=" + secret}
					probe.env = []string{"HOME=" + home, "TMPDIR=" + home, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", coreFoundationEncoding}
					probe.args = []string{secret, secret, filepath.Join(home, "copied"), canary}
					scenarioCalibrated := calibrated && mode != "uncalibrated-read"
					operationMode := strings.TrimSuffix(strings.TrimPrefix(mode, "uncalibrated-"), "-without-denial")
					ordinary := profile(computation)
					if mode == "recreate-without-denial" {
						ordinary = strings.Replace(ordinary, `(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))`, "", 1)
					}
					control := ordinary + `
(deny file-read-data (literal (param "PLANTED")) (with send-signal SIGKILL))
`
					if os.Chtimes(secret, old, old.Add(24*time.Hour)) != nil {
						t.Fatal("set initial access time")
					}
					before, err := os.Stat(secret)
					if err != nil {
						t.Fatal("stat planted file")
					}
					r := probe.invokeOn(t, ordinary, "file-secret-"+operationMode, false)
					at, err := accessTime(secret)
					if err != nil {
						t.Fatal("inspect access time")
					}
					after, err := os.Stat(secret)
					if err != nil {
						t.Fatal("stat planted file after run")
					}
					stable := os.SameFile(before, after)
					moved := !at.Equal(old)
					t.Logf("ordinary-exit=%d access-time-moved=%v", r.status, moved)
					c := probe.invokeOn(t, control, "file-secret-"+operationMode, false)
					if mode == "recreate-without-denial" {
						return correct(r, "performed\nfallback\n") && moved && c.killed() && !stable && !(stable && readMeasured(scenarioCalibrated, moved, c.killed()))
					}
					if mode == "times" || mode == "setattr" || mode == "fsetattr" || mode == "replace" || mode == "rename-times" || mode == "recreate" {
						return r.killed() && c.killed() && !moved
					}
					if !correct(r, "performed\nfallback\n") {
						return false
					}
					if mode == "stat" {
						return correct(c, "performed\nfallback\n") && !moved
					}
					if mode == "open" {
						return c.killed() && !moved && !readMeasured(scenarioCalibrated, moved, c.killed())
					}
					if !scenarioCalibrated {
						if mode != "uncalibrated-read" {
							t.Error("read proof not measured: volume calibration did not record reads")
							return false
						}
						t.Log("not-measured: deliberately uncalibrated control")
						return stable && !readMeasured(scenarioCalibrated, moved, c.killed())
					}
					return stable && readMeasured(scenarioCalibrated, moved, c.killed())
				})
			})
		}
	}
	t.Log("open limit: another process reading the planted file can move its access time")
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
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			repeat(t, func() bool {
				if !correct(f.invokeOn(t, profile(computation), "file-home", false), "home written\n") {
					return false
				}
				data, err := os.ReadFile(filepath.Join(f.root, "home", "state"))
				return err == nil && string(data) == "decoy"
			})
		})
	}
}

func TestHomeLinkInspectionSmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
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
				if !probe.invokeOn(t, profile(computation), "file-symlink", false).killed() {
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

func TestHomeRootProtection(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"root-replace", "root-rename"} {
			for _, form := range []string{"confined", "twin", "positive", "remove-denial", "remove-kill"} {
				runScenario(t, profileName(computation)+"/"+mode+"/"+form, func(t *testing.T) {
					repeat(t, func() bool {
						root, err := filepath.EvalSymlinks(t.TempDir())
						if err != nil {
							t.Fatal("resolve root decoy")
						}
						home := filepath.Join(root, "home")
						if os.Mkdir(home, 0700) != nil {
							t.Fatal("create root decoy")
						}
						good := filepath.Join(home, "child")
						if form == "twin" && os.Mkdir(good, 0700) != nil {
							t.Fatal("create twin directory")
						}
						probe := f
						probe.params = []string{"HOME=" + home}
						probe.env = []string{"HOME=" + home, "TMPDIR=" + home, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", coreFoundationEncoding}
						destination := filepath.Join(home, "destination")
						if form != "twin" {
							destination = filepath.Join(root, "destination")
						}
						probe.args = []string{destination, good}
						p := profile(computation)
						line := `(deny file-write-unlink file-write-create (literal (param "HOME")) (with send-signal SIGKILL))`
						if mode == "root-rename" {
							// Isolate removal of the source root from the destination boundary.
							probe.params = append(probe.params, "ROOT_DEST="+destination)
							p += `
(allow file-write* (literal (param "ROOT_DEST")))
`
						}
						if form == "positive" {
							p = ""
						}
						if form == "remove-denial" {
							p = strings.Replace(p, line, "", 1)
						}
						if form == "remove-kill" {
							p = strings.Replace(p, line, strings.Replace(line, " (with send-signal SIGKILL)", "", 1), 1)
						}
						r := probe.invokeOn(t, p, mode, form == "twin")
						info, err := os.Lstat(home)
						unchanged := err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
						if form == "confined" {
							return r.killed() && unchanged
						}
						if form == "remove-denial" {
							return correct(r, "performed\nfallback\n")
						}
						if form == "remove-kill" {
							return correct(r, "fallback\n") && unchanged
						}
						return correct(r, "performed\nfallback\n")
					})
				})
			}
		}
	}
}

func TestPlantedTimestampForgery(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	line := `(deny file-write* (literal (param "PLANTED")) (with send-signal SIGKILL))`
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"times", "setattr", "fsetattr", "replace", "rename-times", "recreate"} {
			for _, form := range []string{"confined", "twin", "positive", "remove-denial", "remove-kill"} {
				runScenario(t, profileName(computation)+"/"+mode+"/"+form, func(t *testing.T) {
					repeat(t, func() bool {
						home, err := filepath.EvalSymlinks(t.TempDir())
						if err != nil {
							t.Fatal("resolve forgery root")
						}
						secret, other := filepath.Join(home, "planted"), filepath.Join(home, "other")
						var random [16]byte
						if _, err := rand.Read(random[:]); err != nil {
							t.Fatal("make canary")
						}
						canary := hex.EncodeToString(random[:])
						old := time.Unix(946684800, 0)
						for _, path := range []string{secret, other} {
							if os.WriteFile(path, []byte(canary), 0600) != nil || os.Chtimes(path, old, old.Add(24*time.Hour)) != nil {
								t.Fatal("prepare forgery decoys")
							}
						}
						probe := f
						probe.params = []string{"HOME=" + home, "PLANTED=" + secret}
						probe.args = []string{secret, other, filepath.Join(home, "fresh"), canary}
						p := profile(computation)
						switch form {
						case "positive":
							p = ""
						case "remove-denial":
							p = strings.Replace(p, line, "", 1)
						case "remove-kill":
							p = strings.Replace(p, line, strings.Replace(line, " (with send-signal SIGKILL)", "", 1), 1)
						}
						before := decoySnapshot(secret)
						beforeAccess, err := accessTime(secret)
						if err != nil {
							t.Fatal("inspect planted access time")
						}
						r := probe.invokeOn(t, p, "file-secret-"+mode, form == "twin")
						afterAccess, err := accessTime(secret)
						if err != nil {
							t.Fatal("inspect planted access time after run")
						}
						otherAccess, err := accessTime(other)
						if err != nil {
							t.Fatal("inspect twin access time")
						}
						unchanged := before == decoySnapshot(secret)
						if form == "confined" {
							return r.killed() && unchanged
						}
						if form == "remove-kill" {
							return correct(r, "fallback\n") && unchanged
						}
						if form == "twin" {
							return correct(r, "performed\nfallback\n") && unchanged && !otherAccess.Equal(old)
						}
						return correct(r, "performed\nfallback\n") && !afterAccess.Equal(beforeAccess)
					})
				})
			}
		}
	}
}

func TestReadMeasurementInputs(t *testing.T) {
	for _, row := range []struct {
		name                            string
		calibrated, moved, killed, want bool
	}{
		{"read", true, true, true, true},
		{"volume-without-read-times", false, true, true, false},
		{"unchanged-access-time", true, false, true, false},
		{"control-survived", true, true, false, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			if readMeasured(row.calibrated, row.moved, row.killed) != row.want {
				t.Error("read measurement differs")
			}
		})
	}
}

func TestEndOfFileReadObservation(t *testing.T) {
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		runScenario(t, profileName(computation), func(t *testing.T) {
			movedCount := 0
			for range repetitions {
				home, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal("resolve observation root")
				}
				path := filepath.Join(home, "planted")
				var random [16]byte
				if _, err := rand.Read(random[:]); err != nil {
					t.Fatal("create observation canary")
				}
				canary := hex.EncodeToString(random[:])
				old := time.Unix(946684800, 0)
				if os.WriteFile(path, []byte(canary), 0600) != nil || os.Chtimes(path, old, old.Add(24*time.Hour)) != nil {
					t.Fatal("prepare observation")
				}
				probe := f
				probe.params = []string{"HOME=" + home, "PLANTED=" + path}
				probe.args = []string{path, path, filepath.Join(home, "unused"), canary}
				r := probe.invokeOn(t, profile(computation), "file-secret-eof", false)
				at, err := accessTime(path)
				if err != nil {
					t.Fatal("inspect observation")
				}
				if !at.Equal(old) {
					movedCount++
				}
				t.Logf("open limit: a zero-byte read at end of file moved access time=%v; normal successful exit=%v", !at.Equal(old), correct(r, "performed\nfallback\n"))
			}
			t.Logf("executed=%d observations=%d access-time-moved=%d; no read credit or pass claimed", repetitions, repetitions, movedCount)
		})
	}
}
