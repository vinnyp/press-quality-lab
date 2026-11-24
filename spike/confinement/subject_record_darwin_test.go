//go:build confinementspike && darwin

package confinement

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type recordKey struct {
	Words       []string `json:"words"`
	Offline     []string `json:"offline_args"`
	Env         []string `json:"env"`
	Runbook     *string  `json:"runbook"`
	Step        *string  `json:"step"`
	Computation []string `json:"computation"`
	Home        string   `json:"home"`
}
type recordEntry struct {
	Key          recordKey `json:"key"`
	Budget       int       `json:"budget_seconds"`
	StoreControl bool      `json:"store_control"`
	Planted      bool      `json:"planted"`
}
type recordPlan struct {
	Form        string        `json:"form"`
	Violating   bool          `json:"violating"`
	Invocations []recordEntry `json:"invocations"`
}
type recordInvocation struct {
	Key        recordKey        `json:"key"`
	Unconfined map[string]any   `json:"unconfined"`
	Confined   []map[string]any `json:"confined"`
}
type recordSubject struct {
	Store  string `json:"store"`
	Secret struct {
		Path, Token string
		Commands    [][]string
	} `json:"secret"`
	Offline []struct {
		Applies [][]string `json:"applies_to"`
		Env     map[string]string
	} `json:"offline"`
	Commands []struct {
		Words []string
		Codes []int `json:"exit_codes"`
	} `json:"commands"`
	Runbooks []struct {
		Name  string
		Steps []struct {
			Name   string
			Checks []struct{ Expected, Precondition json.RawMessage }
		}
	} `json:"runbooks"`
}

type recordNode struct {
	Kind   string
	Digest string
}

func recordMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func recordRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	recordMust(t, err)
	return b
}
func recordJSON(t *testing.T, path string, value any) {
	t.Helper()
	recordMust(t, json.Unmarshal(recordRead(t, path), value))
}
func recordHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func recordCommand(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=readonly")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("recorder build/metadata command failed: %s", out)
	}
	return out
}
func recordSource(t *testing.T, root, tag string) string {
	t.Helper()
	var pkg struct{ GoFiles []string }
	recordMust(t, json.Unmarshal(recordCommand(t, root, "go", "list", "-tags", tag, "-json", "./testdata/collection-subject"), &pkg))
	slices.Sort(pkg.GoFiles)
	entries := make([][3]any, 0, len(pkg.GoFiles))
	for _, name := range pkg.GoFiles {
		b := recordRead(t, filepath.Join(root, "testdata/collection-subject", name))
		if bytes.Contains(b, []byte("operation-ready")) {
			t.Fatal("form contains reserved harness line")
		}
		entries = append(entries, [3]any{name, "file", recordHash(b)})
	}
	b, err := json.Marshal(entries)
	recordMust(t, err)
	return recordHash(b)
}

