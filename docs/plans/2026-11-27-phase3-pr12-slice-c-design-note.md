# Design note: PR12, slice c (authored checks)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR12. **Units:** U13, U26, U27.

This note follows the slice b2 note, `docs/plans/2026-11-27-phase3-pr10-slice-b2-design-note.md`, and the slice b1 note before it. Their definitions and controls hold here, among them:
- control 2, the full comparison on QE19's fields and the deciding level;
- control 3, the exact, case-derived trace.

This note says only what slice c changes. Its design review is logged in `docs/agent-reviews/2026-11-27-phase3-pr12-design-note-reviews.md`.

Slice c builds the three levels of the precedence table still unbuilt: level 4 (QA7), level 6 (QO6, QA6 and QA4's cut capture), and level 8 for authored checks (QA9). The work is all untagged code (KTD13): the scripted source serves recorded runs, and the darwin source joins in PR15.

## What the slice makes true

### Runbooks (U27)

The loop runs each runbook of the required set step by step, in the order written (QA7):

1. **Has the runbook stopped?** Before a step, the loop asks `decide` whether the runbook has stopped. It passes the resolutions of every earlier step's required checks. A step is not held when any of its required checks is not held or not observed. These are resolutions *before* QS5's final override and the QD3 override, which run after every runbook, so they stop no runbook (QA7's last sentence; `case-022`, `case-063` to `case-066`). `decide` owns that judgment, so `collect` performs no rollup.
2. **Before serving.** For each required check of the step, levels 1 to 5 are tried with no invocation, then level 6's cut-capture rule. Level 4 reads the earlier resolutions. An id in `earlier` that has no resolution is a check outside the required set, and it never stops a runbook (QA9).
3. **Serving.** The step's invocation is served once, when at least one of its checks passed step 2. Every check of the step then judges that one run (QA7). After it, each check that passed step 2 has its own computation keys served, in the order `plan.Check.Keys` gives them.
4. **Resolving.** Each check is resolved from levels 1 to 8 over its evidence: the step's run, its own computation runs, and the run of any earlier step it captures.
5. **A step with no required check** still runs in its place (E-49), so the shared home and its captures are what the author wrote. That holds unless the runbook has stopped, a grade-wide QO1 condition holds, or its command depends on a placeholder (QS5). Such a step is never held or not held, so it never stops the runbook.
6. **The output** keeps the plan's check order. Only the order of serving changes.

The plan carries each runbook's steps: each step's key, which is built as `plan.go` builds a check's step key today, and the indices of its required checks. A step all of whose checks are outside the required set has no `plan.Check`, so this list is what lets the loop run it.

**A destructive runbook** in the required set is not run in this slice. Its checks stop at a level-4 sentinel naming QA8, so the grade exits 70. U14 builds it in PR13, which owns `case-002` and `case-059`.

### Computations (U26)

**Level 6** decides in this order. Every step gives `computation_failed` except the first, and a check passes level 6 only when none applies:

| Order | Condition | Outcome |
|---|---|---|
| 1 | a capture names a step whose standard output the engine cut (QA4, QE10); the computation is never served | `harness_fault` |
| 2 | the precondition's or the expected value's computation was unconfined, could not start, or ended by any signal: a kill by its confinement, by the engine at its budget or outside it, and one with a memory or code-signing detail included (QO6). An executable that is absent at run time is QO6's "could not start" (E-23) | `computation_failed` |
| 3 | it exited with a code other than 0 (QA6; `case-052` records a valid value with exit 1) | `computation_failed` |
| 4 | its printed value is invalid: more than 1 MiB, or anything other than exactly one JSON value §3 accepts inside a record (QA6, E-12). In a recording, this is `printed_value` `null` (QT5) | `computation_failed` |
| 5 | the expected value's type is wrong for its predicate: not a whole number for `exit_code`, `stdout_bytes_at_most` or `row_count` (QA9); not a string, or the empty string, for the four string predicates (E-48, E-50) | `computation_failed` |

QO6's second bullet is level 1's, as for every check (E-15).

**In the final resolve,** a check with a computation and no served run for it is an engine fault, exit 70, never a pass.

**The precondition (QA9):** its observed value is `true` when it printed `true` and `false` for any other value. The check's outcome is still its predicate's. `precondition_observed` is set for a held or not-held check, a `wrong_exit` included (QE8).

**Real-path pieces** are built here as pure, untagged functions and joined to the darwin source in PR15:
- **QA6's parse.** It takes the drained bytes, the cut flag and the count, and gives the value or invalid. It applies the 1 MiB bound ("more than", so exactly 1 MiB is accepted), §3's reader and E-12's depth. The depth allowance is 64 minus the judge input's nesting at the place the engine writes a computed value in an evidence item's `expected`. That place is one named constant, derived from the record the engine builds.
- **QA4's substitutions.** They replace `{SEED}`, `{SUBJECT_DIR}` and `{CAPTURE:<step>}` in every argument after the first, by E-25's token grammar. The first argument is never substituted.

The scripted source needs neither: it serves the recorded value. Capture files, the scratch directory as working directory and home, and the shared home are the darwin source's, in PR15.

### Predicates (U13)

Level 8 decides an authored check in this order:
1. **A cut stream.** A predicate other than `stdout_bytes_at_most` that needs a cut stream is `harness_fault`. Standard error is the stream for `stderr_contains`, and standard output for the rest. `exit_code` needs no stream.
2. **The predicate,** as QA9's table says. Every predicate reads drained bytes, never decoded text:
   - **`exit_code`** holds at level 8 whenever it is reached. Level 7 has already decided a differing code as `wrong_exit` (QO5), and the corpus agrees: 369 checks, none not held with `predicate`.
   - **`stdout_equals`, `stdout_contains`, `stdout_not_contains` and `stderr_contains`** compare bytes with the expected string's UTF-8 bytes.
   - **`stdout_json_equals`** reads standard output as one JSON value with `internal/strictjson` under §3. It compares by QA9's equality: numbers exactly by value, never through a float, strings by code points with no normalisation, arrays in order, and objects by keys in any order.
   - **`json_path_equals` and `row_count`** resolve `path` as an RFC 6901 pointer: the empty pointer, `~0` and `~1`, an array index with no leading zero, and `-` naming nothing. A path naming nothing, or output that is not one JSON value, makes either predicate not hold. A `row_count` path that names a non-array also does not hold.
   - **`stdout_bytes_at_most`** compares the drained byte count, so a cut stream's full count decides (`case-153`).
   - **`stdout_empty`** holds on zero drained bytes.
3. **`target_size`.** An empty-satisfiable check that is held, or not held with `predicate`, reports the target's size:
   - the bytes it evaluated for `stdout_not_contains`;
   - the drained count for `stdout_bytes_at_most`;
   - the array's items for a computed `row_count`, or `null` when there is no array at `path` (QE8).

   Otherwise it reports `null`.

**The expected value** is the literal, or the computation's printed value (QA2). Level 7's accepted code for an `exit_code` check is that value too (see the first defect below).

**E-50 in the subject rules.** A literal empty string for the four string predicates rejects the file with exit 2. The check sits in QA9's place in E-9's order, beside the type rules U9 built.

## Three defects this slice must not inherit

- **`acceptedCode` and a computed expected code.** `internal/decide/level7.go` reads an `exit_code` check's code only from `expected.literal`. With a computed value, the code is `nil` and every exit would be `wrong_exit`. It must read the expected value as QA2 gives it, through the same accessor level 8 uses. No corpus case reaches this, so a unit row does.
- **Serving per check.** `collect` serves every key of every check. Two checks of one step would observe the step twice, and a real source would run it twice. A step is served once, and its checks share the run.
- **Evidence fields.** `internal/grade/evidence.go` sets `target_size` only for the byte bound, and never sets `precondition_observed`. Both are set as above.

## Readings the build settled from the text

- **Level 4 reads pre-override resolutions** (QA7's last sentence). The corpus fixes it: in `case-063` to `case-066` and `case-022`, later steps hold after an override has made an earlier check `placeholder_unfilled`.
- **Level 4 against levels 5 and 6.** A check in a step after a not-held step is `earlier_step_failed` before a changed copy or a cut capture is considered (`QA7>QH8`; `case-005`, `case-092`, `case-152`).
- **Inside level 6, a cut capture comes first,** because QA4 says such a computation "is not run" (`case-153`).
- **A capture of a step that did not run because of a placeholder** (E-49's exception) depends on that placeholder. Under QS5's "nothing that depends on a placeholder runs", its check is not observed with `placeholder_unfilled` at level 2.
- **`stdout_json_equals` and §3's depth.** The predicate reads standard output "under the judge contract's §3", with no "inside a record". So its depth counts from the output's own top level. E-12's record nesting applies to a computation's printed value only, by QA6's words.
- **QA6's "absent"** is an executable missing at run time. E-23 says such an executable "fails at run time under QA6", and QO6 then decides it as `computation_failed`.

## Owner rulings

These are recorded in `docs/specs/conformance-collection-1.0-errata.md`, and no corpus record changes:
- **E-48:** a computed value that is not a string, for a string predicate, is `computation_failed`.
- **E-49:** a step with no required check still runs and never stops its runbook.
- **E-50:** a literal empty expected string rejects the file, and a computed one is `computation_failed`.

## Predicted ratchet set

PR12's built set is `scripts/ratchet/pr10.json` with `levels` set to `[1, 2, 3, 4, 5, 6, 7, 8]`.

The script also gains one unbuilt token, `QA8.destructive_runbook`. A check in a destructive runbook that is in the required set needs it, so `case-002` and `case-059` stay pending until PR13, as the plan's U14 says. Without the token, the script credits them, which the research for this note showed.

`scripts/predict-ratchet.py --built scripts/ratchet/pr12.json` computes `scripts/ratchet/pr12-prediction.json`. `check.sh` and the Go readers switch to it. PR6's to PR10's files reproduce exactly in the script's tests.

**Predicted:** 140 cases, PR10's 58 plus 82. The research's 84 newly passing, computed without the token, less `case-002` and `case-059`. The build regenerates the file, and any other difference is a finding against this note. The plan's named cases are among them: `case-001` green with exit 0, the discovery cases, `case-005`, `case-063` to `case-066` and `case-092`.

| Level the new cases exercise | Cases |
|---|---|
| 4 | `case-005`, `case-092` (with level 5), `case-057`, `case-058`, `case-080` to `case-084`, `case-152` |
| 6, a computation failing | `case-051` to `case-056` |
| 6, a cut capture | `case-153` |
| 6, preconditions | `case-093`, `case-094` |
| 6, captures | `case-139`, `case-140`, `case-154`, `case-155` |
| 8, a predicate not held | `case-010`, `case-011`, `case-067`, `case-140`, `case-156`, `case-157` |
| QE11's markers | `case-067` (exit 3), `case-068` (exit 0) |

Most of the 82 pass at level 8 only, with every predicate held. A level 8 that always held would fail only the cases above, so the unit rows under Controls carry the rest.

### Every pending case's predicted ending

| Cases | Ending |
|---|---|
| `case-069`, `case-070`, `case-143` | excluded, the real verifier |
| `case-071` | exit 70, `QE4.anchor` |
| `case-130` to `case-134`, `case-137` | exit 70, QE4 |
| `case-002`, `case-059` | exit 70, QA8 |

## Rules this slice moves off KTD22's list

The rule is unchanged: a rule moves only when its whole text is decision logic this slice builds and proves. Each mutation is fixed now.

| Rule | Proof | Its mutation |
|---|---|---|
| QA2 | Unit rows for a literal and a computed expected value on every predicate type, `exit_code` at level 7 included, plus the corpus | read a computed expected value from `literal` |
| QA9 | Level 8's rows below, one per predicate and per rule of QA9's text, plus the corpus | compare `stdout_json_equals` numbers through float64 |
| QO5 | Rows for each accepted-code source (an `exit_code` check's expected code, literal and computed; a check's own `exit_codes`; the command's codes), and `predicate` from level 8 | accept the command's codes for a check that gives its own `exit_codes` |
| QO6 | Rows for each end of level 6's step 2, attributed through the untagged attribution function, plus `case-051` and `case-055` | make an engine kill outside the budget on a computation `harness_fault` |
| QE11 | `case-067` and `case-068`, and a row with two markers on one runbook | drop a marker's dimension |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QA1, QH8 | the real path's private copies and runs (PR15) |
| QA4 | the substitutions, capture files and scratch directory used by the darwin source (PR15) |
| QA6 | the darwin source calling the parse on drained bytes (PR15) |
| QA7 | the shared home on the real path (PR15). Its sequencing is proven here |
| QA8 | destructive runbooks (PR13) |
| QE8, QT5 | real-path draining and a computation's recorded value from a real run (PR15) |
| QH16 to QH20 | a computation run by the darwin source, and GR15's `macos_evidence` entries for QH16, QH18 and QH20 (PR15) |

Their mutations are fixed now:

| Rule | Its mutation |
|---|---|
| QA4 | substitute a token in the first argument |
| QA6 | accept 1 MiB plus one byte |
| QA7 | serve a step after its runbook stopped |

## Controls

1. **The ratchet.** The 82 cases leave `internal/corpusrunner/pending.txt` and must pass. Every other case must still fail.
2. **Every check, compared in full** (control 2), `target_size` and `precondition_observed` included, with the deciding level.
3. **The trace** (control 3), extended. A case-derived reader, never the engine, gives for each runbook of the required set:
   - each step's key once, in step order, up to and including the first step that is not held, and none after it;
   - a step with no required check, unless E-49's exception applies;
   - each computation key of a check that passes levels 1 to 5 and has no cut capture, after its step's key;
   - no key of a destructive runbook.

   A step served twice fails the test, as does a key served after a stop.
4. **Unit rows for every branch the corpus lacks.** The research found these gaps:
   - **QA7 stopping:** a runbook stopped by `predicate`, `crashed`, `over_budget`, `could_not_start` and `computation_failed`; a level-2 stop followed by a step that does not depend on the placeholder; a check outside the required set in an included runbook; E-49's step, run and skipped.
   - **Level 6:** each QO6 end, a computation that could not start, and a precondition failing as QA6 says. The type rule's integers are checked at 0, 2^53−1 and 2^53, and `-0`, `1.0` and `1e1` are rejected; a computed string and empty string are checked too. Exactly 1 MiB is accepted and 1 MiB plus one byte is not. E-12's depth is checked at the limit and one past.
   - **Level 8:** every predicate not holding; an empty target for `stdout_not_contains` and `stdout_bytes_at_most`; a cut stream against each predicate, standard error included.
   - **JSON Pointer:** RFC 6901 §5's example table, written into the test from the RFC by an independent reference. Added to it: a nested path, a path naming nothing, a non-array under `row_count`, and non-JSON output.
   - **Exactness:** two integers that round to one double, the plan's U13 scenario. A multi-code own `exit_codes`, and one that holds 0.
   - **E-50's file rejection,** one row per predicate.
   - **The `exit_code` defect:** a computed expected code at level 7.
5. **Mutations.** Each rule's mutation above turns at least one row or case red, and the build review records the run.

## Stated limits of this slice

- The real path's substitutions, capture files, scratch directories, shared home and QA6 parse on real bytes are unit-tested here, and joined to a darwin run only in PR15.
- Destructive runbooks wait for PR13.
- No recording holds a cut standard error, or a computation with any end other than an exit, a confinement kill or a budget kill. Those branches are proven by unit rows only.
