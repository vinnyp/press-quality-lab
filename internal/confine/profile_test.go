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
						n := count
						if strings.Contains(line, "DECLARED_n") {
							n = count + 1
						}
						if strings.Contains(line, "SEED_n") {
							n = count + 2
						}
						for i := 1; i <= n; i++ {
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
				if got := render(computation, control, count, count+1, count+2); got != want {
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
			p.Seed = []string{bad, bad, bad}
		} else {
			p.PlantedDirs = []string{bad, bad}
		}
		args, err := Arguments(computation, false, p, []string{"hidden"})
		if err != nil {
			t.Fatal(err)
		}
		if args[0] != "-p" || args[1] != render(computation, false, len(p.PlantedDirs), len(p.Declared), len(p.Seed)) {
			t.Fatal("profile changed")
		}
		names := []string{"PLANTED"}
		if computation {
			names = append(names, "DECLARED_1", "SEED_1", "SEED_2", "SEED_3")
		} else {
			names = append(names, "PLANTED_DIR_1", "PLANTED_DIR_2")
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

func TestInvalidProfileParameters(t *testing.T) {
	for _, kind := range []string{"unknown", "dangling", "other_values", "other_files", "other_dirs"} {
		t.Run(kind, func(t *testing.T) {
			p := testPaths(t)
			for _, name := range computationNames {
				delete(p.Values, name)
			}
			computation := false
			switch kind {
			case "unknown":
				p.Values["UNKNOWN"] = p.Values["HOME"]
			case "dangling":
				path := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), path); err != nil {
					t.Fatal(err)
				}
				p.Values["PLANTED"] = path
			case "other_values":
				p.Values["TTY_DEVICE"] = p.Values["HOME"]
			case "other_files":
				p.Declared = []string{p.Values["HOME"]}
			case "other_dirs":
				computation = true
				p = testPaths(t)
				p.PlantedDirs = []string{p.Values["HOME"]}
			}
			if _, err := Arguments(computation, false, p, nil); err == nil {
				t.Fatal("invalid profile parameters accepted")
			}
		})
	}
}

func TestAbsentComputationParameters(t *testing.T) {
	// QH11: absent final names resolve their existing parent, never a dangling link.
	for _, name := range []string{"PLANTED", "SUBJECT", "SUBJECT_COPY"} {
		t.Run(name, func(t *testing.T) {
			p := testPaths(t)
			for _, subjectName := range []string{"SUBJECT", "SUBJECT_COPY"} {
				p.Values[subjectName] = filepath.Join(p.Values["HOME"], subjectName)
				if err := os.WriteFile(p.Values[subjectName], []byte("subject"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Arguments(true, false, p, nil); err != nil {
				t.Fatalf("positive control: %v", err)
			}

			root := p.Values["HOME"]
			resolved, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(t.TempDir(), "parent")
			if err := os.Symlink(root, link); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(link, "absent")
			p.Values[name] = path
			args, err := Arguments(true, false, p, nil)
			if name != "PLANTED" {
				if err == nil {
					t.Fatal("absent subject accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(args, name+"="+filepath.Join(resolved, "absent")) {
				t.Fatalf("parameters=%v", args)
			}
			if err := os.Symlink(filepath.Join(root, "missing"), path); err != nil {
				t.Fatal(err)
			}
			if _, err := Arguments(true, false, p, nil); err == nil {
				t.Fatal("dangling link accepted")
			}
		})
	}
	for _, name := range append(slices.Clone(commonNames), "BIN_DIR", "USR_BIN_DIR", "TTY_DEVICE", "DECLARED", "SEED", "PLANTED_DIR") {
		if name == "PLANTED" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			p := testPaths(t)
			computation := name != "PLANTED_DIR"
			if !computation {
				for _, key := range computationNames {
					delete(p.Values, key)
				}
				p.PlantedDirs = []string{p.Values["HOME"]}
			}
			if name == "DECLARED" {
				p.Declared = []string{p.Values["HOME"]}
			}
			if name == "SEED" {
				p.Seed = []string{p.Values["HOME"]}
			}
			if _, err := Arguments(computation, false, p, nil); err != nil {
				t.Fatal(err)
			}
			absent := filepath.Join(p.Values["HOME"], "absent")
			switch name {
			case "DECLARED":
				p.Declared[0] = absent
			case "SEED":
				p.Seed[0] = absent
			case "PLANTED_DIR":
				p.PlantedDirs[0] = absent
			default:
				p.Values[name] = absent
			}
			if _, err := Arguments(computation, false, p, nil); err == nil {
				t.Fatal("missing required path accepted")
			}
		})
	}
}
