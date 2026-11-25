# Peer-review gate — phase 3 PR5, the corpus harness and the rule ledger (2026-11-25)

**Mode:** build. **Reviewers:**
- peer-code-reviewer, peer-test-reviewer and peer-experiment-reviewer as the measurement lens;
- a different-model peer-code-reviewer on agy;
- `ce-code-review` on the rebased branch.

**persona-version:** cache/1.9.5.

**Design round:** none. The plan's KTD2, KTD15 and KTD22 are the design; design notes start at PR6. The measurement lens ran in build mode because this change decides what counts as a passing case.

**Tier rationale:**
- Tier 1 code and test always run.
- The measurement lens fired: the runner, the ratchet and the ledger are checks whose false green would let an unevaluated case or rule count as proved.
- No security, data or interface surface changes.

Each reviewer ran in its own clone.

## Round 1

**Range:** U45, U34 and U8 as first built. The same pass (codex) wrote the code and its tests.

**Verdicts:**
- code: ready after one Blocker;
- test: trustworthy after one Blocker;
- measurement: not sound enough to merge (one Blocker, three Majors);
- agy: timed out (rc 124), so it was re-run in round 2.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The ledger counted 25 rules as proved from CASES.md's stated limits, 7 of them rules a scripted run cannot break (GR15), against U45's "every rule unmapped" and KTD22's `macos_evidence` requirement. The growth check would have locked that in | code, test, measurement (Blocker) | Confirmed against the plan. The coordinator's brief caused it | Fixed: every rule unmapped; a GR15 rule needs a `macos_evidence` proof; limits come only from KTD22's two sections |
| 2 | Any existing test function counted as proof of any rule | measurement (Major), code | Mutant survived | Fixed: a proof must cite its rule in the named function and build under the tags CI runs |
| 3 | The three real-verifier cases could leave the pending list unevaluated | measurement, test (Major) | Mutant survived | Fixed: `TestCorpus` keeps them pending |
| 4 | Export required a baseline no scripted case produces | measurement (Major) | Confirmed against QT2 and `case-071` | Fixed: records only; a baseline is validated where one is stored |
| 5 | An unread trace (`Required == nil`) passed every guard | code (Major) | Confirmed | Fixed: "trace not read" |
| 6 | The schema oracle had never run | test (Major) | Mutants survived | Fixed: controls built from the kit's examples |
| 7 | Minors: exit codes typed `any`; absent-on-both passes; loose key matching; trace order; field-set length; trailing JSON; growth `main()` untested; three pending-list parsers; `python3 -I` writing bytecode; row names; `mkfifo` shell-out | all | Confirmed | Fixed |

## Round 2 (different model, agy)

**Range:** the round-1 fix commit. The verdict was ready after one Blocker.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 8 | The trace guard refused keys for every check expected as `placeholder_unfilled`, but QS5's final override marks checks that ran, so six placeholder cases would fail once `grade` exists | agy (Blocker) | Confirmed: in `case-022`, 18 checks are overrides | Fixed: each check is classified from the subject file alone, as level 2 (never runs) or override (ran). The plan's KTD2 item 6 sentence was corrected. A control over the nine placeholder cases checks the split |
| 9 | A `macos_evidence` proof was not checked to build on darwin | agy (Major) | Confirmed | Fixed: it must build for darwin/arm64 under a gate evidence tag set |
| 10 | A limit route only had to mention its rule; `scope` was missing from case assembly | agy (Minor) | Confirmed | Fixed |

The first attempt at #8 also cross-checked each class against the evidence's `observed` field. The corpus broke that check, because `observed` is the collector's choice (QE19, `collection-contract.md` line 237). The check was dropped. This was the coordinator's error and was not a conflict with the sealed text.

## Round 3 (ce-code-review on the branch rebased onto main 203a4e2, run `20261125-143657-b2ac38ae`)

**Verdict:** "Ready with fixes", with no P0 or P1. U34, U8 and U45 are met.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 11 | The trace guard accepts a discovery walk when `declared_count` is a placeholder (`case-064`) | correctness, raised to P0 under REVIEW.md | Validator rejected: `case-064`'s sealed record lists the six help keys among its invocations | No change. QS5's "with `declared_count` a placeholder, discovery is not observed" (`collection-rules.md` line 71) names the check's result, not whether the walk runs; line 163 settles that: "Discovery makes every help request QD2 reaches … and whatever `declared_count` holds". The sealed case agrees, so the walk runs and the guard is right to allow its keys |
| 12 | The dimensions QS5's override can never reach had no control | testing (P2) | Validator confirmed | Fixed: `excluded_*` rows |
| 13 | A rule citation outside the proof function was accepted | testing (P2) | Validator confirmed | Fixed: `outside_comment` |
| 14 | The export check's extra-file branch and the record schema pass had no control | project-standards (P2) | Validator confirmed | Fixed |
| 15 | Five smaller untested branches, and two maintainability items | testing, maintainability | Mutants survived | Fixed |

## Mutation record

Each mutant was run, the named row failed on it, and the code was restored. Rows marked with a reviewer were re-run independently through `mutation-verify.sh`.

| Rule | Mutation | Row that failed |
|---|---|---|
| Ratchet | a pending case passes; a required case fails | `TestResultAndRatchet` (reviewers) |
| QE19 comparator | each compared field changed; evidence added, removed or duplicated; a field renamed | `TestComparatorMutations/*` (reviewer: `tier`) |
| Top-level field set | an extra field | `TestComparatorMutations/top_level_extra` |
| Host and subject guards | each guard dropped | `TestEachHostGuard/*` (reviewer: `private_copy`, `self_test`) |
| Unread trace | accept `Required == nil` | `TestTraceGuards/zero_trace` |
| Trace order and shape | `home` before `required`; a three-field event | `TestIndependentReaders/*` |
| Cut computation | serve a cut capture | `TestCutComputationTrace` (reviewer) |
| QS5 classes | ignore the command placeholder; block an override key; accept an unclassifiable check; ignore `declared_count`; drop the excluded dimensions; skip the secret commands | `level_two`, `override`, `neither`, `TestPlaceholderCorpus`, `excluded_*`, `placeholder` |
| Discovery | ignore the last word; drop the exemption | `earlier_help`, `no_help`, `discovery` |
| Precondition computation | drop its key | `TestPreconditionComputationKey` |
| Real-verifier cases | remove them from pending | `TestCorpus` real-case guard |
| Case assembly | omit seed or baseline; leak an oracle file; home inside the subject; FIFO as a file; trailing JSON; drop `scope` | `TestEveryCaseAssembles/*`, `TestFixtureKinds`, `TestTrailingJSON` |
| Ledger | extractor count; a missing test; a removed mapping; mapped and listed; a GR15 rule mapped to a limit only; a limit from "Stated limits"; a limit naming another rule; evidence in an untagged or Linux file; a citation outside the function; a `../` path | `TestRuleLedger` (reviewer), `TestLedgerControls/*` (reviewer: `missing_test`, `removed_mapping`) |
| Growth checks | allow an added or swapped entry; allow the list's removal; a missing CI base; `main()` wiring | `TestRuleGrowthControls` (reviewer), `CorpusControls`, `test_main_*` |
| Record validation | wrong count; an extra file; a judge-rejected record; a schema-invalid record or baseline; an unknown pending id | `test_independent_count` (reviewer), `test_schema_examples`, `invalid_record` |
