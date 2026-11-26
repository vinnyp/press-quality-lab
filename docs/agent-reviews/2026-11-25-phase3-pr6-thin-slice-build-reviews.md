# Peer-review gate — phase 3 PR6, the thin end-to-end slice, build (2026-11-25)

**Mode:** build. The design note's three design-mode rounds are in `docs/agent-reviews/2026-11-25-phase3-pr6-design-note-reviews.md`.

**Reviewers:**
- `ce-code-review` at full depth (run `20261125-185251-e2a68499`), with these lenses: correctness, project standards, testing (with about 90 mutations in a scratch copy), maintainability, security, reliability and learnings;
- a cross-model adversarial pass on codex. It requested gpt-6.1-terra at high effort. Its receipt supported the request, model_actual was gpt-6.1-terra, effort_actual was unverified, and independence was verified;
- an independent validator over the selected findings;
- a delta review of the fixes, run by peer-code-reviewer.

**persona-version:** cache/1.9.5.

## Round 1 (head 142ddaf)

**Verdict:** Not ready. 19 candidate findings were merged into 7. The validator confirmed all 4 it checked; #4 skipped validation on cross-model agreement.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The QS2 proof's bound rows had non-object roots, so they named QS2 whatever bounds `Parse` passed | codex (P0) | Validator confirmed | Fixed: each bound is exceeded inside an otherwise valid subject object, with a row at the bound |
| 2 | `TestPublishRequiresIdentity` passed through the parent-mode guard, not the identity check | testing (P0 under REVIEW.md) | Validator confirmed by mutation | Fixed: a 0700 parent, and a positive control |
| 3 | The binary was read with plain `os.Open` joined onto the subject directory, with an unbounded read: `../…/etc/hosts` reached a stored verdict carrying that file's digest, and a FIFO hung the run | security (P1), reliability, correctness | Reproduced on the released build; validator confirmed | Fixed: read before the pre-check through the subject directory's `os.Root`, regular files only, hashed by streaming. An escape, a missing file or a special file ends the run with exit 70 naming QS8 |
| 4 | The seed snapshot followed links, opened FIFOs and read without a bound | security (P1), codex, reliability | Reproduced (a hang) | Fixed: links and special files in the seed are refused with exit 70 naming QS9, and reads are bounded |
| 5 | The stored record's subject, seed and binary digests were never asserted | testing (P2) | Validator confirmed | Fixed: asserted against digests the test computes itself |
| 6 | On the released build, a file that breaks a rule the schema cannot detect gets a verdict (exit 4, never green), where the spec says exit 2 | correctness (P2) | Confirmed against the code | The owner accepted this interim behaviour on 2026-11-25. It is a stated limit in the design note, and U9 removes it |
| 7 | An unreadable binary ended the run under QE17 after the judge decided | report leaf (unrequested behaviour) | Confirmed | Resolved by #3: the binary is now read before the pre-check |
| — | Maintainability items, the remaining testing gaps and the residual risks | maintainability, testing, reliability | Confirmed | Fixed: the `if true` wrappers, rule ids carried by type instead of error text, the `Required` failure test, `store.Write`'s clean-path test, and the level-6 trigger rows. The rest are recorded as residual risks in the run |

## Round 2 (delta, 142ddaf to a7f1491)

**Verdict:** Ready to merge. All five findings are closed. The reviewer re-broke each one in a scratch copy: it loosened and tightened each bound, disabled the identity check, and tried every escape, special file and link for the binary and the seed, plus five digest mutations. No refusal reaches a verdict or a stored record, and no FIFO is ever opened. The changed predictions for `case-110` (QS8) and `case-112` (QS9) match the case files.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 8 | The seed-root guard had no test, and a link on the seed's path was treated two ways (`seed` itself a link refused, `p/seed` through a linked `p` graded) | delta (Minor) | Reproduced | Fixed: any link on the seed's path ends the run with exit 70 naming QS9. Whether QS9 allows such a link is recorded in the design note for the owner to decide at U9 |
| 9 | A 16 MiB cap on declared files named QS8, which sets no size limit | delta (Minor) | Confirmed against QS8 | Fixed: declared files, computation executables and seed files are hashed by streaming |
| 10 | The seed's total size was not bounded | delta (Minor) | Confirmed | Fixed by #9: nothing holds the files' bytes |
| 11 | `plan` still returned an untyped QE2 error | delta (Minor) | Confirmed | Fixed: a typed planning fault |
| 12 | The design note's fault list and stated limit had drifted | delta (Minor) | Confirmed | Fixed: the safe-read refusal is the fifth kind of fault |
| 13 | Nits: the check after the open had no test, there was no negative-PID row, and the rejection line had no category | delta (Nit) | Confirmed | Fixed |

## The private re-score

A separate scorer ran phase 1's private scoring and seeded-mistake catalogue at 142ddaf, because the scope rules moved into `internal/rulebook`. Counts only:
- 383 published and 369 private cases, 0 wrong. Every per-case result, timing aside, equals phase 1's final baseline.
- 161 seeded mistakes, all evaluated: 151 caught by the build's own tests, 10 caught by the published cases, 0 survived. 44 were ported to the code's new sites.
- Nothing under `internal/judge`, `internal/strictjson`, `internal/jcs` or `internal/rulebook` changed after 142ddaf.

## Mutation record

Each mutant was run, the named test failed, and the code was restored.

| Rule or guard | Mutation | Test that failed |
|---|---|---|
| QS1 | accept any version, including a missing one or the number `1.0` | `TestVersion`, `TestCorpus/case_129` |
| QS2 | accept a repeated key; raise depth to 65, number length to 33 or exponent digits to 5; let a lone surrogate through | `TestStrictReading/*`, `TestCorpus/case_100` |
| QS4 | skip each fact's validation; hard-code one fact | `TestFacts/*`, `TestPreservesModel` |
| QE6 | flip each row's cause, X1 to X12 | `TestPrecheckRows/X1` to `X12`, `TestCorpus/case_136` |
| QE15 | drop the stored rejected record; print its verdict | `TestRejectedRecord` |
| QH3 | check the real user id; remove the production `Geteuid` | `TestEffectiveRoot`, `TestProductionEffectiveID` |
| QO1 (level 1) | drop each grade-wide condition; fire on a passing host | `TestGradeWide`, `TestPassingHost`, `TestPredictedCheckEndings/case_001` |
| Unbuilt levels | force each trigger true or false; skip the level 5 or level 8 sentinel | `TestTriggers/*`, `TestUnknownTriggers` |
| Safe reads (QS8, QS9) | skip the binary snapshot; remove the regular-file checks; allow seed links; use `Stat` for a seed component; skip the check after the open | `TestSafeReads/*`, `TestSpecialFilesNeverOpened`, `TestOpenedHandleMustBeRegular` |
| Stored digests | blank the subject digest; drop the seed or binary digest; truncate the binary hash | `TestStoredDigests` |
| Pending endings | ignore a pending case's exit or rule; ignore a required-set difference | `TestPredictedEndingControls`, `TestRequiredTraceControls` |
| Publish identity | disable the identity check; accept a negative PID | `TestPublishRequiresIdentity`, `TestInvalidPublication/NegativePID` |
| KTD20 guard | an `os.Open` in a second `internal/grade` file | `TestGradeInputException` |
| KTD11 source selection | include test imports; disable enforcement | `TestSourceSelectionClassifier/*` |

The per-unit tables, with every mutation each unit's author ran, are in the run artifacts of each dispatch. The rows above are the ones that prove a rule or a guard.
