# Design note: PR9, slice b1 (discovery and the invocation outcome rules)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR9. **Units:** U11, U44.

This note follows the slice a note, `docs/plans/2026-11-25-phase3-pr8-slice-a-design-note.md`, and the thin slice's before it. Their definitions hold here: the kinds of engine fault, the prediction script, the controls and the readings. This note says only what slice b1 changes.

## What the slice makes true

- **Discovery decides (U11).** The discovery check walks help as QD2 says:
  - one form, `<command words> --help`, from the bare binary down, depth first, each command's children in the order its help lists them;
  - an empty home, no offline setting, a 10-second budget;
  - stdout, or stderr when stdout is empty;
  - at most 4,096 requests and 64 words deep.

  It reads command lists as `credit-and-routes.md` §5 says, for Cobra's, urfave/cli's and clap's headings, leaving out `help`, `completion` and hidden commands. Errata E-26 and E-27 apply. Discovery then decides by QO7's rule for discovery and by QD3, QD4 and QD5:
  - QD4 is level 3 of the precedence table, for discovery and every other generated check;
  - QD3 and QD5, and the walk's bounds, decide discovery at level 8.
- **QS5's QD3 trigger (U11).** A command discovery found but the file does not declare makes every otherwise held check on an always-applicable dimension `placeholder_unfilled`. This holds only when every declared command was found, and it comes after every runbook, as the final override does.
- **The invocation outcome rules (U44).**
  - **Level 5 (QH8).** A check with an invocation that would use a changed private copy is not observed with `harness_fault`.
  - **Level 7 (QO2 to QO5).** QO2 to QO5 decide each invocation's end, for every check but the planted-secret probe. That probe combines its invocations by QO7's rule, which comes with U37 in slice b2.
  - **QO4's known-fast condition** reads the grade's control result (QH24). Running that control on macOS is PR11's U29.
- **KTD13's attribution function (U44).** An untagged function turns an invocation's raw end into what ended it, as QH21 and QO5 say. Its inputs are:
  - the signal;
  - whether the launcher's marker was seen;
  - any kill the engine sent, and whether it was at the budget;
  - whether the program was still running at the budget;
  - the system's exit detail.

  Every source fills the same raw facts. The scripted source fills them from the recording and the host, and the darwin source from PR11's U30. No source-specific file names a decision value (KTD13's guard).
- **The loop serves invocations.** For each check that reaches level 7, the loop observes its invocations in QE8's target order. Discovery's walk observes each help request as it goes. KTD2.6's trace records every served key.

## Evidence rules

Levels 1, 2, 3, 5 and 7 are built. Level 4 (QA7) and level 6 (QO6, QA4's cut capture) stay unbuilt, as does level 8 for every check but discovery. The planted-secret probe's level 7 stays unbuilt. Its sentinel names QO7, the rule that will decide it, and the prediction carries that name.

## Readings the build settled from the text

- **A declared `help` or `completion` command.** QD2 leaves those names out of the count, so discovery never finds a declared one. QD4 then decides, as for any declared command not found.
- **QD4 and a changed private copy.** With a changed copy, discovery has exactly one target (QE8, `collection-contract.md` line 172), and that request fails at level 5. QD4 needs discovery to have "read every help it requested" (`collection-rules.md` line 188), so it cannot hold. QH8 decides.
- **QH8 for a check with several invocations.** A changed copy "fails the grade's invocations that would use it" (`confinement.md` line 20). A check is not observed with `harness_fault` when any of its invocations would use a changed copy. A check with no invocation is not decided at level 5.
- **The scripted host's one `private_copy` value.** QT4 gives one value for the check of the private copy before each use. In a scripted run, `changed` means every copy, the subject's and each computation's executable, is found changed at every use.
- **A help request that crashes or exits non-zero.** QO7 says "For each help request, QO1 to QO4 decide as for any invocation, and every other end is QD5" (line 163). So such a request is QD5, never QO5's `crashed` or `wrong_exit`.
- **A signal other than `SIGKILL` with no marker.** QH6 covers only `sandbox-exec` exiting, and a `SIGKILL` before the marker (`confinement.md` line 17). Any other signal is not QH6's case, so QO5 decides it: `crashed`.
- **QD3's two tests.** QD3 holds when every declared command was found and the count found equals `declared_count`. These are tested separately, so a `declared_count` that differs from the declared list decides by the count.

## Predicted ratchet set

PR9 adds one change to the prediction script. Discovery's level-8 rules (QD2's bounds, QD3 and QD5) and QO7's discovery combination get their own `builds` tokens. Level 7 for the planted-secret probe is gated by a token of its own, which slice b2 builds. Without the split, the script credits `case-095` to this slice, because its one planted-secret check ends at level 7. It also cannot credit `case-061` and `case-062`, whose discovery check is held and then overridden.

