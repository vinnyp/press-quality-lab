# Peer-review gate — phase 3 PR10 design note, slice b2 (2026-11-27)

**Mode:** design.

**Under review:**
- `docs/plans/2026-11-27-phase3-pr10-slice-b2-design-note.md`;
- `scripts/ratchet/pr10.json` with its prediction and the script's changes;
- errata E-35 to E-43.

**Reviewers:**
- peer-experiment-reviewer, the measurement lens;
- a different-model peer review on agy.

**persona-version:** cache/1.9.5.

**Tier rationale:** the plan requires a design-mode measurement review of each slice's design note before its build. The different-model pass follows AGENTS.md.

## Round 1 (range ce0531e to 94eeddb)

**Verdicts:**
- **measurement:** one Blocker, five Majors, six Minors and a Nit. The prediction reproduced byte for byte, and an independent classification of the corpus agreed.
- **agy:** three Blockers, six Majors, four Minors and a Nit.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | QE18 cannot leave the list: it is a GR15 rule needing a `macos_evidence` entry, and its scorecard clause is U18's | measurement (Blocker), agy (Blocker) | Confirmed against `gr15-rules.txt` and `ledger_test.go` | QE18 stays listed. The plan's U24 is amended |
| 2 | Moving QP6, QP8, QP9, QP12 and QP13 removed the only anchor for the real path's store control, snapshot and scan, which join in PR15 (U41, U32) | measurement (Major), agy (Major) | Confirmed against the plan's U41 and U32 and QT5 | All stay listed, each with what it lacks. QP9's control copy is U41's |
| 3 | QP7's scorecard clause is unbuilt | measurement (Major), agy (Blocker) | Confirmed against KTD16 | QP7 stays listed |
| 4 | A seed-supplied side file was called settled but reads three ways | measurement (Major) | Confirmed by grep | Put to the owner, who ruled E-41: QP8 adds to QP6 |
| 5 | E-10 and the plan's pre-registered unreadable-file scenarios were missing | measurement (Major), agy (Major) | Confirmed against U36, U37 and E-10 | Restored in control 6, at step 5 of the planted-secret table |
| 6 | Canary-only encoder rows cannot tell the url-safe forms from the standard ones | measurement (Major) | Worked through the 6-bit groups | Control 4 adds a declared value that holds `+` and `/`; control 5 checks the exact replacement |
| 7 | QO7's steps cross levels 7 and 8; level 7 could decide step 2 ahead of a leak or the control | agy (Blocker) | Confirmed against `decide.Resolve` and the script | The note's table gives each step's level, and level 7 defers. The script's deciding level follows QO7's order |
| 8 | The host's control results were not applied to the run's control facts, so E-37's attribution would not run | agy (Major) | Confirmed against `scripted/reader.go` | The note states the seam: the scripted source writes the host's results into the facts |
| 9 | Masking a padded form after its unpadded prefix leaves a stray `=` | agy (Major) | Confirmed against §4's note | Forms are masked longest first |
| 10 | The scan's file order and partial reads before a bound | agy (Minor) | Confirmed | Lexical order; each file read up to the room left; a leak in the bytes read wins over a bound |
| 11 | The 2^53−1 ceiling, and every integer `intValue` reads | measurement (Minor), agy (Minor) | Confirmed against the judge schema | An engine fault before the judge |
| 12 | A command listed twice in `secret.commands` | measurement (Minor) | Two-way | Put to the owner, who ruled E-42: reject the file |
| 13 | A canary in a name or link target | measurement (Minor) | Two-way | Put to the owner, who ruled E-43: a stated limit |
| 14 | `check-case-outcomes.py` contradicted E-38; PR10 was not in the regeneration test; QP1's mutation repeated PR8's; QP2's budget and QP3's "within" | agy, measurement (Minor) | Confirmed | Fixed |
| 15 | E-39's unit should be code points | measurement (Minor) | Confirmed | The note says code points |
| 16 | The pending cases, not the ratchet, discriminate | measurement (Minor) | Confirmed | A rule-to-case table in the note |

## Round 2 (delta, measurement lens; 94eeddb to f286329)

**Verdict:** no Blocker. One Major, six Minors and five Nits. Every round-1 disposition was checked and found done, and the prediction reproduced byte for byte.

| # | finding | verify | disposition |
|---|---|---|---|
| 17 | Control 6's unreadable rows could not fail on Linux: the walk's `chmod` resets a mode-0000 file and root reads it anyway | Confirmed against KTD20 | The rows inject `EACCES` through an opener seam after the reset, in the comparison and the scan |
| 18 | Control 5 could not tell E-39's unit; control 7's 64 KiB head limit would fail a correct collector on invalid UTF-8; E-41's consequence for an emptied seed WAL | Confirmed | A non-ASCII and a 7-byte value; the head is the decoding of the first 64 KiB; the limit stated |
| 19 | The mutations of rules that stay listed were dropped; QO7's proof should cite discovery's rows; the script's ordering rows are mostly decided by the reason table | Confirmed | Mutations restored in a table; QO7 cites PR9's rows; the ordering proof is the Go rows |
| 20 | Nits: the control runs only where QT4 calls for it; E-10 with a change elsewhere; byte-wise order and the total's scope; E-39's wording; E-35 against the control is a choice | Confirmed | Each stated. E-39 now says code points, the owner's ruling made precise |

**Design gate:** passed at the commit that adds this round. No Blocker or Major is open.
