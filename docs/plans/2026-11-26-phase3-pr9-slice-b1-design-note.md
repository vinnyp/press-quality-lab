# Design note: PR9, slice b1 (discovery and the invocation outcome rules)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR9. **Units:** U11, U44.

This note follows the slice a note, `docs/plans/2026-11-25-phase3-pr8-slice-a-design-note.md`, and the thin slice's before it. Their definitions hold here: the kinds of engine fault, the prediction script, the controls and the readings. This note says only what slice b1 changes. Its design review is logged in `docs/agent-reviews/2026-11-26-phase3-pr9-design-note-reviews.md`.

## What the slice makes true

- **The discovery walk (U11).** Discovery walks help as QD2 says:
  - one form, `<command words> --help`, from the bare binary down, depth first, each command's children in the order its help lists them;
  - an empty home, no offline setting, a 10-second budget;
  - stdout, or stderr when stdout is empty;
  - at most 4,096 requests and 64 words deep.

  It reads command lists as `credit-and-routes.md` §5 says, for Cobra's, urfave/cli's and clap's headings, leaving out `help`, `completion` and hidden commands. Errata E-26 and E-27 apply. A help request that ran lists the commands its output names whatever its exit code, and the walk descends below it. A request that did not run, or whose binary could not start, lists none.
- **When the walk runs.** The walk runs before any generated check is decided past level 2, because QD4 (level 3) and QS5's QD3 trigger both need its result. It runs whatever level later decides discovery's own check, `declared_count` a placeholder included (QO7: "whatever `declared_count` holds"). It does not run, and discovery has exactly one target, the bare `--help` that did not run, when:
  - QO1 holds grade-wide;
  - the binary is a placeholder;
  - the private copy changed (QE8, `collection-contract.md` line 172).
- **How discovery decides (U11).** Each help request's end goes through level 7's per-invocation rules (QO1 to QO4). When any request is decided there, the first such request in QD2's order decides discovery at level 7 (QO7). Every other end of a request, including a crash, a non-zero exit, an engine kill outside the budget and a `SIGKILL` with a memory or code-signing exit detail, is QD5 (QO7, line 163; the plan's KTD11 is amended to match).
  - **QD4** is level 3. It holds only when the walk ran, no help request was decided by QO1 to QO4, QD5 or a bound of QD2 does not decide discovery, and a declared command was not found. It then decides discovery and every other generated check.
  - **QD3 and QD5, and the walk's bounds,** decide discovery at level 8.
- **QS5's QD3 trigger (U11).** A command discovery found but the file does not declare makes every otherwise held check on an always-applicable dimension `placeholder_unfilled`. This holds only when every declared command was found, and it comes after every runbook, as the final override does.
- **The invocation outcome rules (U44).**
  - **Level 5 (QH8).** A check with an invocation that would use a changed private copy is not observed with `harness_fault`.
  - **Level 7 (QO2 to QO5).** QO2 to QO5 decide each invocation's end, for every check but the planted-secret probe. That probe combines its invocations by QO7's rule, which comes with U37 in slice b2.
  - **QO5's accepted codes,** all four of its bullets:
    - an `exit_code` predicate's expected code;
    - the bare-invocation and declared-exit-code probes' codes, as QP2 and QP4 say;
    - an authored check's own `exit_codes`;
    - otherwise, the codes of a successful run of the command (QS6).

    An accepted exit, and every other result, then goes to level 8.
  - **QO4's known-fast condition** reads the grade's control result (QH24). Running that control on macOS is PR11's U29.
  - **A signal before the launcher's marker** is QH6's case (E-28), so level 1 decides it with `network_unverifiable`.