`scripts/predict-ratchet.py --built scripts/ratchet/pr9.json` computes `scripts/ratchet/pr9-prediction.json`, which `check.sh` regenerates and compares. PR6's and PR8's files stay unchanged and reproduce exactly in the script's tests.

**Predicted:** 37 cases, PR8's 35 plus:

| Case | What decides it |
|---|---|
| `case-061`, `case-062` | 28 checks at level 2, and discovery held at level 8, then made `placeholder_unfilled` by QS5's final override |

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

The script computes these counts; the build regenerates and reports them.

## Rules this slice moves off KTD22's list

A rule leaves the list only when a test proves its whole text. Each mutation below is fixed now, before the build, and the pull request records the test that failed on it.

| Rule | Proof | Its mutation |
|---|---|---|
| QD5 | Unit rows for each bullet, at the bare binary and below it: a help that exits non-zero, a help killed or timed out (which QO7 sends through QO3 and QO4 first), and a bare help with no section while commands are declared. Plus the corpus: `case-044` to `case-046` | read a help that exits non-zero as listing its commands for QD5 |
| QO2 | A unit row from the could-not-start marker with every exit code the program might also use, plus `case-095`'s recording through the scripted source | read the program's exit code instead of the could-not-start marker |
| QO4 | Unit rows for each combination of "still running" and the known-fast result. The `harness_fault` branch for a process no longer running is tested with fakes, since the scripted source cannot produce it (KTD13) | ignore the known-fast control |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QD1 | the scorecard's found and missed names (U18) |
| QD2, QD3, QD4 | the scorecard naming the bound, the undeclared commands, both counts and the missing names (U18) |
| QD6 | a stated limit, documented with the scorecard (U18) |
| QH8 | making and checking the copy on the real path (PR11, PR15) |
| QH21 | the system's exit detail on the real path (PR11) |
| QH24 | running the control (PR11's U29) |
| QO3 | the next action naming the causes (QE12, U18) |
| QO5 | the accepted codes of predicates and probes (slices b2 and c) |
| QO7 | the read proof and the planted secret (slice b2) |
| QS5 | the scorecard naming each placeholder (U18), and the runbook interaction (QA7, slice c) |

## Controls

1. **The ratchet.** `case-061` and `case-062` leave `internal/corpusrunner/pending.txt` and must pass, and every other case must still fail. A case that passes outside the prediction is traced to its rule.
2. **Every check's ending, and the trace.** The readers that name `pr8-prediction.json` switch to `pr9-prediction.json`. For each case that runs, the served keys in the trace are exactly the keys this slice observes, in QE8's order. For discovery, the walk's keys equal the help requests its recording holds.
3. **Discovery's rules from the text, one row each:**
   - each generator's headings, including a Cobra group heading ending in ` Commands:`;
   - help on stderr when stdout is empty;
   - a hidden command, with and without an alias (E-27);
   - `help` and `completion` left out, and never asked for help;
   - the depth bound and the request bound, each one past and one at its limit;
   - a cycle;
   - a headingless help below the bare binary with a declared command under it (E-26);
   - a missing declared command (AE27).
4. **QD4 against QD5 and the bounds.** When QD5 or a bound decides discovery, the other generated checks stand on their own invocations even when the walk also missed a declared command (QD4's second sentence). Pairs test each order the text sets.
5. **The attribution function, independently.** Unit rows cover every raw-end combination the text distinguishes, including those the scripted source cannot produce:
   - an engine kill outside the budget;
   - a budget kill of a process no longer running;
   - a `SIGKILL` with a memory or code-signing exit detail;
   - a non-`SIGKILL` signal with no marker.

   A test feeds the scripted source's ends for every recorded invocation through the function, and compares the result with the recording's own `by` field.
6. **QS5's QD3 trigger** is tested where every declared command was found and one more was found, which triggers it, and where one was missing, which leaves QD4 to decide. The override stops no runbook.

## Stated limits of this slice

- The known-fast control, the system's exit detail and the private copy's per-use check run only on macOS (PR11). Until then the scripted host supplies them, and unit tests cover the branches.
- No corpus recording has a help request that is killed, timed out or crashed. QO7's discovery rule for those ends is proven by unit tests.
