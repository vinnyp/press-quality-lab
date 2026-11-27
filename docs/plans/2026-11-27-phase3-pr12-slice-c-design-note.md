# Design note: PR12, slice c (authored checks)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR12. **Units:** U13, U26, U27.

This note follows the slice b2 note, `docs/plans/2026-11-27-phase3-pr10-slice-b2-design-note.md`, and the slice b1 note before it. Their definitions and controls hold here, among them:
- control 2, the full comparison on QE19's fields and the deciding level;
- control 3, the exact, case-derived trace.

This note says only what slice c changes. Its design review is logged in `docs/agent-reviews/2026-11-27-phase3-pr12-design-note-reviews.md`.

Slice c builds the three levels of the precedence table still unbuilt: level 4 (QA7), level 6 (QO6, QA6 and QA4's cut capture), and level 8 for authored checks (QA9). The work is all untagged code (KTD13): the scripted source serves recorded runs, and the darwin source joins in PR15.

## What the slice makes true

### Runbooks (U27)

The plan carries each runbook's steps:
- each step's key, built as `plan.go` builds a check's step key today;
- the indices of its required checks;
- the ids of its checks that are outside the required set.

A step all of whose checks are outside the required set has no `plan.Check`, so this list is what lets the loop run it.

The loop runs each runbook of the required set step by step, in the order written (QA7). Every judgment below is made by a function in `decide`; `collect` only serves keys and passes results back (KTD11).

1. **Has the runbook stopped?** Before a step, `decide.Stopped` is given every earlier step's required-check resolutions. A step is not held when any of its required checks is not held or not observed.
   - **Pre-override.** These are resolutions *before* QS5's final override and the QD3 override, which run after every runbook, so they stop no runbook (QA7's last sentence; `case-022`, `case-063` to `case-066`).
   - **Membership comes from the plan.** An `earlier` id the plan marks as outside the required set never stops a runbook (QA9). An `earlier` id in the required set with no resolution, or one the plan neither requires nor marks as outside, is an engine fault, exit 70, never a skip.
2. **Before serving.** Each required check of the step tries levels 1 to 5 with no invocation. Level 4 is `earlier_step_failed` once the runbook has stopped, and level 5 here is the grade-wide copy fact only.
3. **Serving the step.** When at least one required check passed step 2, the step's invocation is served once. Every check of the step judges that one run (QA7).
4. **Serving computations.** For each check that passed step 2, in check order:
   - levels 1 to 5 are tried again with the step's run in its evidence, so a run without the confinement marker (QO1) or with a changed copy (QH8) serves no computation;
   - level 6's cut-capture rule is decided next, from the captured step's recorded run;
   - a check without a cut capture then has its own computation keys served, the precondition before the expected value.

   Two checks of one step can name equal computations. Such a key is served once and both checks read it. Two computations of one step with equal argument lists that differ in anything else reject the subject file with exit 2 (E-52), a rule the subject checks gain in QA4's place in E-9's order.
5. **Resolving.** Each check is resolved from levels 1 to 8. Its evidence is its own step run and its own computation runs. A captured step's run is kept apart, in `Evidence.Captures`. Only two rules read it: level 1, which makes the check `network_unverifiable` when the captured run was not confined (E-53), and level 6's cut-capture rule. No other level, level 7 above all, ever judges it. A capture with no run in `Evidence.Captures` is an engine fault, exit 70, never read as uncut.
6. **A step with no required check** still runs in its place (E-49), so the shared home and its captures are what the author wrote. `decide.StepRuns` says whether it runs. It does not run when:
   - the runbook has stopped;
   - a grade-wide QO1 condition holds;
   - the private copy changed (QH8);
   - its command depends on a placeholder (QS5).

   Such a step is never held or not held, so it never stops the runbook.
7. **The output** keeps the plan's check order. Only the order of serving changes.

**A destructive runbook** in the required set is not run in this slice. No step of it is served, a step with no required check included. Its checks stop at a level-4 sentinel naming QA8, so the grade exits 70. U14 builds it in PR13, which owns `case-002` and `case-059`.

**Level 2 and captures (E-51).** A check whose computation captures a step whose command depends on a placeholder is itself `placeholder_unfilled` at level 2. `internal/decide/level2.go` and the corpus runner's `levelTwoPlaceholder` both gain that rule.

