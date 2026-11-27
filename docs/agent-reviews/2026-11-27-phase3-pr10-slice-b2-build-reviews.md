# Peer-review gate — phase 3 PR10, slice b2 (2026-11-27)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer.

The design note had its different-model review on agy, logged in `docs/agent-reviews/2026-11-27-phase3-pr10-design-note-reviews.md`. AGENTS.md asks for a different model on specifications and plan phases, not on builds.

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Security:** the subject's output and its used home are untrusted, and the canary is a secret the engine must never show.
- **Testing:** six rules leave the unmapped list, and masking and the scan are proven only by unit tests.
- **Code:** every probe now decides.

## Round 1 (head 6b8b2bd)

**Verdicts:**
- **code:** ready after two Majors;
- **test:** trustworthy after five Majors, with no Blocker;
- **security:** safe as the scripted slice after one Medium.

All three confirmed that the decision logic follows the sealed text and the errata, and that all six ledger moves fail their named tests.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The home walk re-resolved each full path (14 s at depth 400), and a walk failure returned an error that discarded the verdict | security (Medium) | Measured | Walk by descriptor with a depth bound. The owner ruled E-46: an unfinished inspection is `harness_fault` for its check, keeping found leaks |
| 2 | Masking at the record seam was unproven: dropping the masker's wiring left every test green | code (Major), test (Major) | Reproduced | A `grade.Run` test with a declared value proves the record, the judge input and the terminal |
| 3 | QO7 step 5's unreadable half was untested | code (Major), test (Major) | Reproduced | A pairwise step and an explicit row |
| 4 | E-41's modified side file, the scan's encoded forms, and E-10's chmod, read-error and deadline branches were untested | test (Major ×3) | Reproduced | Rows for each, with injected failures |
| 5 | Overlapping masked values left part of one visible | security (Low), code (Minor) | Reproduced | The owner ruled E-44: every covered byte is replaced |
| 6 | A canary in an extended attribute or resource fork is never scanned | security (Info) | Confirmed | The owner ruled E-45: a stated limit |
| 7 | The terminal re-masked the already-masked judge document, corrupting it; escaping ran before masking | code (Minor) | Reproduced | Value-level masking only, and masking before escaping |
| 8 | A recorded cut head, stream objects recognised by shape, unmasked map keys, a null control end panicking, identity and `Nlink` guards, integer-check order, masking speed | code, security, test (Minor, Low) | Confirmed | Fixed |

43 of 45 mutations were caught. The two survivors are a pair of guards that back each other up, and removing both is caught.

## Round 2 (delta, 6b8b2bd to 1dddd47)

**Verdict:** one Major, five Minors. No Blocker.

| # | finding | verify | disposition |
|---|---|---|---|
| 9 | A cut-short snapshot invented changes, so an unchanged home read as not held | Reproduced | The owner ruled E-47: a change actually found decides, and only entries both snapshots inspected count. An unfinished walk is tracked separately |
| 10 | The masking hold-back was quadratic and unbounded on self-overlapping runs | Measured | One sorted pass, with a bounded carried state |
| 11 | Stdout carried subject text without KTD16's escape | Confirmed | KTD16's classes become JSON escapes on stdout |
| 12 | Masked keys could collide; a hard link hid a leak; three tests did not prove what they claimed | Reproduced | Fixed |
| 13 | A byte both a canary and a declared value cover | Two-way | The owner ruled it shows the canary's replacement (E-44) |

All 17 mutations were caught.

## Round 3 (confirmation, 1dddd47 to ba3941e)

No Blocker or Major remained. The reviewer's 14 mutations confirmed the fixes. Three nits are left, none affecting a verdict:
- the scripted reader builds runs in map order;
- `pql judge` writes stdout without the terminal escape, but its input is an evidence record;
- two mutations change nothing observable.

## Owner rulings during this review

E-44 (with its addition), E-45, E-46 and E-47, in `docs/specs/conformance-collection-1.0-errata.md`.

## Evidence

- `gate.sh`, `check.sh` and `SCAN_COMMITS=<base>..HEAD ./scan.sh` pass.
- 58 predicted cases pass, and all 91 pending judge-checked cases end where `scripts/ratchet/pr10-prediction.json` says.
- macOS evidence at ba3941e: 1,200 passed, 0 failed, no temporary directory left behind.
