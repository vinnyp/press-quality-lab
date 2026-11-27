# Design note: PR10, slice b2 (the probes, the leak scan and masking)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR10. **Units:** U12, U36, U38, U37, U24.

This note follows the slice b1 note, `docs/plans/2026-11-26-phase3-pr9-slice-b1-design-note.md`. Its definitions and controls hold here, among them control 2's full comparison on QE19's fields and the deciding level, and control 3's exact, case-derived trace. This note says only what slice b2 changes.

## What the slice makes true

- **The bare, byte-bound and exit-code probes (U12).** They decide at level 8 as QP2 to QP4 say:
  - bare holds on a code QS6 accepts for the bare binary;
  - byte bound holds when the bytes drained from standard output and standard error together stay within the bound;
  - exit codes holds on a declared code.

  Byte bound is empty-satisfiable, and its `target_size` is that sum, as QE8 says. Every corpus check that carries one agrees (398 checks).
- **The store-unchanged probe (U36).** It decides at level 8 as QP6 to QP9 say.
  - It compares the home's names, kinds and regular-file bytes before and after, after QP7's exclusions, QP8's side-file rules and `.pql-tmp`.
  - A link or special file the subject created is always a change, and is never followed or opened (QH25).
  - QP9's `empty_target` and its control-copy `harness_fault` come before the comparison.
- **The planted-secret probe (U37).** QO7's planted-secret combination is built at level 7, and QP10 to QP13 at level 8:
  - step 1 is a leak in any invocation;
  - E-35 (an engine kill) and the read proof's control (QO7, E-37, E-38) come next;
  - step 2 is an invocation QO2 to QO4 decide, in listing order;
  - QO1 on any listed invocation is decided at level 1 first (E-36);
  - step 3 is a scan bound;
  - step 4 is a proved read (QP11, QH26 to QH28);
  - step 5 is `input_missing`.

  A non-success exit or a crash on a planted invocation only means it proved no read (QO7, last line). This needs a QO1-to-QO4-only helper, as discovery has, not level 7's generic `InvocationOutcome`.
- **The leak scan and its encoder (U38).** One encoder produces the nine forms of `credit-and-routes.md` §4, written from the table. The scan reads each drained stream up to QE10's bound, and each regular file of the used home other than the planted one, bookkeeping paths and `.pql-tmp` included, up to QP13's bounds.
- **Masking (U24).** With the same encoder, every record and the terminal mask the canary and each declared value of 8 bytes or more, as QE18 and KTD16 say, with E-39's width. Masking happens before any head is cut, and never changes a byte count, a digest or an outcome.
- **The evidence fields.** `target_size` and the bounded `expected` and `observed` stream objects (QE8, QE10) are filled for the probes that have them.
- **Real-path code.** The real path's code for this slice lives in `internal/throwaway` (KTD20): the seed copy into a fresh home (QP5, QH14), the home snapshot and inspection (QH25), the file scan, and the canary planting with its access-time setting. It is unit-tested on temporary trees. The released source still refuses to observe until PR15 joins the darwin source, so the corpus proves the decisions from recorded facts, and the unit tests prove the file code.

## Two defects this slice must not inherit

- **`acceptedCode` and the planted secret.** `acceptedCode` (`internal/decide/level7.go`) reads `words` from the check's preimage, which a planted-secret check does not have, so it falls back to the bare command's codes. For the planted secret, each invocation's accepted codes come from its own listed command, by the invocation's key.
- **Byte counts above int64.** The scripted reader's `intValue` returns 0 for an integer above int64. That would make QP3 read 0 bytes and hold wrongly. Byte counts are read exactly. A count that cannot be held exactly is an engine fault, never a silent 0.

## Evidence rules

