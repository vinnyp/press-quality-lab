---
title: Grade Command - Plan
type: feat
date: 2026-11-24
topic: grade-command
artifact_contract: ce-unified-plan/v1
product_contract_source: ce-plan-bootstrap
execution: code
---

# Grade Command - Plan

This plan covers phase 3 of the [engine plan](2026-11-21-engine-plan.md). Its own ids start with G: GR for requirements, GKD for product decisions. A bare R, AE, A or F number is an entry in [the requirements](../requirements.md). Q ids are clauses of the sealed conformance collection specification, `docs/specs/conformance-collection-1.0/`; X, N, E and R ids after "judge rule" are the judge rulebook's.

## Goal Capsule

- **Objective:** A person can grade a subject with `pql grade` and trust the result as far as the synthetic subject proves it: every case of the sealed collection corpus gives the record and exit code the specification fixes, and the known-good subject is green and exits zero.
- **Means:** a strict JSON leaf, then a thin end-to-end slice, then six slices, each its own pull request; a darwin track that starts with a launcher spike and runs beside slices a to c; and a corpus runner that drives every case through the real `grade` command (KTD1, KTD2, KTD14).
- **Authority:** the sealed judge specification 1.0 and conformance collection specification 1.0 with its errata note, then `AGENTS.md` and `CODING_STANDARDS.md`, then this plan. An input the sealed text can honestly read two ways goes to the owner with both readings (GR6).
- **Stop conditions:** a change that would need judge 1.1 or collection 1.1, including a launcher that cannot start under the unchanged QH11 template (U21); a corpus case the engine cannot meet without departing from the text; a confinement rule that fails on macOS 27.2.
- **Execution profile:** Deep. Fifteen pull requests (Implementation Units). A coding agent writes the code and the Linux-runnable tests from a brief per unit; the main session writes briefs, reviews, and runs every macOS test outside any sandbox (KTD19).
- **Who finishes:** the executing session opens each pull request and stops. The owner merges, or the session merges on the owner's say-so.
- **Dates:** phase 3 done by 2027-01-16, ahead of the halfway check on 2027-01-25 (engine plan §6). Strict JSON leaf by 2026-12-02; launcher spike by 2026-12-04; end-to-end slice by 2026-12-09; slice a by 2026-12-14; slice b by 2026-12-19; slice c by 2026-12-29; the darwin track by 2027-01-02; slice d by 2027-01-08; slices f and e and the close by 2027-01-16. If the owner adopts a collection 1.1 under E-3, slice e waits for its seal and the close moves with it.

---

## Product Contract

### Summary

`pql grade` reads a subject file, fixes the required set, collects evidence under confinement, builds the evidence record, calls the judge through the same function `pql judge` uses, stores the run, prints a scorecard and exits with the code the judge's output gives. It is built in the six slices the engine plan names, on a thin end-to-end slice that runs three cases through the whole pipeline, with a darwin track that proves the confinement launcher early and builds the real verifier beside the first slices. A Go corpus runner assembles every corpus case and runs it through the scripted build in CI; the three real-verifier cases and the confinement evidence run on a local macOS 27.2 machine. The build is then scored against the private half of the corpus.

### Problem Frame

The judge decides a finished evidence record, and phase 2 sealed how that record is produced. Nothing yet produces one, so the engine cannot grade anything, and phase 4's first real grade cannot start until it can. The trial behind the engine plan found every judge build wrong at its JSON boundary and nowhere else, and the engine plan's largest risk is a collector that is weaker than the judge because its corpus is new: wrong evidence that the judge then grades faithfully (engine plan §7). Phase 4 then runs a real, untrusted CLI on the owner's machine, so the confinement, and everything the engine does unconfined around it, must hold before it does.

### Requirements

**Outcome**

- GR1. Every public case of the collection corpus passes through `pql grade` or `pql anchor` as QE19 defines a conforming collector, with the exit code the case expects: the scripted cases in CI on Linux, and `case-069`, `case-070` and `case-143` in a local run on macOS 27.2.
- GR2. The known-good synthetic subject grades green and exits zero (`case-001` scripted, `case-069` real).
- GR3. The exit code, the scorecard and the stored record of every run are read from the judge's one output document, through the bytes-in, bytes-out function `pql judge` exposes (engine plan §4 rule 1, QE14, AE21).
- GR4. A record the judge rejects ends the run with exit 70 and is kept (QE15).
- GR5. The build is scored against the private half of the corpus with zero wrong records, and, as the private harness measures it, no seeded collection mistake survives the build's own CI tests. Feedback to the builder names a rule id only.

**Specification authority**

- GR6. Every rule is built from the sealed text for every input its schemas allow. An input the text can honestly read two ways is reported to the owner with both readings, and the settled reading goes in the errata note or a new version, never in a workaround.
- GR7. The test entry is QT1 to QT6: a scripted verifier and host exist only in a build made with the `scripted` tag, and the released build rejects the scripted flags with the exit codes QT1 gives.

**The slices** (engine plan §5, phase 3)

- GR8. Slice a: the subject file, `pql create` and placeholders, proved by AE20.
- GR9. Slice b: command discovery and the six generated probes, proved by AE27.
- GR10. Slice c: authored checks, ordered runbooks, time budgets and computed expected values, proved by AE8, AE22 and AE23.
- GR11. Slice d: safety classes, scope and the no-network check with the real macOS verifier, proved by AE9 for conformance, AE24 and AE30.
- GR12. Slice e: the store's baselines, comparison and anchoring, proved by AE16 to AE19, AE31 and AE33 as the owner's decision on E-3 settles.
- GR13. Slice f: the scorecard, and agreement of exit code, scorecard and stored record, proved by AE21 and by the naming halves of AE8, AE20, AE23, AE24 and AE27.

**Confinement evidence**

- GR14. Each self-test program is shown necessary on macOS: a profile that grants only that program's operation makes the self-test fail. This runs as a test, never in a grade (QH9).
- GR15. Every rule a scripted run cannot break is proved by a local macOS run, stated in its pull request as `AGENTS.md` requires.
- GR16. Nothing the engine does unconfined around a subject (cleanup, inspection, scanning, rendering) acts on anything outside the throwaway directories it made, whatever the subject or a computation leaves in them.

### Key Decisions

- GKD1. **A thin end-to-end slice comes before slice a.** It runs `case-003`, `case-004` and `case-129` through `grade`, each stage only as wide as those cases need, so every later slice changes outcomes on a pipeline that already runs. The judge's JSON refactor lands first, in its own pull request. Governs GR1, GR3. (session-settled: user-approved — chosen over starting at slice a as the engine plan's table does; the split that keeps it thin was chosen after the gate's third pass over keeping one large pull request.)
- GKD2. **The corpus runner runs inside the existing CI checks.** It adds no new required check on `main`. Governs GR1. (session-settled: user-approved — chosen over a new CI job the owner would make required.)
- GKD3. **The harness assembles each case**, and `grade` takes only what QT1, the specification and GKD8 name (errata E-2). Governs GR1, GR7. (session-settled: user-directed — chosen over a scripted-build `grade` that takes a case directory.)
- GKD4. **The acceptance examples the corpus does not reach go to the owner first** (errata E-3); slice e is not built until the owner decides on a collection 1.1. Governs GR12. (session-settled: user-directed — chosen over proving them with engine tests and macOS runs alone, and over dropping them from the proof.)
- GKD5. **The build is scored against the private half, as phase 1 was.** Governs GR5. (session-settled: user-directed — chosen over the public corpus alone.)
- GKD6. **The launcher, the self-test programs and the known-fast control are hidden modes of the `pql` binary, written in Go,** and copied once per grade. Governs GR11, GR14, GR15. (session-settled: user-directed — chosen over embedding the spike's C programs.)
- GKD7. **Darwin-only code may use `golang.org/x/sys` as one reviewed dependency, with raw syscalls where it falls short, and no cgo,** so the Linux gate compiles and lints all of it. Governs GR11, GR15. (session-settled: user-directed — chosen over cgo and over the standard library alone.)
- GKD8. **The command line:** `pql grade <subject-file> [--scope conformance[,destructive]] [--baseline <file>]`; `pql anchor <subject-file> [--retired <file>] [--accept <file>]`; the QT1 flags on both; the missing-id list written to the run's store directory; a subject-file or pre-check rejection prints one stable line `pql: rejected: <rule id>: <cause>` on standard error; a malformed command line exits 64 before any exit 2. Governs GR1, GR7. (session-settled: user-directed — chosen over deciding the command line at the end-to-end slice's review.)
- GKD9. **macOS evidence is taken on 27.2 only.** The phase close states 27.0.1 as unverified for phase 3, and the owner decides whether that stands. Governs GR1, GR15. (session-settled: user-directed — chosen over evidence on both supported versions.)

### Scope Boundaries

