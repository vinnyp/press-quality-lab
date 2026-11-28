# Peer-review gate — phase 3 PR15a, the real producer (2026-11-28)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer, because this is the confinement path that runs an untrusted subject.

The design note had its different-model review on agy (`docs/agent-reviews/2026-11-28-phase3-pr15-design-note-reviews.md`).

**macOS runs.** U41's macOS tests ran in the main session, outside the tool sandbox (KTD19). Codex wrote them and never ran them.

## Round 1 (head 2fca319)

**Verdicts:**
- **code:** no Blocker, one Major.
- **test:** one Blocker, six Majors.
- **security:** no Critical, one High (latent), three Mediums.

All 17 of the first macOS source tests passed, 25 repeats each.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The darwin source kept whatever `PLANTED_DIR_n` the resolver returned. For a nested planted file, a subject could rename the directory, set the file's times through the new path, and rename it back, unkilled | security (High), code (Minor) | Confirmed against QH11 and QH27. The macOS witness added in round A is killed at the first rename | The source derives the directories from the home and the planted path, and refuses any a resolver supplies |
| 2 | No test read the planted bytes, so planting the zero canary or the original bytes survived, and a leaking subject would be held | test (Blocker) | Reproduced by mutation | The planted file is read back against the literal canary, in throwaway and in collect's fake run |
| 3 | Homes were created outside QE20's swept run directory, and kept until the grade ended | code (Major), security (Medium) | Confirmed against QE20 | Homes under the published run workspace, each removed after its invocation, with a runbook's kept to its last step |
| 4 | The canary never reached the record masker | security (Medium) | Confirmed: nothing called `AddCanary` | collect registers each canary before the source sees the key |
| 5 | The stand-in planted path for a home with no secret sat inside the home, where a runbook step could create it | security (Medium) | Confirmed | Taken from an engine-owned 0700 directory outside every home |
| 6 | The canary's shape, the control fold's markers, QP9's not-detected branch, a computation's own copy, separate runbook homes, and a positive leak row were all untested | test (Major) | Reproduced by mutation | Each has rows. The leak witness gives exactly five leaks: the four base64 forms that §4 keeps, and the plain one |
| 7 | Minors and Lows: QH6's derivation in a darwin file, the attribution order, the seed copy unchecked against the plan, E-55's path resolving outside `/bin`, `Copies` taking a non-canonical path, the release check's coverage, test-only API in production code, and several untested seams | code, test, security | Confirmed | Each fixed in rounds A and B |

**Found while building the fixes:**
- **My error:** the round B brief predicted two leaks for the leak witness, and codex stopped on the conflict with §4. The right count is five.
- **`Copies`' new canonical check broke `TestSourceComputation`,** because its temp path ran through `/var`, a link. The fixture now canonicalizes its path, as PR16's resolver will have to.

## Round 2 (delta: 2fca319 to 8e896fa, and 6273b19 to d610f80)

6273b19 merges `main`, which brought PR14 (#31). The delta leaves it out, and the code reviewer checked that the merge left PR15a consistent with PR14.

**Verdicts:**
- **security:** every round 1 finding is fixed. One new Low.
- **code:** every round 1 finding is fixed, and one new Major.
- **test:** all 22 round 1 survivors now turn a test red, and four narrower Minors remain.

22 macOS source tests passed, including:
- the nested-directory rename;
- the five-leak scan;
- E-55's in-place `/usr/bin/printf`, beside a private copy killed by code signing.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 8 | The seed check compared a fresh home with the check's seed, but E-49's synthetic check for a runbook step with no required check carries none. A seeded runbook whose first step has no required check exited 70 | code (Major) | Reproduced | E-49's synthetic check takes the plan's seed. A seeded `TestOutsideStepRuns` row and its twin |
| 9 | Round B moved the changed-copy invariant's test from invocations to captures, leaving invocations untested | code, test (Minor) | Reproduced by mutation | Each row runs through both paths |
| 10 | The read control's serve-time assertion is untested, and the copies test does not assert `BIN_PARENT` or the launcher target | test (Minor) | Reproduced by mutation | A control-path row, and both assertions in `TestCopiesPerExecutable` |
| 11 | `Options.AfterObserve` is a test hook in the production struct | security (Low) | Confirmed | Behind `necessity_test`, and named in `TestCollectionTestAPIBoundary`. An empty run directory is refused too |

## Round 3 (confirmation, d610f80 to ecd1d06)

**Verdict:** code and test reviewers found no Blocker or Major.
- **code:** each fix confirmed by a run or a mutation. The plan's seed is right for every check kind, because `plan.Build` gives every check the same entries.
- **test:** all five requested mutations and four more turn a test red, with no false greens.

Left as they are:
- `collect.Build` calls `Collect` without options, which now faults. It has no caller.
- The control row has no explicit served twin, though mutation M24b shows the condition holds.
- Three round 1 Nits: the control profile's bytes are rendered by the test itself; the encodings are computed in the darwin source; `kern.osversion` is not carried to the record.

## Evidence

- `gate.sh` and `check.sh` pass at each fix head.
- The broad macOS set at the final code commit, named in the pull request.
