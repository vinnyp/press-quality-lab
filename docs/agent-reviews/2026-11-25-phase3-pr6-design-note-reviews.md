# Peer-review gate — phase 3 PR6 design note (2026-11-25)

**Mode:** design. **Under review:**
- `docs/plans/2026-11-25-phase3-pr6-thin-slice-design-note.md`;
- `scripts/predict-ratchet.py`, with `scripts/ratchet/pr6.json` and the script's tests.

**Reviewers:**
- peer-experiment-reviewer (the measurement lens);
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build. The different-model pass follows AGENTS.md's rule for plans. No code under `internal/` changes in this round.

## Round 1

**Range:** commit b2fe47a.

**Verdicts.** Both reviews found the predicted set of nine cases correct, and the script's reason-to-level table accurate against `collection-rules.md` §2.
- measurement: sound once fixed, with five Majors, four Minors and three Nits;
- agy: changes required, with three Majors, three Minors and one Nit.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | A pending run stops at its first unbuilt level, so a check decided wrongly elsewhere in a pending case is never seen; QO1 firing on a passing host is the example | measurement (Major), agy (Major) | Confirmed: the ratchet only needs a pending case to fail | Fixed: a tagged in-process test compares every check's ending with the committed prediction file (control 3), and the runner checks each pending case's ending |
| 2 | The script put the computation-keys blocker ahead of earlier unbuilt levels and scanned checks in check-id order, so its "first blocker" was not the run's ending | agy (Major), measurement | Reproduced: `case-001`'s first check shows both | Fixed: planning gaps come first, since the engine fixes every key before any check runs; each check's ending is then its first unbuilt level in level order. A pending case's predicted ending is a set of rules |
| 2a | The fix for #2 made every missing key a planning gap, which dropped `case-003` and `case-004` from the prediction | coordinator, from the script's output | Confirmed: the coordinator's brief had said the engine fixes every invocation key before collection. U5 builds preimages; a key is needed only when its invocation is served | Fixed: the script separates planning gaps from key gaps, and a key gap is reached only at the level that would serve the key |
| 3 | No control catches QO1 firing on a passing host | measurement (Major), agy (Major) | Confirmed against the controls | Fixed: control 2's last row, and controls 3 and 4 run from a passing host through the real level 1 |
| 4 | QO1, QT2, QT4, QE14 and QE17 are built only in part | measurement (Major), agy (Major) | Confirmed against each clause and the plan's later units | Fixed: all five stay listed, with what each lacks. QE15 and QH3, which PR6 proves whole, move instead |
| 5 | QE6's proof tested two of its twelve rows | measurement (Major), agy | Confirmed: QE6 is a table | Fixed: X1 to X12 through the injected judge |
| 6 | The sentinel subject cannot show that the engine never ran one, since the profile would refuse its write | measurement (Major) | Confirmed against the profile text; not run on macOS | Fixed: the released source's `Observe` fails the test if called, the trace shows zero homes, and a sentinel computation is added |
| 7 | QO1's scope over a check with no invocation can be read two ways | measurement | Not two-way. §2's opening sentence (`collection-rules.md` line 119) and "every check that runs the subject" in QH1, QH6 and QH9 (`confinement.md` lines 9, 17, 31) settle it: QO1 covers checks that run the subject or a computation | The note states the reading with its clauses. No owner question |
| 8 | QS1 for a missing or numeric version can be read as QS1 or QS2 | measurement | Not two-way. A missing version does not "state its own version" (line 42), and errata E-9 checks QS1 first and names QS2 only when no named rule is broken | The note states the reading; U4 tests all three inputs |
| 9 | The planted defects for `case-102`, `case-103` and `case-116` were one coarse mutant | measurement (Minor) | Confirmed | Fixed: one mutant per clause, each turning only its own case red |
| 10 | The hand audit's `.schema` and `.semantic` labels were unchecked | measurement (Minor), agy (Minor) | Confirmed | Fixed: a test checks each label against the subject-file schema |
| 11 | `case-136`'s prediction ignored its pre-check's dependencies | measurement (Minor), agy (Minor) | Confirmed | Fixed in the script |
| 12 | QO1's marker test also counted computation invocations, and left out half of QH6's condition | agy (Minor), measurement (Nit) | Confirmed | Fixed in the script |
| 13 | The table of partly built rules and three of the plan's controls were missing | agy (Minor) | Confirmed | Fixed: the full "stays listed" table, and controls 6 to 8 |
| 14 | The reason table was printed but not used; `rejects` and `builds` entries were not checked against real rule ids | measurement (Nit) | Confirmed | Fixed in the script |
| 15 | Script limits for later slices: QO7's multi-invocation checks, and level 8's split | measurement (Minor) | Confirmed | Recorded in the note's last section; they bind slice b2 |
| 16 | The agy reviewer could not run the script (its sandbox refused) | agy (Minor) | Reproduced as a tool error on its side | No change. The measurement reviewer ran it, and its counts matched |
| 17 | `check.sh` runs `unittest` without `-B` | agy (Nit) | Rejected: no such line in `check.sh`; every Python step uses `-I -B` | No change |

The agy review also described three predicted cases' violations wrongly. It gave `case-100` an unknown field, `case-101` a relative directory, and `case-129` a trailing comma. The files show a repeated `declared_count`, `read_only: "no"`, and version `2.0`, as the script says. Its prediction verdict did not depend on those descriptions.
