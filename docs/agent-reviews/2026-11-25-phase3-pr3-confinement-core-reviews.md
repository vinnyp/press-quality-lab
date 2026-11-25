# Peer-review gate — phase 3 PR3, the confinement core (2026-11-25)

**Mode:** build. **Reviewers:** peer-code-reviewer, peer-test-reviewer, peer-security-reviewer, a different-model peer-code-reviewer on agy, then delta security and test reviewers; after them, two `ce-code-review` runs. **persona-version:** cache/1.9.5.

**Design round:** none. The plan `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` (U42, U21, U15; KTD12 to KTD14, KTD17, KTD19, KTD20) is the design. Design notes start at the thin slice (PR6).

**Tier rationale:**
- Tier 1 code and test always run.
- Tier 2 security fired, because this is the confinement boundary between an untrusted program and the engine.
- The measurement lens did not fire. No verdict or threshold changes here, and the raw facts reach `decide` in a later PR.
- `ce-code-review` added reliability (process lifecycle, drains, kills) and an independent cross-model adversarial pass on codex.

Each reviewer ran in its own clone. No reviewer started `sandbox-exec`. The coordinator ran all macOS evidence outside the tool sandbox.

## Round 1

**Range:** the U42, U21 and U15 commits. The same pass (codex) wrote the code and its tests.

**Verdicts:**
- code: ready after fixing one Blocker;
- test: trustworthy, with Major gaps;
- security: ship after fixing one High;
- agy: ready after fixing two Blockers.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The program's stdout and stderr were seekable `O_RDWR` files. A subject could `lseek`/`pwrite` over the launcher's marker, turning a violation into "confinement not applied", or `pread` it to forge the could-not-start line | security (High), code (Major) | The security reviewer reproduced erase and forge on an unconfined child wired as `Start` wired it | Fixed: pipe write ends, drained into bounded buffers. `TestMacOSMarkerPipeAttacks` passes on 27.2 |
| 2 | `Setpgid` instead of `Setsid`, so the program kept the engine's session and terminal, against QH13 | agy (Blocker), security, code | Confirmed against QH13 and QH16 | Fixed: `Setsid`. `TestNewSession` and `TestMacOSCoreSession` |
| 3 | The exit detail was read from `fflags`, so it was always 0. XNU keeps it in the event's data | code (Blocker) | Checked against `NOTE_PDATAMASK` and `NOTE_EXIT_DETAIL_MASK` | Fixed: an untagged `exitDetail`/`decodeEvent`, tested with constructed events |
| 4 | The private-copy check's digest, inode and launcher paths were untested. The only evidence test hit the entry-count check first | agy (Blocker), test (Major) | Mutants survived | Fixed: `TestChangedCopies` per property, and a second clean launch on the same copies |
| 5 | `Kill` after reap could signal a reused group, and late kills were misattributed | security, code | Confirmed by reading | Fixed: a reaped guard under the lock; engine-killed is frozen at exit |
| 6 | The group kill was one-shot, with no check that the group had ended | security, test | Confirmed | Fixed: repeats until empty or zombie-only |
| 7 | `EVFILT_PROC` was registered in `Wait`, not at spawn | agy, code | Confirmed | Fixed: registered right after `ForkExec`. The ESRCH path is tested |
| 8 | The subject's environment reached the launcher's Go runtime. `GODEBUG=inittrace=1` from a subject file would put output before the marker | security (Low) | Confirmed by reading | Fixed: the launcher runs with an empty environment, and the program's environment is passed positionally. `TestMacOSCoreSubjectEnvironment` |
| 9 | The gate's tagged `errcheck` passes ignored their tags, because errcheck does not read tags from `GOFLAGS` | coordinator, while reviewing PR5 | Reproduced in a scratch module | Fixed before round 1: `errcheck -tags`, with controls that read `gate.sh`'s own lines |
| 10 | Launcher failures after the marker, NUL and invalid-UTF-8 words, argument size, marker validation, descriptor closing, profile edges, gate step and tag-list coverage, `GOARCH`, `GOFLAGS` in builders, transitive import guards, `go.mod` | all | Confirmed | Fixed |

## Round 2 (delta)

**Range:** the round-1 fix commit. Security found no Critical or High; every fix held. Test reproduced two defects on this machine.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 11 | Darwin's `kill(-pgid)` returns EPERM while a member is mid-exit, so the repeat loop aborted. A second `Kill` failed in 30 of 50 runs, and `-race` failed `TestLateKillAttribution` | test (Major) | Reproduced | Fixed: EPERM counts as "not yet ended" until the deadline. `TestConsecutiveKill`, `TestConcurrentKillWait`, `-race -count=20` |
| 12 | A descendant that leaves the group and holds a pipe made `Wait` block until it exits | security, test (Major) | Reproduced: 8 s with the leader reaped | Fixed: a 100 ms grace after the reap, then the read ends close. `OutputCutAfterExit` records it. `TestEscapedWriter` |
| 13 | `TestGroupEnd` passed with the group kill removed | security (Medium) | Mutant survived (60 s pass) | Fixed: it fails in 0.11 s |
| 14 | `Wait` was not idempotent; launcher-side validation rows; the `decodeEvent` call site; the tty assertion; the untagged gate test step; `Start` passing the engine environment | security, test | Confirmed | Fixed |

## Round 3 (ce-code-review, run `20261125-130633-69926b02`)

