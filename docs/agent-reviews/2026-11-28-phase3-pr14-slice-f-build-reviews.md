# Peer-review gate — phase 3 PR14, slice f, the scorecard (2026-11-28)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies.

The design note had its different-model review on agy (`docs/agent-reviews/2026-11-28-phase3-pr14-design-note-reviews.md`).

**Tier rationale:**
- **Code:** the scorecard and `--json` change `pql grade`'s output contract, and the funnel decides what a run that exits 70 prints.
- **Test:** the corpus checks only the verdict lines, so fixtures and unit rows carry the subject-derived display.

## Round 1 (head 69ef522)

**Verdicts:**
- **code:** no Blocker, one Major. QE14 and GR3 hold, the copy before the bound is taken at the right point, and the bound decision is exactly today's.
- **test:** no Blocker, four Majors. About 80 mutations ran. The agreement check killed nine renderer mutations on the real corpus.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The funnel's `exits 70` lines reached stdout unmasked, because the scorecard is written past the stream masker | code (Major) | Reproduced with a post-judge panic carrying a declared value | Each fault string is masked before it is kept. `TestPR14PostJudgeMask` |
| 2 | The text-mode wiring through `grade.Run` had no end-to-end test. Identity escaping (G1), a nil pre-bound copy (G2), a pathless E-11 line (G3) and missing placeholders (V5) all survived | test (Major) | Reproduced by mutation | `TestPR14TextRun` and `TestPR14RunBeforeBound` drive `grade.Run` in text mode. Each mutation now fails |
| 3 | The evidence mapping: swapped bound keys, hard-coded cut flags and a count taken from the list length all survived | test (Major), code (Minor) | Reproduced | A `TestPR14DiscoveryEvidence` row with distinct flags and a count above the list length |
| 4 | Level 3's request-bound branch had no row after the flag split | test (Major) | Reproduced (D4 survived) | `TestDiscoveryMissingAndOverride/RequestMissing` |
| 5 | The QE14 twin agreed with the exit-to-word mapping, so it could not catch its own mutation | test (Major) | Reproduced | The doctored document contradicts the mapping. The ledger cites the tests that catch it |
| 6 | Masking rewrote engine-owned keys equal to a declared value | code (Minor) | Reproduced with a declared value `undeclared_names` | Values are masked, and keys only inside subject-supplied maps |
| 7 | Minors: the `exits 70` lines printed last, `any` comparisons in lookups, agreement legs with no planted defect (A1 to A3, A5 to A8), the own-verdict label, the rejected-mode row's rule, the scope line, comparison words outside their dimension, ledger proofs that did not kill their mutations, QS5's whole text | code, test | Confirmed | Each fixed. The ledger cites the catching tests and the subject package's QS5 tests |
| 8 | The note was amended during the build without review | code (Minor) | Confirmed | Logged in the design-review log as a build-time amendment |

## Evidence

- `gate.sh` and `check.sh` pass at each fix head.
- macOS evidence: the broad set, named in the pull request.