### Computations (U26)

**Level 6** decides in this order. Every step gives `computation_failed` except the first, and a check passes level 6 only when none applies:

| Order | Condition | Outcome |
|---|---|---|
| 1 | a capture names a step whose standard output the engine cut (QA4, QE10); the computation is never served | `harness_fault` |
| 2 | the precondition's or the expected value's computation was unconfined, could not start, or ended by any signal: a kill by its confinement, by the engine at its budget or outside it, and one with a memory or code-signing detail included (QO6). An executable that is absent at run time is QO6's "could not start" (E-23) | `computation_failed` |
| 3 | it exited with a code other than 0 (QA6; `case-052` records a valid value with exit 1) | `computation_failed` |
| 4 | its printed value is invalid: more than 1 MiB, or anything other than exactly one JSON value §3 accepts inside a record (QA6, E-12). In a recording, this is `printed_value` `null` (QT5) | `computation_failed` |
| 5 | the expected value's type is wrong for its predicate. For `exit_code`, `stdout_bytes_at_most` and `row_count`, it is not a whole number as §3 spells an integer, where `-0` is zero and accepted (QA9). For the four string predicates, it is not a string, or is the empty string (E-48, E-50) | `computation_failed` |

QO6's second bullet is level 1's, as for every check (E-15).

**The cut capture and QO1.** The cut capture is decided after the step's own run is served. So a step run without confinement is decided by QO1 at level 1, ahead of it, as §2's order says.

**The guards inside level 6,** in the final resolve:
- order 1 first looks up every step the check captures in `Evidence.Captures`; a missing run is an engine fault, exit 70, and a cut standard output gives `harness_fault`;
- after order 1, a check with a precondition or expected-value computation and no served run for it is an engine fault, exit 70, never a pass.

**The precondition (QA9):** its observed value is `true` when it printed `true` and `false` for any other value. The check's outcome is still its predicate's. `precondition_observed` is set for a held or not-held check, a `wrong_exit` included (QE8).

**Recorded values in `strictjson` form.** The scripted reader keeps a recorded `printed_value` in `internal/strictjson`'s value types, as recorded, so the predicates' exact comparisons see the numbers' text. It no longer converts them to `encoding/json` types. KTD13's projection test, which writes the source's facts back as a recording, must keep passing.

**Real-path pieces** are built here as pure, untagged functions and joined to the darwin source in PR15:
- **QA6's parse.** It takes the drained bytes, the cut flag and the count, and gives the value or invalid. It applies the 1 MiB bound ("more than", so exactly 1 MiB is accepted), §3's reader and E-12's depth.
  - **The depth limit.** A computed expected value is written directly as the evidence item's `expected`. That puts it inside the judge input's top object, its evidence array and the evidence item, so its own depth may be at most 61.
  - **A precondition's value** is never written, but it is held to the same 61, as the value QA6 bounds "inside a record".
  - **The stored record** nests the judge input one level deeper (`store.Document`), so a 61-deep value reaches level 65 there. The store and its readers (U18, the corpus runner) are not held to §3's 64 for that reason, and a row stores and reads back a 61-deep value.
  - **The constant.** One named constant holds the 61. A test derives it from the evidence the engine builds, so the two cannot drift apart.
- **QA4's substitutions.** They replace `{SEED}`, `{SUBJECT_DIR}` and `{CAPTURE:<step>}` in every argument after the first, by E-25's token grammar. The first argument is never substituted.

The scripted source needs neither: it serves the recorded value. The darwin source owns the rest, in PR15:
- capture files;
- the scratch directory as working directory and home;
- the shared home;
- the plan's U26 scenarios run for real: a computation that pipes its input, and one that runs the subject.

The corpus already covers both scenarios through the scripted source, in `case-050` and `case-051`.

### Predicates (U13)