func TestRecordSubjectBehaviour(t *testing.T) {
	root, err := filepath.Abs("../..")
	recordMust(t, err)
	plans, err := filepath.Glob(filepath.Join(root, "testdata/collection-subject/plans/*.json"))
	recordMust(t, err)
	if len(plans) == 0 {
		t.Fatal("no recording plans")
	}
	temporary, err := filepath.EvalSymlinks(t.TempDir())
	recordMust(t, err)
	launcher := filepath.Join(temporary, "launcher")
	recordCommand(t, root, "/usr/bin/clang", "-Wall", "-Wextra", "-Werror", "-o", launcher, filepath.Join(root, "spike/confinement/testdata/programs/launcher.c"))
	version := strings.TrimSpace(string(recordCommand(t, root, "/usr/bin/sw_vers", "-productVersion")))
	build := strings.TrimSpace(string(recordCommand(t, root, "/usr/bin/sw_vers", "-buildVersion")))
	commit := strings.TrimSpace(string(recordCommand(t, root, "git", "rev-parse", "HEAD")))
	for _, path := range plans {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".json"), func(t *testing.T) {
			var plan recordPlan
			recordJSON(t, path, &plan)
			if plan.Form != strings.TrimSuffix(filepath.Base(path), ".json") || len(plan.Invocations) == 0 {
				t.Fatal("invalid form plan")
			}
			subjectDir := filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects", plan.Form)
			var subject recordSubject
			recordJSON(t, filepath.Join(subjectDir, "subject.json"), &subject)
			binary := filepath.Join(temporary, plan.Form)
			tag := "form_" + strings.ReplaceAll(plan.Form, "-", "_")
			recordCommand(t, root, "go", "build", "-tags", tag, "-o", binary, "./testdata/collection-subject")
			f := recordFixture(t, temporary, launcher, binary)
			private, release, err := f.prepareImagesOn(t)
			recordMust(t, err)
			defer release()
			f.params = []string{"SUBJECT=" + binary, "SUBJECT_COPY=" + private.probe}
			invocations := make([]recordInvocation, len(plan.Invocations))
			for i, e := range plan.Invocations {
				invocations[i] = recordInvocation{Key: e.Key, Confined: make([]map[string]any, 0, 5)}
			}
			start := 0
			if plan.Violating {
				start = -1
			}
			for repetition := start; repetition < 5; repetition++ {
				homes := map[string]string{}
				captures := map[string]map[string][]byte{}
				for i, entry := range plan.Invocations {
					home := ""
					if entry.Key.Home == "runbook" {
						if entry.Key.Runbook == nil || entry.Key.Step == nil {
							t.Fatal("runbook key lacks names")
						}
						home = homes[*entry.Key.Runbook]
						if home == "" {
							home = recordHome(t, subjectDir, true)
							homes[*entry.Key.Runbook] = home
							captures[*entry.Key.Runbook] = map[string][]byte{}
						}
					} else {
						home = recordHome(t, subjectDir, entry.Key.Home == "seed" || entry.Key.Home == "planted")
					}
					run, out := recordSubjectRun(t, f, entry, subject, subjectDir, home, captures, repetition < 0)
					if entry.Key.Home == "runbook" {
						captures[*entry.Key.Runbook][*entry.Key.Step] = out
					}
					if repetition < 0 {
						invocations[i].Unconfined = run
					} else {
						invocations[i].Confined = append(invocations[i].Confined, run)
					}
				}
			}
			for _, entry := range invocations {
				for _, run := range entry.Confined[1:] {
					if !reflect.DeepEqual(recordAgreement(entry.Confined[0]), recordAgreement(run)) {
						t.Fatalf("five confined runs disagree for key %+v; nothing written", entry.Key)
					}
				}
			}
			recording := map[string]any{"behaviour_version": "1.0", "form": plan.Form, "violating": plan.Violating, "form_sha256": recordSource(t, root, tag), "subject_profile_sha256": recordHash([]byte(plantedProfile(false, 0))), "computation_profile_sha256": recordHash([]byte(plantedProfile(true, 0))), "launcher_sha256": recordHash(recordRead(t, filepath.Join(root, "spike/confinement/testdata/programs/launcher.c"))), "system": map[string]string{"product_version": version, "build": build}, "commit": commit, "invocations": invocations}
			destination := filepath.Join(root, "docs/specs/conformance-collection-1.0/behaviour", plan.Form+".json")
			if os.Getenv("PQL_RECORD") == "1" {
				data, err := json.MarshalIndent(recording, "", "  ")
				recordMust(t, err)
				recordMust(t, os.MkdirAll(filepath.Dir(destination), 0755))
				recordMust(t, os.WriteFile(destination, append(data, '\n'), 0644))
			} else {
				var previous struct {
					FormSHA     string             `json:"form_sha256"`
					Invocations []recordInvocation `json:"invocations"`
				}
				recordJSON(t, destination, &previous)
				if previous.FormSHA != recording["form_sha256"] || len(previous.Invocations) != len(invocations) {
					t.Fatal("recording source or invocation count changed")
				}
				for i, entry := range invocations {
					old := previous.Invocations[i]
					if !reflect.DeepEqual(old.Key, entry.Key) || len(old.Confined) != 5 {
						t.Fatal("recording keys or run count changed")
					}
					// JSON normalization removes Go integer-vs-float representation differences.
					for j, run := range entry.Confined {
						if !bytes.Equal(recordCanonical(t, recordAgreement(run)), recordCanonical(t, recordAgreement(old.Confined[j]))) {
							t.Fatalf("recorded agreement fields changed for invocation %d run %d", i, j)
						}
					}
				}
			}
		})
	}
}
func recordCanonical(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	recordMust(t, err)
	return b
}
func recordAgreement(run map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range run {
		switch k {
		case "stdout", "stderr":
			s := v.(map[string]any)
			out[k] = map[string]any{"bytes": s["bytes"], "sha256": s["sha256"], "cut": s["cut"]}
		case "home_changes":
			out[k] = v.(map[string]any)["changes"]
		default:
			out[k] = v
		}
	}
	return out
}
func recordFixture(t *testing.T, root, launcher, program string) fixture {
	t.Helper()
	var fs syscall.Statfs_t
	recordMust(t, syscall.Statfs(root, &fs))
	var mount []byte
	for _, b := range fs.Mntonname {
		if b == 0 {
			break
		}
		mount = append(mount, byte(b))
	}
	return fixture{root: root, launcher: launcher, probe: program, volume: string(mount), owner: t}
}
func recordHome(t *testing.T, subjectDir string, seed bool) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	recordMust(t, err)
	if seed {
		recordMust(t, os.CopyFS(home, os.DirFS(filepath.Join(subjectDir, "seed"))))
	}
	recordMust(t, os.Mkdir(filepath.Join(home, ".pql-tmp"), 0700))
	return home
}

