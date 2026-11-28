# Design note: PR15, the real producer and parity

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR15. **Units:** U41 and U32.

This note follows the slice d note, `docs/plans/2026-11-27-phase3-pr13-slice-d-design-note.md`, and the self-test track's build review, `docs/agent-reviews/2026-11-27-phase3-pr11-self-test-track-build-reviews.md`. Their definitions and controls hold here. Its design review is logged in `docs/agent-reviews/2026-11-28-phase3-pr15-design-note-reviews.md`.

## Delivery: two pull requests

Today nothing produces a real run. `ReleasedSource` refuses to observe (KTD21). No darwin `observe.Source` exists, and `collect` creates no home, plants no canary and runs neither of QP9's and QH27's controls, all of which KTD13 gives it. U32's parity replay cannot start until those exist. So the group lands as two pull requests, in this order:
- **PR15a, the real producer** (U41 and the loop work it needs). `collect` owns homes and controls, the darwin source runs and inspects, and U41's read proof is shown on macOS. The released build still refuses to grade: KTD21's switch moves only in PR16.
- **PR15b, parity** (U32). A darwin test replays the recorded invocations through `collect` with the darwin source, and compares each with its recording.

Each pull request has its own build review and its own macOS evidence. PR16 waits on both.

## PR15a: the real producer

### The loop owns homes and controls (KTD13)

