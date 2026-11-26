# Design note: PR8, slice a (the subject file and placeholders)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR8. **Units:** U39, U9, U10.

This note follows the thin slice's design note, `docs/plans/2026-11-25-phase3-pr6-thin-slice-design-note.md`. That note's definitions still hold here: the five kinds of engine fault, the prediction script, the controls and the readings. This note says only what slice a changes.

## What the slice makes true

- **The whole required set (U39).** The engine fixes every check's preimage, id and required entry for every judge-checked case (QE2, QE3, QE7). That covers:
  - every preimage kind and every invocation key, computation keys included;
  - QS5's candidate checks;
  - QA7's exclusion of destructive runbooks outside that scope;
  - QA3's crediting, with `credit-and-routes.md` §2;
  - QA9's `asserts_emptiness` and preconditions.

  No check ends at a planning gap any more, except at a baseline (QE4), which slice e builds.
- **Every subject-file rule (U9).** Every rule of QS1 to QS12 rejects what it must before anything runs, with exit 2 on GKD8's line naming itself. So do the rejections QP7, QP10, QA4 and QA9 make.
  - Errata E-9 orders the rules.
  - QS8's home is the account's home from the user record (E-8).
  - A link in the path that names the seed is resolved, and only a link or special file inside the seed rejects the file (E-17, ruled by the owner on 2026-11-25).

  PR6's interim exit-70 safe-read refusals become these rules' exit-2 refusals.
- **Placeholders and `pql create` (U10).** Level 2 of the precedence table (QS5) is built, and so is QS5's final override. `pql create` writes a subject file with a placeholder for every value it cannot know, and never runs the subject (GKD11).

PR6's stated limit ends here: a file that breaks a subject-file rule is now refused, never graded.

## Evidence rules

Two levels of the precedence table are built: level 1 (QO1, unchanged from PR6) and level 2 (QS5). A check that depends on a placeholder is not observed with `placeholder_unfilled` at level 2.

QS5's final override comes "last of all" (`collection-rules.md` line 131), after level 8. Building it now cannot let a check pass an unbuilt level, because a check reaches the override only after it has passed every level through 8. Until level 8 exists, the override is proved by unit tests.

## Predicted ratchet set

`scripts/predict-ratchet.py --built scripts/ratchet/pr8.json` computes it into `scripts/ratchet/pr8-prediction.json`, which `check.sh` regenerates and compares. PR6's built file and its prediction stay unchanged, and they reproduce exactly in the script's tests.

**Predicted:** 35 cases. These are the 9 PR6 predicted, plus 26 more:

| Cases | Deciding rule |
|---|---|
| `case-060` | QS5, at level 2 |
| `case-090` | QO1, at level 1, now that U39 builds its required set |
| `case-104`, `case-105`, `case-106` | QS6 |
| `case-107`, `case-108`, `case-109` | QS7 |
| `case-110` | QS8 |
| `case-111`, `case-112` | QS9 |
| `case-113` | QS10 |
| `case-114`, `case-115` | QS11 |
| `case-117` | QS12 |
| `case-118` | QP7 |
| `case-119` to `case-122` | QP10 |
| `case-123`, `case-124` | QA4 |
| `case-125` to `case-128` | QA9 |

Both cases the plan names for this slice, `case-060` and `case-090`, are predicted. Every derivation-only case in the corpus is predicted; none stays pending. The derivation-only cases no longer rest on PR6's hand audit of which part of a rule a file breaks, because every rule is now built whole.

### Every pending case's predicted ending

All 114 pending judge-checked cases end with exit 70. The 3 real-verifier cases are not run.

| Ending (exit 70, one of) | Cases |
|---|---|
| QA7, QD4, QH8 (unbuilt levels) | 80 |
| QD4 | 23 |
| QD4, QH8 | 4 |
| QE4 (a baseline's planning gap) | 6 |
| `QE4.anchor` | 1 |

No case ends at a key gap or a record gap. Every planning gap except QE4's is now built.

## Rules this slice moves off KTD22's list

A rule leaves the list only when a test proves its whole text, with its mutation recorded in the pull request or its review log.

| Rule | Proof |
|---|---|
| QS3, QS5 to QS12 | U9's fixtures, one per rejecting clause, written from the specification's text. Each exits 2 on GKD8's line naming its rule, quoting no subject-file value. QS5 also needs U10's level-2 and final-override tests |
| QE2 | U39: every preimage kind equals `preimages.json` for every judge-checked case, and QE2's four worked ids hold |
| QE3, QE7 | U39: for every judge-checked case, the required set and each required entry equal `input.json` |

**Stay listed,** because the slice builds only part of each:
- **QP7, QP10, QA4, QA9:** their rejections are built, but their probe, computation and predicate clauses come with slices b2 and c.
- **QA3, QA7:** crediting and exclusion are built, but their outcome clauses come with slice c.
- **QT5:** all keys are built, but its observation rules come with later levels.

The build may find a rule above that a test cannot prove whole. If so, the rule stays listed and the review log says why.

## Controls

1. **The ratchet.** All 35 predicted cases leave `internal/corpusrunner/pending.txt` and must pass; every other case must still fail.
2. **Every check's ending.** PR6's in-process test and the runner's pending-ending check extend to this slice's prediction file. For every case, the required set read from the trace equals `input.json`'s `required`. That is KTD2.7's `required` layer, which U39 switches on.
3. **One fixture per rule clause (U9).** Each refusal is tested from a fixture that a positive control in the same test shows to be accepted, varying only the clause under test (`CODING_STANDARDS.md`, Tests). A mutation that disables each clause turns its row red. A file that breaks two named rules names the first in E-9's order.
4. **QS5 from the specification, never from `create` (AE20).** `create` then `grade`, through the scripted build with a passing host and a recording, gives `placeholder_unfilled` exactly where QS5 says. The expected side is written from QS5's text. Each kind of placeholder also gets a unit test of QS5's outcomes.
5. **`create` never runs the subject.** A sentinel binary that writes a file when run must leave no file after `create`, and a positive control runs it directly and sees the file.
6. **Literals near QS11's bound.** Each side of 64 KiB is tested in RFC 8785 form (U9).
7. **E-17.** A link in the seed's name is resolved and accepted, and a link inside the seed is refused under QS9. Both cases share one fixture.

## Stated limits of this slice

- QA5's warning appears on the terminal, in the help of `grade` and `anchor`, and in the README. Its exact text is tested; its placement in the README is reviewed by eye.
- QS5's final override is reachable in the corpus only once level 8 exists (slices b2 and c). Until then, unit tests prove it.
