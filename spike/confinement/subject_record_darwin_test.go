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
	"net"
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
	Violates     bool      `json:"violates"`
	PeerPID      bool      `json:"peer_pid"`
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
			canonical := filepath.Join(root, "docs/specs/conformance-collection-1.0/subjects", plan.Form)
			// Computations receive a real subject directory with its binary. Keep
			// generated executables out of the canonical corpus in the checkout.
			subjectDir := filepath.Join(temporary, plan.Form+"-subject")
			recordMust(t, os.CopyFS(subjectDir, os.DirFS(canonical)))
			recordMust(t, os.MkdirAll(filepath.Join(subjectDir, "bin"), 0700))
			var subject recordSubject
			recordJSON(t, filepath.Join(subjectDir, "subject.json"), &subject)
			binary := filepath.Join(subjectDir, "bin/subject")
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
					run, out := recordSubjectRun(t, f, entry, subject, subjectDir, home, captures, repetition < 0, plan.Form)
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

func recordSubjectRun(t *testing.T, f fixture, entry recordEntry, subject recordSubject, subjectDir, home string, captures map[string]map[string][]byte, unconfined bool, form string) (map[string]any, []byte) {
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
	protected := planted
	if !entry.Planted {
		protected = filepath.Join(home, ".pql-tmp", "no-planted-file")
	}
	params := append(slices.Clone(f.params), "HOME="+home, "PLANTED="+protected)
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
	decoys := recordPrepareDecoys(t, form, home, planted, entry.PeerPID)
	defer decoys.close(t)
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
		if form == "forges-atime" {
			decoys.files[0].info = plantedBefore
		}
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
	states, touched := decoys.observe(t)
	if entry.Violates {
		if unconfined && !touched {
			t.Fatalf("%s positive control did not show its act", form)
		}
		if !unconfined && (end["kind"] != "signal" || end["signal"] != "SIGKILL" || end["by"] != "confinement" || touched) {
			t.Fatalf("%s act was not refused by confinement with its decoy untouched", form)
		}
	}
	if form == "links-decoy" && touched {
		t.Fatal("link creation touched private_file")
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
	if computation {
		printed = recordPrintedValue(result{output: string(stdout), status: r.status})
	}

	scan := recordLeakScan(t, home, planted, []byte(r.output), []byte(diagnostic), canary)
	return map[string]any{"markers": map[string]any{"confinement": r.marked, "could_not_start": r.startFailed}, "end": end, "stdout": recordStream(stdout), "stderr": recordStream(stderr), "home_changes": map[string]any{"changes": recordChanges(before, after), "control_detected": detected}, "planted": proof, "leak_scan": scan, "decoys": states, "printed_value": printed}, stdout
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
			if !info.Mode().IsRegular() {
				_ = file.Close()
				return fmt.Errorf("home snapshot changed kind; nothing written")
			}
			b, err := io.ReadAll(file)
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
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

// A JSON number that the judge rejects must not be embedded in the recording.
func recordPrintedNumbers(v any) bool {
	switch x := v.(type) {
	case json.Number:
		text := string(x)
		if len(text) > 32 {
			return false
		}
		if i := strings.IndexAny(text, "eE"); i >= 0 {
			exponent := strings.TrimLeft(text[i+1:], "+-")
			if len(exponent) > 4 {
				return false
			}
		}
	case []any:
		for _, child := range x {
			if !recordPrintedNumbers(child) {
				return false
			}
		}
	case map[string]any:
		for _, child := range x {
			if !recordPrintedNumbers(child) {
				return false
			}
		}
	}
	return true
}

func TestSubjectRecorderEdges(t *testing.T) {
	t.Run("cut", func(t *testing.T) {
		data := bytes.Repeat([]byte("w"), 17<<20)
		stream := recordStream(data)
		if stream["bytes"] != len(data) || stream["cut"] != true || stream["kept"] != nil || stream["sha256"] != recordHash(data[:1<<20]) {
			t.Fatal("cut stream lost byte count, kept digest, or null kept")
		}
	})
	t.Run("utf8", func(t *testing.T) {
		stream := recordStream([]byte{'a', 0xff, 0xff, '\n'})
		if stream["head"] != "a\ufffd\ufffd\n" {
			t.Fatal("each ill-formed byte needs its own replacement")
		}
	})
	t.Run("ends", func(t *testing.T) {
		for _, row := range []struct {
			signal  syscall.Signal
			timeout bool
			kind    string
		}{
			{syscall.SIGKILL, false, "confinement"}, {syscall.SIGKILL, true, "budget"}, {syscall.SIGABRT, false, ""},
		} {
			end := recordEnd(t, result{marked: true, timedOut: row.timeout, status: syscall.WaitStatus(row.signal)}, false)
			if end["kind"] != "signal" {
				t.Fatal(end)
			}
			if row.signal == syscall.SIGABRT {
				if end["signal"] != "SIGABRT" || end["by"] != nil {
					t.Fatal(end)
				}
			} else if end["by"] != row.kind {
				t.Fatal(end)
			}
		}
	})
	t.Run("home", func(t *testing.T) {
		home := t.TempDir()
		before := recordSnapshot(t, home, "")
		recordMust(t, os.WriteFile(filepath.Join(home, "data.json-wal"), []byte{}, 0600))
		recordMust(t, os.WriteFile(filepath.Join(home, "data.json-shm"), make([]byte, 32768), 0600))
		recordMust(t, os.Symlink("data.json", filepath.Join(home, "latest")))
		recordMust(t, os.WriteFile(filepath.Join(home, "big.log"), bytes.Repeat([]byte("x"), (16<<20)+1), 0600))
		after := recordSnapshot(t, home, "")
		changes := recordChanges(before, after)
		if len(changes) != 4 || after["big.log"].Digest != recordHash(bytes.Repeat([]byte("x"), (16<<20)+1)) || after["latest"].Kind != "link" {
			t.Fatal("snapshot lost link, side file, or bytes past scan bound")
		}
	})
	t.Run("encodings", func(t *testing.T) {
		// Only the masked canary is ever written, including in test scratch files.
		canary := "pql" + "canary" + strings.Repeat("0", 32)
		original := recordEncodings([]byte(strings.Repeat("l", 41)))
		for i, encoding := range recordEncodings([]byte(canary)) {
			t.Run(encoding[0], func(t *testing.T) {
				home := t.TempDir()
				recordMust(t, os.WriteFile(filepath.Join(home, "debug.log"), []byte(encoding[1]), 0600))
				scan := recordLeakScan(t, home, "", []byte(encoding[1]), nil, canary)
				leaks := scan["leaks"].([]any)
				found := map[string]bool{}
				for _, value := range leaks {
					leak := value.(map[string]any)
					if leak["encoding"] == encoding[0] {
						found[leak["place"].(string)] = true
					}
				}
				if !found["stdout"] || !found["home/debug.log"] {
					t.Fatal("encoding absent from stream or file scan", scan)
				}
				if !bytes.Equal(recordMask([]byte(original[i][1]), strings.Repeat("l", 41)), []byte(encoding[1])) {
					t.Fatal("encoding was not masked")
				}
			})
		}
	})
	t.Run("scan-bound", func(t *testing.T) {
		home := t.TempDir()
		canary := "pql" + "canary" + strings.Repeat("0", 32)
		content := append(bytes.Repeat([]byte("x"), 16<<20), []byte(canary+"\n")...)
		recordMust(t, os.WriteFile(filepath.Join(home, "big.log"), content, 0600))
		scan := recordLeakScan(t, home, "", nil, nil, canary)
		if scan["bound_reached"] != true || len(scan["leaks"].([]any)) != 0 {
			t.Fatal("scan crossed its file bound", scan)
		}
	})
	t.Run("number-form", func(t *testing.T) {
		if recordPrintedNumbers(json.Number(strings.Repeat("1", 33))) || recordPrintedNumbers(map[string]any{"n": json.Number("1e00001")}) || !recordPrintedNumbers(json.Number("1e9999")) {
			t.Fatal("printed number limits changed")
		}
	})
}

// Decoys are owned by one invocation. Exclusive creation refuses a stale or
// foreign root; cleanup removes only the root this invocation created.
const recordDecoyRoot = "/private/tmp/pql-decoys"

type recordDecoyFile struct {
	name, path string
	info       os.FileInfo
	atime      time.Time
	content    []byte
	writesOnly bool
}
type recordDecoys struct {
	root        bool
	files       []recordDecoyFile
	listener    *net.TCPListener
	victim      *exec.Cmd
	waited      chan error
	victimEnded bool
	home, form  string
}

func recordPrepareDecoys(t *testing.T, form, home, planted string, peer bool) *recordDecoys {
	t.Helper()
	d := &recordDecoys{home: home, form: form}
	switch form {
	case "tries-network", "live-first", "starts-child", "listens", "ignores-home", "cache-outside-home", "reads-key-file", "signals-process", "reads-process-env", "forges-atime", "links-decoy":
	default:
		return d
	}
	recordMust(t, os.Mkdir(recordDecoyRoot, 0700))
	d.root = true
	// Register failure cleanup before allocating listeners or a process.
	t.Cleanup(func() { d.close(t) })
	add := func(name, path string, content []byte, writesOnly bool) {
		recordMust(t, os.MkdirAll(filepath.Dir(path), 0700))
		recordMust(t, os.WriteFile(path, content, 0600))
		info, err := os.Stat(path)
		recordMust(t, err)
		old := info.ModTime().Add(-24 * time.Hour)
		recordMust(t, os.Chtimes(path, old, info.ModTime()))
		info, err = os.Stat(path)
		recordMust(t, err)
		d.files = append(d.files, recordDecoyFile{name, path, info, old, content, writesOnly})
	}
	switch form {
	case "ignores-home":
		add("home_store", recordDecoyRoot+"/user/data.json", []byte("{\"widgets\":[{\"name\":\"decoy\",\"tag\":\"metal\"}]}\n"), false)
	case "cache-outside-home":
		add("outside_cache", recordDecoyRoot+"/cache/widgets.cache", []byte("untouched\n"), false)
	case "reads-key-file":
		var random [16]byte
		_, err := rand.Read(random[:])
		recordMust(t, err)
		secret := "pql" + "canary" + hex.EncodeToString(random[:])
		t.Setenv("WIDGETS_API_KEY", secret)
		add("key_file", recordDecoyRoot+"/user/.ssh/id_widgets", []byte(secret), false)
	case "links-decoy":
		add("private_file", recordDecoyRoot+"/private.txt", []byte("private decoy\n"), false)
	case "forges-atime":
		info, err := os.Stat(planted)
		if os.IsNotExist(err) {
			break
		}
		recordMust(t, err)
		// Reading the baseline precedes the recorder's planted-read clock reset.
		content := recordRead(t, planted)
		d.files = append(d.files, recordDecoyFile{name: "token_file", path: planted, info: info, content: content, writesOnly: true})
	case "tries-network", "live-first":
		address, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:47391")
		recordMust(t, err)
		d.listener, err = net.ListenTCP("tcp4", address)
		if err != nil {
			t.Fatal("decoy loopback port 47391 unavailable")
		}
	}
	if peer {
		var random [16]byte
		_, err := rand.Read(random[:])
		recordMust(t, err)
		victim := exec.Command("/bin/sleep", "600")
		victim.Env = []string{"WIDGETS_API_KEY=pql" + "canary" + hex.EncodeToString(random[:])}
		recordMust(t, victim.Start())
		d.victim = victim
		d.waited = make(chan error, 1)
		go func() { d.waited <- d.victim.Wait() }()
		recordMust(t, os.WriteFile(filepath.Join(home, "peer.pid"), []byte(strconv.Itoa(d.victim.Process.Pid)+"\n"), 0600))
	}
	return d
}
func (d *recordDecoys) observe(t *testing.T) ([]any, bool) {
	t.Helper()
	states := make([]any, 0)
	anyTouched := false
	add := func(name string, touched bool) {
		states = append(states, map[string]any{"name": name, "touched": touched})
		anyTouched = anyTouched || touched
	}
	for _, f := range d.files {
		info, err := os.Stat(f.path)
		recordMust(t, err)
		at, err := accessTime(f.path)
		recordMust(t, err)
		// Inspect access time before reading content, which itself changes it.
		touched := !os.SameFile(f.info, info) || !f.info.ModTime().Equal(info.ModTime())
		if f.writesOnly {
			before := f.info.Sys().(*syscall.Stat_t)
			after := info.Sys().(*syscall.Stat_t)
			touched = touched || before.Ctimespec != after.Ctimespec
		} else {
			touched = touched || !f.atime.Equal(at)
		}
		touched = touched || !bytes.Equal(f.content, recordRead(t, f.path))
		add(f.name, touched)
	}
	if d.listener != nil {
		recordMust(t, d.listener.SetDeadline(time.Now().Add(50*time.Millisecond)))
		c, err := d.listener.AcceptTCP()
		connected := err == nil
		if connected {
			recordMust(t, c.Close())
		} else if e, ok := err.(net.Error); !ok || !e.Timeout() {
			recordMust(t, err)
		}
		add("loopback_connection", connected)
	}
	if d.victim != nil {
		select {
		case err := <-d.waited:
			d.victimEnded = true
			exit, ok := err.(*exec.ExitError)
			if !ok || !exit.Sys().(syscall.WaitStatus).Signaled() || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
				t.Fatal("victim ended without SIGTERM")
			}
		case <-time.After(50 * time.Millisecond):
		}
		if d.form == "signals-process" {
			add("peer_process", d.victimEnded)
		} else {
			_, err := os.Stat(filepath.Join(d.home, "peer-observed"))
			add("peer_environment", err == nil)
		}
	}
	for _, item := range []struct{ form, name, path string }{{"starts-child", "child_process", "child-ran"}, {"listens", "loopback_listener", "listener-ran"}} {
		if d.form == item.form {
			_, err := os.Stat(filepath.Join(d.home, item.path))
			add(item.name, err == nil)
		}
	}
	return states, anyTouched
}
func (d *recordDecoys) close(t *testing.T) {
	t.Helper()
	if d.listener != nil {
		recordMust(t, d.listener.Close())
		d.listener = nil
	}
	if d.victim != nil {
		if !d.victimEnded {
			_ = d.victim.Process.Kill()
			<-d.waited
		}
		d.victim = nil
	}
	if d.root {
		recordMust(t, os.RemoveAll(recordDecoyRoot))
		d.root = false
	}
}

// Exit status does not decide whether a computation printed one JSON value.
func recordPrintedValue(r result) any {
	stdout := []byte(r.output)
	if len(stdout) > 1<<20 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	decoder.UseNumber()
	var value, extra any
	if decoder.Decode(&value) != nil || decoder.Decode(&extra) != io.EOF || !recordPrintedNumbers(value) {
		return nil
	}
	return value
}
func TestRecordPrintedValue(t *testing.T) {
	for _, row := range []struct{ output, want string }{{"3\n", "3"}, {"true", "true"}, {"3 4", "null"}, {"not json", "null"}, {"", "null"}} {
		got := string(recordCanonical(t, recordPrintedValue(result{output: row.output})))
		if got != row.want {
			t.Errorf("printed %q: got %s, want %s", row.output, got, row.want)
		}
	}
	// This is the computation fixture's actual nonzero exit, without confinement
	// or a recording run. Its printed value still survives.
	cmd := exec.Command("/bin/sh", "-c", "printf '3\\n'; exit 1")
	out, err := cmd.Output()
	if err == nil {
		t.Fatal("nonzero fixture exited successfully")
	}
	if got := recordPrintedValue(result{output: string(out), status: cmd.ProcessState.Sys().(syscall.WaitStatus)}); got != json.Number("3") {
		t.Fatalf("nonzero computation printed value = %v", got)
	}
}