- Phase 4's real CLI, steering (phases 5 to 7) and any edit to a sealed folder are out.
- Writing a collection 1.1 is the owner's specification work, outside this plan; if adopted under E-3, slice e builds against it.
- AE9's steering half and AE18's steering entries belong to phase 6; collection 1.0's `prompts` holds only a version.
- Linux confinement is out: version 1.0 supports macOS only. On any other system, the real verifier gives every check that runs the subject `network_unverifiable` (QH1).
- No macOS runner is added to CI; macOS evidence comes from local runs (`AGENTS.md`).
- Stated limits from `CASES.md` and the specification, such as QH19 and QP14, are named as limits in the pull request that meets them, never given a test that cannot fail.
- Considered and not built: a store-location flag. QE17 fixes the store under the user's application-support directory, and tests isolate it through the home directory instead (KTD5). Build one only if a released-build user needs a second store.
- Considered and not built: comparing a case's whole judge input. QE19 leaves `expected` and `observed` to the collector, so the runner compares the fields QE19 names, the top-level fields and the whole judge output (KTD2).
- Considered and not built: a JSON Schema library in Go. Schema validation runs in `check.sh` through the pinned Python `jsonschema` the repository already uses (KTD15); build one only if a Go-side validation is needed at run time.
- Stated limit: with the launcher being the whole `pql` binary (GKD6), a computation could run the private launcher copy if it found its path. The copy's folder name is random, and running it gives no more than the profile already allows; the plan states this in slice d rather than building around it.

### Deferred to Follow-Up Work

- Extending `internal/docrefs` to plan and spec citations of Q ids under `docs/`.

---

## Planning Contract

### Key Technical Decisions

- KTD1. **Fifteen pull requests in dependency order, with a darwin track.** The strict JSON leaf, the thin end-to-end slice, then slices a, b and c on Linux; beside them, the launcher spike and four darwin pull requests; then slice d, which integrates the real verifier; then slices f and e and the close. A case leaves the pending list when the last rule it needs lands, and 100 of the 121 judge-checked cases carry runbooks, so most cases pass only from slice c on. Each slice's design note predicts its ratchet set; its pull request traces every case that passed outside the prediction to the rule that produced it, and diagnoses every predicted case still pending. The store's run records (QE17) come with the end-to-end slice, because the stored record is the only place `grade` exposes a case's judge input and output. The engine plan requires at least one pull request per slice.
- KTD2. **The corpus runner is a Go test behind the `scripted` tag, with a two-sided ratchet and guards against vacuous passes.**
  1. It finds the cases, asserts their number, and derives the real-verifier exclusions from `input.verifier`, asserting they are exactly `case-069`, `case-070` and `case-143`.
  2. It assembles each case as QT6 and E-2 say, asserting the assembled directory holds exactly QT6's entries and never a case's `case.json`, `input.json` or `preimages.json`, with the run's home and working directory outside it (QS8), and runs the scripted `pql grade` or `pql anchor` as `input.request` says.
  3. For a judge-checked case it requires a stored record and compares: the QE19 fields; the judge input's top-level fields other than `required` and `evidence`; `required` and `evidence` as QE19 says, after sorting by check id; the process's exit code; and the whole judge output. Lists in the judge input are compared as multisets, since the judge contract gives input order no meaning. It then re-judges the stored input with `pql judge` to confirm the stored output. A compared field absent on both sides is a failure, and so is a judge-checked case with no stored record.
  4. For a derivation-only case it requires the process's exit code, no stored record, and the rejection line's rule (GKD8) equal to `expect.rule`. It also requires that no invocation was requested and no per-run home made, read from a trace file the scripted build appends to (point 5), since QE20's cleanup leaves nothing to inspect afterwards.
  5. The scripted build reads a trace path from an environment variable only it knows, outside the workspace. The scripted invoker appends each requested invocation to it, and the workspace creator appends each home. KTD12's symbol check keeps the trace out of the released binary.
  6. A checked-in list names the cases not yet expected to pass: every case off it must pass and every case on it must still fail. From slice a on, a separate layer requires every judge-checked case's `required` array to match. A `check.sh` step compares the list with its version at the base commit as a set and fails on any entry added and on the list's removal. The list may be absent at the base only when the base also has no corpus runner, which is true once. In CI the `cases` job fetches full history and is given the pull request's base, or the push's previous commit on `main`, and a missing base commit is a failure; a local run defaults to the merge base with `origin/main` and says so.
  7. A mismatch is first a hypothesis about the case: re-derive the record from the text, the recording and the case's preimages before changing code (`docs/solutions/best-practices/judge-agreement-does-not-validate-a-case-record.md`).
  8. In its macOS mode, behind KTD19's evidence tag, it builds the released `pql`, builds the known-good form and checks its source digest against the recording (QT6, as `internal/collectionkit/subject_test.go` does), runs the three real-verifier cases under KTD18's repeat rule, and reports them not run on a system QH1 does not name.

  The list is empty when phase 3 is done.
