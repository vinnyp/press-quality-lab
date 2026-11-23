//go:build confinementspike && darwin

package confinement

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableDirectoryBoundarySmoke(t *testing.T) {
	t.Log("smoke test: these kills do not isolate the operation rule")
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, mode := range []string{"image-list", "image-parent", "image-write", "image-chmod", "image-unlink"} {
			for _, form := range []string{"confined", "twin", "positive"} {
				runScenario(t, profileName(computation)+"/"+mode+"/"+form, func(t *testing.T) {
					repeat(t, func() bool {
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						probe := f
						probe.freshImages = true
						var release func()
						var err error
						probe, release, err = probe.prepareImagesOn(t)
						if err != nil {
							t.Fatal("prepare private image")
						}
						defer release()
						before := decoySnapshot(probe.probe)
						writeDecoy(t, filepath.Join(f.home(), "state"))
						r := probe.invokeOn(t, p, mode, form == "twin")
						if form == "confined" && mode != "image-list" {
							return r.killed() && before == decoySnapshot(probe.probe)
						}
						return correct(r, "performed\nfallback\n")
					})
				})
			}
		}
	}
}

func TestWorkingDirectoryListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"confined", "twin", "positive", "attribution"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					probe := f
					probe.args = []string{f.root}
					p := profile(computation) + "\n(allow fs-info)\n"
					if form == "attribution" {
						probe.params = append(probe.params, "DIRECTORY="+f.root)
						p += "(allow file-read-data (literal (param \"DIRECTORY\")))\n"
					}
					if form == "positive" {
						p = ""
					}
					r := probe.invokeOn(t, p, "directory-list", form == "twin")
					if form == "confined" {
						return r.killed()
					}
					return correct(r, "performed\nfallback\n")
				})
			})
		}
	}
}

// Model the shell's shared-temporary-directory probe on a decoy only. The
// permissions needed by access(W_OK) also permit removing that directory.
func TestWritableDirectoryPermissionsSmoke(t *testing.T) {
	t.Log("smoke test: the writable-directory grant changes several permissions and does not isolate one rule")
	t.Parallel()
	f := newFixture(t)
	for _, computation := range []bool{false, true} {
		for _, form := range []string{"strict-query", "allowed-query", "allowed-removal", "positive-removal"} {
			runScenario(t, profileName(computation)+"/"+form, func(t *testing.T) {
				repeat(t, func() bool {
					root, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatal("resolve writable-directory decoy")
					}
					target := filepath.Join(root, "directory")
					if os.Mkdir(target, 0700) != nil {
						t.Fatal("create writable-directory decoy")
					}
					probe := f
					probe.args = []string{target}
					probe.params = []string{"DECOY_DIRECTORY=" + target}
					p := profile(computation)
					if form != "strict-query" {
						p += `
(allow file-read-data file-write-data file-write-create file-write-unlink (literal (param "DECOY_DIRECTORY")))
`
					}
					mode := "directory-writable"
					if form == "allowed-removal" || form == "positive-removal" {
						mode = "directory-remove"
					}
					if form == "positive-removal" {
						p = ""
					}
					r := probe.invokeOn(t, p, mode, false)
					_, statErr := os.Lstat(target)
					if form == "strict-query" {
						return r.killed() && statErr == nil
					}
					if form == "allowed-query" {
						return correct(r, "writable\n") && statErr == nil
					}
					return correct(r, "performed\nfallback\n") && os.IsNotExist(statErr)
				})
			})
		}
	}
	t.Log("open limit: permitting a shared directory's access(W_OK) probe also permits removing it; the shell's probe is refused")
}

func TestChangedImageSetFailsClosed(t *testing.T) {
	for _, change := range []string{"mode", "change-time", "unlink", "extra-entry", "directory-mode"} {
		runScenario(t, change, func(t *testing.T) {
			f := newFixture(t)
			prepared, _, err := f.prepareImagesOn(t)
			if err != nil {
				t.Fatal("prepare images")
			}
			switch change {
			case "mode", "change-time":
				if os.Chmod(prepared.probe, 0700) != nil {
					t.Fatal("change image mode")
				}
				if change == "change-time" && os.Chmod(prepared.probe, 0500) != nil {
					t.Fatal("restore image mode")
				}
			case "directory-mode":
				defer func() {
					if os.Chmod(filepath.Dir(prepared.probe), 0700) != nil {
						t.Error("restore own image directory for cleanup")
					}
				}()
				if os.Chmod(filepath.Dir(prepared.probe), 0500) != nil {
					t.Fatal("change image directory mode")
				}
			case "unlink":
				if os.Remove(prepared.probe) != nil {
					t.Fatal("remove own temporary image")
				}
			case "extra-entry":
				if os.WriteFile(filepath.Join(filepath.Dir(prepared.probe), "extra"), nil, 0600) != nil {
					t.Fatal("add image entry")
				}
			}
			repeat(t, func() bool {
				executed.Add(1)
				_, _, err := f.prepareImagesOn(t)
				return err != nil
			})
		})
	}
}
