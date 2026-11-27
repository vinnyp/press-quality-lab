# Design note: PR13, slice d (scope, safety classes and destructive runbooks)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR13. **Unit:** U14.

This note follows the slice c note, `docs/plans/2026-11-27-phase3-pr12-slice-c-design-note.md`. Its definitions and controls hold here, among them control 2's full comparison and control 3's case-derived, ordered trace. This note says only what U14 changes. Its design review is logged in `docs/agent-reviews/2026-11-27-phase3-pr13-design-note-reviews.md`.

## What the slice makes true

- **A destructive runbook runs in a scope that includes `destructive`** (QS6, QA7, QA8). PR12's level-4 sentinel naming QA8 is removed. Such a runbook's checks then go through the precedence table like any other runbook's: levels 1 to 3, then level 4's `earlier_step_failed`, and so on.
- **A fail-closed guard replaces the sentinel.** Today the sentinel is the loop's only brake on a destructive step, and `plan.go`'s exclusion is the only other gate. So the plan now records, on each runbook, whether the requested scope permits a destructive step:
  - **What the plan records.** `Runbook.Destructive` is true when any step's command is `destructive`. `Plan.DestructivePermitted` is true when the requested scope includes `destructive`.
  - **The loop's guard.** Before serving any key of a runbook, the loop asks `decide.RunbookMayRun(plan, runbook)`, which gives exit 70 for a destructive runbook when the scope does not permit it. A plan builder that forgot the exclusion would then stop the grade, never run the command.
  - **No new judgment in `collect`.** The plan's exclusion stays where U39 built it: a destructive runbook outside a destructive scope is not in the plan at all, and none of its checks is in the required set (QA7, QE3). The guard is defence in depth, and `collect` still judges nothing (KTD11).