Through `internal/throwaway`, before each `Source.Observe`, `collect` prepares the home the key names (QT5's `home`):
- `seed`: a fresh copy of the seed (QP5);
- `empty`: an empty home (discovery's);
- `planted`: a fresh seed copy with a fresh canary from `crypto/rand` (QP10, QH14). The access time is set a day before the modification time (QH27);
- `runbook`: the runbook's one shared home, created at its first step and kept to its last (QA7);
- `scratch`: a computation's scratch directory (QA4).

`observe.Spec` gains what the source needs to run in that home: the home's path, its kind, and for a planted home the planted file's path. The canary itself stays in `collect`, which hands the leak scan its encodings. The source never chooses or keeps one.

The scripted source ignores the new fields. A test asserts that the corpus trace is unchanged (control 3), and that every prediction file regenerates byte for byte.

**The two controls stay in `collect`:**
- **QP9's control copy.** For each invocation the store-unchanged probe makes, `collect` makes a second fresh copy of the same seed. It snapshots it, appends one byte to the declared store, and asks `throwaway.CompareHomes` whether the comparison reports the change. The answer is that run's `home_changes.control_detected` raw fact. With no store, or no regular file of one byte or more after the exclusions, nothing is appended, and QP9's `empty_target` decides, as `decide.storeUnchanged` already does.
- **QH27's read control.** When QO7's condition holds after the ordinary invocation, `collect` serves the same key again in a fresh planted home, with the same canary and `Spec.Control` set, so the profile gains QH11's extra line. The control run's end is the ordinary run's `planted.control_end`. U37 already decides from it.

### The darwin source (KTD13: it only spawns, waits, kills the group and reads files)

`internal/confine` gains a darwin `Source`. Its `Observe` does these things, in order:
1. **The private copy (QH8).** It checks the private copy of the binary, or of the computation's executable, against the bytes it was copied from before each use. That gives the `PrivateCopyUnchanged` raw fact.
2. **The run.** It renders QH11's profile for the key's variant through `confine.Arguments`, and spawns through `Copies.Start` and `Wait` (U15). It drains each stream, keeping QE10's byte count, digest, cut and head. The digest and the counts are of the drained bytes.
3. **The planted file,** read through a new `throwaway` function before anything else reads it: its access time against the value before the run, and its device and inode against the values before. Every later read of the planted file moves its access time. So the snapshot, the comparison and the leak scan below all come after this step, and the snapshot reads the planted file's content only after its facts are taken.
4. **The home's changes:** `throwaway.SnapshotHome` against the before-snapshot `collect` took when it prepared the home, then `throwaway.CompareHomes` (QH25, QP6, QP8, E-46, E-47).
5. **The leak scan:** `throwaway.ScanHome` and the drained streams, with the encodings `collect` passed (QP12, QP13).
6. **A computation's printed value:** QA6's parse, untagged (KTD13).

It fills `observe.Run` and its `Details` in QT5's run shape, and nothing else: no outcome, reason or attribution. KTD13's guard stays, and covers the new darwin files.

**Grade-wide controls.** The darwin source's `Controls()` produces these:
- `ConfinementApplied` and `SandboxExecExit`, from QH6's raw end of the first run;
- `SystemNamed`, from `kern.osproductversion` against QH1's two versions, 27.0.1 and 27.2, checked before any check runs (QH2);
- `SelfTestPassed`, `KnownFastWithin` and `ReadCalibrationMoved`, from `RunControlSuite`, which exists.

`StoreControlDetected` and `ReadControlKilled` are per run, from the controls above, never grade-wide.

### What stays out of PR15a

- **The released build still refuses to observe.** PR15a adds the darwin source and a `scripted`-free test constructor for it. `grade` does not select it until PR16. A gate test asserts that `ReleasedSource` still refuses, and that the released binary still holds no reference to the darwin source's constructor.
- **No subject is graded on this machine** outside the `macos_evidence` tests below.

### U41's macOS tests (KTD19: written by codex, run here)

Each is behind `darwin && macos_evidence`, runs with the tool sandbox off, and repeats 25 times, passing only 25 of 25 (KTD18: access times).

**A real read of the planted file:**
- the access time moved;
- the device and inode are unchanged;
- the control run, with QH11's extra line, is killed by the confinement after its marker (QH21 attributes it, E-37);
- the decision is held (QP11).

**An access-time forgery,** using the `forges-atime` form's operation: the subject sets the planted file's access time without reading it.
- With a write, a rename or a replace, the confinement kills it (QH27).
- With `utimes` and no read, the access time moved but the control is not killed, so no read is proved. The test asserts the raw facts, and that the decision is not held.

**A run that never reads,** the `never-reads-secret` form: the access time did not move. The test asserts this after the snapshot and the scan have run. That proves step 3's ordering: reversing it moves the access time and turns this test red.

**QP9's control copy, real:** one byte appended, the comparison detects it, `control_detected` is true. Its twin appends nothing, and must read false.

**Each test is paired with a control that the restriction it shows leaves clean runs alone,** and each asserts the exact raw facts it printed, never an exit status alone (`docs/solutions/best-practices/clean-control-must-assert-on-output.md`, `sandbox-evidence-must-prove-the-decided-operation.md`).

### Linux-runnable tests (written by codex, run in the gate)

- `collect`'s home sequencing on a fake source:
  - each key gets the home kind QT5 names;
  - a fresh home for every invocation except within a runbook;
  - a fresh canary for each planted home;
  - the control run served only when QO7's condition holds, with the same canary.

  Each rule has a twin.
- QP9's control on fixture trees: detected, not detected, no store, an empty store.
- The planted-facts ordering, with a fake clock on the throwaway reader: a snapshot taken before the facts makes the test fail.

## PR15b: parity (U32)

### What is replayed

There are 1,415 recorded invocations across 54 forms, and every case-named key exists in a recording. 1,206 are named by at least one case's `invocations` field, and 209 by none.

**Which contexts.** U32 step 3 replays an invocation "once per context" the cases give it. But QT6's `invocations` field also names invocations a case implies but never ran, and some case contexts cannot produce a real run: a placeholder seed or binary, or a host that forces a changed copy, no confinement or a failed self-test. A real run in such a context cannot match a recording made in the canonical context. So this note replays only the contexts the corpus's own scripted run actually served (the trace's `Served`).

The build checks first that this set covers every case-named key at least once. Any key it misses is listed in the pull request, beside the 209 that no case names.

**The 209 unnamed invocations** are replayed in their canonical context, the form's `subjects/<form>/` subject file, and reported separately. U32 step 7 leaves it to the owner whether that coverage stands. The pull request asks, through the owner question tool.

### How each invocation is replayed

1. **Build.** Each form is built with `formtest.Build`, and its source digest is checked against `form_sha256`. The machine's version and build are recorded beside the recordings' 27.2 (26B123).
2. **Decoys.** The harness prepares the decoys QT5 names, rebuilt from the spike's decoy set (PLAN U32 step 2), not copied:
   - files under `/private/tmp/pql-decoys/`, in an owner-only directory checked with `Lstat`;
   - a loopback listener;
   - for a form that signals or reads another process, a decoy process that is an unreaped child of the harness, checked alive before its process id is written to `peer.pid`.

   A stale `/private/tmp/pql-decoys` or a busy port fails the run as not evaluated. It never passes.
3. **Replay.** Each invocation runs through `collect` with the darwin source, with a runbook's steps in order in one shared home. It runs five times, and 25 times for the 58 planted invocations and the 4 that a budget ends (KTD18: access times and timing).
4. **Compare.** Each run's raw facts go through `observe.ProjectRun`, KTD13's serialiser, and are compared with the recording's run on every field QT5's agreement clause lists:
   - the markers;
   - the end;
   - the leak scan;
   - the planted file;
   - the decoys;
   - the home's change list;
   - a computation's printed value;
   - every stream's byte count, digest and cut.

   The comparison runs after the canary is replaced by the zero canary in every encoding, and the digests are taken after the replacement, as QT5 takes them. That is QT5's rule for a recording, and the reverse of QE18's rule for a record. The harness owns this masking, and the engine's record path is unchanged.
5. **Positive controls.** A violating form's unconfined positive control runs with QH12's built environment and a throwaway home, both asserted, on each invocation whose recording's unconfined run touched a decoy. If it touches nothing, the form is not evaluated, and the test fails.
6. **Mismatches.** Each is a hypothesis to triage:
   - an observer defect is fixed in the producer, with a Linux test where one can be written;
   - a build difference is recorded;
   - a recording defect, or a disagreement that may be a specification matter, goes to the owner through the owner question tool, and its invocation stays failing until the owner rules.

### Fields outside the agreement clause

These are not compared: `head`, `kept`, `control_detected` and the unconfined run's fields other than its decoys. The profile and launcher digests cannot match either. The engine's launcher is Go and the spike's was C, and QT5 says a collector reads none of them.

### Running it

The suite runs here, outside the tool sandbox (KTD19), with automatic updates off (GKD9). Today they are on, and the owner is asked to turn them off before the run. The spike's lesson applies: one private copy per grade, not a fresh executable per run (`docs/solutions/test-failures/fresh-executable-per-run-stalls-parallel-sandbox-suite.md`).

The pull request reports:
- the invocations replayed out of the recorded 1,415;
- the 209 unnamed ones, by count and name;
- every mismatch and its disposition.

## Readings the build settled

None is a two-way reading of the sealed text. These were grepped across the sealed folder and the errata: `context`, `implies`, `replay`, `parity`, `decoy`, `peer.pid`, `unreaped`, `pql-decoys`, `touched`, `positive control`, `five confined`, `unconfined`.
- **"Once per context"** means the contexts a scripted run served. The plan's step, not a sealed rule.
- **Five runs against KTD18's 25.** KTD18's 25 applies to the access-time and timing invocations, and U32's five to the rest. These are two plan sentences that apply to different invocations.
- **Digests after replacement,** for parity only (QT5:296), against QE18's raw digests for records.
- **"Touch its decoy"** applies per invocation whose recording's unconfined run touched one. The plan's `violates` flag and the recordings disagree on one invocation, `forges-atime` `account` in a seed home, which writes the home legitimately. The recording governs.

## Rules this group moves off KTD22's list

Each needs a proof test whose first body comment line names the rule. Where the rule needs macOS, a `macos_evidence` entry (KTD22, GR15). Each records its mutation and the test that fails. The PR15a and PR15b columns say which pull request moves it.

| Rule | Moves in | Proof | Mutation, and the test that fails |
|---|---|---|---|
| QP9 | 15a | the fixture-tree rows and the real control copy | skip the append; compare nothing |
| QH27 | 15a | the read and forgery tests | drop QH11's control line; take the facts after the snapshot; ignore the device and inode |
| QH26 | 15a | the existing calibration tests, now used by the real source | no backdating |
| QH25 | 15a | the real inspection on the source path, plus the hostile-entry tests | follow a link; open a FIFO |
| QP5, QP10, QH14 | 15a | the home-sequencing rows | reuse a home; reuse a canary |
| QP6, QP8, QP12, QP13 | 15b | parity on the store and leak forms | drop the scan bound; skip an unreadable file |
| QH5, QH7, QH8, QH21, QE10, QE8, QA1, QA4, QA6, QA7, QS6 | 15b | parity on real runs | each listed in the PR15b note's table |
| GR15's macOS rules | 15a or 15b | each mapped to a named `macos_evidence` test | — |

**Stay listed:** QH9, until the second self-test run after PR15 (PLAN U16); QE18, until PR14's scorecard is in; QP14, QH19 and QH28, which are stated limits.

## Stated limits

- **The real producer grades nothing in the released build until PR16.**
- **Parity proves the recorded invocations only,** in the contexts the corpus served, on this machine's build.
- **An unrelated process reading a decoy,** such as Spotlight or a backup agent, would make an access-time decoy read as touched (QH28's limit, applied to decoys). The harness runs with the decoy directory excluded from Spotlight, and says so.