func recordSubjectRun(t *testing.T, f fixture, entry recordEntry, subject recordSubject, subjectDir, home string, captures map[string]map[string][]byte, unconfined bool) (map[string]any, []byte) {
	t.Helper()
	computation := entry.Key.Computation != nil
	canary := ""
	planted := filepath.Join(home, subject.Secret.Path)
	if entry.Planted {
		var random [16]byte
		_, err := rand.Read(random[:])
		recordMust(t, err)
		canary = "pql" + "canary" + hex.EncodeToString(random[:])
		b := recordRead(t, planted)
		if bytes.Count(b, []byte(subject.Secret.Token)) != 1 {
			t.Fatal("seed token does not appear exactly once")
		}
		recordMust(t, os.WriteFile(planted, bytes.Replace(b, []byte(subject.Secret.Token), []byte(canary), 1), 0600))
	}
	args := append(slices.Clone(entry.Key.Words), entry.Key.Offline...)
	p := plantedProfile(false, 0)
	if entry.Planted {
		depth := len(strings.Split(filepath.ToSlash(filepath.Dir(subject.Secret.Path)), "/"))
		if filepath.Dir(subject.Secret.Path) == "." {
			depth = 0
		}
		p = plantedProfile(false, depth)
	}
	params := append(slices.Clone(f.params), "HOME="+home, "PLANTED="+planted)
	if computation {
		p = plantedProfile(true, 0)
		argv := slices.Clone(entry.Key.Computation)
		if !filepath.IsAbs(argv[0]) {
			f.probe = filepath.Join(subjectDir, argv[0])
		} else {
			f.probe = argv[0]
		}
		settings := recordComputation(t, subject, entry.Key)
		for _, capture := range settings.Captures {
			b, ok := captures[*entry.Key.Runbook][capture]
			if !ok || len(b) > 1<<20 {
				t.Fatal("missing or cut computation capture")
			}
			capturePath := filepath.Join(home, "capture-"+strconv.Itoa(len(params)))
			recordMust(t, os.WriteFile(capturePath, b, 0600))
			for i := range argv {
				argv[i] = strings.ReplaceAll(argv[i], "{CAPTURE:"+capture+"}", capturePath)
			}
		}
		for i := range argv {
			argv[i] = strings.NewReplacer("{SEED}", filepath.Join(subjectDir, "seed"), "{SUBJECT_DIR}", subjectDir).Replace(argv[i])
		}
		args = argv[1:]
		var seedFiles []string
		recordMust(t, filepath.WalkDir(filepath.Join(subjectDir, "seed"), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type().IsRegular() {
				seedFiles = append(seedFiles, path)
			}
			return nil
		}))
		slices.Sort(seedFiles)
		p, params = recordGrants(p, params, "SEED", seedFiles)
		declared := make([]string, 0, len(settings.Files))
		for _, path := range settings.Files {
			declared = append(declared, filepath.Join(subjectDir, path))
		}
		p, params = recordGrants(p, params, "DECLARED", declared)
	}
	before := recordSnapshot(t, home, canary)
	var plantedBefore os.FileInfo
	var old time.Time
	if entry.Planted {
		info, err := os.Stat(planted)
		recordMust(t, err)
		old = info.ModTime().Add(-24 * time.Hour)
		recordMust(t, os.Chtimes(planted, old, info.ModTime()))
		plantedBefore, err = os.Stat(planted)
		recordMust(t, err)
	}
	env := []string{"HOME=" + home, "TMPDIR=" + filepath.Join(home, ".pql-tmp"), "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC", coreFoundationEncoding}
	if !computation {
		for _, setting := range subject.Offline {
			for _, words := range setting.Applies {
				if len(entry.Key.Words) >= len(words) && slices.Equal(entry.Key.Words[:len(words)], words) {
					for _, name := range entry.Key.Env {
						value, ok := setting.Env[name]
						if ok {
							env = append(env, name+"="+value)
						}
					}
					break
				}
			}
		}
	}
	f.params = params
	f.env = env
	r := recordExecute(t, f, p, args, home, entry.Budget, unconfined)
	end := recordEnd(t, r, unconfined)
	var proof any
	if entry.Planted {
		after, err := os.Stat(planted)
		recordMust(t, err)
		at, err := accessTime(planted)
		recordMust(t, err)
		moved := !at.Equal(old)
		same := os.SameFile(plantedBefore, after)
		var control any
		success := false
		for _, command := range subject.Commands {
			if slices.Equal(command.Words, entry.Key.Words) {
				success = r.status.Exited() && slices.Contains(command.Codes, r.status.ExitStatus())
			}
		}
		if success && moved && same {
			// Repeat in an identical fresh home so the control cannot add run changes.
			controlHome := recordHome(t, subjectDir, true)
			controlPath := filepath.Join(controlHome, subject.Secret.Path)
			b := recordRead(t, controlPath)
			recordMust(t, os.WriteFile(controlPath, bytes.Replace(b, []byte(subject.Secret.Token), []byte(canary), 1), 0600))
			info, err := os.Stat(controlPath)
			recordMust(t, err)
			recordMust(t, os.Chtimes(controlPath, info.ModTime().Add(-24*time.Hour), info.ModTime()))
			c := f
			c.params = append(slices.Clone(f.params), "HOME="+controlHome, "PLANTED="+controlPath)
			c.env = slices.Clone(env)
			c.env[0] = "HOME=" + controlHome
			c.env[1] = "TMPDIR=" + filepath.Join(controlHome, ".pql-tmp")
			controlProfile := p + "\n(deny file-read-data (literal (param \"PLANTED\")) (with send-signal SIGKILL))\n"
			control = recordEnd(t, recordExecute(t, c, controlProfile, args, controlHome, entry.Budget, unconfined), unconfined)
		}
		proof = map[string]any{"atime_moved": moved, "same_file": same, "control_end": control}
	}
	after := recordSnapshot(t, home, canary)
	var detected any
	if entry.StoreControl {
		control := recordHome(t, subjectDir, true)
		original := recordSnapshot(t, control, "")
		file, err := os.OpenFile(filepath.Join(control, subject.Store), os.O_APPEND|os.O_WRONLY, 0)
		recordMust(t, err)
		_, err = file.Write([]byte{0})
		recordMust(t, err)
		recordMust(t, file.Close())
		detected = len(recordChanges(original, recordSnapshot(t, control, ""))) > 0
	}
	diagnostic := r.diagnostic
	if r.marked {
		diagnostic = strings.TrimPrefix(diagnostic, "confined\n")
		diagnostic = strings.TrimPrefix(diagnostic, "launch-failed\n")
	}
	stdout := recordMask([]byte(r.output), canary)
	stderr := recordMask([]byte(diagnostic), canary)
	var printed any
	if computation && r.status.Exited() && r.status.ExitStatus() == 0 && len(stdout) <= 1<<20 {
		decoder := json.NewDecoder(bytes.NewReader(stdout))
		decoder.UseNumber()
		var extra any
		if decoder.Decode(&printed) != nil || decoder.Decode(&extra) != io.EOF {
			printed = nil
		}
	}
	scan := recordLeakScan(t, home, planted, []byte(r.output), []byte(diagnostic), canary)
	return map[string]any{"markers": map[string]any{"confinement": r.marked, "could_not_start": r.startFailed}, "end": end, "stdout": recordStream(stdout), "stderr": recordStream(stderr), "home_changes": map[string]any{"changes": recordChanges(before, after), "control_detected": detected}, "planted": proof, "leak_scan": scan, "decoys": []any{}, "printed_value": printed}, stdout
}

