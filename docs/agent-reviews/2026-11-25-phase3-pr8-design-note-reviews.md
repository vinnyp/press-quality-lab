# Peer-review gate — phase 3 PR8 design note, slice a (2026-11-25)

**Mode:** design.

**Under review:**
- `docs/plans/2026-11-25-phase3-pr8-slice-a-design-note.md`;
- `scripts/ratchet/pr8.json` with its prediction and the script's changes;
- errata E-17 and E-18.

**Reviewers:**
- peer-experiment-reviewer, the measurement lens;
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build. The different-model pass follows AGENTS.md.

## Round 1

**Range:** commit 3524507.

**Verdicts:** both reviews reproduced the 35-case prediction and found the final override safe to build now.
- **measurement:** close to sound, with four Majors, seven Minors and one Nit;
- **agy:** conditionally sound, with four Majors, four Minors and one Nit.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | QS5 cannot leave the ledger: its QD3 trigger (U11), its scorecard clause (U18) and its runbook interaction (QA7) are not built. The predictor credited QD3-triggered overrides (`case-022`) to U10's token | measurement (Major), agy (Major) | Confirmed against `collection-rules.md` lines 75, 76 and 290 | QS5 stays listed with what it lacks. The predictor splits the override into `QS5.final_override` (U10) and a QD3 trigger (U11) |
| 2 | QS6, QS7 and QS9 have clauses that act at run time or in the scorecard (budgets, `exit_codes: null`, masking, settings, copying the seed) | measurement (Major), agy (Major) | Confirmed against lines 80 to 105 | All three stay listed, each with its lacking clauses |
| 3 | E-17's control never tested a link that escapes, an intermediate link, or a link to a file | measurement (Major), agy (Minor) | Confirmed; PR6's build review had found both shapes | Control 6 adds the three rows and a mutation |
| 4 | The QE2 and QE3 proofs relied on the corpus, which never has a probe check with no command | measurement (Major) | Confirmed by scanning every `preimages.json` | Synthetic fixtures written from QE2's text, plus the secret's `null` and placeholder rows |
| 5 | QA5 is built and tested whole but was not moved | agy (Major) | Confirmed against line 279 and CASES.md | QA5 moves, with the README covered by the stated-limit route |
| 6 | `create`'s output was unspecified, the plan's conditional was dropped, and an engine that marks everything `placeholder_unfilled` might pass control 4 | measurement (Minor), agy (Major) | Confirmed against GKD11, U10 and QS5 line 62 | Control 4 now has per-position unit tests in isolation, `create`'s output stated field by field, the plan's conditional kept, and `case-061`/`case-062` as the discriminating evidence |
| 7 | E-17's "only" read as removing QS9's other rejections, and the subject directory's own links were not mentioned | measurement (Minor) | Confirmed | E-17 reworded: QS9's other rejections stand, and the subject directory's path is resolved too |
| 8 | A regular file inside the directory that cannot be read: exit 2 or exit 70? | measurement (Minor) | Confirmed two-way; the reviewer searched the sealed folder | Put to the owner, who ruled exit 2 under QS8 on 2026-11-25 (errata E-18) |
| 9 | Mutations were not fixed before the build | measurement (Minor) | Confirmed | The ledger table carries one mutation per moved rule |
| 10 | An E-8 fixture with a different `$HOME`, and the order of the required trace before the QE4 gap | measurement (Minor) | Confirmed | Added to QS8's proof and to the pending-endings section |
| 11 | QA3 and QE7 were split inconsistently, and QP1 was missing from the "stays listed" list | measurement (Minor) | Confirmed | QA3 moves with a per-row test of §2's credit conditions; QP1 stays listed |
| 12 | The predictor's E-17 test passes whatever the link is | measurement (Minor) | Reproduced | Made to discriminate, or deleted with its reason (script change) |
| 13 | Level 2 was partial, the runner kept its own placeholder classifier, and the level 3 and 4 sentinels still accepted incomplete preimages | agy (Minor) | Confirmed against `internal/decide` and the runner | Control 7 |
| 14 | QS11's proof should show how `0.1`, `0.10` and `-0` enter a check id | agy (Nit), measurement | Confirmed against line 114 | Added to QS11's proof |
| 15 | Wording: "passed every level through 8", "every case", the hand audit, the sentinel's path | measurement (Nit) | Confirmed | Fixed |
| 16 | The agy reviewer could not run the script | agy (Minor) | Tool error on its side | No change; the measurement reviewer ran it and its output was byte-identical |
