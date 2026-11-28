# Peer-review gate — phase 3 PR13 design note, slice d (2026-11-27)

**Mode:** design.

**Under review:** `docs/plans/2026-11-27-phase3-pr13-slice-d-design-note.md`.

**Reviewers:**
- peer-experiment-reviewer, the measurement lens;
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build, and AGENTS.md a different model.

## Round 1 (head 9957a3c)

**Verdicts:**
- **measurement:** two Majors and seven Minors. The prediction reproduced: 142 cases.
- **agy:** three Majors, one Minor and one Nit.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The guard read the plan's own results, so it failed together with the plan builder it backs up, and it replaced the one check that works a step's class out from the subject | measurement, agy (Major) | Confirmed against `plan.go`, `level4.go` and two collect tests | `MayServe` recomputes from the subject and the raw scope tokens; the two tests keep their construction and expect exit 70 |
| 2 | The guard sat on the runbook loop only; the first loop serves unscheduled checks and probes | agy (Major) | Confirmed in `collect.go` | `MayServe` covers every key; a probe's destructive command is refused in any scope; an unscheduled authored check is exit 70 |
| 3 | The QE12 table test could not fail on two rows and never compared the text | measurement, agy (Major) | Confirmed | The test parses the sealed table and requires exact equality, and pins the closed set from two sealed sources |
| 4 | The closed set's members were unnamed and `no_evidence` misplaced; no outcome-reason pairing | measurement (Minor), agy (Major) | Confirmed against the evidence schema and `record.go` | Nine reasons named; `record.Build` checks the pairing; rows for each refused reason |
| 5 | A check's next-action reason can also come from the judge (C1, C3, C4) | agy (Major) | Confirmed on `case-011` and `case-093` | Reading 3 amended: U18 reads both kinds from the judge's output |
| 6 | Minors: the help claim cited QD6 for the wrong point; QA8's proof cases did not fail on its mutation; the trace rule could not change a result; AE24's tests lacked mutations; missing rows for nested commands, an E-49 destructive step and a placeholder-class step; the placeholder row's field; the stated limit's citation; PR12 missing from the script's history loops | measurement, agy | Confirmed | Fixed in the note |

No finding was a two-way reading of the sealed text.

## Round 2 (delta, 9957a3c to 8d5f5a4)

**Verdicts:** both reviewers found no Blocker or Major, and every round-1 finding is closed.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 7 | `MayServe` as written would refuse discovery's `remove --help`; an exemption keyed on `--help` would let a destructive step through | measurement, agy (Minor) | Confirmed against `collect.go` | The discovery check's own keys pass, identified by the check; computation keys pass |
| 8 | `MayServe` had no input for the raw scope tokens | measurement, agy (Minor) | Confirmed | `Plan.Scopes` carries them verbatim |
| 9 | The placeholder-class reading said "runs nothing", wrong for a mixed runbook | measurement (Minor) | Confirmed | Reworded with the step order; a mixed-runbook row; a placeholder brake at serve |
| 10 | Refusing unscheduled authored checks breaks existing serving tests' fixtures | agy (Minor) | Confirmed in `u44_test.go` | Their checks are scheduled; one subtest keeps the unscheduled check, expecting exit 70 |
| 11 | QA8's first mutation does not fail `case-001`, whose runbook check is out of scope anyway | agy (Minor) | Confirmed against `case-001` | Split into two mutations, each with what fails |
| 12 | Nits: `MayServe` on a runbook key checks the whole runbook from the subject; the corpus count; the stated limit's section; `TestDestructiveSentinel`'s construction; QE13's second mutation; literal QE12 data | measurement, agy | Confirmed | Fixed in the note |

No Blocker or Major remains, so the build proceeds.
