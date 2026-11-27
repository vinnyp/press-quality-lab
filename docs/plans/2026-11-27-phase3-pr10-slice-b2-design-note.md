# Design note: PR10, slice b2 (the probes, the leak scan and masking)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR10. **Units:** U12, U36, U38, U37, U24.

This note follows the slice b1 note, `docs/plans/2026-11-26-phase3-pr9-slice-b1-design-note.md`. Its definitions and controls hold here, among them control 2's full comparison on QE19's fields and the deciding level, and control 3's exact, case-derived trace. This note says only what slice b2 changes. Its design review is logged in `docs/agent-reviews/2026-11-27-phase3-pr10-design-note-reviews.md`.

## What the slice makes true

- **The bare, byte-bound and exit-code probes (U12)** decide at level 8 as QP2 to QP4 say.
  - **Bare** holds on a code QS6 accepts for the bare binary. Undeclared, that is 0, with a 10-second budget.
  - **Byte bound** holds when the bytes drained from standard output and standard error together are at most the bound. It is empty-satisfiable, and its `target_size` is that sum (QE8). All 398 corpus checks that carry one agree.
  - **Exit codes** holds on a declared code.
  - **A probe that runs no command** enters as one check, not observed with `empty_target` (QP1).
- **The store-unchanged probe (U36)** decides at level 8 as QP6 to QP9 say.
  - It compares the home's names, kinds and regular-file bytes before and after, after QP7's exclusions, `.pql-tmp`, and QP8's side-file rule. E-41 makes QP8 a condition added to QP6.
  - A link or special file the subject created is always a change, and is never followed or opened (QH25).
  - QP9's `empty_target` and its control-copy `harness_fault` come before the comparison.
  - An entry the inspection cannot read is `harness_fault`, never unchanged (E-10).
- **The planted-secret probe (U37).** QO7's combination is decided by one function, applied in QO7's order. Each step reports the level the table below gives, and level 7 decides a planted-secret check only when the combination's deciding step is one of its own. A leak and the read proof's control come ahead of step 2, so level 7 defers to level 8 when either applies.

  | Order | Step | Outcome | Level |
  |---|---|---|---|
  | — | any listed invocation unconfined (E-36) | `network_unverifiable` | 1 |
  | 1 | a leak in any invocation | `not_held`, `predicate` | 8 |
  | 2 | an engine kill outside the budget on any invocation (E-35) | `harness_fault` | 7 |
  | 3 | the read proof's control ran and ended any way other than the confinement's kill (QO7, E-37); not run when the calibration failed (E-38) | `harness_fault` | 8 |
  | 4 | the first invocation, in listing order, that QO2 to QO4 decide | its result | 7 |
  | 5 | a scan stopped by a bound, or an entry it could not read (QP13, E-10) | `harness_fault` | 8 |
  | 6 | at least one invocation proved a read (QP11, QH26 to QH28) | held | 8 |
  | 7 | otherwise, or `secret` is `null` | `input_missing` | 8 |

  A non-success exit or a crash on a planted invocation only means it proved no read (QO7, last line). So the combination uses a QO1-to-QO4-only helper, as discovery does, never level 7's generic `InvocationOutcome`.
- **The control facts' seam.** In a scripted run, the host's results replace the recording's for the same control (QT4, line 270). The scripted source writes the host's `read_control` and `store_control` into the run's control facts before any decision reads them:
  - `killed` becomes a confinement kill after the marker;
  - `not_killed` becomes an end that is not one;
  - `detected` and `not_detected` become `control_detected`.

  The decision then reads the facts through the attribution function (E-37), never a separate host flag. The darwin source fills the same facts from its own runs.
- **The leak scan and its encoder (U38).**
  - **The encoder** produces the nine forms of `credit-and-routes.md` §4, written from the table.
  - **What the scan reads:** each drained stream up to QE10's kept bytes, and each regular file of the used home other than the planted one, bookkeeping paths and `.pql-tmp` included.
  - **The order:** files are walked in lexical order of their paths relative to the home. Each is read up to the smaller of 16 MiB and the room left in the 256 MiB total.
  - **Bounds and leaks:** a leak in the bytes read is reported even when a bound was also reached, so step 1 wins over step 5.
  - **Unread files:** a file the scan cannot read is E-10's `harness_fault`, at step 5.
  - **Names are not scanned (E-43).**
