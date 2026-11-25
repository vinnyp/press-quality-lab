# Design note: PR6, the thin end-to-end slice

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR6. **Units:** U2, U4, U5, U33, U23, U6, U46, U17, U7, U25.

The plan says that from the thin slice to slice e, each pull request gets this note before its build (Implementation Units, "Before the build"). The note names four things: the slice's evidence rules, its predicted ratchet set, the rules it moves off KTD22's list, and its controls. The build waits on a design-mode review of the note through the measurement lens.

## What the slice makes true

After this slice, `pql grade` runs end to end:
- it reads a subject file;
- it fixes the required set for the anchor cases;
- it collects through the seam and the loop, with only the precedence table's level 1 built;
- it builds the record, has the judge decide it, stores it, and exits with the judge's code.

Every other level ends the run with exit 70, naming its rule, whenever a check reaches it and its trigger holds or cannot be ruled out (KTD11). An unbuilt rule is therefore an engine fault and never a pass.

A released `grade` gives every subject-running and computation-running check `network_unverifiable` and still reaches the judge (KTD21).

## Evidence rules

The only evidence rule this slice decides is **QO1** (confinement not applied), at level 1.
- For a scripted run, the facts QO1 reads come from the scripted host (QT4).
- For a released run, they come from KTD21's fixed path.

No other reason is assigned. Every other observation the slice makes feeds the record and the trace, never a resolution:
- the required set;
- check ids and invocation keys;
- the host and the recording, read as raw facts.

## Predicted ratchet set

`scripts/predict-ratchet.py --built scripts/ratchet/pr6.json` computes it. Its tests are in `scripts/test_predict_ratchet.py`, run by `check.sh`.

**Predicted:** `case-003`, `case-004`, `case-100`, `case-101`, `case-102`, `case-103`, `case-116`, `case-129`, `case-136`.

| Case | Kind | Why it passes |
|---|---|---|
| `case-003` | judge-checked | every check decided at level 1, by QO1: confinement not applied |
| `case-004` | judge-checked | every check decided at level 1, by QO1: the self-test failed |
| `case-100` | derivation-only | QS2: a repeated key, refused by the strict reader |
| `case-101` | derivation-only | QS4: a subject fact that is not a boolean |
| `case-102` | derivation-only | QS3, as far as the schema detects it: `commands` missing |
| `case-103` | derivation-only | QS5, as far as the schema detects it: a placeholder where none may stand |
| `case-116` | derivation-only | QS11, as far as the schema detects it: a control character in a step name |
| `case-129` | derivation-only | QS1: an unsupported version |
| `case-136` | derivation-only | QE6: the pre-check rejects, and the cause is the user's |

All seven cases the plan names for this slice are predicted. U25's verification asks for the derivation-only cases "whose rule U4's checks name, among them" four, so the list is open. `case-103` and `case-116` are the two others U4's schema checks reach. In each of those two files, the named rule takes precedence over QS2, as QT6 and U4 say.

### How the script decides

- **Judge-checked cases.** The script maps each expected evidence entry to the level that decides it, from `collection-rules.md` §2. A table in the script gives the line for each reason. A check decided at level k passes through levels 1 to k-1 first. For each unbuilt level whose KTD11 trigger holds on the way, the script predicts exit 70.

  The case is predicted only if two things hold:
  - every check's deciding level is built;
  - every rule, or part of a rule, that the required set and the record need is in the slice's `builds` list.
- **Derivation-only cases.** The script predicts one when the case's rule, or the part its violation falls in, is in `rejects` and is the first rule the file breaks.

  The part, and the list of rules a file breaks, come from a hand audit of each derivation-only subject file. The audit is kept in the script with a line citation, and pinned to the file's SHA-256. A changed file makes the script refuse to run until the file is audited again. This is the one judgment the script does not compute. It is a fixed record over the sealed corpus, not a classifier of arbitrary files.
- **Real-verifier cases** (`case-069`, `case-070`, `case-143`) are never predicted in a scripted slice.

### What blocks the rest

Of the 143 unpredicted cases, the first blocker the script reports is:

