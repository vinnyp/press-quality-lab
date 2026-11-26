# Peer-review gate — phase 3 PR9 design note, slice b1 (2026-11-26)

**Mode:** design.

**Under review:**
- `docs/plans/2026-11-26-phase3-pr9-slice-b1-design-note.md`;
- `scripts/ratchet/pr9.json` with its prediction and the script's changes;
- errata E-26 and E-27.

**Reviewers:**
- peer-experiment-reviewer, the measurement lens;
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build. The different-model pass follows AGENTS.md.

## Round 1 (range 3ba1f36 to 7da380a)

**Verdicts:**
- **measurement:** not yet, with six Majors, six Minors and two Nits. It reproduced the prediction byte for byte and found it credits nothing this slice does not build.
- **agy:** not ready, with four Majors and five Minors. It could not run the script.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The per-check test compared only level and outcome. A discovery that always says held would pass, and QO4's two branches sit at one level | measurement (Major) | Confirmed against `prediction_scripted_test.go` and the cases' evidence | Control 2 compares outcome, reason, `failure_kind` and targets with `input.json` for every decided check |
| 2 | QO5's accepted codes were deferred, yet 179 held checks with expected code 2 must pass level 7 | measurement (Major) | Reproduced over the preimages and the prediction | All four of QO5's bullets are built in U44. QO5 stays listed for level 8's predicate |
| 3 | The walk ran only for checks reaching level 7, but QD4, QS5's QD3 trigger and `case-064`'s targets need it whatever decides discovery | measurement (Major) | Confirmed against QO7, QE8 and `case-064` | The note states when the walk runs and when it does not |
| 4 | An engine kill on a help request: the plan said `harness_fault`, but QO7 says "every other end is QD5" | measurement (Major) | Settled by QO7's text, which outranks the plan | The plan's KTD11 is amended, and QD5's proof gains the row |
| 5 | A signal other than `SIGKILL` before the marker was called settled as `crashed` | measurement (Major) | Two-way: QO5's premise is that the program ran | Put to the owner, who ruled E-28: QH6's case, so QO1 decides |
| 6 | QD5's proof missed rows, and its mutation described correct behaviour | measurement (Major), agy (Major) | Confirmed against QD2, QD5 and `help-fails` | Rows added, and the mutation restated |
| 7 | E-26's last sentence named an empty class, and its "wherever it is in the walk" took in the bare binary, where QD5's second case decides | measurement (Minor), agy (Major) | Confirmed against §5 and QD5 | E-26 narrowed to helps below the bare binary, the owner's ruling, and its last sentence restated |
| 8 | E-27 did not say whether a tab counts | measurement (Nit) | Two-way | Put to the owner, who ruled that a tab counts as a blank. E-27 updated |
| 9 | QD4 when a help request is decided at level 7 was unstated | agy (Major) | Confirmed against QO7, QD4 and `case-095` | A reading and a control 5 row |
| 10 | The trace control's expected keys came from the engine, and failed when the walk does not run | measurement (Nit), agy (Major) | Confirmed | Control 3 takes its keys from the case, with the walk's absence stated |
| 11 | Every level-8 sentinel named discovery's built rules, and `QO7.discovery` did not gate level 7 | measurement (Minor), agy (Minor) | Reproduced | Per-probe level-8 rule lists, and the token at level 7 (script change) |
| 12 | The attribution seam carried two representations of one fact, and the round trip ignored the host's overrides | measurement (Minor), agy (Minor) | Confirmed against `observe.End` and QT4 | One representation stated, an untagged exit-detail decoder, and the seam check includes the overrides |
| 13 | The ledger left out rules this slice builds parts of (QE8, QH5 to QH7, QS6, QS7, QT5) | measurement (Minor), agy (Minor) | Confirmed | Each is listed with what it still lacks |
| 14 | Control rows: section ends, trailing commas, every hidden marker, both streams non-empty, a cycle bounded by depth, the QD3 trigger under QD5 | measurement (Minor), agy (Minor) | Confirmed against §5 and QD2 | Rows added |
