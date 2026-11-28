# Design note: PR15, the real producer and parity

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR15. **Units:** U41 and U32.

This note follows the slice d note, `docs/plans/2026-11-27-phase3-pr13-slice-d-design-note.md`, and the self-test track's build review, `docs/agent-reviews/2026-11-27-phase3-pr11-self-test-track-build-reviews.md`. Their definitions and controls hold here. Its design review is logged in `docs/agent-reviews/2026-11-28-phase3-pr15-design-note-reviews.md`. Erratum E-55 (owner ruling, 2026-11-28) is part of it.

## Delivery: two pull requests

Today nothing produces a real run:
- `ReleasedSource` refuses to observe (KTD21);
- no darwin `observe.Source` exists;
- `collect` creates no home, plants no canary, and runs neither QP9's nor QH27's control, though KTD13 gives all of these to it.

U32's parity replay cannot start until those exist, so the group lands as two pull requests, in order:
- **PR15a, the real producer** (U41, and the loop work it needs). The released build still refuses to grade: KTD21's switch moves only in PR16, which re-runs PR15a's macOS tests through `grade`'s wiring.
- **PR15b, parity** (U32).

Each has its own build review and macOS evidence. PR16 waits on both.

## PR15a: the real producer

### The loop owns homes and controls (KTD13)

