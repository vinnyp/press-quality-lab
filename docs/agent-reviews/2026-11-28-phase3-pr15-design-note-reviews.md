# Peer-review gate — phase 3 PR15 design note, the real producer and parity (2026-11-28)

**Mode:** design.

**Reviewers:**
- peer-architecture-reviewer, because KTD13's seam between `collect` and the sources is the core of PR15a;
- a different-model review on agy (gemini-4-argon-high). Its first attempt passed the dispatcher's 15-minute bound and was killed, with no result, so it ran again on the revised note.

## Round 1 (note at 6fff675)

**Verdict (architecture):** no Blocker, nine Majors.

| # | finding | verify | disposition |
|---|---|---|---|
| 1 | Serving the read control as a second `Observe` that the scripted source ignores would flip every read proof to `harness_fault`. Having `collect` write `control_detected` itself would override QT4's precedence | Confirmed against `reader.go`, `trace.go`, `decide/planted.go`, and the two cases that force controls | Both controls pass through the source: `Spec.StoreControl`, and `Observe` with `Control`. The scripted source answers both from the host. QO7's condition moves out of the reader. Control 3 is restated as "served keys unchanged" |
| 2 | Darwin `Controls()` left out `PrivateCopyUnchanged`, which would stop discovery | Confirmed | Set from the once-per-grade check |
| 3 | A before-snapshot taken after backdating reads the planted file and uses up its one access-time update. The reviewer measured this on this machine | Reproduced by the reviewer | A fixed six-step order, the planted digest taken from the planting step, a serve-time assertion and a fake-clock row |
| 4 | The forgery test predicted the wrong facts: the template kills `utimes` and `setattrlist` | Confirmed against template line 171 and the spike record | Each forgery is a killed row with exact facts. QH28's harness-read case and the control profile's own control are added |
| 5 | Digests of files in the home cannot be masked after the fact, so the leak-file forms could never agree | Reproduced by the reviewer: `debug.log` holds the digest of the zero canary | The harness digests the used home again with the exact canary replaced, through `throwaway` |
| 6 | Decoys: who fills them, `peer.pid` showing as a change, decoys missing from the list, and confined agreement proving little | Confirmed | A decoy table, a test-only pre-snapshot hook, the baseline reset per run, positive controls around the confined repeats, 25 repeats for access-time decoys |
| 7 | One replay was undefined, and whole-grade repeats would cost tens of thousands of runs and self-test suites | Confirmed by count | One grade per served context, targeted repeats through an exported one-key function, extras reported, and the estimated cost |
| 8 | QH8 copying a `/bin` or `/usr/bin` executable, which macOS refuses to run | Confirmed against the PR11 measurement and QA4 | Put to the owner, who ruled E-55: such an executable runs in place |
| 9 | Ledger mutations no planned test would catch: QH27's device and inode, QH26's backdating, QP8, the deferred row | Confirmed | Each moved rule now names a catching test. PR15b's moves wait for named mutations, and QP8, QE8 and QS6 stay listed |
| 10 | Minors: the stream digest is over the kept bytes, QH8's check wording, E-38, the same canary, `.pql-tmp` and the per-step snapshots, `SystemNamed` untagged, the symbol check on a darwin build, the `remove guard` scope and the unreachable steps, the masking checks, Spotlight's status, evidence for a build difference, the plan amendment, the re-run in PR16, and the source's own writes | Confirmed | Each fixed in the note |

## Round 2 (note at c60ef5e)

**Verdicts:**
- **architecture:** no Blocker. All nine round 1 Majors are fixed, and two new Majors.
- **agy:** no Blocker, five Majors, two of them the same as the architecture review's.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 11 | `token_file`'s change-time rule cannot work: the snapshot's `chmod` and the backdating both move the change time | architecture (Major), agy (Major) | Reproduced by the architecture reviewer | Touched on the modification time or inode, against a baseline taken after backdating |
| 12 | A runbook step or a capturing computation cannot be repeated alone in a fresh home | architecture (Major), agy (Major) | Confirmed against the `add twice` recordings | A targeted repeat replays the whole runbook. The function enforces `MayServe` |
| 13 | A run refused because its private copy changed is voided by level 1 as `network_unverifiable` before level 5's QH8 `harness_fault` can decide it. This is a defect in today's `decide`, hidden on the scripted path by recorded markers | agy (Major) | Confirmed against `level1.go`, `level5.go`, `process_darwin.go:535`, QH6 and QH8 | Level 1 skips a `CopyChanged` run. A `Resolve` row and its twin. `Copies` holds one copy per executable |
| 14 | `reads-process-env` cannot use a platform binary as its victim under SIP | agy (Major) | Confirmed against the spike recorder at 490d650 | A non-platform victim built locally. The four process and network decoys named with their rules |
| 15 | The scripted reader would keep the recording's `control_end` on ordinary runs, so a `collect` that never runs the control would pass | agy (Major) | Confirmed against `reader.go` | The ordinary `Observe` clears it. The decorator folds the control run in before comparing |
| 16 | The partition was miscounted: 1,176 served, 30 named but unserved, 209 unnamed with 52 `remove guard` steps; 9 `live-first` keys the canonical subject file cannot produce | agy (Major) | Accepted from agy's count against all recordings, subjects, plans and cases | Counted in the note, and replayed from the recorded key |
| 17 | Minors: decoy reset and read order, bracketing only on targeted repeats, a fixed point for QH28's read, dedicated forgery programs, the serve-time assertion in a grade, `SystemNamed` on the product version only, E-55's `BIN` parameters, a typed pre-snapshot option, the QH26 and other ledger rows, the untagged access-time helper, projecting discovery runs before the walk drops bytes | architecture, agy | Confirmed | Each fixed in the note |

## Round 3 (confirmation, note at 25b12b1)

**Verdict:** architecture, no Blocker or Major. Both round 2 Majors are fixed, and agy's rows 13, 15 and 16 are faithful to the sealed text, each with two Minors.

| # | finding | disposition |
|---|---|---|
| 18 | Make the changed-copy skip an invariant, and say whether E-53's capture loop skips it too | Both loops skip it. A changed-copy run that carries a marker or an end is an engine fault |
| 19 | Parity does not check `collect`'s own fold of the control. Keep the projection test field for field | The harness compares `collect`'s `control_end` for planted keys. The projection test stays strict |
| 20 | A key gives only the words, the offline arguments and the environment names. Steps after a failed step are served regardless | Stated in the note |

The design review is closed.