Level 8 decides an authored check in this order:
1. **A cut stream.** A predicate other than `stdout_bytes_at_most` that needs a cut stream is `harness_fault`. Standard error is the stream for `stderr_contains`, and standard output for the rest. `exit_code` needs no stream.
2. **The predicate,** as QA9's table says. Every predicate reads drained bytes, never decoded text:
   - **`exit_code`** compares the exit code with the expected value. Level 7 should already have decided a differing code as `wrong_exit` (QO5), so a mismatch that reaches level 8 is an engine fault, exit 70. That keeps a level-7 regression from becoming a held check.
   - **`stdout_equals`, `stdout_contains`, `stdout_not_contains` and `stderr_contains`** compare bytes with the expected string's UTF-8 bytes.
   - **`stdout_json_equals`** reads standard output as one JSON value with `internal/strictjson` under §3. Its depth counts from the output's own top level. It compares by QA9's equality: numbers exactly by value, never through a float, strings by code points with no normalisation, arrays in order, and objects by keys in any order.
   - **`json_path_equals` and `row_count`** resolve `path` as an RFC 6901 pointer: the empty pointer, `~0` and `~1`, an array index with no leading zero, and `-` naming nothing. A path naming nothing, or output that is not one JSON value, makes either predicate not hold. A `row_count` path that names a non-array also does not hold.
   - **`stdout_bytes_at_most`** compares the drained byte count, so a cut stream's full count decides (`case-153`).
   - **`stdout_empty`** holds on zero drained bytes.
3. **`target_size`.** An empty-satisfiable check that is held, or not held with `predicate`, reports the target's size:
   - the bytes it evaluated for `stdout_not_contains`;
   - the drained count for `stdout_bytes_at_most`;
   - the array's items for a computed `row_count`, or `null` when there is no array at `path` (QE8).

   Otherwise it reports `null`. A check that an override makes `not_observed` reports `null` for both `target_size` and `precondition_observed`.

**The expected value** is the literal, or the computation's printed value (QA2). One accessor gives it to level 7 and level 8 alike.

**E-50 in the subject rules.** A literal empty string for the four string predicates rejects the file with exit 2. The check sits in QA9's place in E-9's order, beside the type rules U9 built.

## Defects this slice must not inherit

- **`acceptedCode` and a computed expected code.** `internal/decide/level7.go` reads an `exit_code` check's code only from `expected.literal`. With a computed value, the code is `nil` and every exit would be `wrong_exit`. It must read the value through the accessor above.
- **Serving per check.** `collect` serves every key of every check. Two checks of one step would observe the step twice, and a real source would run it twice. A step is served once, and its checks share the run.
- **The printed value's types.** `internal/scripted/reader.go`'s `standardValue` turns a recorded value into `encoding/json` types, which `strictjson.EqualJSON` does not compare. It is kept in `strictjson` form.
- **Captured runs as the check's own.** Level 7 judges every non-computation run in a check's evidence. A captured step's run must therefore never be among them, and `internal/grade/evidence.go` builds `observed` only from the check's own invocations.
- **Evidence fields.** `internal/grade/evidence.go` sets `target_size` and `expected` only for the byte bound, and never sets `precondition_observed`. All three are set for authored checks as above.
- **The trace comparison.** `compareTrace` in `internal/grade/pr9_trace_scripted_test.go` compares sets, so a step served twice passes. It compares ordered lists.

## Readings the build settled from the text

