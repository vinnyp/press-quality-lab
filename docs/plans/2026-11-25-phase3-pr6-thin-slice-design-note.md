# Design note: PR6, the thin end-to-end slice

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR6. **Units:** U2, U4, U5, U33, U23, U6, U46, U17, U7, U25.

The plan says that from the thin slice to slice e, each pull request gets this note before its build (Implementation Units, "Before the build"). The note names four things: the slice's evidence rules, its predicted ratchet set, the rules it moves off KTD22's list, and its controls. The build waits on a design-mode review of the note through the measurement lens. Round 1 of that review is in `docs/agent-reviews/2026-11-25-phase3-pr6-design-note-reviews.md`.

## What the slice makes true

After this slice, `pql grade` runs end to end:
- it reads a subject file;
- it fixes the required set and every invocation key for the anchor cases;
- it collects through the seam and the loop, with only the precedence table's level 1 built;
- it builds the record, has the judge decide it, stores it, and exits with the judge's code.

**Three kinds of engine fault.** Each ends the run with exit 70 and names a rule on the GKD8 rejection line:
- **A planning gap.** The engine fixes the required set, its preimages and check ids, and the pre-check before any check runs. If a check needs a preimage or required-set kind the slice does not build, such as a placeholder's preimage, the run ends there, naming that rule. Nothing has been collected at that point.
- **An unbuilt level.** A check that reaches an unbuilt level ends the run, naming that level's rule, whenever its trigger holds or cannot be ruled out (KTD11).
- **A key gap.** An invocation key, in QT5's shape, is needed only when the loop serves that invocation: at level 6 for a computation, at level 7 for the subject. A check meets a missing key kind, such as a computation's key, only if it gets that far. A check decided at level 1 never needs a key.

Either way, an unbuilt rule is an engine fault and never a pass.

A released `grade` gives every subject-running and computation-running check `network_unverifiable` and still reaches the judge (KTD21).

## Evidence rules

The only evidence rule this slice decides is **QO1** (confinement not applied), at level 1, through its two grade-wide conditions. The scripted host (QT4) supplies the facts for both:
- the confinement could not be applied, which is the host's `confinement.applied`;
- the self-test failed (QH9), which is the host's `self_test`.

**Which checks QO1 covers.** QO1 covers the checks that run the subject or a computation, and no others. Three clauses say this:
- §2 opens "These rules turn one invocation of the subject into an outcome for the check it serves" (`collection-rules.md` line 119);
- QH1, QH6 and QH9 each say "every check that runs the subject" (`confinement.md` lines 9, 17 and 31);
- QO6 applies the same rules to a computation.

A check with no invocation, such as an `empty_target` probe, is decided by its own rule, which this slice does not build. No corpus case has such a check under a level-1 host.

**What this slice does not build of QO1:**
- its per-invocation clause, where confinement was not applied to one invocation. That clause is QH6, read from the recording's markers and end (QT4, "each invocation's markers and end"), and it belongs to level 7;
- its system clause (QH1). A scripted run checks no system version (QT3), and the real verifier comes with U35.

Every other observation the slice makes feeds the record and the trace, never a resolution:
- the required set;
- check ids and invocation keys;
- the host and the recording, read as raw facts.

## Predicted ratchet set

`scripts/predict-ratchet.py --built scripts/ratchet/pr6.json` computes it. It writes the full prediction to `scripts/ratchet/pr6-prediction.json`: every case, and every check's deciding level and ending. `check.sh` regenerates that file and fails on any difference, and runs the script's tests.

**Predicted:** `case-003`, `case-004`, `case-100`, `case-101`, `case-102`, `case-103`, `case-116`, `case-129`, `case-136`.

| Case | Kind | Why it passes |
|---|---|---|
| `case-003` | judge-checked | every check decided at level 1, by QO1: confinement not applied |
| `case-004` | judge-checked | every check decided at level 1, by QO1: the self-test failed |
| `case-100` | derivation-only | QS2: a repeated key, refused by the strict reader |
| `case-101` | derivation-only | QS4: a subject fact that is not a boolean |
| `case-102` | derivation-only | QS3, as far as the schema detects it |
| `case-103` | derivation-only | QS5, as far as the schema detects it |
| `case-116` | derivation-only | QS11, as far as the schema detects it |
| `case-129` | derivation-only | QS1: version `2.0` |
| `case-136` | derivation-only | QE6: the pre-check rejects, and the cause is the user's |