type recordComputationSettings struct {
	Files    []string `json:"declared_files"`
	Captures []string `json:"captures"`
	Argv     []string `json:"argv"`
}

func recordComputation(t *testing.T, subject recordSubject, key recordKey) recordComputationSettings {
	t.Helper()
	for _, book := range subject.Runbooks {
		if book.Name != *key.Runbook {
			continue
		}
		for _, step := range book.Steps {
			if step.Name != *key.Step {
				continue
			}
			for _, check := range step.Checks {
				for _, source := range []json.RawMessage{check.Expected, check.Precondition} {
					var wrapper struct{ Computation *recordComputationSettings }
					recordMust(t, json.Unmarshal(source, &wrapper))
					if wrapper.Computation != nil && slices.Equal(wrapper.Computation.Argv, key.Computation) {
						return *wrapper.Computation
					}
				}
			}
		}
	}
	t.Fatal("computation not declared in subject")
	return recordComputationSettings{}
}
func recordGrants(profile string, params []string, name string, paths []string) (string, []string) {
	lines := make([]string, 0, len(paths))
	for i, path := range paths {
		key := fmt.Sprintf("%s_%d", name, i+1)
		lines = append(lines, fmt.Sprintf("(allow file-read-data (literal (param %q)))", key))
		params = append(params, key+"="+path)
	}
	return strings.Replace(profile, fmt.Sprintf("(allow file-read-data (literal (param %q)))", name), strings.Join(lines, "\n"), 1), params
}
func recordExecute(t *testing.T, f fixture, p string, args []string, home string, budget int, unconfined bool) result {
	t.Helper()
	prepared, release, err := f.prepareImagesOn(t)
	recordMust(t, err)
	defer release()
	parameters := prepared.parameters()
	// Depth grants use the current home's directories, including control homes.
	values := map[string]string{}
	for _, parameter := range parameters {
		name, value, _ := strings.Cut(parameter, "=")
		values[name] = value
	}
	current := home
	relative, err := filepath.Rel(home, filepath.Dir(values["PLANTED"]))
	recordMust(t, err)
	if relative != "." {
		for i, part := range strings.Split(relative, string(filepath.Separator)) {
			current = filepath.Join(current, part)
			parameters = append(parameters, fmt.Sprintf("PLANTED_DIR_%d=%s", i+1, current))
		}
	}
	argv := []string{"-p", p}
	for _, parameter := range parameters {
		argv = append(argv, "-D", parameter)
	}
	argv = append(argv, prepared.launcher, prepared.probe)
	argv = append(argv, args...)
	binary := "/usr/bin/sandbox-exec"
	if unconfined {
		binary = prepared.launcher
		argv = append([]string{prepared.probe}, args...)
	}
	executed.Add(1)
	return runBudgetWithObserver(binary, argv, f.env, home, time.Duration(budget)*time.Second, nil, nil, nil)
}
func recordEnd(t *testing.T, r result, unconfined bool) map[string]any {
	t.Helper()
	if r.harnessKilled || r.startFailed {
		t.Fatal("recorder harness/launcher failed; nothing written")
	}
	if !unconfined && !r.marked {
		t.Fatal("recorder launcher did not mark confinement; nothing written")
	}
	if r.status.Exited() {
		return map[string]any{"kind": "exit", "code": r.status.ExitStatus()}
	}
	if !r.status.Signaled() {
		t.Fatal("recorder has no process exit status")
	}
	signal := r.status.Signal()
	name := "SIG" + strings.ToUpper(strings.ReplaceAll(signal.String(), " ", ""))
	for _, item := range []struct {
		signal syscall.Signal
		name   string
	}{{syscall.SIGKILL, "SIGKILL"}, {syscall.SIGSEGV, "SIGSEGV"}, {syscall.SIGABRT, "SIGABRT"}, {syscall.SIGTERM, "SIGTERM"}, {syscall.SIGPIPE, "SIGPIPE"}, {syscall.SIGILL, "SIGILL"}, {syscall.SIGBUS, "SIGBUS"}, {syscall.SIGINT, "SIGINT"}, {syscall.SIGTRAP, "SIGTRAP"}} {
		if signal == item.signal {
			name = item.name
		}
	}
	var by any
	if signal == syscall.SIGKILL {
		switch {
		case r.timedOut:
			by = "budget"
		case r.exitDetail&syscall.NOTE_EXIT_MEMORY != 0:
			by = "memory"
		case r.exitDetail&syscall.NOTE_EXIT_CSERROR != 0:
			by = "code_signing"
		case !r.marked:
			by = "before_marker"
		default:
			by = "confinement"
		}
	}
	return map[string]any{"kind": "signal", "signal": name, "by": by}
}
func recordStream(b []byte) map[string]any {
	kept := b
	cut := len(b) > 1<<20
	if cut {
		kept = b[:1<<20]
	}
	head := kept
	if len(head) > 64<<10 {
		head = head[:64<<10]
	}
	var encoded any
	if !cut {
		encoded = base64.StdEncoding.EncodeToString(kept)
	}
	return map[string]any{"bytes": len(b), "sha256": recordHash(kept), "cut": cut, "head": string([]rune(string(head))), "kept": encoded}
}
func recordEncodings(b []byte) [][2]string {
	percent := func(upper bool) string {
		var out strings.Builder
		for _, value := range b {
			if upper {
				fmt.Fprintf(&out, "%%%02X", value)
			} else {
				fmt.Fprintf(&out, "%%%02x", value)
			}
		}
		return out.String()
	}
	return [][2]string{{"plain", string(b)}, {"base64", base64.StdEncoding.EncodeToString(b)}, {"base64-nopad", base64.RawStdEncoding.EncodeToString(b)}, {"base64url", base64.URLEncoding.EncodeToString(b)}, {"base64url-nopad", base64.RawURLEncoding.EncodeToString(b)}, {"hex-lower", hex.EncodeToString(b)}, {"hex-upper", strings.ToUpper(hex.EncodeToString(b))}, {"percent-upper", percent(true)}, {"percent-lower", percent(false)}}
}
func recordMask(b []byte, canary string) []byte {
	if canary == "" {
		return b
	}
	masked := recordEncodings([]byte("pql" + "canary" + strings.Repeat("0", 32)))
	for i, value := range recordEncodings([]byte(canary)) {
		b = bytes.ReplaceAll(b, []byte(value[1]), []byte(masked[i][1]))
	}
	return b
}
func recordSnapshot(t *testing.T, home, canary string) map[string]recordNode {
	t.Helper()
	nodes := map[string]recordNode{}
	recordMust(t, filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == home {
			return nil
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		node := recordNode{}
		switch {
		case d.Type().IsRegular():
			node.Kind = "file"
			file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if err != nil {
				return err
			}
			info, statErr := file.Stat()
			if statErr != nil {
				_ = file.Close()
				return statErr
			}
			if !info.Mode().IsRegular() || info.Size() > 16<<20 {
				_ = file.Close()
				return fmt.Errorf("home snapshot exceeded file bound or changed kind; nothing written")
			}
			b, err := io.ReadAll(io.LimitReader(file, 16<<20))
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			if len(b) > 16<<20 {
				return fmt.Errorf("home snapshot exceeded file bound; nothing written")
			}
			node.Digest = recordHash(recordMask(b, canary))
		case d.IsDir():
			node.Kind = "directory"
		case d.Type()&os.ModeSymlink != 0:
			node.Kind = "link"
		default:
			node.Kind = "special"
		}
		nodes[string(recordMask([]byte(filepath.ToSlash(relative)), canary))] = node
		return nil
	}))
	return nodes
}
func recordChanges(before, after map[string]recordNode) []any {
	paths := map[string]bool{}
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	slices.Sort(ordered)
	changes := make([]any, 0)
	for _, path := range ordered {
		a, aok := before[path]
		b, bok := after[path]
		if aok && bok && a == b {
			continue
		}
		change := "modified"
		node := b
		if !aok {
			change = "created"
		}
		if !bok {
			change = "removed"
			node = a
		}
		var digest any
		if node.Kind == "file" && bok {
			digest = node.Digest
		}
		changes = append(changes, map[string]any{"path": path, "kind": node.Kind, "change": change, "sha256": digest})
	}
	return changes
}
func recordLeakScan(t *testing.T, home, planted string, stdout, stderr []byte, canary string) map[string]any {
	t.Helper()
	leaks := make([]any, 0)
	bound := false
	if canary == "" {
		return map[string]any{"bound_reached": false, "leaks": leaks}
	}
	encodings := recordEncodings([]byte(canary))
	scan := func(place string, b []byte) {
		for _, value := range encodings {
			if bytes.Contains(b, []byte(value[1])) {
				leaks = append(leaks, map[string]any{"place": place, "encoding": value[0]})
			}
		}
	}
	for i, b := range [][]byte{stdout, stderr} {
		if len(b) > 1<<20 {
			b = b[:1<<20]
			bound = true
		}
		scan([]string{"stdout", "stderr"}[i], b)
	}
	total := 0
	recordMust(t, filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == planted || !d.Type().IsRegular() {
			return nil
		}
		remaining := (256 << 20) - total
		limit := min(16<<20, remaining)
		if limit <= 0 {
			bound = true
			return nil
		}
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		info, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return statErr
		}
		if !info.Mode().IsRegular() {
			_ = file.Close()
			return fmt.Errorf("home scan file changed kind")
		}
		if info.Size() >= int64(limit) {
			bound = true
		}
		b, err := io.ReadAll(io.LimitReader(file, int64(limit)))
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if len(b) > limit {
			b = b[:limit]
			bound = true
		}
		total += len(b)
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		scan("home/"+string(recordMask([]byte(filepath.ToSlash(relative)), canary)), b)
		return nil
	}))
	return map[string]any{"bound_reached": bound, "leaks": leaks}
}
