# Peer-review gate — phase 3 PR2, the strict JSON leaf (2026-11-25)

**Mode:** build. **Reviewers:** peer-code-reviewer, peer-test-reviewer, and a different-model peer-code-reviewer on agy. **persona-version:** cache/1.9.5.

**Design round:** none. The plan `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` (U3, U22, KTD3, KTD4) is the design. The plan requires design notes only from the thin slice (PR6) on.

**Tier rationale:**
- Tier 1 code and test always run in build mode.
- No Tier 2 lens fired:
  - U3 moves the judge's reader without changing any verdict, and an output differential guards that;
  - U22 adds a canonicaliser that no command uses yet;
  - no data, permission or network surface changes.
- The measurement lens did not fire, because no gate, threshold or verdict changes. The owner's private judge scoring is the guard U3's verification names (below).

**Per-lens runtime:**

- peer-code-reviewer: runtime=claude
- peer-test-reviewer: runtime=claude
- peer-code-reviewer: runtime=agy (different model)

Each reviewer ran in its own clone.

## Round 1

**Range:** the U3 and U22 commits. The same pass (codex) wrote the code and its tests.

**Verdicts:**
- code (Claude): ship-ready, with 2 Minor;
- code (agy): ship-ready, with 3 Minor;
- test: trustworthy after two Majors.

Both code reviewers built `pql` before and after the change and compared its output. Claude's reviewer compared 4,621 inputs (the corpus plus fuzzed variants) and the test reviewer compared 504 case inputs. Neither found a difference.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | No test of RFC 8259's escape whitelist: accepting `\x` survives every test and all 383 sealed cases | test (Major) | Reproduced: a `\x` input is graded instead of rejected X12 under the mutant | Fixed: six rejected escape rows in the leaf, one X12 row through the judge |
| 2 | Unknown-key rejection holds only because the member count changes; a renamed key at the same count is unguarded | test (Major) | Reproduced: the mutant turns X12 into a judge panic | Fixed: a same-count rename row in the judge, and a synthetic `Child` row in the leaf |
| 3 | The leaf never tests its own hook contract (`Valid`, `Container`, `End`, `Materialize`) | test (Minor) | Mutants survive in the leaf package | Fixed: synthetic-hook rows |
| 4 | `TestStrictLeafMatchesJudge` checks shared rejections only | test (Minor) | Confirmed | Fixed: accepted at-limit rows per bound |
| 5 | The `precondition` Note policy and the discarded free-string skip have no memory guard | code (Minor) | Mutants survive | Fixed: 16 MiB rows under a 2 MiB cap. Allocations are about 165 KB normally and 16.9 MB under the mutants |
| 6 | The writer copies unchecked constructed `Number` text, and its all-zero branch is unreachable | code, agy, test | Confirmed by probe (`Number("abc")` written as `abc`) | Fixed: one validated integer path; an invalid constructed number panics |
| 7 | jcs overflow check has no distinct assertion; an unreachable key-quoting branch | agy | Confirmed | Fixed: exact error asserted; branch removed |
| 8 | jcs cycle identity rejects aliased sub-slices | code (Nit) | Reproduced | Fixed: length added to the identity |
| 9 | Row names with `-`, `.` or `+` cannot be knocked out singly, so several mutants stay unproven | code, agy | Confirmed with `mutation-verify.sh` | Fixed: every row name matches `^[A-Za-z0-9_]+$` |
| 10 | The mutation log was checked in under `internal/jcs/testdata/` | agy | `CODING_STANDARDS.md` puts it in the pull request or its review log | Fixed: file removed; the record is below |
| 11 | Nits: keys re-encoded per comparison, a copied bounds literal, dropped doc comments, `ScopeIncludes` silent on an unknown scope, a redundant check, no leaf import guard, vectors never checked against their generator | code, agy | Confirmed | Fixed |

## Round 2 (delta)

**Range:** the round-1 fix commit. One test reviewer re-ran the mutants and found every round-1 finding fixed. The judge's output was unchanged (383 of 383 cases).

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 12 | The memory rows never take the escape branch, so a mutant that buffers escaped discarded strings survives; it allocates 65 MB against a 2 MiB cap | test (Major) | Reproduced by probe | Fixed: escaped rows for both fields |
| 13 | Uppercase hex in `\u` escapes has no accept test | test (Minor) | Mutant survives | Fixed: `É` accepted and decoded |
| 14 | The leaf never checks the count `End` receives, and never checks `End` on objects | test (Minor) | Mutants survive in the leaf | Fixed |
| 15 | A constructed non-integer `Number` (`1e`, `x.y`) is still copied unchecked | test (Minor) | Probe | Fixed: the whole number grammar is validated |
| 16 | Nits: a duplicate call, an unchecked field, Python `-I` ignoring `PYTHONDONTWRITEBYTECODE`, unnamed rows | test | Confirmed | Fixed |