| First blocker | Cases |
|---|---|
| computation keys (`QT5.computation_keys`, U39) | 70 |
| level 3 (QD4) | 31 |
| level 5 (QH8) | 7 |
| level 2 (QS5) | 5 |
| derivation-only: a rule or part not yet in `rejects` (QS6 to QS12's semantic halves, QP7, QP10, QA4, QA9) | 24 |
| placeholder preimages (`QE2.placeholders`, U39) | 1 |
| level 4 (QA7) | 2 |
| real verifier | 3 |

## Rules this slice moves off KTD22's list

A rule leaves the unmapped list only when a test proves its whole text, and the pull request records the mutation that broke it and the test that failed (`CODING_STANDARDS.md`, Tests). A rule this slice builds only in part stays listed.

| Rule | Proof |
|---|---|
| QS1 | U4: each unsupported version is refused, naming QS1 |
| QS2 | U4: strict reading and schema refusal, with a named rule's precedence |
| QS4 | U4 refuses each fact of the wrong type; U7 passes the facts as given |
| QO1 | U23: each condition QO1 lists, forced through the scripted host, gives `network_unverifiable` at level 1 |
| QT1 | U2: the flag matrix in released and scripted builds |
| QT2 | U7: every tier is `scripted`; the judge refuses an anchor of it |
| QT4 | U6: every host field forces its control's raw result; each invalid host exits 2 |
| QE6 | U25: a user-caused pre-check rejection exits 2, an engine-caused one 70 |
| QE14 | U25: an injected judge's doctored output drives the exit code, scorecard and record |
| QE17 | U17: the store's location and modes, with nothing kept inside a repository |

**Stays listed,** each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QS3, QS5, QS11 | the semantic halves, from U9 |
| QE2 | placeholder preimages, from U39 |
| QE3, QE7 to QE10 | the other required-set kinds |
| QE5 | full subject-file validation |
| QE16 | the codes later slices produce |
| QT5 | computation keys |
| QE20 | the darwin half, from U40 |

None of the ten is a GR15 rule, so none needs a `macos_evidence` entry.

## Controls

1. **The ratchet.** Every predicted case leaves `internal/corpusrunner/pending.txt` and must pass; every other case stays and must still fail (KTD2.7).
2. **A planted defect per predicted case.**

   | Case | Planted defect | Turns it red because |
   |---|---|---|
   | `case-003` | drop QO1's confinement branch | the case no longer resolves at level 1 |
   | `case-004` | ignore the failed self-test | the case no longer resolves at level 1 |
   | `case-100` | accept a repeated key | the file is no longer refused |
   | `case-101` | accept a non-boolean fact | the file is no longer refused |
   | `case-102`, `case-103`, `case-116` | skip the schema check | refusal names QS2, or the file is not refused |
   | `case-129` | accept any version | the file is no longer refused |
   | `case-136` | swap QE6's cause | it exits 70, not 2 |
3. **Prediction against outcome.** The pull request traces every case that passed outside the prediction to the rule that produced it, and diagnoses every predicted case still pending (KTD1).

   For each case still pending, the build also records its actual exit code and named rule beside the script's first blocker. A case predicted to stop at an unbuilt level must exit 70 naming that level's rule. Any other ending is diagnosed before merge, so a pending case that fails for an unexpected reason is not hidden by the ratchet.
4. **The unbuilt-level sentinel.** U23 forces a check to reach each unbuilt level with its trigger true and expects exit 70 naming the rule. A mutant that lets the check pass on must turn the test red.
5. **No subject ever runs** (KTD21). Two tests cover this:
   - a sentinel `bin/subject` that writes a file when run must leave no file after a released `grade`;
   - a positive control runs the sentinel directly and sees the file.
6. **The trace.** For `case-003` and `case-004`, zero served keys and zero homes, since confinement was not applied or the self-test failed (KTD2.6). For each derivation-only case, no home.
7. **Independent decoding.** The runner keeps `encoding/json` with `UseNumber` for records, so a reader defect in `internal/strictjson` cannot cancel out (KTD2.3).

## Questions for the review

- Is the derivation audit's hand classification acceptable as the prediction's input, given that it is pinned by digest and the outcome trace (control 3) checks it?
- Is each of the ten ledger moves a whole-rule proof, or does any rule have a clause this slice cannot reach?
- Does control 3's per-case exit and rule record close the gap a one-sided "still fails" check leaves?