- **Scope as a token and as a dimension are one test.** QS6, QA7 and QA8 say "a scope that includes `destructive`", and the plan tests whether the scope includes `destructive-guard`. They agree, because only the `destructive` token includes that dimension (`internal/rulebook/scope.go`), and an unknown token includes nothing (E-7). The permission is computed once, by that test, and both the plan and the guard read it.
- **A help request is not running the command.** Discovery asks every listed command for help, `remove --help` included, in every scope. QD6 says a help request is not the command's action, and QS5 lets a placeholder-class command have its help during discovery. The corpus records `remove --help` in every case, `case-001` included. So the QS6 gate applies to a command's invocation as a probe or a runbook step, never to its help.
- **An unclassified command is never invoked** (AE24): a command whose class is a placeholder (QS5), and a command discovery found that the file does not declare (QD3). Both are already true. U14 adds the named tests the plan asks for, through the served keys (KTD2.6): `case-022` serves only `export --help`, and `case-063` serves only `account --help`, while the `account` probe checks end `placeholder_unfilled`.
- **QE13: `network_attempted` is never reported.** The engine's reasons become a closed set in `internal/decide`. `network_attempted` is not in it, and the record builder refuses any reason outside it with exit 70. The corpus's QO3 cases, `case-080` to `case-087`, already give `network_unverifiable`, which is the positive half.
- **QE12's table, as data.** The sealed table of next actions is written into `internal/record` as data, one row per reason. A test checks two things: every reason in the engine's closed set has a row, and every row's reason is one the engine or the judge reports. The stored record's `next_actions` stays empty until U18 fills it and the scorecard shows it (KTD16's interim rule). QE12 stays listed until then.

## Readings the build settled from the text

- **`has_destructive_ops` against class.** QS6, QA7 and QA8 gate a destructive command on its class and the scope only. So in a destructive scope the engine runs a destructive runbook even when the subject fact `has_destructive_ops` is false or a placeholder. The judge then decides the dimension from the fact (QS4), and the engine reports the check's evidence as it saw it. No case reaches this.
- **"Every reason this specification reports"** (QE12) is the table's eleven rows. The judge also reports `live_not_run` on the steering dimensions, and the record schema allows other reasons. Steering is outside this specification (`credit-and-routes.md` §2, line 47), and no engine path emits the others, so the closed set holds the table's check reasons only.
- **Where a next action's reason comes from.** A dimension's own reason, `scope_not_requested` or `placeholder_unfilled` on an unstated fact, is the judge's (QE14). U18 reads it from the judge's output, and so it is outside this slice.

## Predicted ratchet set

PR13's built set is `scripts/ratchet/pr12.json` with `QA8.destructive_runbook` added to `builds`. The script needs no other change: the token is already modelled. `scripts/predict-ratchet.py --built scripts/ratchet/pr13.json` computes `scripts/ratchet/pr13-prediction.json`. `check.sh` and the Go readers switch to it, and the earlier files reproduce in the script's tests.

**Predicted:** 142 cases, PR12's 140 plus `case-002` (exit 0) and `case-059` (exit 4). Both decide at level 8. The plan's "QE13 cases the design note computes" adds none: `case-080` to `case-087` are already among PR12's 140.

- **`case-002`.** Its destructive runbook is one step, `remove gear` without `--confirm`. The recording refuses it with exit 3 and a standard-error message naming `--confirm`, so the `destructive-guard` check is held, and every required check is held.
- **`case-059`.** It adds a `destructive-guard` check on a step whose command is not destructive. That check holds but does not credit (`credit-and-routes.md` §2: "the step's command is `destructive`"), which proves the credit condition.

| Cases | Ending |
|---|---|
| `case-069`, `case-070`, `case-143` | excluded, the real verifier |
| `case-071` | exit 70, `QE4.anchor` |
| `case-130` to `case-134`, `case-137` | exit 70, QE4 |

## Rules this slice moves off KTD22's list

| Rule | Proof | Its mutation |
|---|---|---|
| QA8 | `case-002` and `case-059`, plus unit rows: a destructive runbook in scope runs, and out of scope it is not in the required set | run a destructive runbook in a scope without `destructive` |
| QE13 | The closed-set guard, plus `case-080` to `case-087` | add `network_attempted` to the closed set and emit it for a QO3 kill |

**Stay listed:**

| Rule | Still lacks |
|---|---|
| QS6 | the real path running a `safe` command under full confinement (PR15) |
| QA7 | the shared home on the real path (PR15) |
| QE12 | the stored record's `next_actions` and the scorecard (U18) |

Their mutations are fixed now:

| Rule | Its mutation |
|---|---|
| QS6 | let a probe take a `destructive` command |
| QE12 | drop a row from the table |

## Controls

1. **The ratchet.** `case-002` and `case-059` leave `internal/corpusrunner/pending.txt` and must pass. Every other pending case must still fail at its predicted ending.
2. **Every check, compared in full** (control 2).
3. **The trace** (control 3). The reader's hard-coded "no key of a destructive runbook is served" becomes a rule derived from the case. When the case's requested scope lacks `destructive`, no key of a runbook with a destructive step is served. When it has `destructive`, the runbook's keys are served by the rules every runbook follows. `case-002` and `case-059` then expect `remove gear`, and every conformance-only case expects only `remove --help`.
4. **Unit rows for every branch the corpus lacks:**
   - **A mixed runbook:** one whose destructive step is not its first, in scope and out of it, and a destructive step after a not-held step, which is `earlier_step_failed`.
   - **A destructive-guard check that is not held:** an exit 0, and a standard error without the message.
   - **A destructive runbook under a grade-wide QO1 condition** (E-15), and with a placeholder in its step's command.
   - **The guard:** a plan built without the exclusion, which must give exit 70 before any key is served.
   - **Scope at the command line:** `--scope destructive` alone, an unknown token and a repeated token each give exit 2 with nothing served (E-7, X1).
   - **The fact:** class `destructive` with `has_destructive_ops` false, in a destructive scope.
   - **The closed set:** a reason outside it, refused with exit 70.
5. **Mutations.** Each fixed mutation above turns a row or case red. So does each of these: removing the guard while leaving the plan's exclusion broken, and serving `remove gear` in `case-001`.

## Stated limits of this slice

- **QA8's text.** The engine does not test that an author's `destructive-guard` check really omits the confirmation (`credit-and-routes.md` §2, line 49). A destructive runbook that passes the confirmation would really delete inside its throwaway home in a destructive-scope run. Only the home is at risk, and the confinement still holds.
- The real path's runs of a destructive step wait for PR15.
