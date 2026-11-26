# Peer-review gate — phase 3 PR8, slice a (2026-11-26)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer;
- a different-model review on agy, which timed out twice at its 15-minute bound and wrote nothing. AGENTS.md asks for a different model on specifications and plan phases, not on builds.

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Security:** the subject file and every file it names are untrusted input read by an unconfined engine.
- **Testing:** ten rules leave the unmapped list on these tests.
- **Code:** every subject-file rule and the whole required set change.

## Round 1 (head 7a32af7)

**Verdicts:**
- **code:** ready after three Majors;
- **security:** ship after two Mediums;
- **test:** trustworthy after one Blocker and seven Majors.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | QS8's home clause could be deleted with the suite green. The rows at and above the home were refused for a missing binary | test (Blocker) | Reproduced | Fixed: rows with a valid binary, the cause pinned |
| 2 | The home check compared path strings, so a case-variant or firmlink spelling of the home passed | security (Medium), code (Minor) | Reproduced | Fixed: compared by file identity with the home and each ancestor. The darwin case-variant row runs in the macOS evidence |
| 3 | Declared files were hashed once per mention, so a file QS11 refuses could hang the engine first | security (Medium) | Reproduced | Fixed: one digest per resolved path |
| 4 | QE2's `earlier` left out checks outside the scope, so ids changed with the scope | code (Major) | Reproduced | Fixed: every earlier check's id, whatever the scope |
| 5 | QH12's `DYLD_` and `__CF_` prefixes were not refused | code (Major) | Reproduced. E-9's order does not list QH12 | Put to the owner, who ruled E-21: QS7. Fixed |
| 6 | A placeholder-shaped literal was accepted by U9 and read as a placeholder by the override | code (Major), test (Major) | Reproduced. The text left it open | Put to the owner, who ruled E-22: a literal is data, and a placeholder has exactly one key. Fixed in all four classifiers |
| 7 | Schema-only violations inherited the enclosing rule's name | code (Minor) | Reproduced against E-9 | Fixed: QS2 unless the rule's clause states the condition. Round 2 corrected an overshoot (row 14) |
| 8 | A secret command with a declared non-bare prefix named QS6, not QP10 | code (Minor) | Reproduced | Fixed |
| 9 | An unterminated `{CAPTURE:` was rejected | code (Minor) | Confirmed against QA4 line 271 | Settled by the text: literal text. Fixed |
| 10 | The override ran inside `Resolve`, so it could stop a runbook (QA7) | code (Minor), test (Minor) | Confirmed | Fixed: applied once, after every check resolves, and tested as wired |
| 11 | A linked working directory gave a false QS8; a FIFO as the subject file blocked; the Linux account lookup could fall back to `$HOME` | security (Low ×3) | Reproduced, and the last confirmed in Go's source | Fixed |
| 12 | Untested: a bare command with a placeholder class or `writes`; a token straddling the 32 KiB read boundary; the QS12 branch for absolute executables; E-9's order beyond two pairs; two QA3 routes; QS9, QP7, QA9 and QP10 rows | test (Major ×6, Minor) | Reproduced by mutation | Fixed, each with a test that fails without it. QS12's absolute branch is settled by the text (QA4 line 270, QS12 line 115) |
| 13 | Smaller: level 2 failed open on a nil subject, an inert precondition loop, no corpus count guard, `chmod 0000` rows under root, EACCES mislabelled, invalid UTF-8 in `pql create` | code, test (Minor, Nit) | Confirmed | Fixed |

43 mutations, each turning a named test red, are recorded with round 1's fix.

## Round 2 (delta, 7a32af7 to dc1f8fd)

Two delta reviews: code and security, and test. Every round-1 fix had a test that failed without it, and both found the override move sound.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 14 | Round 1 dropped attributions the clauses state: `budget_seconds: 1.5` named QS2 though QS6 says "a whole number of seconds" | code (Minor), test (Major) | Reproduced. Settled by E-9, the clauses and line 310, with E-16 as the precedent | Fixed: clause-stated types name their rule, and bounds the text does not state stay QS2. The design note records the reading |
| 15 | The subject directory was derived lexically, so `lnk/../subject.json` paired one directory's subject file with another's binary. This was already in the base | code (Major) | Reproduced with a decoy binary | Fixed: the derived directory must hold the file whose descriptor was read, or the file is refused under QS8 |
| 16 | An unreadable absolute executable exited 70 | code (Minor), test (Minor) | Two-way: E-18 covers only relative kinds | Put to the owner, who ruled E-23: exit 2 under QS12, and a missing one is not compared. Fixed |
| 17 | QH12's reserved-name table was skipped for a setting with an empty `applies_to` | code (Minor) | Reproduced | Fixed |
| 18 | An absent account home exited 70; the ancestor-stat failure was untested | code (Minor), test (Minor) | Reproduced by mutation | Fixed: existing lexical ancestors are compared, and other failures exit 70 under test |
| 19 | Nested capture tokens, attribution keyed on magic values, the E-9 pair test not checking each second rule alone, a masked bookkeeping row, a silent darwin skip, an untested pre-open check | code (Nit), test (Minor) | Confirmed | Fixed |

20 mutations, each turning a named test red, are recorded with round 2's fix.

## Owner rulings during this review

E-21, E-22 and E-23, in `docs/specs/conformance-collection-1.0-errata.md`.

## Evidence

- `gate.sh`, `check.sh` and `scan.sh` pass at c02ab20.
- 35 predicted cases pass, 114 pending judge-checked cases end where `scripts/ratchet/pr8-prediction.json` says, and the prediction regenerates byte-identical.
- macOS evidence at c02ab20: 590 passed, 0 failed, no temporary directory left behind.