## Round 3 (ce-code-review)

**Range:** the whole branch against `main`. Five reviewers ran: correctness, project standards, testing, maintainability and the learnings search. The receipt is `20261125-120819-bdba0d4f`, with the verdict "Ready with fixes" and no P0 or P1. Both of the group's units (U3, U22) are met.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 17 | `jcs.decimalKey` duplicates `strictjson.Normalize`, which panics on a valid number whose exponent overflows `int` | maintainability (P2) | Confirmed by the validator | Fixed: `Normalize` holds the exponent as a big integer, and jcs uses it |
| 18 | The writer's and the comparison's defect panics have no test | testing (gap) | Mutants survive | Fixed: `TestUnwritableValue`, `TestInvalidComparison` |
| 19 | A discarded wildcard free string has no memory guard | testing (gap) | Mutant survives | Fixed: `TestFreeStringMemory/expected_*` |
| 20 | No test has an exponent beyond int64 | testing (gap) | Mutant survives | Fixed: `TestHugeExponent` |

After every round, the coordinator built `pql` at the base and at the head and compared stdout and exit code over every JSON file under `docs/specs` and `testdata` (1,609 files). There were 0 differences.

## Mutation record

Each mutant below was run, and the named test failed on it, then the code was restored. Rows marked with a reviewer were re-run independently through `mutation-verify.sh`, with a single-row knock-out.

| Rule | Mutation | Test that failed |
|---|---|---|
| Escape whitelist | accept `\x`, `\a`, `\'` | `TestReaderLimits/escape_x`, `escape_a`, `escape_quote` (reviewer) |
| Short or bad `\u` | accept | `TestReaderLimits/escape_short_unicode`, `escape_bad_hex` |
| Uppercase hex | `c <= 'F'` → `c <= 'A'` | `TestReaderLimits/uppercase_hex` |
| Unknown key, same count | `Child` ignored | `TestStrictLeafMatchesJudge/unknown_key_same_count` (reviewer), `TestHookContract/child` |
| Hook contract | `Valid`, `Container`, `End`, `Materialize` ignored; `End` count off by one | `TestHookContract/valid`, `container`, `end`, `end_object`, `materialize`, `array_count`, `object_count` |
| Bounds | byte, depth, number length, exponent length ±1 | `TestReaderLimits/*_over`, `TestStrictLeafMatchesJudge/*_at` |
| Duplicate keys, surrogates, UTF-8, trailing | accept | `TestReaderLimits/duplicate`, `low_surrogate`, `surrogate`, `utf8`, `trailing` (reviewer) |
| Retention | `precondition` Note → Keep; decode discarded strings; escaped strings buffered | `TestFreeStringMemory/precondition`, `model`, `*/escaped` |
| Exact equality | compare through float64; compare spellings | `TestExactEquality` (reviewer) |
| Writer | pass numbers through; drop long-integer branch; skip validation | `TestAnchorPromptNegativeZero` (reviewer), `TestWriter`, `TestInvalidConstructedNumber/*` |
| Judge tables | scope and applicability changes | `TestExportedDimensionTables` (reviewer), `TestUnknownScopePanics` |
| Leaf | import an internal package or Cobra | `TestStandardLibraryOnly` |
| jcs key order | byte order instead of UTF-16 | `TestVectors/utf16` (reviewer) |
| jcs numbers | thresholds, `-0`, exponent sign, shortest digits, round trip | `TestVectors/number_1e_6`, `number_1e21`, `number__0`, `number_1E_30`, `number_0_10`, `TestErrors/integer_loss` |
| jcs escapes | each escape | `TestVectors/escapes` (reviewer) |
| jcs errors | overflow, invalid string or key, grammar, cycles, aliasing | `TestErrors/overflow`, `invalid_string`, `invalid_key`, `leading_zero`, `object_cycle`, `array_cycle`, `TestAliasedSlices` |
| QE2 and the corpus | truncate a digest | `TestWorkedIDs`, `TestCorpusPreimages/*` |
| Checked-in vectors | edit one digest | `test_checked_in_vectors` (Python) |

## The private re-score (waived)

U3's verification asks for a re-run of phase 1's private judge scoring and its seeded-mistake catalogue on this build. On 2026-11-25 a separately dispatched agent looked for that material outside the repository. It found only the collection's private half, whose tools score collector records, not the judge alone. The owner then said phase 1's private judge cases and catalogue were never kept outside the repository, and waived the re-score for this pull request.

The evidence it merges on:
- 0 output differences between the base and the head over 1,609 JSON inputs, checked after every round;
- the 383 published judge cases;
- the review rounds above.

KTD3 asks for the same re-score after any later change under `internal/strictjson`. That needs its own ruling, because the material it names does not exist.