Before each `Source.Observe`, `collect` prepares the home the key names (QT5's `home`), through `internal/throwaway`:
- `seed`: a fresh copy of the seed (QP5);
- `empty`: discovery's empty home;
- `planted`: a fresh seed copy with a canary that `collect` draws from `crypto/rand` (QP10, QH14). It is planted by a new `throwaway` function that takes the canary as an argument, so that the read control can plant the same one;
- `runbook`: the runbook's one shared home, from its first step to its last (QA7);
- `scratch`: a computation's scratch directory, with its capture files in place (QA4).

Every home gets an empty `.pql-tmp` (QH12, `confine/environment.go`).

**The order for a planted home is fixed,** because on this machine the first read of a file after its last write or backdating moves its access time, and later reads do not:
1. copy the seed;
2. plant the canary;
3. take the before-snapshot, with the planted file's digest supplied by the planting step, not read from the file;
4. backdate the planted file's access time to a day before its modification time (QH27);
5. record its access time, device and inode;
6. serve the key.

Nothing reads the planted file between step 4 and the run. An untagged assertion checks at serve time that the access time is still at least 24 hours before the modification time, and fails the run as not evaluated if it is not. A fake-clock row on the throwaway reader fails if a snapshot reads the file after step 4.

**A runbook's later steps** each get a before-snapshot of the shared home taken just before they are served. The recordings show a later step's `home_changes` as `[]` after step 1 created a directory.

`observe.Spec` gains what the source needs:
- the home's path and kind;
- for a planted home, the planted file's path and the before-facts of step 5;
- `Control`, for QH27's control run;
- `StoreControl`, QP9's result for this invocation.

The source never chooses or keeps a canary. The leak scan's encodings come from `collect`.

### The two controls pass through the source

The scripted source answers both controls from the host today, inside `Observe` (`internal/scripted/reader.go`). Two corpus cases force them: one with `store_control: not_detected` and one with `read_control: not_killed`. So both controls must reach the decision through the source, never around it:
- **QP9.** For each invocation the store-unchanged probe makes, `collect` makes a second fresh copy of the seed, snapshots it, appends one byte to the declared store, and asks `throwaway.CompareHomes` whether the change shows. It passes the answer in `Spec.StoreControl`:
  - the darwin source copies it into the run's `home_changes.control_detected`;
  - the scripted source replaces it with the host's value, as QT4 says.

  With no store, or no regular file of one byte or more after the exclusions, nothing is appended, and QP9's `empty_target` decides, as `decide.storeUnchanged` already does.
- **QH27.** `collect` decides QO7's condition, untagged, after the ordinary run. The condition requires that the read calibration passed (E-38): when it failed, the control is not run. When the condition holds, `collect` plants the same canary in a fresh home and calls `Observe(key, Spec{Control: true})`:
  - the darwin source renders the profile with QH11's extra line;
  - the scripted source answers from the host's `read_control`, and does not record the call as served in its trace.

  The control's end becomes the ordinary run's `planted.control_end`. The QO7 condition the scripted reader evaluates today is deleted from the reader, so the condition lives only in untagged `collect`.

**Control 3 is restated as "the served keys are unchanged":** the trace's `Served` events, in order, for every case. The two controls' extra calls are not served keys. Every prediction file must regenerate byte for byte, and every corpus case must still pass.

### The darwin source (KTD13)

`internal/confine` gains a darwin `Source`. Its `Observe`:
1. **The private copy (QH8, E-55).** It checks the private copy of the binary, or of a computation's executable given as a relative path, with the existing `fingerprintOf` and `SameCopy`: the directory's entries, inode, mode, size, times and digest. An executable under `/bin` or `/usr/bin` runs in place (E-55).
2. **The run.** It renders QH11's profile for the key's variant, spawns through `Copies.Start` and waits through `Wait`. It drains each stream. QE10's byte count is of the bytes written. The digest, the cut and the head are of the kept bytes, as the `huge-output` recording shows: its digest is that of the first MiB.
3. **The planted file's after-facts:** its access time, device and inode, read through `throwaway` before anything else reads the file.
4. **The home's changes:** `throwaway.SnapshotHome` against `collect`'s before-snapshot, then `CompareHomes` (QH25, QP6, QP8, E-46, E-47).
5. **The leak scan:** `throwaway.ScanHome` and the drained streams (QP12, QP13).
6. **A computation's printed value,** through QA6's untagged parse.

It fills `observe.Run` and its `Details` in QT5's run shape, and nothing else. KTD13's guard covers the new darwin files.

**The darwin source also writes, in two places.** `Controls()` prepares the self-test files, and it makes the private copies inside `confine` (`process_darwin.go`). Both are unconfined work on throwaway directories, and they go through `throwaway` like the rest (KTD20). The note says so rather than claim the source only reads.

**Grade-wide controls.** `Controls()` sets:
- `ConfinementApplied` and `SandboxExecExit`, from QH6's raw end;
- `PrivateCopyUnchanged`, from the once-per-grade private-copy check. Its zero value would stop discovery and void every check;
- `SelfTestPassed`, `KnownFastWithin` and `ReadCalibrationMoved`, from `RunControlSuite`;
- `SystemNamed`, from `kern.osproductversion` and `kern.osversion`, matched against QH1's 27.0.1 and 27.2 by an untagged function tested on Linux with U35's prefix cases (QH2).

### What stays out of PR15a

- **The released build still refuses to observe.** A gate test on a `GOOS=darwin` build asserts that `ReleasedSource` still refuses, and that the released binary holds no reference to the darwin source's constructor. A Linux build never compiles the darwin files, so it could not show this. The released path does not call the new home preparation.
- **No subject is graded** outside the `macos_evidence` tests below.

### U41's macOS tests (KTD19: codex writes them, the main session runs them)

Each is behind `darwin && macos_evidence`, runs with the tool sandbox off, and repeats 25 times, passing only 25 of 25 (KTD18). Each asserts the exact raw facts it printed, never an exit status alone, and is paired with a control that the restriction leaves clean runs alone (`docs/solutions/best-practices/clean-control-must-assert-on-output.md`, `sandbox-evidence-must-prove-the-decided-operation.md`).
- **A real read.** The access time moved, the device and inode are unchanged, the control is killed by the confinement after its marker (QH21, E-37), and the decision is held (QP11). The rendered control profile's bytes hold QH11's extra line.
- **The control profile's own control.** The same control profile with `never-reads-secret`, which never opens the file, exits cleanly. That shows the extra line kills only on a read.
- **Forgery attempts, each killed** by the template's write rule on the planted file and its directories: `utimes`, `setattrlist`, a write, a rename and a replace. Each asserts its exact raw facts: a confinement kill, the access time not moved, and the same file.
- **QH28's case, which only the control tells apart.** While `never-reads-secret` runs, the harness itself reads the planted file. The run exits 0, the access time moved, the file is the same, and the control is forced to run and exits 0 without being killed. The decision is `harness_fault` (QO7, E-37), never held.
- **A run that never reads.** With `never-reads-secret`, the access time did not move. The test asserts this after the snapshot and the scan, which proves that the after-facts were read first.
- **QP9's control copy, real.** One byte appended gives `control_detected` true. Its twin appends nothing, and gives false.

### Linux-runnable tests (codex writes them, the gate runs them)

- **`collect`'s home sequencing on a fake source.** Each rule has a twin:
  - every home kind;
  - a fresh home per invocation, except within a runbook;
  - a fresh canary per planted home;
  - the planted order of steps 1 to 6;
  - the read control served only when QO7's condition and the calibration hold, with the same canary;
  - `Spec.StoreControl` passed for each store-unchanged invocation.
- **A swapped planted file.** The `throwaway` reader's device and inode test, with a fixture that swaps the file. No confined run can show it, since the template kills every rename and replace.
- **QP9's control on fixture trees:** detected, not detected, no store, an empty store.
- **The scripted reader without its QO7 condition,** against the two forced-control corpus cases.

## PR15b: parity (U32)

### What is replayed

There are 1,415 recorded invocations across 54 forms. 1,206 are named by at least one case's `invocations` field, and 209 by none. Every case-named key exists in a recording.

**One replay is one grade of one served context.** A served context is a corpus case whose scripted run served at least one key. Its replay grades that case's subject file, seed and scope through `collect` with the darwin source and all controls passing. Only the keys in that case's served trace are compared. A key the real grade serves that the scripted one did not, as in a case whose host forced a failure, is reported as an extra, never compared.

Narrowing U32 step 3's "once per context" to the contexts a scripted run served is a plan amendment. This PR records it in the plan.

**Repeats.** Each served context is graded five times. The 58 planted invocations, the 4 a budget ends, and every invocation with a decoy detected by its access time run 25 times. These repeats are targeted: an exported `collect` function serves one key with exactly the home preparation and controls a grade gives it, through the darwin source. So they never re-run whole grades or the self-test suite.

**The 209 unnamed invocations** are replayed with the same function, in their form's canonical subject file. 54 of them are `remove guard` steps, which run in a scope that includes `destructive`. Some recorded steps can never be served, because QT6 names a step "whether or not an earlier failure would stop it". The build lists those by name, and does not count them as replayed. U32 step 7 leaves it to the owner whether that coverage stands, and the pull request asks.

**The cost, estimated.** About 600 grades, some 24,000 confined runs, plus the repeats: three to six hours. Decoy forms run one at a time, because the decoy directory and the port are fixed. The build measures the real time and reports it.

### The decoys

QT5 names three kinds of decoy. The harness rebuilds the spike's decoy set (U32 step 2) from the names and paths alone, without copying its code. The build lists each decoy in a table: its name, where it lives, and the rule that calls it touched. Among them:
- **files under `/private/tmp/pql-decoys/`:** touched when the access time moved, or the modification time, the inode or the content changed;
- **`token_file`, in the home:** touched when the change time moved, because it only detects writes;
- **the loopback listener:** touched when it accepts a connection;
- **the child-process and peer-process decoys:** touched when the process ended, or its environment was read.

The detector's baseline is reset before every run.

**`peer.pid`.** QT5 takes `home_changes` after `peer.pid` is written. `collect` gains a test-only option: a hook that runs after the home is created and before the before-snapshot. Only the parity harness sets it, so that `peer.pid` never shows as a change. Released and scripted code leave it unset, and a test asserts that.

**The signal decoy** is an unreaped child of the harness, checked alive before its process id is written.

**Not evaluated, never passed.** A stale `/private/tmp/pql-decoys`, a busy port, or a decoy that fails to prepare marks the run not evaluated, and the run fails.

**Confined runs prove little about decoys,** because only one invocation's confined runs touch one. So each violating form's unconfined positive control runs directly before and directly after its confined repeats (QH12's built environment and a throwaway home, both asserted). The confined runs agree with the recording's untouched decoys only between two controls that touched them.

### How a run is compared

Each run's raw facts go through `observe.ProjectRun`, KTD13's serialiser, and are compared with the recording's run on every field QT5's agreement clause lists:
- the markers;
- the end;
- the leak scan;
- the planted file;
- the decoys;
- the home's change list;
- a computation's printed value;
- every stream's byte count, digest and cut.

**Masking for parity.** A recording's digests are taken after the canary is replaced by the zero canary, in every encoding (QT5). That includes the digests of files in the home: `leak-file`'s `debug.log` holds the digest of the zero canary and a newline. A digest cannot be masked after the fact. So the harness's decorator digests the used home again, with the exact canary from `Spec` replaced, through a new `throwaway` function. It does this after `Observe` returns and before `collect` removes the home. It does the same for stream digests. The producer and the record path stay unchanged.

**Masking checks:**
- the canary is not the zero canary;
- wherever the recording shows a leak, at least one replacement was made;
- only the exact canary from `Spec` is replaced, never any 41-character string of its shape.

So a producer that planted the zero canary cannot agree.

**Not compared:**
- `head`, `kept` and `control_detected`;
- the unconfined run, apart from its decoys;
- the profile and launcher digests. The engine's launcher is Go, the spike's was C, and QT5 says a collector reads none of them.

### Mismatches

Each one is a hypothesis to triage:
- **An observer defect** is fixed in the producer, with a Linux test where one can be written.
- **A build difference** is recorded only with evidence. The recorded invocation must reproduce on the form built from its recorded source digest, and the difference must trace to the machine or the toolchain. Otherwise it is an observer defect.
- **A recording defect, or a possible specification matter,** goes to the owner through the owner question tool. Its invocation stays failing until the owner rules.

### Running it

The suite runs here, outside the tool sandbox (KTD19), as one private copy per grade. It never makes a fresh executable per run (`docs/solutions/test-failures/fresh-executable-per-run-stalls-parallel-sandbox-suite.md`).

Automatic updates must be off (GKD9). Today they are on, and the owner is asked to turn them off before the run.

The run records Spotlight's indexing status for `/private/tmp`. The engine does not run as root and does not change it.

The pull request reports:
- the invocations replayed out of 1,415;
- the extras;
- the steps that cannot be served;
- the 209 unnamed invocations, by count and name;
- every mismatch and its disposition;
- the measured time.

## Readings the build settled

None is a two-way reading of the sealed text. Both reviews grepped the sealed folder and the errata for: `context`, `implies`, `replay`, `parity`, `decoy`, `peer.pid`, `unreaped`, `pql-decoys`, `touched`, `positive control`, `five confined` and `unconfined`.
- **"Once per context"** means the contexts a scripted run served, as a plan amendment.
- **Five runs, or KTD18's 25:** the 25 applies to the access-time, timing and access-time-decoy invocations, and the five to the rest.
- **Digests after replacement** apply to parity only (QT5). Records keep QE18's raw digests.
- **"Touch its decoy"** applies to each invocation whose recording's unconfined run touched one. The recording governs where the plan's `violates` flag disagrees. That is one invocation, `forges-atime` `account` in a seed home, which writes the home legitimately.
- **QH8 against `/bin` and `/usr/bin`:** settled by E-55.

## Rules this group moves off KTD22's list

Each move needs a proof test whose first body comment line names the rule. Where the rule needs macOS, the proof is a `macos_evidence` entry (KTD22, GR15). Each records a mutation that a named test catches.

| Rule | PR | Proof | Mutation, and the test that fails |
|---|---|---|---|
| QP9 | 15a | the fixture-tree rows, the `Spec.StoreControl` row and the real control copy | skip the append: the real copy reads false; drop `StoreControl` from `Spec`: the sequencing row |
| QH27 | 15a | the real read, the forgery tests, the QH28 case and the swap fixture | drop QH11's control line: the real read's control is not killed; take the after-facts after the snapshot: the never-reads test; ignore the device and inode: the swap fixture |
| QH26 | 15a | the serve-time access-time assertion and the fake-clock row | read the planted file after backdating: the fake-clock row and the serve-time assertion |
| QH25 | 15a | the real inspection on the source path, and the hostile-entry tests | follow a link; open a FIFO |
| QP5, QP10, QH14 | 15a | the home-sequencing rows | reuse a home; reuse a canary |
| QH8 | 15a | the once-per-grade check and E-55's in-place row | skip the check; copy a `/usr/bin` executable |

**PR15b's moves** are listed in its own pull request. A rule moves only where the build names a mutation that parity catches, such as a producer that drops the leak scan's bound for QP13. Rules parity cannot break stay listed: QP8, which parity compares before its exclusions, then QE8 and QS6.

**Stay listed:** QH9, until the second self-test run after PR15 (PLAN U16); QE18, until PR14's scorecard is in; QP14, QH19 and QH28, as stated limits.

## Stated limits

- **The real producer grades nothing** in the released build until PR16.
- **Parity proves only the recorded invocations,** in the contexts the corpus served, on this machine's build.
- **An unrelated process reading a decoy,** such as Spotlight or a backup agent, makes an access-time decoy read as touched. This is QH28's limit, applied to decoys. The run records the indexing status.