- **KTD13's attribution function (U44).** An untagged function turns an invocation's raw facts into what ended it, as QH21 and QO5 say. It reads one representation, which every source fills:
  - the signal and its name;
  - whether the launcher's marker was seen, from `Markers`. `End.AfterMarker` is derived from the same fact and never set separately;
  - whether the engine sent a kill, whether the budget had expired, and whether the program was still running at the budget;
  - whether the system reported a memory or a code-signing kill.

  The darwin source keeps the raw exit detail as evidence, and an untagged decoder turns it into the memory and code-signing facts. The scripted source fills the same facts from the recording's `by` field, and applies QT4's `kill_attribution` overrides to them. No source-specific file names a decision value (KTD13's guard).
- **The loop serves invocations.** Discovery's walk observes each help request as it goes. For each other check that reaches level 7, the loop observes its invocations in QE8's target order. KTD2.6's trace records every served key.

## Evidence rules

Levels 1, 2, 3, 5 and 7 are built. Level 4 (QA7) and level 6 (QO6, QA4's cut capture) stay unbuilt, as does level 8 for every check but discovery. The planted-secret probe's level 7 stays unbuilt. Its sentinel names QO7, the rule that will decide it, and the prediction carries that name.

## Readings the build settled from the text

- **A declared `help` or `completion` command.** QD2 leaves those names out of the count, so discovery never finds a declared one. QD4 then decides, as for any declared command not found.
- **QD4 and a changed private copy.** With a changed copy, discovery has exactly one target, the request that did not run (QE8). QD4 needs discovery to have "read every help it requested" (`collection-rules.md` line 188), so it cannot hold. QH8 decides.
- **QD4 and a help request decided at level 7.** A request that QO1 to QO4 decide was not read, so QD4 cannot hold. The other generated checks stand on their own invocations. `case-095` shows it: the bare `--help` could not start, discovery has one target, and every other check is decided by its own invocation.
- **QH8 for a check with several invocations.** A changed copy "fails the grade's invocations that would use it" (`confinement.md` line 20). A check is not observed with `harness_fault` when any of its invocations would use a changed copy. A check with no invocation is not decided at level 5.
- **The scripted host's one `private_copy` value.** QT4 gives one value for the check of the private copy before each use. In a scripted run, `changed` means every copy, the subject's and each computation's executable, is found changed at every use.
- **A help request's other ends.** QO7 says "For each help request, QO1 to QO4 decide as for any invocation, and every other end is QD5" (line 163). So a crash, a non-zero exit, an engine kill outside the budget and a memory or code-signing kill on a help request are QD5, never QO5's.
- **A cycle in the help.** QD2 has no cycle rule. A help that lists itself is walked until the 64-word depth bound or the 4,096-request bound stops it, as `case-048` records with 65 requests.
- **QD3's two tests.** QD3 holds when every declared command was found and the count found equals `declared_count`. These are tested separately, so a `declared_count` that differs from the declared list decides by the count.

## Predicted ratchet set

PR9 adds one change to the prediction script. Discovery's level-8 rules (QD2's bounds, QD3 and QD5) and QO7's discovery combination get their own `builds` tokens, and the discovery token gates discovery at level 7 too. Level 7 for the planted-secret probe is gated by a token of its own, which slice b2 builds. Without the split, the script credits `case-095` to this slice, because its one planted-secret check ends at level 7. It also cannot credit `case-061` and `case-062`, whose discovery check is held and then overridden. Each probe's level-8 sentinel names only that probe's own rules, so a discovery rule is never accepted as a pending case's ending.

`scripts/predict-ratchet.py --built scripts/ratchet/pr9.json` computes `scripts/ratchet/pr9-prediction.json`, which `check.sh` regenerates and compares. PR6's and PR8's files stay unchanged and reproduce exactly in the script's tests.

**Predicted:** 37 cases, PR8's 35 plus:

| Case | What decides it |
|---|---|
| `case-061`, `case-062` | 28 checks at level 2, and discovery held at level 8, then made `placeholder_unfilled` by QS5's final override |

These two cases cannot tell a real discovery from one that always says held, because the override hides discovery's result. The evidence of discovery's logic is in pending cases, which control 2 checks check by check.

### Every pending case's predicted ending

All 112 pending judge-checked cases still end with exit 70. The 3 real-verifier cases are not run.

| Ending (exit 70, one of) | Cases |
|---|---|
| QA7, QO6, QO7 (planted secret) and level 8 | 65 |
| QA7, QO7 and level 8 | 7 |
| QA7, QO6 and level 8 | 6 |
| QA7 only | 2 (`case-005`, `case-092`) |
| QO6, QO7 and level 8 | 2 |
| QO7 and level 8 | 22 |
| QO7 only | 1 (`case-095`) |
| QE4 (a baseline's planning gap) | 6 |
| `QE4.anchor` | 1 |

The build regenerates these counts from the script and reports them.

## Rules this slice moves off KTD22's list

A rule leaves the list only when a test proves its whole text. Each mutation below is fixed now, before the build, and the pull request records the test that failed on it.

| Rule | Proof | Its mutation |
|---|---|---|
| QD5 | Unit rows for each end that makes a help unreadable, at the bare binary and below it:<br>• a non-zero exit, where the walk still descends below it (`case-044`'s 6 targets);<br>• an exit 0 with no output on either stream;<br>• a crash by a signal other than `SIGKILL`;<br>• a `SIGKILL` with a memory or code-signing exit detail;<br>• an engine kill outside the budget.<br>Also: the bare help with no section while commands are declared, and QO1 to QO4 against QD5 in both orders, with the level-7 request deciding. Plus `case-044` to `case-046` | treat a help that exits non-zero as readable, so QD5 does not decide |
| QO2 | A unit row from the could-not-start marker with every exit code the program might also use, 0 included. Plus `case-095`'s per-check evidence through the scripted source | read the program's exit code instead of the could-not-start marker |
| QO4 | Unit rows for each combination of "still running" and the known-fast result, with the process-no-longer-running branch tested with fakes, since the scripted source cannot produce it (KTD13). Plus `case-072`'s `over_budget` and `case-073`'s `harness_fault`, compared check by check | ignore the known-fast control |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QD1 | the scorecard's found and missed names (U18) |
| QD2, QD3, QD4 | the scorecard naming the bound, the undeclared commands, both counts and the missing names (U18) |
| QD6 | a stated limit, documented with the scorecard (U18) |
| QE8 | the probes' and computations' targets (slices b2 and c) |
| QH5 | the real launcher's marker on the darwin path (PR11, PR15) |
| QH6 | the real `sandbox-exec` exits on the darwin path (PR11) |
| QH7 | the real launcher writing the could-not-start marker, shown on macOS (PR11). Reading it as QO2 is now built |
| QH8 | making and checking the copy on the real path (PR11, PR15) |
| QH21 | the system's exit detail on the real path (PR11) |
| QH24 | running the control (PR11's U29) |
| QO3 | the next action naming the causes (QE12, U18) |
| QO5 | judging every other result by the check's own predicate (level 8, slices b2 and c) |
| QO7 | the read proof and the planted secret (slice b2) |
| QS5 | the scorecard naming each placeholder (U18), and the runbook interaction (QA7, slice c) |
| QS6 | the destructive scope (QA7) and `writes: false` for the store-unchanged probe (QP6). `budget_seconds` and `exit_codes`, through QO4 and QO5, are now built |
| QS7 | masking in every record and scorecard (QE18, U24) and the scorecard showing each setting (U18). Settings never applying to discovery is now built |
| QT5 | the observation rules of levels 4, 6 and 8 |

## Controls

1. **The ratchet.** `case-061` and `case-062` leave `internal/corpusrunner/pending.txt` and must pass, and every other case must still fail. A case that passes outside the prediction is traced to its rule.
2. **Every check's ending, compared in full.** The readers that name `pr8-prediction.json` switch to `pr9-prediction.json`. For every check whose ending the prediction gives as decided, the engine's result must equal that check's evidence in `input.json`: outcome, reason, `failure_kind` and target ids. A check that stops at a sentinel must name a rule its probe's sentinel allows.
3. **The trace.** The expected served keys come from the case, never from the engine:
   - when the walk runs, discovery's help requests in the case's `case.json` invocations, in QD2's order;
   - when it does not, no discovery key, and one target that did not run;
   - for each other check the prediction decides at or past level 7, its targets.

   A pending case's trace must hold those keys up to its first sentinel.
4. **Discovery's rules from the text, one row each:**
   - each generator's headings, including a Cobra group heading ending in ` Commands:`, several sections in one help, and a section ending at an empty line or a line with no leading space;
   - a trailing comma removed from a command's first word;
   - each hidden marker, `(hidden)`, `[hidden]` and `HIDDEN`, with and without an alias, after a tab, and not when it follows a single space or comes later in the description (E-27);
   - help on stderr when stdout is empty, and stdout alone when both have output;
   - `help` and `completion` left out, and never asked for help;
   - the depth bound and the request bound, each one past and one at its limit;
   - a help that lists itself, stopped by the depth bound after 65 requests;
   - a headingless help below the bare binary with a declared command under it (E-26);
   - a missing declared command (AE27).
5. **QD4 against the other deciders,** in pairs that test each order the text sets:
   - QD5 or a bound decides discovery while the walk also missed a declared command: the other generated checks stand on their own invocations (QD4's second sentence);
   - a help request below the bare binary is decided at level 7 while a declared command is missing: QD4 does not hold;
   - `declared_count` is a placeholder and a declared command is missing: the walk runs, and QD4 decides the other generated checks.
6. **The attribution function.**
   - **Unit rows** take their expected values from QH21's and QO5's text. They cover every raw combination the text distinguishes, including those the scripted source cannot produce: an engine kill outside the budget, a budget kill of a process no longer running, a `SIGKILL` with a memory or code-signing exit detail, and the exit-detail decoder.
   - **A seam check** feeds the scripted source's facts for every recorded invocation through the function. It compares the result with the recording's `by` field, or with the host's `kill_attribution` where one replaces it (`case-078`, `case-079`, `case-089`). This shows the reader and the function agree, not that either is right; the unit rows do that.
7. **QS5's QD3 trigger** is tested in three cases:
   - every declared command was found and one more was found: it triggers;
   - one declared command was missing: QD4 decides instead;
   - QD5 or a bound decides discovery while one more was found: the trigger needs only the found commands, so it applies whenever every declared command was found.

   The override stops no runbook.

## Stated limits of this slice

- The known-fast control, the system's exit detail and the private copy's per-use check run only on macOS (PR11). Until then the scripted host supplies them, and unit tests cover the branches.
- No corpus recording has a help request that is killed, timed out or crashed. QO7's discovery rule for those ends is proven by unit tests.
