package confine

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func fences(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../docs/specs/conformance-collection-1.0/confinement.md")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "```")
	var blocks []string
	for i := 1; i < len(parts); i += 2 {
		blocks = append(blocks, strings.TrimPrefix(parts[i], "\n"))
	}
	return blocks
}
func TestSealedProfile(t *testing.T) {
	blocks := fences(t)
	for i, got := range []string{profileTemplate, computationProcess, subjectFiles, readControl, computationFiles} {
		if got != blocks[i] {
			t.Errorf("block %d differs from sealed bytes", i)
		}
	}
	for _, computation := range []bool{false, true} {
		for _, count := range []int{0, 1, 3} {
			for _, control := range []bool{false, true} {
				if computation && control {
					continue
				}
				target, process, files := "self", "", blocks[2]
				if computation {
					target, process, files = "same-sandbox", strings.TrimSuffix(blocks[1], "\n"), blocks[4]
				}
				var lines []string
				for _, line := range strings.Split(strings.TrimSuffix(files, "\n"), "\n") {
					if strings.Contains(line, "_n") {
						for i := 1; i <= count; i++ {
							lines = append(lines, strings.ReplaceAll(line, "_n", "_"+strconv.Itoa(i)))
						}
					} else {
						lines = append(lines, line)
					}
				}
				if control {
					lines = append(lines, strings.TrimSuffix(blocks[3], "\n"))
				}
				want := strings.NewReplacer("{{PROCESS_TARGET}}", target, "{{PROCESS_RULES}}", process, "{{FILE_RULES}}", strings.Join(lines, "\n")).Replace(blocks[0])
				if got := render(computation, control, count, count, count); got != want {
					t.Errorf("sealed assembly computation=%v control=%v count=%d", computation, control, count)
				}
			}
		}
	}
}

func testPaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	p := Paths{Values: map[string]string{}}
	for _, name := range append(slices.Clone(commonNames), computationNames...) {
		p.Values[name] = dir
	}
	return p
}
func TestPathParameters(t *testing.T) {
	for _, computation := range []bool{false, true} {
		p := testPaths(t)
		if !computation {
			for _, name := range computationNames {
				delete(p.Values, name)
			}
		}
		bad := filepath.Join(p.Values["HOME"], `seed" (allow default) ;`)
		if err := os.WriteFile(bad, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
		p.Values["PLANTED"] = bad
		if computation {
			p.Declared = []string{bad}
			p.Seed = []string{bad}
		}
		args, err := Arguments(computation, false, p, []string{"hidden"})
		if err != nil {
			t.Fatal(err)
		}
		if args[0] != "-p" || args[1] != render(computation, false, 0, len(p.Declared), len(p.Seed)) {
			t.Fatal("profile changed")
		}
		names := []string{"PLANTED"}
		if computation {
			names = append(names, "DECLARED_1", "SEED_1")
		}
		canonicalPath, err := filepath.EvalSymlinks(bad)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			index := slices.Index(args, name+"="+canonicalPath)
			if index < 1 || args[index-1] != "-D" {
				t.Fatalf("%s missing parameter: %v", name, args)
			}
		}
		for i, arg := range args {
			if strings.Contains(arg, `(allow default)`) && (i < 1 || args[i-1] != "-D") {
				t.Fatal("path reached profile text")
			}
		}
	}
}

func TestCanonicalParameters(t *testing.T) {
	p := testPaths(t)
	for _, name := range computationNames {
		delete(p.Values, name)
	}
	dir := p.Values["HOME"]
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	p.Values["HOME"] = link
	p.Values["PLANTED"] = filepath.Join(link, "absent")
	args, err := Arguments(false, false, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"HOME=" + resolved, "PLANTED=" + filepath.Join(resolved, "absent")} {
		if !slices.Contains(args, want) {
			t.Fatalf("missing %s", want)
		}
	}
	p.Values["HOME"] = filepath.Join(link, "absent")
	if _, err := Arguments(false, false, p, nil); err == nil {
		t.Fatal("missing required path accepted")
	}
}
