# Peer-review gate — phase 3 PR12 design note, slice c (2026-11-27)

**Mode:** design.

**Under review:**
- `docs/plans/2026-11-27-phase3-pr12-slice-c-design-note.md`;
- errata E-48 to E-51.

**Reviewers:**
- peer-experiment-reviewer, the measurement lens;
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build. The different-model pass follows AGENTS.md.

## Round 1 (head 3f3bbef)

**Verdicts:**
- **measurement:** one Blocker and five Majors. The prediction reproduced in a scratch copy: 140 cases, with `case-002` and `case-059` ending exit 70 naming QA8.
- **agy:** four Majors, two Minors and a Nit. The run reached its time limit after writing its findings.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | `-0` was rejected as a computed whole number, against §3's "`-0` is the integer zero and is accepted" | measurement (Blocker), agy (Major) | Confirmed against judge-contract §3 and `internal/subject/rules.go` | Accepted, with `-1` rejected |
| 2 | Level 6's cut capture was decided before the step was served, so the step's own run was skipped (`case-153`) and its QO1 pre-empted | agy (Major), measurement (Minor) | Confirmed against §2's order | The step is served on levels 1 to 5, and the cut capture is decided after |
| 3 | Level 8 held `exit_code` without comparing, so a level-7 regression would end held | measurement (Major) | Confirmed | Level 8 compares; a mismatch reaching it is exit 70 |
| 4 | Level 4 read a missing resolution as "outside the required set", which fails open | measurement (Major) | Confirmed against QE2's `earlier` | Membership comes from the plan; a missing required resolution is exit 70 |
| 5 | The trace control would fail a correct engine, and `compareTrace` compares sets | measurement, agy (Major) | Confirmed against `case-153`, `case-022` and the test | A case-derived reader with stated rules, compared as ordered lists |
| 6 | A capture of a placeholder step was a two-way reading decided quietly | measurement (Major), agy (Minor) | Grepped "depend" in every sealed file; QS5's list names no captures | Put to the owner, who ruled E-51 |
| 7 | QA9's move lacked rows for invalid UTF-8, normalisation, big numbers and §3's refusals | measurement (Major) | Confirmed against control 4 | Rows added, plus a "return held" mutation per predicate |
| 8 | Unlisted inherited defects: the printed value in `encoding/json` types, captured runs judged by level 7, and `expected` unset | agy (Major) | Confirmed in `internal/scripted/reader.go` and `internal/decide/level7.go` | Added to the defects, with `Evidence.Captures` kept apart |
| 9 | Minors: E-49 steps of a destructive runbook, a changed copy as an E-49 exception, the QA8 token breaking history, shared computation keys, the precondition's depth, E-49's judgment in `collect`, missing rows, the corpus table and the ledger notes | measurement, agy | Confirmed | Fixed in the note |