- **Masking (U24).** The same encoder serves masking. Every record and the terminal mask the canary and each declared value of 8 bytes or more, as QE18 and KTD16 say.
  - **The width:** one `*` per Unicode code point of the encoded form (E-39).
  - **Before cutting:** masking runs before any head is cut, and never changes a byte count, a digest or an outcome.
  - **Order of forms:** a value's forms are masked longest first, so a padded `base64` form is masked whole before its unpadded prefix could match. No `=` or `==` is left behind.
- **The evidence fields.** `target_size` and the bounded `expected` and `observed` stream objects (QE8, QE10) are filled for the probes that have them. QE19 does not compare `expected` or `observed`, so control 7 checks their shape and bounds directly.
- **Real-path code.** The real path's code for this slice lives in `internal/throwaway` (KTD20) and is unit-tested on temporary trees:
  - the seed copy into a fresh home (QP5, QH14);
  - the home snapshot and inspection (QH25);
  - the file scan;
  - the canary planting, with its access-time setting.

  The released source still refuses to observe until PR15 joins the darwin source. Until then the corpus proves the decisions from recorded facts, and the unit tests prove the file code. QP9's append-one-byte control copy is built with the rest of the real producer in PR15 (U41), not here.
- **A rejection (E-42).** A `secret.commands` that lists the same words twice rejects the subject file under QP10.

## Two defects this slice must not inherit

- **`acceptedCode` and the planted secret.** `acceptedCode` (`internal/decide/level7.go`) reads `words` from the check's preimage. A planted-secret check has none, so it falls back to the bare entry's codes, or to 0 when the bare binary is undeclared. For the planted secret, each invocation's accepted codes come from its own listed command, by the invocation's key.
- **Byte counts too large to carry.** The scripted reader's `intValue` returns 0 for an integer above int64, which would make QP3 read 0 bytes and hold wrongly. The judge's input caps a count at 2^53−1 (`judge-input.schema.json`), while a recording's byte count is unbounded. Byte counts are read exactly. A count, or a byte bound's sum, above 2^53−1 is an engine fault, exit 70, raised before the judge is called, never a silent 0. The same holds for every integer `intValue` reads, `sandbox_exec_exit` included.

## Evidence rules