All eight levels are built for the six probes. Level 4 (QA7), level 6 (QO6, QA4's cut capture) and level 8 for authored checks (QA9 and the predicates) stay unbuilt, for slice c.

## Readings the build settled from the text

- **When a scan bound counts as reached.** QP13 says the scan reads each file "up to 16 MiB" and each stream "up to the bound of QE10". A bound is reached when bytes past it were left unscanned:
  - a file larger than 16 MiB;
  - a cut stream (QE10: "past that point it records that the stream was cut");
  - a file that would take the total past 256 MiB.

  A file of exactly 16 MiB has been read whole and reaches nothing. This matches QP13's reason for the `harness_fault`, which is that a leak could hide in what was not read.
- **A seed-supplied side file.** QP8 says `-wal` and `-journal` "are a change only when they exist with one byte or more after the command". That is QP8's specific rule for those files, so it governs over QP6's general content comparison. A side file of zero bytes after the command is not a change, whatever it held before. No seed in the corpus supplies one.
- **Where the canary lives.** Only the planted-secret probe's homes carry a canary. QT5 calls them `planted` homes, distinct from `seed` (QP10 line 236, QT5 line 284).
- **A crash against QP9 or QP11.** Level 7 comes before level 8, so a store-unchanged check whose invocation crashed ends `crashed` (`case-076`, `case-078`, `case-089`). QP9's `empty_target` and `harness_fault` are ordered only against QP6's comparison.
- **The control's streams and home.** QT5 records only the control's end, and QE8 says a control is never a target, so the control feeds nothing to the leak scan.
- **The plan's "33-digit number" (U12).** This is the plan's own scenario. It means two things: a stream byte count too large for int64 is read exactly or is an engine fault (above), and the `big-number` form's 33-digit JSON number in standard output survives into the record's bounded stream object unchanged.

## Predicted ratchet set

PR10 extends the prediction script as PR9 did for discovery, with one `builds` token per probe level-8 rule, so each probe's level 8 is built separately. Authored checks keep a level 8 that no token builds. `levels` does not gain 8. Without the per-probe tokens, the script credits `case-156` and `case-157`, whose authored QA9 checks are unbuilt. The existing `QO7.planted_secret` token is built.

`scripts/predict-ratchet.py --built scripts/ratchet/pr10.json` computes `scripts/ratchet/pr10-prediction.json`, which `check.sh` regenerates and compares. PR6's, PR8's and PR9's files reproduce exactly in the script's tests.

**Predicted:** 58 cases, PR9's 37 plus 21. The plan's named cases are among them.

| Cases | What decides them |
|---|---|
| `case-012` to `case-015`, `case-088` | store unchanged, not held with `predicate` (a written store, a created file, a link, a WAL with bytes, a link to a decoy) |
| `case-074`, `case-075`, `case-096`, `case-099`, `case-148` | every probe held at level 8 |
| `case-072` | store unchanged, `over_budget` (QO4) |
| `case-073` | store unchanged, `harness_fault` (QO4, the known-fast control over) |
| `case-076`, `case-078`, `case-089` | store unchanged, `crashed` (QO5; 078 and 089 by the host's memory and code-signing attribution) |
| `case-077` | store unchanged, `network_unverifiable` (QO3) |
| `case-079` | store unchanged, `harness_fault` (an engine kill) |
| `case-095` | every probe `could_not_start`, the planted secret included (QO7 step 2) |
| `case-098` | three probe checks `network_unverifiable` (QO3) |
| `case-150` | the planted secret `network_unverifiable` (QO7 step 2), and the store check likewise |
| `case-151` | the planted secret held with two listed commands (QO7 step 4) |

Every other case that exercises the leak scan, the read proof or QE10 stays pending, because it also holds an authored check: `case-030` to `case-040`, `case-142`, `case-006`, `case-008`, `case-018`, `case-019`, `case-087`, `case-144`, `case-149`, `case-016`, `case-017` and `case-023` to `case-025`. Control 2 compares their probe checks check by check.

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

The build regenerates these counts from the script and reports them.

## Rules this slice moves off KTD22's list

Each mutation is fixed now, before the build.

| Rule | Proof | Its mutation |
|---|---|---|
| QP1 | All six probes enter the required set as QP1 says, now that every probe decides | drop a probe's check when it runs no command, rather than entering it with `empty_target` |
| QP2 | Unit rows for a declared and an undeclared bare binary, plus the corpus | accept any code for an undeclared bare binary |
| QP3 | Unit rows within, at and past the bound, including one where each stream is within the bound but the sum is not; `target_size`; plus `case-023`'s check | compare only standard output with the bound |
| QP4 | Unit rows for a declared and an undeclared code | accept the codes of a successful run instead of the declared ones |
| QP6 | Unit rows for each kind of change, and that times, modes and owners are not compared | compare a file's mode |
| QP7 | Unit rows inside, on the way to, and outside a bookkeeping path, and `.pql-tmp` | leave out the directories on the way to a bookkeeping path too broadly, so a sibling's change is hidden |
| QP8 | Unit rows for `-shm`, an empty and a non-empty `-wal` and `-journal`, a changed store with clean side files, and a non-regular `-shm` | count an empty `-wal` as a change |
| QP9 | Unit rows for no store, a seed with no non-empty regular file, the control `not_detected`, and a link or special file inside an excluded path; plus `case-007` and `case-138` | let the comparison decide when the control was not detected |
| QP11 | Unit rows for each of the four conditions failing alone, with E-37 and E-38 | ignore the device and inode |
| QP12 | The encoder's nine rows from the §4 table, a leak in each place (a stream, a home file, a bookkeeping file, `.pql-tmp`), and the planted file excluded | skip `.pql-tmp` in the scan |
| QP13 | Unit rows for each of the three bounds, at and one past each | treat a file of exactly 16 MiB as past the bound |
| QO7 | Unit rows for every pair of steps in both orders, E-35 to E-38, and the non-success exit or crash that proves no read; plus `case-095`, `case-150` and `case-151` | let a crash on a planted invocation decide the check as `crashed` |
| QE18 | U24's injected-value tests: each value in each §4 encoding, present before masking and absent from the store, the streams and the terminal after, including a value straddling the 64 KiB head edge and the digest cut | mask after cutting the head |

**Stay listed**, each with what it still lacks:

| Rule | Still lacks |
|---|---|
| QP5, QP10, QH14 | the darwin source's fresh home per invocation and its canary planting (PR15) |
| QH25 to QH28 | the real path's inspection, calibration and control run (PR15) |
| QE8, QE10 | the computations' and authored checks' targets and values (slice c) |
| QO5 | the predicates' accepted codes and `predicate` (slice c) |

## Controls

1. **The ratchet.** The 21 cases leave `internal/corpusrunner/pending.txt` and must pass, and every other case must still fail.
2. **Every check, compared in full** (the slice b1 note's control 2), now including byte bound's `target_size`.
3. **The trace** (the slice b1 note's control 3). It now includes the planted-secret probe's `planted` keys in listing order. A read-proof control is never a served key, because it is the host's value in a scripted run.
4. **The encoder from the table.** One row per §4 form, with its expected text written from the table and an independent reference, never from the encoder. The table's own note holds: `base64` and `base64url` give the same text for the canary's alphabet, and each unpadded form is a prefix of its padded one.
5. **Masking from the seam** (U24's scenarios). Each injected value is shown present before masking, and absent afterwards, in each form.
6. **Unit rows for every branch the corpus lacks.** The research lists them: no corpus case reaches the special-file kind, a removed change, `.pql-tmp`, a seed side file, `same_file` false, a control end other than the confinement's, a non-success exit or crash on a planted invocation, E-35 to E-38, a stderr leak, a canary straddling the cut, or any single scan bound.

## Stated limits of this slice

- The real path's homes, planting, inspection and controls are unit-tested but not joined to a darwin run until PR15.
- No corpus subject declares an environment value of 8 bytes or more, so QE18's declared-value masking is proven by unit tests only, as `CASES.md` says.