- KTD3. **The strict reader's grammar, the value types, exact equality and the contract writer move into a leaf package, `internal/strictjson`, which the judge imports.** The leaf exports a grammar-and-limits reader that takes the caller's bounds and an opaque per-node hook that decides whether to keep, discard or only note a value; the value types; `equalJSON` and `normalize`; and the writer. The judge keeps `shape`, `inputShape` and its table of what to keep, so its streaming read stays within the allocation cap `internal/judge/review_memory_test.go` sets. The subject file (QS2), host and recording (QT1), computation output (QA6) and the predicates (QA9) read through the same grammar with their own bounds, and each validates its own shape after parsing, in its own package. QA9 compares with the exact-decimal `equalJSON` and `normalize`, never canonical bytes. The writer is not RFC 8785 and never panics on a valid value: an integer within 64 bits is written as today (so `-0` is written `0`), a longer integer by its digits with the sign dropped when every digit is zero, any other number by its written form, and any string-kinded value by its string. It offers a length function for QE10's bounds. `internal/judge` exports `ScopeIncludes` and `AlwaysApplicable`, built on its existing exhaustive switches and used by the judge itself. The move changes no judge output; the judge's published cases, its tests and a re-run of its private scoring guard it, and any later change under `internal/strictjson` re-runs that scoring (U3).
- KTD4. **RFC 8785 lives in `internal/jcs`, above `internal/strictjson`,** used for QE2's check ids and seed digest and for QS11's canonical-form size and round-trip rules. Its oracles are QE2's four worked ids, each case's `preimages.json`, and vectors generated once by `scripts/collection_ids.py` and checked in, so `go test` needs no Python.
- KTD5. **Durable store and per-run workspace are separate.** The durable store (QE17) is the user's configuration directory, resolved only in `cmd/pql` after QH3's root refusal and passed down; on macOS that is the application-support directory. Records, baselines and missing-id lists are written there by temporary file and rename at mode 0600, keyed by a digest of the subject file's canonical path, and an existing store directory's mode is checked. QE20's per-run workspace (homes, scratch, private copies) lives under the canonicalised system temporary directory, in a user-owned 0700 parent checked with `Lstat`, published atomically by renaming in a process-id file that also records each live process group's id and start time. The startup sweep matches only that name pattern, never follows a link, checks the process id and start time, and ends any recorded group still alive before it removes anything. Tests give each run its own home and `XDG_CONFIG_HOME`, and a guard fails if a test reaches the developer's real store. A record is stored only once one exists.
- KTD6. **The real-verifier cases run on the released build.** QT1 puts the scripted verifier only in a scripted build, and the real path is the released build's.
- KTD7. **The necessity demonstration renders its one-grant profiles behind a separate test-only build tag.** `grade` renders only QH11's fixed template from a closed set of variants with no free-text hook; the gate checks that no released or scripted binary holds the test-only renderer.
- KTD8. **A scripted run records the host it actually ran on.** `record.schema.json` requires the host fields and QT3 checks no version, so a Linux run writes Linux's values, read from `uname` on Linux and from sysctls on darwin.
- KTD9. **A pre-check refusal does not stop collection; a rejection does.** QE5 stops a run on a rejection, and `case-071` fixes that an anchor's refusal (judge rule N1) still gives full evidence and exit 5.
- KTD10. **A scripted run's tiers are all `scripted` (QT2), so QE9's offline and live tiers are proved by unit tests and the real-verifier cases.** `case-148` fixes that QT2 takes precedence.
- KTD11. **Packages follow engine plan §4, with leaves added, and the import direction is enforced.**
  - The packages: `internal/strictjson`, `internal/jcs`, `internal/observe` (the seam's types), `internal/subject`, `internal/record` (required set, resolver, builder, masking), `internal/collect` (the untagged driver and the invokers), `internal/store`, `internal/scorecard`, and `internal/grade`, which owns the run's one path and its one exit funnel. `create`, `grade` and `anchor` sit on the existing Cobra root and call `internal/grade`.
  - The directions: `collect` imports `observe` and `record`'s resolver, and never `store` or `judge`.
  - The resolver decides each check's outcome and reason in the order of `collection-rules.md` §2, from the end-to-end slice on. The driver records only facts such as "skipped after step N"; the resolver alone assigns a reason such as `earlier_step_failed`, in §2's order.
  - A step not yet built still looks for its own condition in the raw facts: a recorded timeout or kill, a changed copy, a cut capture. Where the condition holds it gives `not_observed` with `harness_fault`; otherwise it passes the check on. So a record always exists, and an unbuilt step never claims a pass or blocks a case it does not concern.
  - These are the collection specification's evidence rules, not verdicts: the judge still decides requiredness, rollups, comparison and every verdict.
- KTD12. **The gate checks tagged and macOS code on Linux, within its time limit.**
  - `gate.sh` adds `-tags scripted` passes of vet, staticcheck, errcheck and exhaustive across the module, and of test across the packages with tagged files.
  - It adds `GOOS=darwin` passes of build, vet, staticcheck, errcheck and exhaustive, with and without the `scripted` tag and KTD7's tag.
  - A step fails on any darwin file that imports `"C"`, found by `CGO_ENABLED=1 GOOS=darwin go list -f '{{.CgoFiles}}'` and a search of darwin files, since a Linux cross-build drops such files silently (GKD7).
  - The import guard runs `go list -deps -test` for each of Linux and darwin and each tag set, with its environment set explicitly. It enforces Cobra only under `cmd/`, `golang.org/x/sys` only in darwin builds, and KTD11's directions. Its classifier is also tested on synthetic `go list` output, so the `x/sys` rule is shown to fire before `x/sys` enters `go.mod` (U15).
  - A check confirms no scripted-only or test-only symbol is in the released binary. `GOFLAGS` keeps `-mod=readonly` beside each tag.
  - The run time is measured in the end-to-end slice, and the limit raised only if needed.
- KTD13. **The seam between the scripted and real paths is an interface, and every decision is untagged.**
  - **Three layers.** Raw facts are what an invoker returns. `observe` values are what untagged functions derive from them. Resolution is what the resolver decides from those values.
  - **The driver.** An untagged driver in `collect` owns all sequencing: QD2's walk, runbook order, fresh homes, the controls, the planted canary from `crypto/rand` (QP10) and QP9's control copy. It calls `Invoker.Run(key, spec)`, which returns raw facts.
  - **The two invokers.** The scripted invoker looks the key up in the recording and maps the recording's attributed end and the host's override back into raw facts; only its reader may read the recording's attribution strings. The darwin invoker only spawns, waits on kqueue, kills the group and reads files.
  - **Untagged derivations.** Every function from raw facts to `observe` values is untagged and tested on Linux with fakes. These include QH5 and QH7's marker parsing, QH8's copy comparison, QH24's bound and QA6's parse. They also include QH21 and QO5's kill attribution from the raw end: the signal, whether the marker was seen, any kill the engine sent and when, the exit detail, and whether the program was still running at the budget.
  - **Guards.** A type-based guard fails on any tagged, darwin or unsupported-system file outside the scripted reader that refers to `observe` or `record`'s outcome, reason or attribution values. A projection test checks that the scripted invoker's facts for every corpus recording and host serialise back to the recording's run and the host object field for field; U32 compares real facts through the same serialiser.
- KTD14. **The launcher and the self-test programs are hidden modes of `pql` (GKD6).**
  1. A reserved first argument selects a hidden mode at the top of `main`, before Cobra. Its arguments are positional and fixed in number, so no subject word is read as a launcher option.
  2. No package in `pql`'s import graph does work at initialisation that the frozen profile kills. A checked-in list of `GOOS=darwin go list -deps ./cmd/pql` fails the gate on any change. A Linux gate test compares the package names in `GODEBUG=inittrace=1` output for the launcher mode with a checked-in list. A guard fails if `crypto/x509`, `crypto/tls` or `net/http` enter `cmd/pql`'s darwin dependencies. Any pull request that changes either list re-runs U21's launcher check on macOS.
  3. The marker is drawn from `crypto/rand` per invocation and passed in the arguments, since QH12 allows nothing else through the environment. The launcher writes it as the first line of standard error and strips its own arguments. It closes every descriptor above 2 in a loop up to `RLIMIT_NOFILE`, never by listing `/dev/fd` (QH13), and replaces itself with the program, keeping its process id (QH4).
  4. If the replacement fails, the launcher writes the could-not-start marker, which carries the same per-invocation value, as the line directly after the marker. It then exits with its reserved code and writes nothing more (QH7). The engine honours it only there; both marker lines are stripped before any count, digest, keep or scan.
  5. The self-test programs make raw `golang.org/x/sys/unix` calls (socket, connect, sendto, bind, kill, sysctl and the like), never the `net` or `os/exec` packages, whose option calls the profile kills. They print each call's result. Their targets are decoys only, with a process id above 0.
  6. The private copy's folder name inside the run directory is random. The hidden modes are part of the released command surface U2 tests.
- KTD15. **Schema oracles come from the specification, not the code.**
  - Each reader rejects every file in `examples/invalid/` that targets its schema, with the rejection line naming the rule in the example's file name. Boundary tests take each limit ± 1 from the schema files.
  - `check.sh` makes a fresh directory with `mktemp -d`, names it to the corpus runner through an environment variable, and runs the runner with `-count=1`. The runner copies each stored record and baseline there.
  - A `check.sh` step then validates those files against `record.schema.json` and `baseline.schema.json` with the pinned Python `jsonschema`. It fails unless their number equals the number of scripted judge-checked cases off the pending list, which it counts itself from the corpus and the list, never from the runner. It also fails on any record the judge rejected, since no corpus case leads to one (`CASES.md`).
  - The step sits outside the span `scripts/check-draft.py` reads.
- KTD16. **A run's bytes take one path, owned by `internal/grade`.**
  - **Order.** Build, mask (QE18), bound (QE10, using the contract writer's length function), serialise once, judge, then store those exact bytes; the scorecard renders from the stored record.
  - **Masking.** It applies to captured bytes before any head is cut, so a value that straddles the 64 KiB head edge or QE10's digest cut is masked. One §4 encoder serves masking and the leak scan.
  - **Interim fields.** Until U18, the stored record's `next_actions`, `offline_settings` and `bookkeeping_excluded` are empty lists, which no case compares (QE12).
  - **Escaping.** Every value from the subject or the subject file that reaches a terminal or standard stream, in the scorecard, in an error and in the exit funnel, passes one escaping function, whether or not the stream is a terminal. It covers C0, DEL, C1, ESC, bidirectional controls, other format characters, U+2028, U+2029 and the tag characters. Masking runs before escaping, and no subject-file value is quoted in a rejection line's cause.
- KTD17. **Every panic and every interruption ends in the one exit funnel.** The grade and anchor commands recover a panic on the main goroutine, and every goroutine the engine starts recovers its own, as an engine defect, exit 70, because an unrecovered panic exits 2, the code for a rejected request (`CODING_STANDARDS.md`, Judge and evidence code). SIGINT, SIGTERM, SIGHUP and SIGQUIT end every live process group, then clean up, and the run exits as Open Questions settles. The funnel ends live groups (QH23) and runs QE20's cleanup on every exit.
- KTD18. **macOS evidence runs on 27.2 with automatic updates off (GKD9).**
  - **What a run records:** the commit, the version and build, the command and the result.
  - **What the pull request carries:** the summary and the log's SHA-256. A log attached in full first passes `scan.sh`'s home-path and secret rules and has the host name removed.
  - **When it is taken:** once per pull request, on its head after every unit lands.
  - **Repeats:** scenarios that depend on timing or access times, the three real-verifier cases among them, repeat 25 times and pass only 25 of 25, as the spike's did. A repeat that is not evaluated fails.
  - **At the close:** the full evidence suite and the self-test checks run again on the exact commit phase 4 will use.
- KTD19. **Coding agents never run macOS tests, and nothing runs them by default.** Codex runs inside a macOS sandbox, where every confined run exits 71 (QH6). Every test that starts `sandbox-exec` sits behind one opt-in tag, `macos_evidence`, so `go test ./...` and `gate.sh` on a Mac never start one; an evidence run sets the tag and fails if any such test skipped. A darwin unit's brief has the agent write the code and the Linux-runnable tests only; the main session runs the tagged tests with the tool sandbox disabled, or the owner runs them, and a brief that meets exit 71 reports it as an environment fault.
- KTD20. **Unconfined handling of subject-made files never follows a link.**
  - **How the walk works.** Cleanup, inspection (QH25) and scanning open each throwaway directory with `os.OpenRoot` and walk it with `Root.Lstat`. Before descending, they set each directory to 0700 with `Root.Chmod`, which refuses to resolve a link out of the root. They remove each child entry with `Root.RemoveAll`, then remove the directory itself from its parent's root, and never open a special file.
  - **Regular files.** Before a scan or a digest, a regular file in a used home may have its mode changed the same way, since the profile kills `file-link` and `file-clone` and no file in a home can share an inode with one outside it. An entry that still cannot be read never ends as held or unchanged (Open Questions).
  - **Darwin flags and ACLs.** On darwin, darwin files first clear user flags and access-control entries, which the subject's `file-write*` can set.
  - **When cleanup fails.** It is reported, never silent (Open Questions).
  - **Where it is tested.** The walk is untagged and tested in CI; the flag and access-control half is tested on macOS (GR16).
- KTD21. **Until the darwin track lands, the released real path never starts a subject or a computation.** From the end-to-end slice, a released `grade` or `anchor` gives every subject-running and computation-running check `network_unverifiable` (QO1) and still reaches the judge, as it must on any system QH1 does not name. Slice d replaces this with the real verifier once the self-test (U29) and the computation profile (U31) have landed.

### Sequencing

```mermaid
flowchart TB
  P[PR1 plan and errata] --> J[PR2 strict JSON leaf]
  P --> L[PR4 launcher spike]
  J --> S[PR3 end-to-end slice]
  S --> A[PR5 slice a]
  A --> B[PR6 slice b]
  B --> C[PR7 slice c]
  L --> D1[PR8 launcher, invoker, profile]
  S --> D1
  D1 --> D2[PR9 computations and read proof]
  D2 --> D3[PR10 self-test and necessity]
  D3 --> D4[PR11 form replay]
  C --> D4
  D4 --> D[PR12 slice d]
  C --> D
  D --> F[PR13 slice f]
  F --> E[PR14 slice e]
  O[Owner decision on E-3] --> E
  E --> Z[PR15 phase close]
```

### Assumptions

- The private half and its scoring harness are reachable on the owner's machine, as they were for phase 1, and the harness can score a collector build (Open Questions).
- A machine running macOS 27.2 with automatic updates off is available for each pull request that needs local evidence, and the main session can run commands there with the tool sandbox disabled (KTD19).

---

## Implementation Units

From the end-to-end slice to slice e, each pull request first gets a short design note. The note names its evidence rules, its predicted ratchet set and its controls, and is reviewed through the measurement lens in design mode; the build waits on that review. Each pull request then runs the peer-review gate on its diff. Each unit brief gives the exact `go test` command for its tests, with `-tags scripted` where they need it, written to fail first. It also names the test seams it injects: effective user id, system version, clock.

### PR1: plan and errata

Waits on: nothing.

### U1. Errata entries E-2 and E-3

- **Goal:** the owner's reading of case assembly, and the coverage finding behind GKD4, are on record beside the sealed folder.
- **Requirements:** GR6, GR12.
- **Dependencies:** none.
- **Files:** `docs/specs/conformance-collection-1.0-errata.md`.
- **Approach:** E-2 states both readings of QT6 and the owner's choice; E-3 lists AE16, AE19's anchoring parts, AE31 and AE33 and leaves the 1.1 decision to the owner.
- **Test expectation:** none -- a documentation note; the citation test checks its ids.
- **Verification:** the gate passes, and the note changes no case's record.

### PR2: the strict JSON leaf

Waits on: PR1, merge order only.

### U3. The strict JSON leaf, exact equality and the judge's tables

- **Goal:** every input read under the judge contract's §3 uses one grammar, QA9 compares exactly, the writer is total over valid values, and the judge's scope and applicability rules are exported once.
- **Requirements:** GR6.
- **Dependencies:** none.
- **Files:** `internal/strictjson/` (new), `internal/judge/json.go`, `internal/judge/statistics.go`, `internal/judge/requests.go`, `internal/judge/results.go`, their tests.
- **Approach:** KTD3.
- **Test scenarios:**
  - a document over its size bound, with a repeated key, a lone surrogate, too deep, or with trailing content is refused exactly as `pql judge` refuses it, and a caller's own bound applies (QA6's 1 MiB);
  - `TestUnusedValuesMemory` still holds;
  - `9007199254740993` and `9007199254740992` compare unequal, and `1`, `1.0` and `1e0` equal;
  - a 21-digit integer and a 32-digit one serialise without a panic and read back equal;
  - an anchor whose baseline holds an out-of-scope prompt entry with `k: -0` writes `0`, and a writer mutated to pass numbers through turns the test red;
  - `ScopeIncludes` and `AlwaysApplicable` match the rulebook's §6 text, written as literals.
- **Verification:**
  - the `cases` step and the judge's tests pass with no judge output changed;
  - phase 1's private judge scoring and seeded-mistake catalogue are re-run on this build. They show zero wrong outputs and no surviving mistake, and report how many mistakes were evaluated out of the total;
  - a mistake that can no longer be placed at its site blocks the pull request unless the owner waives it by id;
  - the pull request reports counts only.

### U22. RFC 8785 canonicalisation

- **Goal:** one RFC 8785 implementation serves check ids, the seed digest and QS11.
- **Requirements:** GR1.
- **Dependencies:** U3.
- **Files:** `internal/jcs/` (new), its tests and checked-in vectors.
- **Approach:** KTD4.
- **Test scenarios:** key order by UTF-16 code units; number spelling; `\b` and `\f` escapes; each against the checked-in vectors.
- **Verification:** QE2's four worked ids are reproduced.

### PR3: thin end-to-end slice

Waits on: PR2, needed before this group can start. Each stage is only as wide as `case-003`, `case-004` and `case-129` need.

### U2. The command line and the tagged gate

- **Goal:** `grade` and `anchor` exist with the command line GKD8 fixes, the scripted build adds only the QT1 test entry, and the gate checks tagged and darwin code.
- **Requirements:** GR7.
- **Dependencies:** none.
- **Files:** `cmd/pql/main.go`, `cmd/pql/hidden.go` (new, the hidden-mode dispatch and its exit-64 path), `cmd/pql/main_test.go`, `gate.sh`, `internal/importguard/importguard_test.go`, the checked-in darwin dependency and initialisation lists.
- **Approach:** GKD8, KTD12 and KTD14.1-2; scripted files behind `//go:build scripted`, released-only tests behind `!scripted`.
- **Test scenarios:**
  - a released `grade` or `anchor` given any of `--verifier scripted`, `--host` or `--behaviour` exits 2, and an unknown `--verifier` value exits 64 (QT1);
  - a malformed command line exits 64 even with a scripted flag present (GKD8);
  - a scripted build given one or two of the three exits 2;
  - an unknown hidden-mode argument exits 64;
  - a planted unvetted line in a tagged file and in a darwin-only file, a cgo file under darwin and a Cobra import in a darwin file each fail the gate in Linux CI; the `x/sys` rule fires on synthetic `go list` output;
  - a changed darwin dependency list and a changed initialisation list each fail the gate;
  - the released binary holds no scripted-only or test-only symbol.
- **Verification:** the gate's tagged and darwin passes run within its limit, and each planted defect turns them red.

### U34. Case assembly for tests

- **Goal:** one test helper assembles a case's directory as QT6 and E-2 say, for the record tests and the corpus runner alike.
- **Requirements:** GR1.
- **Dependencies:** none.
- **Files:** a test-only package behind the `scripted` tag.
- **Approach:** QT6's entries, including `seed/`, `computations/`, `bin/subject` and `case-112`'s fixture; the run's home and working directory outside the directory (KTD2.2).
- **Test scenarios:** an assembled directory holds exactly QT6's entries; a case file never appears in it.
- **Verification:** every case assembles.

### U4. Subject file model and parse

- **Goal:** `grade` reads a subject file into the model the required set needs.
- **Requirements:** GR1.
- **Dependencies:** U3.
- **Files:** `internal/subject/` (new), its tests.
- **Approach:** the shape of `schema/subject-file.schema.json` by hand on the shared grammar (KTD3, KTD15). Every violation the schema can detect names its rule: a version that is not `1.0` names QS1, a named rule takes precedence over QS2 where the schema also rejects the file (QT6), and the `examples/invalid/` files name theirs. Rules a schema cannot detect wait for U9.
- **Test scenarios:** every judge-checked case's subject file parses; `case-100` is refused as QS2 says and `case-129` as QS1 says; each `examples/invalid/` subject-file example is refused naming its rule.
- **Verification:** the model holds every field the required set needs.

### U5. Preimages and check ids

- **Goal:** the engine builds a check's preimage and id as QE2 says, for the slice's cases.
- **Requirements:** GR1.
- **Dependencies:** U4, U22, U34.
- **Files:** `internal/record/` (new), its tests.
- **Approach:** the preimage kinds `case-003` and `case-004` need, the seed digest and the `files` digests over the assembled directory, the bare binary's defaults, and QS7's offline arguments in preimages; the rest come with U39.
- **Test scenarios:** QE2's worked ids; for `case-003` and `case-004`, each preimage equals `preimages.json`.
- **Verification:** `go test -tags scripted ./internal/record` passes.

### U33. The required set for the slice's cases

- **Goal:** the engine fixes the required set and its entries as QE3 and QE7 say, as far as `case-003` and `case-004` need.
- **Requirements:** GR1.
- **Dependencies:** U5.
- **Files:** `internal/record/`, its tests.
- **Approach:** QP1's probes and their placement, the declared commands those cases carry, and scope through `ScopeIncludes`; the other required-set rules come with U39.
- **Test scenarios:** for `case-003` and `case-004`, the required set equals `input.json`'s `required`.
- **Verification:** `go test -tags scripted ./internal/record` passes.

### U23. The seam, the driver and the resolver

- **Goal:** KTD13's seam and driver and KTD11's resolver exist before any collector does.
- **Requirements:** GR1, GR6.
- **Dependencies:** U33.
- **Files:** `internal/observe/` (new), `internal/collect/` (new), `internal/record/`, their tests.
- **Approach:** the raw-fact and `observe` types; the `Invoker` interface and the untagged driver; the resolver's steps in the order of `collection-rules.md` §2, with QO1 and the QS5 override filled in and every other step built as KTD11 says for an unbuilt step; KTD13's guard and projection test.
- **Test scenarios:**
  - each `order:` pairing the corpus tags, from fixtures written from §2;
  - a tagged or darwin file outside the scripted reader that names an `observe` or `record` outcome, reason or attribution value fails the guard;
  - an unbuilt step gives `harness_fault` when its condition is in the raw facts and passes the check on when it is not, never a pass.
- **Verification:** with an injected failed self-test fact, `case-004`'s 29 checks resolve to `network_unverifiable` through the shared path.

### U6. The scripted invoker

- **Goal:** a scripted run turns its host and recording into raw facts (QT3 to QT5).
- **Requirements:** GR7.
- **Dependencies:** U3, U23.
- **Files:** `internal/collect/` (scripted files behind the tag), its tests.
- **Approach:**
  - hand shapes for the `host` subschema and `schema/behaviour.schema.json` (KTD15);
  - QT4's attribution and QT5's agreement mapped into raw facts;
  - KTD2.5's trace;
  - a recording that lacks an invocation the record needs is an engine fault (exit 70).
- **Test scenarios:**
  - every corpus host and recording reads, and the projection test holds for each;
  - each `examples/invalid/` host and behaviour example exits 2, and so does a host or recording that is unreadable, refused under §3, or breaks QT4 or QT5;
  - a missing invocation exits 70.
- **Verification:** `case-004`'s host and every scripted case's recording load.

### U17. The durable store and the workspace

- **Goal:** every run's record is stored as QE17 says, and the per-run workspace is made and published safely.
- **Requirements:** GR3, GR4.
- **Dependencies:** U3.
- **Files:** `internal/store/` (new), its tests.
- **Approach:**
  - KTD5 and KTD16; QE17's modes and R27; a rejected record kept (QE15); QE20's workspace and sweep, with KTD20's walk added in U40;
  - the stored record's own fields, as `record.schema.json` and QS4 give them: `record_version`, `spec_version`, `host` (KTD8), `subject_file_path`, `subject_file_digest`, `binary_digest`, `seed_digest`, `request`, `scope`, `exit_code`, `judge_input`, `judge_output` and `judge_rejected`.
- **Test scenarios:**
  - file and directory modes; an existing store directory with a wrong mode;
  - a stale run directory swept, and a link, a foreign directory and a directory without its published process-id file left alone;
  - a test that would resolve the developer's real store fails;
  - a store write that fails after the judge has decided exits as Open Questions settles.
- **Verification:** every run the corpus runner makes that reaches a record leaves a stored record under that run's home.

### U7. Record builder

- **Goal:** the engine builds the judge input from the resolver's results.
- **Requirements:** GR3.
- **Dependencies:** U23.
- **Files:** `internal/record/`, its tests.
- **Approach:**
  - QE7 to QE10: required entries; evidence; targets in QE8's order with errata E-1; tiers with KTD10; QE10's bounds through the length function; an unfittable record ends the run with exit 70.
  - The top-level fields `request`, `scope`, `claimed_tier`, `subject_facts`, `markers`, `baseline`, `retired`, `accepted_regressions` and `trials`, as QE9, QE11 and QS4 give them.
- **Test scenarios:** each target kind; QE10's bounds at their limits; an unfittable record exits 70; each top-level field for `case-004`.
- **Verification:** `case-004`'s judge input matches `input.json` under KTD2.3.

### U25. The grade pipeline, pre-check, judge call and exit code

- **Goal:** `internal/grade` runs the pre-check, collects, calls the judge, stores and exits with QE16's code, through one exit funnel.
- **Requirements:** GR3, GR4.
- **Dependencies:** U2, U6, U7, U17.
- **Files:** `internal/grade/` (new), `cmd/pql/main.go`, their tests.
- **Approach:**
  - QE5 and QE6 with KTD9, and KTD16's path;
  - the judge called through `judge.Safe(judge.Judge, …)` for the pre-check and the final call; QE14 and QE16;
  - QH3's root refusal before the store or the sweep;
  - KTD17, and KTD21's released real path;
  - GKD8's rejection line, with a pre-check rejection naming QE6 and the judge's rule in its cause;
  - KTD16's escaping in the exit funnel.
- **Test scenarios:**
  - a rejected record exits 70 and is stored (QE15), shown by a record the test makes the judge reject;
  - a pre-check rejection the user caused exits 2 and one the engine caused exits 70 (QE6);
  - running as root exits 70 before any store is resolved;
  - a panic on the main goroutine and one in a collector goroutine each exit 70;
  - a released `grade` on any system starts no subject or computation process (KTD21), shown by a sentinel `bin/subject` that writes a file when run, with a positive control that runs the sentinel directly and sees the file;
  - an error carrying a control sequence from a subject file reaches standard error escaped.
- **Verification:** `case-003` and `case-004` pass through `grade`.

### U8. The corpus runner

- **Goal:** every corpus case runs through the scripted `grade` or `anchor`, with KTD2's ratchet and guards.
- **Requirements:** GR1, GR2.
- **Dependencies:** U2, U25, U34.
- **Files:** a new test package behind the `scripted` tag, its pending-case list, `check.sh` (KTD15's validation step and the growth check), `.github/workflows/ci.yml` (full history and the base commit for the `cases` job).
- **Approach:** KTD2, KTD5 and KTD15.
- **Test scenarios:**
  - **The ratchet:** a case off the list that fails, and a case on it that passes, each fail the runner. A list with one entry added or swapped fails the growth check, and so do a removed list, a missing base in CI, and a list absent at a base that has a runner. A list absent at a base with no runner passes.
  - **Planted mutations** are each caught: one per QE19 field (`outcome`, `reason`, `failure_kind`, `target_size`, `precondition_observed`, `tier`, a target's kind, id and tier, the `required` array), plus an extra and a missing evidence piece.
  - **Allowed variations** each still pass: shuffled evidence order, different `expected` and `observed` content (QE19), and permuted top-level input lists.
  - **Runner failures:** each of these fails the runner:
    - an empty case directory, a deleted stored record and a renamed field;
    - a wrong rejection rule;
    - a derivation-only case whose trace shows a requested invocation, and one whose trace shows a home made.
  - **Validation step:** fewer records than its own count fails it.
  - two cases run back to back never share a store.
- **Verification:** `case-129`, `case-003` and `case-004` pass, with the other cases the slice's design note predicts. Every judge-checked case reaches a stored record, and the runner runs in the gate and in `check.sh`.

### PR4: the launcher spike

Waits on: PR1, merge order only. Runs beside PR2 and PR3. It merges the spike's record and evidence only; its code is the reference U15 builds on, and no darwin code reaches `main` before KTD12's darwin passes do.

### U21. The launcher spike on macOS

- **Goal:** before any slice builds on it, a Go launcher in `pql` is shown to start under the unchanged QH11 template, and Go self-test programs are shown able to witness their operations.
- **Requirements:** GR11, GR14, GR15.
- **Dependencies:** none.
- **Files:** `docs/spikes/` (the spike's record); the spike's code on its branch, which the record names by commit.
- **Approach:**
  - **Time box:** four days.
  - **What it builds:**
    - KTD14's launcher mode and markers, with QH11's template as fixed text;
    - the private copy once per grade (`docs/solutions/test-failures/fresh-executable-per-run-stalls-parallel-sandbox-suite.md`) and kqueue's end (QH21);
    - one clean program judged by its output (`docs/solutions/best-practices/clean-control-must-assert-on-output.md`), one violator and the not-executable form;
    - the known-fast control, run after the self-test so a timed run is never the copy's first launch, with the first launch's latency measured;
    - the one-grant necessity check for UDP send, TCP connect and a child start.
  - **If the launcher needs an allowance the template lacks:** first rebuild it with a minimal import graph to tell the runtime's needs from `pql`'s imports. If it still needs one, the stop condition fires and the owner is told the same day.
- **Test scenarios:** the clean program's output equals its known value; the violator is killed by the confinement; the not-executable form gives both markers and reads as could not start; each one-grant profile fails the self-test while its granted program succeeds; the marker differs between invocations.
- **Verification:** the spike passes on macOS 27.2, stated in the pull request as KTD18 says, with its first-launch latency.

### PR5: slice a, the subject file and placeholders

Waits on: PR3, merge order only.

### U39. The whole required set and every preimage

- **Goal:** the engine fixes every check's preimage, id and required entry for every judge-checked case (QE2, QE3, QE7).
- **Requirements:** GR1.
- **Dependencies:** U33.
- **Files:** `internal/record/`, its tests, the corpus runner's `required` layer (KTD2.6).
- **Approach:** every preimage kind; QS5's candidate checks; QA7's exclusion of destructive runbooks outside that scope; QA3's crediting with `credit-and-routes.md` §2's credit conditions; QA9's `empty_satisfiable` and `asserts_emptiness`.
- **Test scenarios:** for every judge-checked case, each preimage equals `preimages.json` and the required set equals `input.json`'s `required`.
- **Verification:** `go test -tags scripted ./internal/record` passes on all 121 cases, and the runner's `required` layer is on.

### U9. Subject file rules

- **Goal:** every rule of QS1 to QS12 rejects what it must before anything runs, and names itself.
- **Requirements:** GR8.
- **Dependencies:** U4, U22, U25.
- **Files:** `internal/subject/`, its tests.
- **Approach:**
  - QS1 to QS12, with QS11's size and depth checked on each parsed literal with `internal/jcs`;
  - QS7's names with QH12's, QP7's and QP10's rejections, QA4's capture names, and QA9's type and path rules;
  - QA5's warning on the terminal, in the help of `grade` and `anchor`, and in the README.
- **Test scenarios:**
  - one fixture per rejecting rule, from the specification's text. Each exits 2 with GKD8's line naming that rule and quotes no subject-file value in the cause;
  - QS8's containment and QS9's seed-link rejection;
  - a literal near QS11's 64 KiB in RFC 8785 form, on each side;
  - the QA5 warning's text.
- **Verification:** the derivation-only cases still pending leave the pending list, each rejected by the rule its case names.

### U10. Placeholders and `pql create`

- **Goal:** `pql create` writes a subject file with a placeholder for every value it cannot know, and each placeholder gives the outcomes QS5 states.
- **Requirements:** GR8.
- **Dependencies:** U9.
- **Files:** `cmd/pql/main.go`, `internal/subject/`, their tests.
- **Approach:** QS5's skip and nothing-runs rules as resolver steps. What `create` takes as input is not specified (Open Questions); the slice's design note fixes the smallest input QS5 needs before the brief is written.
- **Test scenarios:**
  - through the scripted build with a passing host and a recording, `create` then `grade` on its output gives `placeholder_unfilled` exactly where QS5 says, the expected side written from QS5, never from `create`'s code (AE20);
  - each kind of placeholder gives QS5's outcomes in a unit test where its corpus case needs a later slice.
- **Verification:** `case-060` leaves the pending list. `case-061` to `case-066` carry runbooks and pass from U27.

### PR6: slice b, discovery, probes and unconfined safety

Waits on: PR5, merge order only.

### U11. Command discovery

- **Goal:** the discovery check walks help as QD1 to QD6 say.
- **Requirements:** GR9.
- **Dependencies:** U23.
- **Files:** `internal/collect/`, its tests.
- **Approach:** QD2's walk in the untagged driver, its bounds and order; QD3 and QD4; QD5; `credit-and-routes.md` §5's headings; QO7's combination for discovery.
- **Test scenarios:** each framework's headings; the depth and cycle bounds; a missing declared command (AE27); help on stderr; a hidden command.
- **Verification:** unit tests cover QD1 to QD6. Every discovery case carries runbooks, so those cases pass from U27.

### U12. The bare, byte-bound and exit-code probes

- **Goal:** these probes collect as QP1 to QP5 say, and the resolver's copy and attribution steps exist for the scripted path.
- **Requirements:** GR9.
- **Dependencies:** U23.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:**
  - QP1 to QP5 in the untagged driver, with QS7's appended arguments and environment;
  - QO2 for a binary that could not start, and QO5's exit-code rules;
  - QE8's targets with errata E-1, and QE10's stream bounds;
  - the resolver's QH8 step (a changed private copy gives `harness_fault`), and QO3 with KTD13's attribution.
- **Test scenarios:** a 17 MB stream, invalid UTF-8 and a 33-digit number; a changed copy; a kill by the confinement, a budget kill and a crash told apart from raw facts.
- **Verification:** `case-005`, `case-095` and `case-096` leave the pending list, with the others the slice's design note predicts; `case-092` also needs a computation and waits for U27. Those that time out or end by a signal wait for U27 and U14.

### U36. The fresh-state and store-unchanged probes

- **Goal:** these probes collect as QP6 to QP9 say, and QH25's inspection reports what changed.
- **Requirements:** GR9, GR16.
- **Dependencies:** U12, U40.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:** QP6 to QP9, with QP9's control copy and QP8's side files; QH25's inspection through KTD20's walk.
- **Test scenarios:**
  - a store-unchanged check that runs no command has no artifact target (E-1);
  - QP7's and QP8's exclusions;
  - a link or FIFO the subject made is reported as a change without being followed or opened;
  - a mode-0000 file in the store-unchanged comparison is read, or never ends as unchanged.
- **Verification:** `case-012` to `case-015`, `case-074` and `case-088` leave the pending list, with the others the slice's design note predicts.

### U37. The planted-secret probe

- **Goal:** the planted-secret probe collects as QP10 to QP13 say.
- **Requirements:** GR9, GR16.
- **Dependencies:** U12, U38, U40.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:** QP10's per-invocation canary planted by the driver; QP11 to QP13 and QO7's combination with the read proof's facts.
- **Test scenarios:** each QP11 to QP13 outcome from raw facts; a mode-0000 file holding the canary is found, or never ends as held; a scan mutated to skip unreadable files turns that test red.
- **Verification:** `case-151` leaves the pending list, with the others the slice's design note predicts.

### U38. The leak scan

- **Goal:** one §4 encoder finds a canary or declared value in every encoding `credit-and-routes.md` §4 lists.
- **Requirements:** GR9.
- **Dependencies:** U23.
- **Files:** `internal/record/`, its tests.
- **Approach:** the §4 table, shared with U24's masking (KTD16).
- **Test scenarios:** one test per §4 encoding, written from the table.
- **Verification:** unit tests cover every row of §4.

### U24. Masking

- **Goal:** the store, the streams and every scorecard carry no canary or declared value QE18 masks.
- **Requirements:** GR3.
- **Dependencies:** U7, U38.
- **Files:** `internal/record/`, `internal/grade/`, their tests.
- **Approach:**
  - QE18 with KTD16's order and one §4 encoder;
  - a process-wide masking writer over standard output and error that knows each invocation's canary and the declared values, and masks across write boundaries;
  - the planted file and QA4's capture files are the only places a plain value may sit, and QE20 removes them.
- **Test scenarios:**
  - a fresh random canary and a declared value of 8 bytes or more are injected at the seam: into stream heads, home-change paths, error text and a computation's standard error, one copy straddling the 64 KiB head edge and one QE10's digest cut;
  - each injected value is shown present before masking, and absent after from the store, the streams and the scorecard, in each §4 encoding;
  - a declared value holding a character the escaper rewrites is masked;
  - the per-run directory holding the planted file is gone after the run.
- **Verification:** the test finds no unmasked value.

### U40. Unconfined safety: cleanup, sweep and interrupted runs

- **Goal:** cleanup, inspection and the sweep act only inside the throwaway directories, and a run that dies leaves no subject running unwatched (GR16).
- **Requirements:** GR16.
- **Dependencies:** U17, U25.
- **Files:** `internal/store/`, `internal/collect/` (darwin files for flags and access-control entries), `internal/grade/`, their tests.
- **Approach:** KTD20; KTD5's recorded process groups and the sweep that ends them; KTD17's signals.
- **Test scenarios:**
  - a link inside a home to an outside directory of mode 0555 holding a file leaves that target unchanged, and replacing the no-follow change with `os.Chmod(path)` turns the test red;
  - a home swapped for a link mid-cleanup deletes nothing outside it;
  - a 0000 directory swapped for a link between its `Lstat` and its mode change leaves the outside directory's mode unchanged;
  - a mode-0000 directory and a FIFO inside a home are removed without the FIFO being opened;
  - a cleanup that cannot remove an entry is reported as Open Questions settles;
  - SIGINT, SIGTERM, SIGHUP and SIGQUIT each end the live group and clean up;
  - a sweep after a killed `pql` ends the recorded group before removing its home;
  - on macOS, behind KTD19's tag: files and directories with `uchg`, `uappnd` and a deny-delete access-control entry are removed.
- **Verification:** the Linux tests pass in CI, and the macOS tests pass on 27.2 as KTD18 says.

### PR7: slice c, authored checks

Waits on: PR6, merge order only.

### U13. Predicates

- **Goal:** each of QA9's ten predicates decides as QA9 says.
- **Requirements:** GR10.
- **Dependencies:** U12.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:** QA1, QA2 and QA9, comparing with `internal/strictjson`'s exact equality.
- **Test scenarios:** each predicate and its type rules; `stdout_json_equals` on two integers that round to one double; invalid UTF-8 and a big number in predicates.
- **Verification:** unit tests cover every predicate; the corpus cases pass from U27.

### U26. Computations

- **Goal:** computed expected values are produced and checked as QA4, QA6 and QO6 say.
- **Requirements:** GR10.
- **Dependencies:** U13.
- **Files:** `internal/collect/`, its tests.
- **Approach:** QA4's substitutions (`{SEED}`, `{SUBJECT_DIR}`, `{CAPTURE:<step>}`), capture files in the scratch directory, and the scratch directory as working directory and home; QA6 with its 1 MiB bound; QO6; a cut capture is never run (QE10).
- **Test scenarios:** each QA6 failure mode; each substitution; a computation that pipes its input; one that runs the subject.
- **Verification:** unit tests cover QA4, QA6 and QO6; the corpus cases pass from U27 (AE23).

### U27. Runbooks, budgets and markers

- **Goal:** ordered runbooks with their budgets and markers collect as QA7, QO4 and QE11 say.
- **Requirements:** GR2, GR10.
- **Dependencies:** U26.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:** QA7's shared home in the driver, which records where a runbook stopped; the resolver's `earlier_step_failed` (KTD11); QO4 with QH24's known-fast control from the host; QA8; QE11's markers.
- **Test scenarios:**
  - a runbook's second step failing, and every later step `earlier_step_failed` (AE22);
  - a step over budget, and a known-fast control over budget giving `harness_fault`;
  - a known defect's marker giving exit 3.
- **Verification:** the cases with runbooks leave the pending list, including `case-001` green with exit 0 (GR2, AE8), the discovery cases and `case-061` to `case-066`.

### PR8: darwin track, launcher, invoker and profile

Waits on: PR3 and PR4, needed before this group can start.

### U15. Launcher and private copies

- **Goal:** U21's launcher becomes the released build's, with the private copy and both markers as QH4 to QH8 say.
- **Requirements:** GR11, GR15.
- **Dependencies:** U21, U23.
- **Files:** `cmd/pql/hidden.go`, `internal/collect/` (darwin files and untagged marker parsing), `go.mod`, `go.sum`, their tests.
- **Approach:** KTD14; QH4 to QH8; the copy made once per grade in a randomly named folder and checked before each use, never silently remade; the launcher's fixed argument layout; `golang.org/x/sys` added to `go.mod`.
- **Test scenarios:**
  - on Linux: a forged marker, a could-not-start line without the per-invocation value, and a could-not-start line not directly after the marker are each not honoured;
  - on macOS, behind KTD19's tag:
    - a subject that execs the launcher copy with its own marker and a missing program does not read as could not start;
    - subject words such as `--`, `-D` and the mode token pass through unchanged;
    - a copy changed between uses fails its invocations.
- **Verification:** the Linux tests pass in CI and the macOS tests on 27.2.

### U30. The darwin invoker: spawn, ends and draining

- **Goal:** the darwin invoker returns each invocation's raw end and streams, so every later macOS test runs on the path that grades the subject.
- **Requirements:** GR11, GR15.
- **Dependencies:** U15.
- **Files:** `internal/collect/` (darwin files), their tests.
- **Approach:** spawning under `/usr/bin/sandbox-exec` by absolute path; kqueue's exit detail; the budget timer and whether the program was still running; the process group's end (QH23); non-blocking draining with the byte count past 1 MiB and the cut flag (QE10). Attribution stays KTD13's untagged function.
- **Test scenarios,** behind KTD19's tag: a kill by the confinement, a budget kill and a crash give distinct raw facts; the `huge-output` form drained and cut.
- **Verification:** these tests pass on macOS 27.2.

### U28. Profile, environment and descriptors

- **Goal:** the subject and each computation run with exactly QH10 to QH15's profile, environment, descriptors and home.
- **Requirements:** GR11, GR15.
- **Dependencies:** U30.
- **Files:** `internal/collect/` (darwin files and untagged builders), their tests.
- **Approach:** QH10 to QH15 with KTD7's closed variants; paths only as `-D` parameters, canonical with every link resolved (`docs/solutions/best-practices/sandbox-profile-later-rule-decides.md`); an argument list too long for the system an engine fault, never a fallback.
- **Test scenarios:**
  - the template and its blocks, extracted from the code fences of `confinement.md`, equal the engine's fixed text byte for byte for both variants;
  - the lines added per invocation (`PLANTED_DIR_n`, QH27's control line, `DECLARED_n`, `SEED_n`) in count and order, and paths under a symlinked temporary root made canonical;
  - seed, declared and secret names holding quotes, parentheses and profile syntax leave the profile bytes unchanged and appear only as `-D NAME=value`;
  - with sentinel variables (`PQL_SENTINEL`, `DYLD_X`, `XDG_CONFIG_HOME`) in the engine's environment, the built environment equals QH12's table plus the declared values, in order; stdin is `/dev/null`; a new session is set; `__CF_USER_TEXT_ENCODING` is the uid in uppercase hex;
  - on macOS, behind KTD19's tag: with a descriptor opened without close-on-exec and sentinels in `pql`'s own environment, a probe program under the real launcher sees neither, and a knock-out run with the closing disabled fails.
- **Verification:** the builder tests pass in CI and the probe passes on macOS 27.2.

### PR9: darwin track, computations and the read proof

Waits on: PR8, needed before this group can start.

### U31. Computation confinement

- **Goal:** each computation runs under its own profile as QH16 to QH20 say.
- **Requirements:** GR11, GR15.
- **Dependencies:** U28.
- **Files:** `internal/collect/` (darwin files), their tests.
- **Approach:** QH16 to QH20's computation variant, tested with a fixture scratch directory as working directory and home; QA4's substitutions join it in slice d (U35).
- **Test scenarios,** behind KTD19's tag:
  - a computation's child runs, and its `setsid` or `setpgid` is killed (QH18);
  - a computation that runs the subject;
  - a QH19 survivor cannot reach a later computation's sandbox, which is the plan's reading. If the run shows otherwise, the finding goes to the owner (GR6) rather than being built around.
  - Survivors outliving the grade are QH19's stated limit, named in the pull request.
- **Verification:** these tests pass on macOS 27.2.

### U41. The read proof's raw facts

- **Goal:** QH26 to QH28's two signals are observed as raw facts.
- **Requirements:** GR11, GR15.
- **Dependencies:** U30.
- **Files:** `internal/collect/` (darwin files), their tests.
- **Approach:** the planted file's access time and inode before and after, and the control's end, as raw facts; the decision stays U37's.
- **Test scenarios,** behind KTD19's tag: an access-time forgery and a real read each give the raw facts the proof names.
- **Verification:** these tests pass on macOS 27.2.

### PR10: darwin track, self-test and necessity

Waits on: PR9, needed before this group can start.

### U29. Self-test, calibration and the known-fast control

- **Goal:** the self-test programs run as QH9 says, for both profiles, with the calibration and the known-fast control.
- **Requirements:** GR11, GR15.
- **Dependencies:** U30, U31, U41.
- **Files:** `internal/collect/` (darwin files), their tests.
- **Approach:** QH9's programs for both profiles as hidden modes with raw calls (KTD14), clean programs judged by their output; QH24; QH26's calibration.
- **Test scenarios,** behind KTD19's tag: every self-test program, with a clean program whose wrong output fails the self-test; a failed self-test gives every subject check `network_unverifiable`.
- **Verification:** the self-test passes on macOS 27.2.

### U16. The necessity and allowance evidence

- **Goal:** a local macOS suite proves each self-test program's necessity and each allowance's role, with no vacuous pass.
- **Requirements:** GR14, GR15.
- **Dependencies:** U29.
- **Files:** a darwin test behind KTD7's and KTD19's tags.
- **Approach:**
  1. For each self-test program, a profile granting only that program's operation makes the self-test fail, the granted program completes its operation as its output shows, and no other program's result changes (GR14).
  2. Each allowance removed in turn breaks the clean program's own operation at the step that allowance governs, judged by its output, never an incidental failure.
  3. A run that should start shows the launcher's marker and then the expected end; a run with no marker there is not evaluated.
  4. A run that should not start (`sandbox-exec` exiting before the marker, a profile it rejects, an allowance the launcher needs to start removed) shows no marker and the expected exit or kill, beside a control in the same session whose unmodified profile does write the marker (QH6).
  5. Every rule a scripted run cannot break has a test: each QH rule `CASES.md` lists as not expressible or not cited, and QE18, except stated limits such as QH19, which the pull request names.
  6. Any run not evaluated, and any repeat under KTD18 not evaluated, fails the suite.
  7. Commit 490d650's spike suite is reference only; nothing is copied without checking it against the sealed text.
- **Test scenarios:** each program's one-grant profile fails the self-test while the granted program succeeds; each allowance's removal breaks its own step; a run with no marker where one was due fails the suite.
- **Verification:** the suite passes on macOS 27.2 as KTD18 says.

### PR11: darwin track, the form replay

Waits on: PR10 and PR7, needed before this group can start, since the replay drives runbooks through the driver.

### U32. The form replay

- **Goal:** the real observation path is proved on defective forms as well as the known-good one.
- **Requirements:** GR15.
- **Dependencies:** U16, U27.
- **Files:** a darwin test behind KTD7's and KTD19's tags, with a decoy harness.
- **Approach:**
  1. **Build.** Each form is built and its source digest checked against `form_sha256`; the build is recorded beside the recordings' 27.2 build.
  2. **Decoys.** The decoys QT5 names are prepared in an owner-only directory checked with `Lstat`, restricted to the spike's decoy set. The signal decoy is an unreaped child of the harness, checked alive before its process id is written.
  3. **Context.** Each invocation's context comes from the cases whose `invocations` field names it: the subject file, the declared values and the captures. Where cases give one invocation different contexts, it is replayed once per context and each must agree.
  4. **Replay.** Each invocation runs five times through KTD13's driver with the darwin invoker, a runbook's steps in order in one shared home. Its facts go through the projection serialiser. Every field QT5's agreement clause lists, the planted file and each decoy's state included, is compared with the recording, after the canary is replaced in every encoding.
  5. **Positive controls.** A violating form's unconfined positive control runs with QH12's built environment and a throwaway home, both asserted, and must touch its decoy, or the form is not evaluated.
  6. **Coverage.** The pull request reports invocations replayed out of the recorded total. It lists those no case names, by count and name, and the owner decides whether that coverage stands.
- **Test scenarios:** a form whose observed invocation differs from its recording fails; a missing decoy, or a positive control that touched nothing, fails as not evaluated.
- **Verification:** every replayed invocation agrees with its recording on macOS 27.2. A disagreement fails until it is diagnosed; only one the owner rules a specification matter is waived, by name.

### PR12: slice d, safety and the no-network check

Waits on: PR7 and PR11, needed before this group can start.

### U14. Scope, safety classes and outcomes

- **Goal:** scope fixes which checks run, and the outcome rules decide each invocation's end.
- **Requirements:** GR11.
- **Dependencies:** U27.
- **Files:** `internal/collect/`, `internal/record/`, their tests.
- **Approach:** QS6, QA7, QA8 and QE3 for scope; QO1, QO3 and QO5's signal rules as resolver steps; QE12 and QE13.
- **Test scenarios:** a destructive command runs only with `destructive` in scope (AE30 with `case-001`); an unclassified command is never invoked (AE24).
- **Verification:** the scope and kill cases leave the pending list.

### U35. The real verifier in `grade`

- **Goal:** the released build's real verifier replaces KTD21's interim path.
- **Requirements:** GR11, GR15.
- **Dependencies:** U14, U29, U32.
- **Files:** `internal/collect/`, `internal/grade/`, their tests.
- **Approach:** the darwin invoker wired into `grade`; QA4's substitutions joined to U31's profile; QE9's tiers (KTD10); the system version read from the `kern.osproductversion` sysctl and matched exactly. On a system QH1 does not name, Linux included, every check that runs the subject is `network_unverifiable` and the run still reaches the judge (QH1).
- **Test scenarios:**
  - a released build on Linux, or with the version check reporting 27.1, 27.20, 27.2.1 or 27.0, grades `case-069`'s subject with every subject-running check `network_unverifiable` and exit 4, and a prefix-matching version check turns those tests red;
  - a sentinel `bin/subject`, with its positive control, shows the subject never started on those systems;
  - QE9's tiers on a record built from real-verifier observations.
- **Verification:** the violator cases leave the pending list, and `case-069` and `case-070` pass through the runner's macOS mode 25 times out of 25 (KTD6, KTD18).

### PR13: slice f, the scorecard

Waits on: PR12, merge order only.

### U18. The scorecard and the agreement check

- **Goal:** the scorecard renders the stored record, and the exit code, scorecard and stored record agree for every case off the pending list (AE21).
- **Requirements:** GR3, GR13.
- **Dependencies:** U25.
- **Files:** `internal/scorecard/` (new), the corpus runner, the README, their tests.
- **Approach:**
  - KTD16 and its escaping, and QE12's next actions per reason.
  - Everything the scorecard must show, each the naming half of an acceptance example:
    - QS5's placeholders by location (AE20);
    - QS7's offline settings and their commands, and QP7's bookkeeping exclusions;
    - QD2's bound when reached, QD3's undeclared commands and the command named (AE24), and QD4's counts and missing names (AE27);
    - the defect's check named (AE8), and a computation's stderr (QA4, AE23);
    - QE18's note for a declared value too short to mask;
    - QE21's anchor exit never shown as green, in the scorecard and the README.
  - The judge errata's display rules where conformance reaches them.
- **Test scenarios:** a judge output that recomputing would contradict is rendered as the judge wrote it; each display item has a fixture; each escaped character class has a fixture.
- **Verification:**
  - the agreement check passes for every case off the pending list;
  - a private scoring run on the build so far shows zero wrong records. It covers every private case that needs no baseline and no real verifier, a non-empty set the scoring party fixes before the run, and is reported as scored out of that set's total.

### PR14: slice e, baselines and anchoring

Waits on: PR13, merge order only; and the owner's decision on E-3 before it starts (Open Questions).

### U19. Baselines, comparison and anchoring

- **Goal:** `grade` compares against a baseline and `pql anchor` saves one, as QE4, QE5 and QE21 say.
- **Requirements:** GR12.
- **Dependencies:** U17, U18.
- **Files:** `cmd/pql/main.go`, `internal/store/`, `internal/record/`, `internal/grade/`, their tests.
- **Approach:** QE4's choice of baseline and each missing id's cause from its stored preimage; the missing-id list written to the durable store; the anchor's pre-check with KTD9; QE21; KTD5's atomic writes; and the cases a collection 1.1 adds, if the owner adopts one.
- **Test scenarios:** a named baseline file read in place without checking its path; the missing-id list's form and causes; an anchor without `--baseline` carrying QE4's latest stored baseline; a written baseline valid against `baseline.schema.json` (KTD15).
- **Verification:** `case-071`, `case-130` to `case-134` and `case-137` pass, U18's agreement check passes again with them, and `case-143` passes on macOS 27.2 in a home of its own.

### PR15: phase close

Waits on: PR14, merge order only.

### U20. Private scoring, final evidence and the phase status

- **Goal:** the build is scored privately, the confinement evidence is current, and the engine plan records phase 3 as done.
- **Requirements:** GR1, GR2, GR5, GR15.
- **Dependencies:** U19.
- **Files:** `docs/plans/2026-11-21-engine-plan.md`.
- **Approach:**
  - The private harness runs the 72 private cases and the 40 seeded mistakes by the procedure in Open Questions, and the pull request reports counts only: cases scored out of 72, and mistakes evaluated out of 40.
  - KTD18's full re-run covers U16, U32, U40's macOS tests, the self-test and the three real-verifier cases, on the commit phase 4 will use.
  - 27.0.1 is stated as unverified (GKD9).
- **Test expectation:** none -- a status edit; the scoring and the macOS run are its evidence.
- **Verification:**
  - zero wrong records and no surviving seeded mistake;
  - no private case unscored and no mistake not evaluated, unless the owner waived it by id;
  - the pending list empty, and the macOS evidence stated.

---

## Verification Contract

| Gate | What it proves |
|---|---|
| `./gate.sh` with its tagged and darwin passes | build, vet, staticcheck, errcheck, exhaustive and every test, the corpus runner included, on released and scripted code; build and lints on darwin code, cgo-free; the import guard on each system and tag set; the darwin dependency and initialisation lists; no scripted or test-only symbol in the released binary; network off; no `sandbox-exec` started |
| `./check.sh` | the gate, the binaries, the sealed kits, the judge's published cases, the corpus checks, the stored records' schema validation against an independent count, and the pending list's growth check, in the `cases` job |
| `./scan.sh` | no home path, email or secret in the diff, the commit messages or an attached log |
| the corpus runner | which cases pass, with KTD2's guards; each slice moves cases off the pending list and none back |
| mutation evidence per rule test | each new rule test fails when its rule breaks, through `mutation-verify.sh` with a knock-out, recorded in the review log (`CODING_STANDARDS.md`, Tests) |
| a local macOS 27.2 run with the `macos_evidence` tag | the launcher spike, the darwin track, U40's flag and access-control tests, the real-verifier cases and the evidence suites, run by the main session with the tool sandbox disabled, stated as KTD18 says |
| the peer-review gate per pull request | a design-mode measurement review of each slice's design note before its build; code and test lenses always; security for the confinement and anything public; the measurement lens on every evidence rule |
| one different-model review | this plan, before slice work starts (`AGENTS.md`, Review); done, recorded in `docs/agent-reviews/2026-11-24-phase3-plan-reviews.md` |
| private scoring | GR5, after slice f on a fixed subset and in full at the close; phase 1's judge scoring after U3 and after any later change under `internal/strictjson` |

---

## Definition of Done

- Every public case passes through `grade` or `anchor`: the scripted ones in CI with the pending list empty, and `case-069`, `case-070` and `case-143` on local macOS 27.2 (GR1).
- The known-good synthetic subject is green and exits zero (GR2).
- The private scoring shows zero wrong records and no surviving seeded mistake, with every case scored and every mistake evaluated or waived by id (GR5).
- Each pull request has passed the peer-review gate with every Blocker and Major resolved or accepted by the owner, and records its mutation evidence and any macOS evidence.
- The confinement evidence is re-run on the commit phase 4 will use, and 27.0.1's status is stated.
- The engine plan records phase 3 as done.
- No abandoned helpers, commented-out code or scratch files remain in any diff.

---

## Open Questions

**Blocking slice e**

- Does a collection specification 1.1 add cases for AE16, AE19's anchoring, AE31 and AE33 (errata E-3)? If it does, slice e waits for its seal and builds against it, and the close date moves. If not, slice e proves what 1.0 reaches, and the pull request names what stays unproved.

**Before the end-to-end slice's design note**

- When the store write fails after the judge has decided, does the engine exit 70 (QE16) or with the judge's code (QE14)? The plan's reading is 70, since the run did not complete; the owner confirms or corrects (GR6).
- QE15 keeps a rejected record, and `record.schema.json` nests the judge input's schema, so a record the judge rejected as schema-invalid cannot validate. The plan's reading is that QE15 wins and the stored record need not validate (KTD15). The owner confirms, and the reading goes in the errata (GR6).
- QE16's table names no interruption. The plan's reading is that a run ended by a signal is an engine fault, exit 70; the owner confirms (GR6).

**Before slice a (U9)**

- QS8 names "the user's home directory". The plan's reading is the account's home from the system's user record, not `$HOME`, since tests override `$HOME`; the owner confirms (GR6).
- When a user's file breaks two named rules, which rule does GKD8's line name at run time? The corpus question and a proposed deterministic order go to the owner before U9's brief is written (GR6).
- What does `pql create` take as input? The specification fixes only that it writes a placeholder for each value it cannot know (U10).

**Before slice b (U36, U37, U40)**

- An entry in a used home that stays unreadable after KTD20's mode change cannot be scanned or compared. The plan's reading is that the check is `not_observed` with `harness_fault`, by analogy with QP13; the owner confirms (GR6).
- A cleanup that cannot remove an entry breaks QE20. The plan's reading is an engine fault, exit 70, with a scorecard note naming the directory; the owner confirms (GR6).

**Before slice c (U26)**

- Is QA6's depth limit for computation output counted from the output, or from where the value sits inside a record? The plan's reading is inside the record, since QA6 says "inside a record"; the owner confirms (GR6).

**For the owner, no unit waits**

- Nothing in the specification bounds how much a subject or computation writes inside its home, so a subject can fill the volume within its budget. This is a candidate stated limit or a 1.1 rule; the engine does not work around it (GR6).
- `credit-and-routes.md` §4's base64 forms are of the canary alone. A canary inside a larger base64 value, at an offset not divisible by three, matches none of them. One example is a Basic authorisation header in a log. This gives a false green on the planted-secret check and leaves a declared value unmasked. It is a candidate stated limit or a 1.1 rule; the engine does not work around it (GR6).

**Before slice f's private scoring**

- Does the private harness score a collector build? The plan's procedure for the seeded mistakes:
  1. Each catalogued mistake becomes a mutation at its rule's site.
  2. The mutant must build and pass every lint, and produce exactly the catalogued wrong record on its private case under QE19's fields.
  3. CI must then fail through a test, and the failing tests are recorded by name. A mutant only a lint or the build catches is not evaluated.
  4. A party other than the session that briefs the next build runs it and returns rule ids only.
  5. A mistake that cannot be expressed this way is not evaluated and is reported by count with its rule id. It blocks GR5 unless the owner waives it by id.
