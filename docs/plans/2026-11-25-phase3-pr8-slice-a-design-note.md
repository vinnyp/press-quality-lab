# Design note: PR8, slice a (the subject file and placeholders)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR8. **Units:** U39, U9, U10.

This note follows the thin slice's design note, `docs/plans/2026-11-25-phase3-pr6-thin-slice-design-note.md`. Its definitions hold here: the kinds of engine fault, the prediction script, the controls and the readings. This note says only what slice a changes. Round 1 of its design review is logged in `docs/agent-reviews/2026-11-25-phase3-pr8-design-note-reviews.md`.

## What the slice makes true

- **The whole required set (U39).** The engine fixes every check's preimage, id and required entry for every judge-checked case, as QE2, QE3 and QE7 give them. That includes:
  - every preimage kind and every invocation key, computation keys among them;
  - QS5's candidate checks;
  - QA7's exclusion of destructive runbooks outside that scope;
  - QA3's crediting, as `credit-and-routes.md` §2 sets it;
  - QA9's `asserts_emptiness` and preconditions.

  No check ends at a planning gap any more, except at a baseline (QE4), which slice e builds.
- **Every subject-file rule (U9).** Each of QS1 to QS12, and the rejections that QP7, QP10, QA4 and QA9 make, rejects what it must before anything runs, with exit 2 on GKD8's line naming itself:
  - errata E-9 orders them;
  - QS8's home is the account's home from the user record (E-8);
  - a link in the path that names the seed is resolved, and of links on that path or within the seed, only a link the seed contains rejects the file, while QS9's other rejections stand (E-17);
  - a named file that is regular and inside the directory but cannot be read rejects the file under QS8 with exit 2 (E-18).

  The owner ruled on E-17 and E-18 on 2026-11-25. PR6's interim exit-70 safe-read refusals become these exit-2 refusals.
- **Placeholders and `pql create` (U10).** U10 builds:
  - level 2 of the precedence table (QS5), completing it for every position QS5 lists, an authored step's command with a placeholder class or `writes` among them;
  - QS5's final override for a check that depends on a placeholder.

  `pql create` writes a subject file and never runs the subject (GKD11). QS5's other override trigger, a discovered command the file does not declare (QD3), needs discovery's outcome and comes with U11 in slice b1.

PR6's stated limit ends here: a file that breaks a subject-file rule is now refused, never graded.

## Evidence rules

Two levels of the precedence table are built: level 1 (QO1, unchanged from PR6) and level 2 (QS5). A check that depends on a placeholder is not observed with `placeholder_unfilled` at level 2.

QS5's final override comes "last of all" (`collection-rules.md` line 131). Its input is a check that level 8 would otherwise hold, and level 8 is not built. A check reaches the override only once it is held at level 8, so building it now cannot let a check past an unbuilt level. Until level 8 exists, unit tests prove these branches:
- held on an always-applicable dimension becomes `placeholder_unfilled`;
- not held stays not held;
- held on another dimension stays held;
- the observed precondition does not matter;
- with no placeholder, held stays held;
- the override stops no runbook (QA7).