All seven cases the plan names for this slice are predicted. U25's verification asks for the derivation-only cases "whose rule U4's checks name, among them" four, so the list is open. `case-103` and `case-116` are the two others that U4's schema checks reach. In each file, the named rule takes precedence over QS2 (QT6, errata E-9).

### How the script decides

- **Judge-checked cases.** The script classifies each gap in `builds` as a planning gap or a key gap, in one table. It finds each check's ending in four steps:
  1. It looks for a planning gap first.
  2. If there is none, it goes through levels 2 to 8 in order, up to the check's deciding level. The ending is the first unbuilt level whose KTD11 trigger holds (`collection-rules.md` §2, with a table in the script giving the line for each reason).
  3. If a key gap comes before any such level, the key gap is the ending instead. A key gap is the level that would serve a missing key.
  4. Otherwise the ending is the deciding level, which must be built.

  A check's ending does not depend on the order in which checks are listed or run. A case is predicted only if every check ends at a built level and every rule or part its required set and record need is in `builds`.
- **Derivation-only cases.** The script predicts one when its rule, or the part its violation falls in, is in `rejects` and is the first rule the file breaks in E-9's order.
  - The part comes from a hand audit of each derivation-only subject file. The audit cites the line, and is pinned to the file's SHA-256: a changed file stops the script until it is audited again.
  - A test checks each label against the subject-file schema. A `.schema` file must be rejected by the schema, and every other label must be accepted by it.
  - `case-136` also needs the required set and the pre-check, since its rejection comes from the judge's pre-check.
- **Real-verifier cases** (`case-069`, `case-070`, `case-143`) are never predicted in a scripted slice.

The hand audit cannot hide a false prediction at the level of a case. An over-prediction fails as a predicted case, and an under-prediction fails the rule that a pending case must still fail. What it could hide is the right rule named for the wrong clause. The per-clause planted defects in control 2 cover that.

### Every pending case's predicted ending

For each pending case, the prediction file holds the ending the build must show. The corpus runner checks it (control 3).

- **A judge-checked case with a planning gap:** exit 70, naming one of its planning-gap rules.
- **Any other pending judge-checked case:** exit 70, naming a rule from the set of its checks' endings, which are unbuilt levels and key gaps. The loop's order across checks is the engine's, so any member of the set may be named.
- **A pending derivation-only case:** not exit 2. The file is refused under no rule, because a refusal naming another rule would be a wrong decision.
- **A real-verifier case:** not run.

For this slice:
- **Pending judge-checked cases:** 116.
  - 24 end at a planning gap.
  - 92 end at an unbuilt level.
  - None ends at a key gap, because levels 3 and 5 trigger first.
- **Pending derivation-only cases:** 24.
- **Real-verifier cases:** 3.

| Ending (exit 70, one of) | Cases |
|---|---|
| QA7, QD4, QH8 | 67 |
| QD4 | 21 |
| QD4, QH8 | 4 |
| `QE2.placeholders`, `QE3.placeholder_candidates` | 9 |
| QE4 | 6 |
| `QA3.capture_crediting`, `QE2.captures` | 6 |
| `QA3.capture_crediting`, `QA9.preconditions`, `QE2.captures`, `QE7.asserts_emptiness` | 2 |
| `QE4.anchor` | 1 |

A part such as `QE2.placeholders` is named on the rejection line by its rule id, QE2.

## Rules this slice moves off KTD22's list

A rule leaves the unmapped list only when a test proves its whole text. The pull request records the mutation that broke the rule and the test that failed (`CODING_STANDARDS.md`, Tests). A rule this slice builds only in part stays listed, because the list can only shrink: once a rule is off, it cannot come back.

| Rule | Proof | Its mutation |
|---|---|---|
| QS1 | U4: version `2.0`, a missing version and the number `1.0` are each refused naming QS1. A missing version does not "state its own version", and E-9 puts QS1 first | accept any version |
| QS2 | U4: each strict-reading clause (§3 of the judge contract, through `internal/strictjson`) and a schema refusal, each naming QS2 when no named rule is broken | accept a repeated key |
| QS4 | U4: a table over each of the five facts and each wrong type; U7 passes every fact as given | hard-code one fact's value |
| QT1 | U2 and U6: the flag matrix in released and scripted builds, and an unreadable or invalid host or recording refused with exit 2 | let a released build accept `--host` |
| QE6 | U25: all twelve rows of QE6's table, X1 to X12, through an injected judge, each with its expected exit code | flip one row's cause |
| QE15 | U25: a record the injected judge rejects exits 70, shows no verdict, and is stored with `judge_rejected: true` | drop the stored rejected record |
| QH3 | U25: an effective user id of 0, through the injected seam, exits 70 before the store or the sweep is resolved | check the real user id instead of the effective one |

