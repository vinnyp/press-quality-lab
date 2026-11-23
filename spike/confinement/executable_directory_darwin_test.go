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
		for _, mode := range []string{"image-list", "image-parent", "image-write"} {
			for _, form := range []string{"confined", "twin", "positive"} {
				runScenario(t, profileName(computation)+"/"+mode+"/"+form, func(t *testing.T) {
					repeat(t, func() bool {
						p := profile(computation)
						if form == "positive" {
							p = ""
						}
						r := f.invoke(p, mode, form == "twin")
						if form == "confined" && mode != "image-list" {
							return r.killed()
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
					r := probe.invoke(p, "directory-list", form == "twin")
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
					r := probe.invoke(p, mode, false)
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