The prediction script splits the override into two tokens, `QS5.final_override` (U10's placeholder trigger) and a QD3 trigger for U11, so a later slice cannot credit one trigger to the other.

## Predicted ratchet set

`scripts/predict-ratchet.py --built scripts/ratchet/pr8.json` computes it into `scripts/ratchet/pr8-prediction.json`, which `check.sh` regenerates and compares. PR6's built file and prediction stay unchanged and reproduce exactly in the script's tests.

**Predicted:** 35 cases, the 9 PR6 predicted plus 26 more:

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

Both cases the plan names for this slice are predicted. Every derivation-only case is predicted, and none stays pending. Each derivation-only prediction still rests on the hand audit's claim that the file breaks no rule earlier in E-9's order. A wrong audit fails loudly through the two-sided ratchet; it cannot show green.

### Every pending case's predicted ending

All 114 pending judge-checked cases end with exit 70. The 3 real-verifier cases are not run.

| Ending (exit 70, one of) | Cases |
|---|---|
| QA7, QD4, QH8 (unbuilt levels) | 80 |
| QD4 | 23 |
| QD4, QH8 | 4 |
| QE4 (a baseline's planning gap) | 6 |
| `QE4.anchor` | 1 |

No case ends at a key gap or a record gap. The required set is fixed and traced before the QE4 gap is reached, so the runner's `required` layer covers those 7 cases too.

## Rules this slice moves off KTD22's list

A rule leaves the list only when a test proves its whole text. The mutation below is fixed now, before the build, and the pull request records the test that failed on it.

| Rule | Proof | Its mutation |
|---|---|---|
| QS3 | U9's fixture per clause | accept a file without `commands` |
| QS8 | U9's fixtures: an escape, a directory, an unresolved link, a special file and an unreadable file (E-18). A fixture whose `$HOME` differs from the account's home tests the home clause both ways (E-8) | read the home from `$HOME` |
| QS10 | U9's fixture per clause | accept two runbooks with one name |
| QS11 | U9's fixtures for size and depth, each side of the 64 KiB bound in RFC 8785 form; `internal/jcs` vectors showing that `0.1` and `0.10` enter a check id as `0.1` and `-0` as `0` | drop the depth count |
| QS12 | U9's fixture per clause | accept the rule's refused input |
| QA5 | the warning's exact text on the terminal and in the help of `grade` and `anchor` (a stated-limit route covers the README, CASES.md) | omit the warning from `anchor`'s help |
| QE2 | every preimage equals `preimages.json` for every judge-checked case; QE2's four worked ids; synthetic fixtures, written from QE2's text, for the shapes the corpus lacks: a byte-bound, exit-codes or store-unchanged probe check with no command (`words` and `budget_seconds` null, `parameters` empty), and a planted-secret probe whose secret is `null` or a placeholder | write `words: []` for a probe with no command |
| QE3, QE7 | the required set and every required entry equal `input.json` for every judge-checked case, and for the synthetic fixtures above | omit a probe that runs no command |
| QA3 | a test per row of `credit-and-routes.md` §2's credit conditions, each where the condition holds and where it fails | credit a check whose condition fails |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QS5 | the QD3 trigger (U11), the scorecard naming each placeholder (U18), and the runbook interaction (QA7, slice c) |
| QS6 | `budget_seconds` on every probe invocation (QO4), `exit_codes: null` (QO5, QP4), a destructive command run only in the destructive scope (QA7), and `writes: false` for the store-unchanged probe (QP6) |
| QS7 | masking in every record and scorecard (QE18, U24), the scorecard showing each setting (U18), and settings never applying to discovery (U11) |
| QS9 | copying the seed by content into each home, and an empty home without a seed (QH14) |
| QP1, QP7, QP10, QA4, QA9 | their probe, computation and predicate clauses (slices b2 and c) |
| QA7 | its outcome clauses (slice c) |
| QT5 | its observation rules (later levels) |

## Controls

1. **The ratchet.** All 35 predicted cases leave `internal/corpusrunner/pending.txt` and must pass, and every other case must still fail. A case that passes outside the prediction is traced to its rule.
2. **Every check's ending, and the `required` layer.** PR6's in-process test and the runner's pending-ending check move to this slice's prediction file. Three Go readers currently name `pr6-prediction.json`, and they switch with it. For every judge-checked case, the required set read from the trace equals `input.json`'s `required` (KTD2.7, switched on by U39).
3. **One fixture per rule clause (U9).** Each refusal is tested from a fixture that a positive control in the same test shows to be accepted, varying only the clause under test (`CODING_STANDARDS.md`, Tests). A file that breaks two named rules names the first in E-9's order.
4. **Placeholders, from QS5's text, never from `create`'s code (AE20).**
   - **Unit tests in isolation.** `decide`'s level 2 and final override, and `plan`'s candidate checks, are tested for each placeholder position QS5 allows, one position at a time: the binary, each fact, `declared_count`, the seed, the store, the secret, and a command's class and `writes`. The class and `writes` row includes an authored step whose command carries the placeholder, so that discovery and unrelated commands are not affected.
   - **`create`'s output.** The build states field by field what `create` writes: the binary named, and a placeholder in every other position QS5 allows. `commands` takes whatever form the schema and QS5 accept. U9 must accept the file, and the build states whether every check of `create` then `grade` ends at level 2. If any check passes level 2, the plan's own rule applies and that test moves to the slice that builds its later levels.
   - **Why an engine that marks everything `placeholder_unfilled` would fail.** `case-061` and `case-062` stay pending: 28 of their checks end at level 2 and one at level 8.
5. **`create` never runs the subject.** A sentinel binary that writes to an absolute path set inside it must leave no file after `create`, and a positive control runs it directly and sees the file.
6. **E-17's resolved seed path**, all rows sharing one fixture:
   - **Accepted:** a link in the seed's name, a link in an intermediate component (`p/seed` through a linked `p`), and a subject directory whose own path contains a link (macOS's `/var` to `/private/var`).
   - **Refused with exit 2 under QS9:** a link the seed contains; a seed-name link whose target leaves the subject directory; a seed-name link whose target is a regular file.
   - Mutation: skip the containment check after resolving.
7. **Level 2 and the runner agree.** U10 completes `internal/decide`'s level 2, and the corpus runner's own placeholder classifier is checked against it, so the two cannot drift. Once U39 fixes every preimage, the level 3 and level 4 sentinels stop accepting a preimage that lacks `probe` or `earlier`: a missing key is an engine fault.

## Stated limits of this slice

- QA5's README placement is reviewed by eye; its text is tested.
- QS5's final override reaches corpus cases only once level 8 exists (slices b2 and c); until then unit tests prove it.