- **Level 4 reads pre-override resolutions** (QA7's last sentence). The corpus fixes it: in `case-063` to `case-066` and `case-022`, later steps hold after an override has made an earlier check `placeholder_unfilled`.
- **Level 4 against levels 5 and 6.** A check in a step after a not-held step is `earlier_step_failed` before a changed copy or a cut capture is considered (`QA7>QH8`; `case-005`, `case-092`, `case-152`).
- **Inside level 6, a cut capture comes first,** because QA4 says such a computation "is not run" (`case-153`). It still comes after level 1, so the step's own run is served.
- **`stdout_json_equals` and §3's depth.** The predicate reads standard output "under the judge contract's §3", with no "inside a record". So its depth counts from the output's own top level. E-12's record nesting applies to a computation's printed value only, by QA6's words.
- **QA6's "absent"** is an executable missing at run time. E-23 says such an executable "fails at run time under QA6", and QO6 then decides it as `computation_failed`.

## Owner rulings

These are recorded in `docs/specs/conformance-collection-1.0-errata.md`, and no corpus record changes:
- **E-48:** a computed value that is not a string, for a string predicate, is `computation_failed`.
- **E-49:** a step with no required check still runs and never stops its runbook.
- **E-50:** a literal empty expected string rejects the file, and a computed one is `computation_failed`.
- **E-51:** a capture of a step whose command depends on a placeholder makes its check `placeholder_unfilled`.
- **E-52:** two computations of one step with equal arguments that differ otherwise reject the file.
- **E-53:** a capture of a step whose run was not confined makes its check `network_unverifiable` at level 1. The build review found this reading.

## Predicted ratchet set

PR12's built set is `scripts/ratchet/pr10.json` with `levels` set to `[1, 2, 3, 4, 5, 6, 7, 8]`.

**The QA8 token.** The script gains one unbuilt token, `QA8.destructive_runbook`. A check in a destructive runbook in the required set needs it, so `case-002` and `case-059` stay pending until PR13, as the plan's U14 says. Without the token, the script credits them.
- **Only when nothing else blocks.** The script applies the token only when no other unbuilt rule already blocks the check. So PR6's to PR10's files reproduce exactly in the script's tests, and their sentinel lists do not gain QA8.
- **The phase-set test** (`scripts/test_predict_ratchet.py`) is widened to allow level 4.
- **E-51 in the script.** Its `depends()` gains E-51's capture rule, so a future case with such a capture is predicted at level 2.

`scripts/predict-ratchet.py --built scripts/ratchet/pr12.json` computes `scripts/ratchet/pr12-prediction.json`. `check.sh` and the Go readers switch to it.

**Predicted:** 140 cases, PR10's 58 plus 82. The design review reproduced this count in a scratch copy, with `case-002` and `case-059` ending exit 70 naming QA8. Any difference in the build is a finding against this note. The plan's named cases are among them: `case-001` green with exit 0, the discovery cases, `case-005`, `case-063` to `case-066` and `case-092`.

| Level the new cases exercise | Cases |
|---|---|
| 4 | `case-005`, `case-092` (with level 5), `case-057`, `case-058`, `case-080` to `case-084`, `case-152` |
| 6, a computation failing | `case-051` to `case-056` |
| 6, a cut capture | `case-153` |
| 6, preconditions | `case-093`, `case-094` |
| 6, captures | `case-139`, `case-140`, `case-154`, `case-155` |
| 8, a cut stream | `case-152` |
| 8, a predicate not held | `case-010`, `case-011`, `case-067`, `case-140`, `case-156`, `case-157` |
| QE11's markers | `case-067` (exit 3), `case-068` (exit 0) |

Most of the 82 pass at level 8 only, with every predicate held. A level 8 that always held would fail only the cases above. The unit rows and the "always held" mutations under Controls carry the rest.

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
| QA9 | Level 8's rows under control 4, one per predicate and per rule of QA9's text, plus the corpus | compare `stdout_json_equals` numbers through float64; and, per predicate, return held |
| QO5 | Rows for each accepted-code source: an `exit_code` check's expected code, literal and computed; a check's own `exit_codes`; the command's codes, as U44 proved. Plus `predicate` from level 8 | accept the command's codes for a check that gives its own `exit_codes` |
| QO6 | Rows for each end of level 6's step 2, attributed through the untagged attribution function, plus `case-051` and `case-055` | make an engine kill outside the budget on a computation `harness_fault` |
| QE11 | `case-067` and `case-068`, and a row with two markers on one runbook | drop a marker's dimension |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QA1, QH8 | the real path's private copies and runs (PR15) |
| QA4 | the substitutions, capture files and scratch directory used by the darwin source (PR15) |
| QA6 | the darwin source calling the parse on drained bytes (PR15) |
| QA7 | its destructive-runbook clause (PR13) and the shared home on the real path (PR15). Its sequencing is proven here |
| QA8 | destructive runbooks (PR13) |
| QS5 | the scorecard naming each placeholder (U18). Its runbook interaction and E-51 are proven here |
| QE8, QE10, QT5 | real-path draining and a computation's recorded value from a real run (PR15) |
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
3. **The trace** (control 3), extended.

   **Its inputs.** The reader derives the served keys from the case alone: the subject file, `input.json`, `preimages.json`, the recording, the host's facts and the prediction's per-check ending. It never uses the engine. It builds each key from the check's preimage, or an E-49 step's key from the subject file, by its own code, never through `plan.go`'s builder.

   **Held before the override** is read from `input.json`'s evidence: an outcome of `held`, or `placeholder_unfilled` with a deciding level of 8, which only QS5's override gives. The prediction's ending alone does not say whether a level-8 check held (`case-152` against `case-153`).

   **Its rules,** for each runbook of the required set:
   - **When a runbook stops.** A runbook has stopped after the first step that has a required check whose prediction, before QS5's override, is not held or not observed. A prediction that ends held at level 8 and is then overridden by QS5 counts as held.
   - **When a step is served.** It is served once, in step order, while the runbook has not stopped, and when at least one of its required checks is decided neither at levels 2 to 4, nor by a grade-wide QO1 condition, nor by the grade-wide copy fact. A check that QO1 decides on its own run still has its step served. A step with no required check is served unless one of E-49's exceptions applies. No step is served after a stop.
   - **When a computation is served.** A check's computation keys are served after its step's key, in check order, precondition first. That happens when the check passes step 2's levels and has no cut capture. A computation key shared by two checks of the step is served once.
   - **Destructive runbooks.** No key of one is served, judged from the subject file's classes.

   The comparison is of ordered lists: the probes' keys in QP1's order, then each runbook's keys in the subject file's order. A step served twice, or a key served out of order or after a stop, fails the test.
4. **Unit rows for every branch the corpus lacks:**
   - **QA7 stopping:**
     - a runbook stopped by `predicate`, `crashed`, `over_budget`, `could_not_start` and `computation_failed`;
     - a level-2 stop followed by a step that does not depend on the placeholder;
     - a check outside the required set in an included runbook;
     - E-49's step, run and skipped for each exception;
     - a required earlier resolution dropped, which must give exit 70;
     - E-51's capture, in both of its forms.
   - **Level 6:**
     - each QO6 end, a computation that could not start, and a precondition failing as QA6 says;
     - the type rule's integers: 0, `-0` and 2^53−1 accepted, and 2^53, `-1`, `1.0` and `1e1` rejected;
     - a computed non-string and a computed empty string;
     - exactly 1 MiB accepted and 1 MiB plus one byte rejected, and E-12's depth at 61 and 62;
     - a cut capture behind an unconfined step run, which QO1 decides;
     - two checks sharing an equal computation, served once.
   - **E-52's file rejection:** equal argument lists differing in budget, in declared files and in captures, each rejecting with exit 2.
   - **Level 8, by predicate:**
     - every predicate holding and not holding, `stdout_bytes_at_most` at, below and one past its bound;
     - an empty target for `stdout_not_contains` and `stdout_bytes_at_most`;
     - a cut stream against each predicate, standard error included;
     - an `exit_code` mismatch reaching level 8, which must give exit 70.
   - **Level 8, by QA9's text:**
     - invalid UTF-8 in standard output, compared as bytes;
     - NFC against NFD strings, which are unequal;
     - numbers beyond binary64 (`1e9999` against `10e9998`) and two integers that round to one double, the plan's U13 scenario;
     - each §3 refusal on standard output: a repeated key, depth 65, a lone surrogate and a 33-character number;
     - a precondition `true`, `false` and `"true"`.
   - **JSON Pointer:** RFC 6901 §5's example table, written into the test from the RFC by an independent reference. Added to it: a nested path, a path naming nothing, a non-array under `row_count`, and non-JSON output.
   - **Own exit codes:** a multi-code `exit_codes`, and one that holds 0.
   - **QA4's substitutions:** each token, a token in the first argument left alone, and E-25's literal forms.
   - **Overrides:** `target_size` and `precondition_observed` cleared when an override makes a check `not_observed`.
   - **E-50's file rejection,** one row per predicate.
   - **The inherited defects:** a computed expected code at level 7; a captured step's non-accepted exit code, which must not decide the capturing check; a recorded number compared exactly after the reader.
5. **Mutations.** Each rule's mutation above turns at least one row or case red, and so does each predicate's "return held". The build review records every run.

## Stated limits of this slice

- The real path's substitutions, capture files, scratch directories, shared home and QA6 parse on real bytes are unit-tested here, and joined to a darwin run only in PR15.
- Destructive runbooks wait for PR13.
- No recording holds a cut standard error, or a computation with any end other than an exit, a confinement kill or a budget kill. Those branches are proven by unit rows only.