**Reviewers:**
- local: correctness, security, project standards, testing, maintainability, reliability, learnings;
- cross-model adversarial pass on codex (requested `gpt-6.1-terra` at high; `model_actual` `gpt-6.1-terra`, `effort_actual` unverified, `independence_verified` true).

**Verdict:** "Ready with fixes". U42, U21 and U15 are all met.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 15 | A late group kill can record a confinement kill as the engine's, in the microsecond window between `Kill`'s state read and its signal | adversarial-codex (P1) | Validator confirmed; incidence not measured | Stated limit, with a code comment. The window cannot change an outcome: a budget kill is attributed by QO4's own "saw the process still running" rule, and any other engine kill comes from the exit funnel, where the run ends 70. Both attributions leave the check not observed |
| 16 | A memory kill before `EVFILT_PROC` registration loses its exit detail | adversarial-codex (P1) | Validator rejected: QH21 says a registration ESRCH means the process exited, and registration precedes the marker | No change |
| 17 | The private copy's size, mtime and ctime checks are untested | testing (P2) | Validator confirmed | Fixed: `TestChangedCopies/size`, `mtime`, `ctime` |
| 18 | The import guard's two-file selection limit is untested | testing (P2) | Validator confirmed | Fixed: `ThreeSelectors` and a `TwoSelectors` control |
| 19 | `kern.argmax` wiring, the capture cut boundary, a fixed sleep in `TestNewSession` | testing (gaps) | Mutants survived | Fixed |

The branch was then rebased onto `main` with PR2 merged. `cmd/pql`'s darwin dependency list gained one line, `internal/strictjson`. Round 4 (`ce-code-review` on the rebased branch) is recorded in the pull request.

## Constraints on later units (from the security review)

- **U40 / the grade pipeline.** Declared and seed paths must be contained (QS8, QS9) before they reach `confine.Arguments`, which canonicalises any existing path and passes it only as a `-D` parameter.
- **U40.** The run directory holding the private copies must never sit inside a subject's home, or the copies fall inside the subject's write grant between the QH8 check and the exec.
- **U30** owns the final draining design: heads, counts and digests. U15's 1 MiB per-stream cap and 100 ms post-exit grace are interim.

## Mutation record

Each mutant was run, the named test failed on it, and the code was restored. Rows marked with a reviewer were re-run independently through `mutation-verify.sh` with a single-row knock-out.

| Rule | Mutation | Test that failed |
|---|---|---|
| Marker adjacency (QH7) | `HasPrefix` → `Contains` | `TestMarkerProtocol/not_adjacent` (reviewers) |
| Sealed profile (QH11) | change the template or a fenced block | `TestSealedProfile` (reviewer) |
| Paths only as `-D` | interpolate a path; swap counts | `TestPathParameters` (reviewer) |
| Canonical paths | drop `EvalSymlinks` | `TestCanonicalParameters`, `TestCanonicalCopies` |
| Hidden protocol | unknown mode, marker length, case and hex, env JSON, NUL, UTF-8 | `TestHiddenProtocol` rows |
| Could-not-start | change exit 72; drop the second marker | `TestCouldNotStartProtocol` |
| Session (QH13) | `Setsid` → `Setpgid` | `TestNewSession` (reviewers) |
| Exit detail (QH21) | read from fflags; swap at the call site | `TestExitDetail`, `TestDecodeEvent` (reviewer) |
| Registration | move into `Wait`; ESRCH as error | `TestImmediateRegistration`, `TestRegistrationESRCH` (reviewer) |
| Kill after reap | drop the reaped guard | `TestPipeCapture` (reviewers) |
| Kill attribution | attribute late kills | `TestLateKillAttribution` (reviewers) |
| Group end (QH23) | one-shot kill; abort on EPERM; remove the kill | `TestGroupEndRepeats` (reviewer), `TestGroupEndEPERM`, `TestGroupEnd` |
| Drains | remove the cap; drop counts or cut flags; extend the grace; `>=` at the cap | `TestBoundedCapture`, `TestEscapedWriter`, `ExactCap`, `OverCap` |
| Private copy (QH8) | ignore digest, mode, inode, size, mtime, ctime, launcher, entries | `TestChangedCopies/*` (reviewers) |
| Argument size | skip the check | `TestArgumentSize`, `TestStartArgumentLimit` |
| Environment | populate the launcher's; pass the program's to spawn | `TestLauncherInvalidWords`, `TestStartEnvironment` |
| Wait | disable the cache | `TestWaitCached` |
| Import guard | Cobra outside `cmd/`; `x/sys` on Linux; transitive spawn; `func init`; cgo via `go list`; the dependency list; three selectors | `TestDependencyClassifier/*` (reviewers), `TestSourceMutations/*`, `TestCgoListMutation`, `TestDependencyListMutations`, `ThreeSelectors` |
| Gate steps | errcheck tags through `GOFLAGS`; drop a vet, test or evidence step; drop a tag set; drop `GOARCH` | `TestTaggedLintControls`, `TestGateExecutionControls/*`, `TestGateTagSets` (reviewer) |
| Builders | inherit `GOFLAGS` | `TestBuilderEnvironment` |

## macOS evidence

The coordinator ran every `macos_evidence` test and the darwin unit tests outside the tool sandbox on macOS 27.2 (build 26B123). The commit, command, result and log SHA-256 are in the pull request.