All eight levels are built for the six probes. Level 4 (QA7), level 6 (QO6, QA4's cut capture) and level 8 for authored checks (QA9 and the predicates) stay unbuilt, for slice c.

## Readings the build settled from the text

- **When a scan bound counts as reached.** QP13 says the scan reads each file "up to 16 MiB" and each stream "up to the bound of QE10". A bound is reached when bytes past it were left unscanned:
  - a file larger than 16 MiB;
  - a cut stream (QE10: "past that point it records that the stream was cut");
  - a file that would take the total past 256 MiB.

  A file of exactly 16 MiB has been read whole and reaches nothing. This matches QP13's reason for the `harness_fault`: a leak could hide in what was not read.
- **Where the canary lives.** Only the planted-secret probe's homes carry a canary. QT5 calls them `planted`, distinct from `seed` (QP10 line 236, QT5 line 284).
- **A crash against QP9.** Level 7 comes before level 8, so a store-unchanged check whose invocation crashed ends `crashed` (`case-076`, `case-078`, `case-089`). QP9's `empty_target` and `harness_fault` are ordered only against QP6's comparison.
- **The control's streams and home.** QT5 records only the control's end, QE8 says a control is never a target, and QT4's "What stays the same" has QP11 decide from the control's raw result exactly as the released build does. So the control feeds nothing to the leak scan.
- **The plan's "33-digit number" (U12).** This is the plan's own scenario, and it covers two things. A byte count too large to carry is an engine fault (above). The `big-number` form's 33-digit JSON number in standard output survives into the record's bounded stream object unchanged.

## Predicted ratchet set

PR10 extends the prediction script as PR9 did for discovery: one `builds` token per probe level-8 rule, so each probe's level 8 is built separately. Authored checks keep a level 8 no token builds, and `levels` does not gain 8. Without the per-probe tokens, the script would credit `case-156` and `case-157`, whose authored QA9 checks are unbuilt. The existing `QO7.planted_secret` token is built.

The script's deciding level for a planted-secret check follows the table above, in QO7's order.

`scripts/predict-ratchet.py --built scripts/ratchet/pr10.json` computes `scripts/ratchet/pr10-prediction.json`. `check.sh` and the Go readers switch to it. PR6's, PR8's and PR9's files reproduce exactly in the script's tests.

**Predicted:** 58 cases, PR9's 37 plus 21. The plan's named cases are among them.

| Cases | What decides them |
|---|---|
| `case-012` to `case-015`, `case-088` | store unchanged, not held with `predicate` (a written store, a created file, a link, a WAL with bytes, a link to a decoy) |
| `case-074`, `case-075`, `case-096`, `case-099`, `case-148` | every probe held at level 8 |
| `case-072` | store unchanged, `over_budget` (QO4) |
| `case-073` | store unchanged, `harness_fault` (QO4, the known-fast control over) |
| `case-076`, `case-078`, `case-089` | store unchanged, `crashed` (QO5; 078 and 089 by the host's attribution) |
| `case-077` | store unchanged, `network_unverifiable` (QO3) |
| `case-079` | store unchanged, `harness_fault` (an engine kill) |
| `case-095` | every probe `could_not_start`, the planted secret included (step 4) |
| `case-098` | three probe checks `network_unverifiable` (QO3) |
| `case-150` | the planted secret `network_unverifiable` (step 4), and the store check likewise |
| `case-151` | the planted secret held with two listed commands, one of which proves no read (step 6) |

**The ratchet alone discriminates little.** In all 21 predicted cases every bare, byte-bound, exit-code and planted-secret check is held or decided at level 7. A level 8 that always held would pass control 1. The pending cases do the discriminating, through control 2's check-by-check comparison.

| Rule | Pending cases that discriminate it |
|---|---|
| QP3 over the bound | `case-023` |
| QP8 | `case-016`, `case-017` |
| QP9 | `case-007`, `case-138` |
| QP11, `input_missing` | `case-006`, `case-018`, `case-019`, `case-144` |
| QO7, the read control not killed | `case-008` |
| QP12, a leak | `case-030` to `case-039` |
| QP13, a bound | `case-040` |

The leak cases record that a leak exists. They test the decision, not the encoder, which control 4 tests.

### Every pending case's predicted ending

All 91 pending judge-checked cases still end with exit 70. No ending names a probe rule or QO7 any more.

| Rules the ending may name | Cases |
|---|---|
| QA7, QA9, QO6 | 71 |
| QA7, QA9 | 7 |
| QA7 | 2 (`case-005`, `case-092`) |
| QO6 | 2 (`case-010`, `case-011`) |
| QA9 | 2 (`case-156`, `case-157`) |
| QE4 (a baseline's planning gap) | 6 |
| `QE4.anchor` | 1 |

## Rules this slice moves off KTD22's list

Only rules whose whole text is decision logic this slice builds and proves move. Every rule that a real-path producer must also satisfy stays listed until that producer is joined, so the list cannot reach empty while the real engine has never been shown to look. Each mutation is fixed now, before the build.

| Rule | Proof | Its mutation |
|---|---|---|
| QP1 | Every probe enters the required set, and level 8 decides each probe that runs no command as `empty_target` | decide a probe that runs no command as held |
| QP2 | Unit rows for a declared and an undeclared bare binary, the undeclared one's 10-second budget included, plus the corpus | accept any code for an undeclared bare binary |
| QP3 | Unit rows below, at and past the bound ("within" is at most); one where each stream is within the bound but the sum is not; `target_size`; plus `case-023` | compare only standard output with the bound |
| QP4 | Unit rows for a declared and an undeclared code | accept the codes of a successful run instead of the declared ones |
| QP11 | Unit rows for each of the four conditions failing alone, with E-37 and E-38 | ignore the device and inode |
| QO7 | The table above, tested step against step in both orders, with the level each reports; E-35 to E-38; a non-success exit and a crash that prove no read; plus `case-095`, `case-150` and `case-151` | let a crash on a planted invocation decide the check as `crashed` |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QP5, QP10, QH14 | the darwin source's fresh home per invocation and its canary planting (PR15) |
| QP6, QP8, QP12, QP13 | the darwin source calling `internal/throwaway`'s snapshot and scan, and U32's parity on the store and leak forms (PR15) |
| QP7 | the scorecard's list of excluded paths (U18) and the real path (PR15) |
| QP9 | the engine's control copy, appending one byte (U41, then U32) |
| QH25 to QH28 | the real path's inspection, calibration and control run (PR15) |
| QE8, QE10 | the computations' and authored checks' targets and values (slice c), and draining on the real path (PR11's U30, joined in PR15) |
| QE18 | a `macos_evidence` entry (GR15), masking of a real canary on the darwin path (PR15), and the scorecard (U18) |
| QO5 | the predicates' accepted codes and `predicate` (slice c) |

The plan's U24 says "QE18 leaves KTD22's list". That contradicts KTD22 and GR15, so the plan is amended to match.

## Controls

1. **The ratchet.** The 21 cases leave `internal/corpusrunner/pending.txt` and must pass. Every other case must still fail.
2. **Every check, compared in full** (the slice b1 note's control 2), byte bound's `target_size` included, with the deciding level the table above gives.
3. **The trace** (the slice b1 note's control 3).
   - **Planted keys:** the planted-secret probe's `planted` keys are served in listing order for a check that reaches level 7, and are not served for a check decided earlier.
   - **The read-proof control:** it is never a served key, since a scripted run takes its result from the host.
4. **The encoder from the table.** One row per §4 form, with its expected text written from the table by an independent reference, never by the encoder. It runs on the canary, and on a declared value whose base64 holds `+` and `/` and whose length is not a multiple of 3. That second value is what tells the url-safe forms from the standard ones, which the canary's alphabet cannot do.
5. **Masking from the seam** (U24's scenarios). Each injected value, the canary and that declared value, is shown present before masking. Afterwards its exact replacement text, of the right length, stands in its place in each form, and nothing is left of the original. A masker that deleted text, or left a padding character, fails.
6. **Unit rows for every branch the corpus lacks:**
   - the special-file kind and a removed change;
   - `.pql-tmp`;
   - a seed side file (E-41);
   - `same_file` false;
   - a control end other than the confinement's;
   - a non-success exit or a crash on a planted invocation;
   - E-35 to E-38;
   - a stderr leak, and a canary straddling the cut;
   - each single scan bound, at and one past;
   - a repeated secret command (E-42);
   - E-10's unreadable entry, in the comparison and in the scan. The plan's U36 and U37 scenarios hold: a mode-0000 file is read, or never ends as unchanged or held, and a scan mutated to skip unreadable files turns a test red.
7. **The stream objects.** Each `expected` and `observed` stream object has exactly QE10's keys, its head within 64 KiB, and its digest over the kept bytes. QE19 does not compare them, so this control does.
8. **The reference script.** `scripts/check-case-outcomes.py` follows E-38: the read proof's control is a candidate only when the calibration moved.

## Stated limits of this slice

- The real path's homes, planting, inspection, scan and controls are unit-tested but not joined to a darwin run until PR15, and QP9's control copy is built there.
- No corpus subject declares an environment value of 8 bytes or more, and recordings hold no canary. So QE18's masking is proven by unit tests only, as `CASES.md` says.
- Base64 alignment in the scan is E-14's stated limit, and a canary in a name is E-43's.