None of the seven is a GR15 rule.

**Stays listed,** each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QO1 | the per-invocation clause (QH6, level 7) and the system clause (QH1, U35 and macOS) |
| QT2 | an anchor of a scripted run refused, and scripted baselines from files (U19) |
| QT4 | the rules that decide from six of the eight host fields: QO4, QP9, QP11, QH21 and QO5 |
| QE14 | the scorecard half (U18, whose agreement check proves it again) |
| QE17 | baselines and trials in the store (U19), and the store's application-support location on macOS |
| QS3, QS5, QS6, QS7, QS9, QS11, QP10, QA4, QA9 | the clauses the schema cannot detect (U9), and QA9's preconditions (U39) |
| QE2, QE3, QE5, QE7 to QE11, QP1, QA3 | the other preimage, required-set, crediting and record kinds |
| QE16 | the codes later slices produce |
| QT3 | a stated limit, routed with CASES.md |
| QT5 | computation keys |
| QE20 | the darwin half (U40) |

## Controls

1. **The ratchet.** Every predicted case leaves `internal/corpusrunner/pending.txt` and must pass; every other case stays and must still fail (KTD2.7).
2. **One planted defect per clause.** Each mutant turns exactly its own case red, and the test states the new ending.

   | Case | Planted defect | New ending |
   |---|---|---|
   | `case-003` | drop QO1's confinement condition | exit 70 at the first unbuilt level |
   | `case-004` | ignore the failed self-test | exit 70 at the first unbuilt level |
   | `case-100` | accept a repeated key | not refused |
   | `case-101` | accept a non-boolean fact | not refused |
   | `case-102` | drop `commands` from the required fields | not refused |
   | `case-103` | allow a placeholder where QS5 forbids one | not refused |
   | `case-116` | drop the control-character range from the step-name pattern | not refused |
   | `case-129` | accept any version | not refused |
   | `case-136` | swap QE6's cause | exit 70, not 2 |
   | every case | **QO1 fires on a passing host** | caught by controls 3 and 4 |
3. **Every check's ending, checked by a test.**
   - **The in-process test.** A tagged test runs `plan` and `decide` in process over every scripted case's subject, host and recording, and compares each check's ending with `pr6-prediction.json`. `decide` is pure. A check decided wrongly inside a pending case is therefore seen, even though that case's run stops early. QO1 firing on a passing host turns this test red on every case with a passing host.
   - **The corpus runner.** It requires each pending case's actual exit code and named rule to match its predicted ending. It reads the rule from the GKD8 line, as it already does for derivation-only cases.

   The script's prediction is independent of the engine's code, so the test's oracle does not come from the code under test.
4. **The unbuilt-level sentinel.** U23 tests each level's trigger against real check values, positive and negative, and forces each unbuilt level with its trigger true. A test from a passing host goes through the real level 1 and must reach the first unbuilt level whose trigger holds. A mutant that lets a check pass on, or one whose trigger returns false where it holds, must turn a test red.
5. **No subject or computation ever runs** (KTD21). The released source's `Observe` fails the test if it is ever called, and the trace shows zero homes. A sentinel subject and a sentinel computation each write a file when run, and a positive control runs each directly and sees the file.
6. **The trace.** For `case-003` and `case-004`:
   - the traced required set equals `input.json`'s `required`;
   - there are zero served keys and zero homes, since confinement was not applied or the self-test failed (KTD2.6).

   For each derivation-only case, there is no home.
7. **The record path.** The runner keeps these checks:
   - it decodes with `encoding/json` and `UseNumber` (KTD2.3);
   - it judges each stored input again with `pql judge` (KTD2.4);
   - it exports the records of `case-003` and `case-004` for `check.sh` to validate against `record.schema.json` (KTD15). That validation now covers two records, not zero.
8. **The store and the funnel** (U17, U25):
   - file and directory modes;
   - a stale run directory swept, and links and foreign directories left alone;
   - the root refusal before the store;
   - a stored rejected record;
   - QE16's exit-code cross-check;
   - a panic in either goroutine exits 70.

## Limits the script carries into later slices

- **Several invocations.** Discovery and planted-secret checks combine their invocations by QO7, through their own rules (U11, U37). The script treats them only as far as level 7. Slice b2 extends it before it predicts them.
- **Level 8.** It is one level in the script's input. Slice b2 builds the probes' half and slice c the predicates', so the input must split it by then.
