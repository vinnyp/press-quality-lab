# Peer-review gate — phase 3 PR13, slice d, destructive runbooks (2026-11-27)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies.

The design note had its different-model review on agy (`docs/agent-reviews/2026-11-27-phase3-pr13-design-note-reviews.md`).

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Code:** the guard is the one thing that keeps a destructive command out of a scope without `destructive`. It must read the subject and the raw scope tokens, never the plan's own results.
- **Test:** the corpus does not catch a dropped plan exclusion, as the note predicts, so unit rows carry the guard.

No confinement path changes, so no security reviewer.

## Round 1 (head 0a158d7)

**Verdicts:**
- **code:** no Blocker or Major, and one reading for the owner.
- **test:** no Blocker or Major. Every mutation the note fixed in advance turned a test red, and so did 13 more the reviewer added. Three mutations survived.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The note let the earlier safe steps of a runbook with a placeholder-class step run in a conformance scope. That reads QA7 against QS5's "a placeholder is never read as false" | code | Confirmed against QA7 and QS5; the text is silent | Put to the owner, who ruled E-54: outside a destructive scope no step of such a runbook runs |
| 2 | `MayPlanServe` allowed a key when the check had no subject, `commands` or `runbooks` had the wrong type, or the key named no subject runbook. Nothing was served only because level 2 then panicked | code (Minor) | Reproduced | Each case is now an engine fault, exit 70, with a row for each; a key must name exactly one runbook and one of its steps |
| 3 | `record.Build`'s refusal of an unknown outcome had no row | code, test (Minor) | Reproduced; deleting the clause left the suite green | `UnknownOutcome` row |
| 4 | Nits: the computation exemption did not require a computation; the walk's `MayServe` call cannot refuse; the startup check and the serve loop took different checks' subjects; `FactFalse` repeated `Inside`; a curly apostrophe | code, test | Confirmed | Fixed; the walk's call is kept for uniformity, with a comment |

## Round 2 (delta, 0a158d7 to e6adeb8: E-54 and the round 1 fixes)

**Verdicts:**
- **code:** no Blocker or Major. Each round 1 finding is fixed. E-54 is built as ruled and only where it applies: it keys on class, never `writes`; it leaves probes alone; and in a destructive scope the runbook runs normally.
- **test** (on 0a158d7, finishing its second mutation batch): four Minors, three Nits.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 5 | The computation exemption's `home == "scratch"` conjunct had no row just outside it | test (Minor) | Reproduced; the mutation survived | Fixed: a row with only the home changed |
| 6 | Negative rows asserted any error, not the fault the guard raises, so a fault that fires earlier would keep them green | test (Minor) | Confirmed against `CODING_STANDARDS.md`'s negative-row rule | Every negative row asserts its rule. That showed `TestServeSafetyBrakes`' two placeholder rows had been refused by round 1's QA7 resolution guard, not by QS5; their fixture now resolves (#10) |
| 7 | No grade-level row asserted that a guard refusal gives exit 70 with no record written | test (Minor) | Confirmed by reading `grade.go` | Added; reworked in rounds 3 and 4 (#13, #16) |
| 8 | `RunbookHelp` was now refused by the new resolution guard (QA7), not by the class rule, so it no longer showed the note's control | code (Minor) | Reproduced | A real `remove --help` step in `mixed`, a conformance scope, and QS6 asserted |
| 9 | Nits: every `TestMayServe` key carried a computation; E-51's `writes` rows lost their run outside a destructive scope; the E-54 test in `StepRuns` and the nil-subject guard each repeat a later guard; `Check.Scopes` is load-bearing for E-54 with no comment saying so | code, test | Confirmed; the two repeated guards survive their mutations because the later guard refuses the same input | Fixed, and the repeated guards are commented |

## Round 3 (delta: e6adeb8 to 4922624 for code; 0a158d7 to 4922624 for test)

**Verdicts:**
- **code:** no Blocker, Major or Minor. One Nit.
- **test:** one Major and three Minors, from 24 mutations.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 10 | `TestServeSafetyBrakes`' placeholder rows never reached QS5 | caller, from the round's fault assertions | Reproduced | The subject's step is renamed with the key, and the rows run where E-54 lets the runbook through; both turn red when the serve brake is dropped |
| 11 | The scopes on an unchecked step's synthetic check (`collect.go:177`) had no row. Dropping them stopped such a step in a destructive scope, against E-54's "runs as any runbook does" | test (Major) | Reproduced; the mutation left every suite green | `TestPlaceholderUncheckedStep` became an Outside/Inside pair; dropping the field turns Inside red |
| 12 | `/Runbooks` was refused by the resolution guard, and the nil-subject guard could not be observed | test (Minor, Nit) | Reproduced | A probe key, and each row asserts its Cause |
| 13 | The grade-level refusal row passed on its own twin: its override ran a plan `grade.Run` did not build | test (Minor) | Reproduced | It runs through the plan `grade.Run` builds, beside an Allowed twin that reaches `Write` once |
| 14 | `WritesPlaceholder` did not need the destructive scope | code (Nit) | Reproduced | Back under conformance |

## Round 4 (delta, 4922624 to 700ce79)

**Verdict:** no Blocker or Major from either reviewer. Two Minors from each.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 15 | The guard let a non-exempt key with null words pass. `servingCommand` compares with `reflect.DeepEqual`, so nil never matches the bare binary's `[]`, and the key resolved to no command. The computation-exemption rows hid this, because a nil-subject guard refused them first | test (Minor) | Reproduced. I first doubted this one, thinking null words resolve to the bare binary. They do not | The guard now refuses such a key (QS6). The rows run under a real subject, with a twin whose words no command prefixes, which still passes as the bare binary (collection rules, "The bare binary" and "An invocation's command") |
| 16 | The grade `Refused` row was refused at the bare probe, before the runbook step | code (Minor) | Reproduced | It flips a `remove` command that no probe takes, and asserts the collect fault's Cause |
| 17 | The note's sentence on `StepRuns`' second brake misstated which mutations turn tests red | code (Minor) | Reproduced | Corrected |
| 18 | The brake rows' QS5 is shared by E-54 and the serve brake | test (Nit) | Confirmed | They assert the Cause too |

## Round 5 (delta, 700ce79 to 80299a2)

**Verdict:** both reviewers found no Blocker, Major or Minor. Both traced every kind of key a valid plan builds, and none reaches the new refusal. Moving the refusal ahead of the exemption turns 87 corpus cases red, and widening it to empty words turns 111 red.

Three Nits are left as they are:
- `TestDestructiveServeRefusal` counts discovery's help requests with the probes, which is right only while the fixture's help lists no command;
- the note's exemption bullet cites "The bare binary" but not "An invocation's command";
- the `Unknown` twin's subject declares no command, so it shows no match rather than a declared command that does not prefix the words.

## Evidence

- `gate.sh`, `check.sh` and `SCAN_COMMITS=<base>..HEAD ./scan.sh` pass at the merged head.
- macOS evidence: the broad set, named in the pull request.
- Unrelated to this PR: `judge.TestScaling` failed twice in the test reviewer's runs, while six mutation suites ran at once. It looks sensitive to machine load.
