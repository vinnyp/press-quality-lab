# PR3 build: review log

The plan is `docs/plans/2026-11-21-2309-feat-conformance-collection-spec-plan.md`, PR3 group. This log records the review of the test entry's specification text and the corpus's map to the plan's acceptance examples. Later sections record the reviews of the frozen text and the corpus, and the blind derivation by a different model.

## The test entry's text

The reviewer was a different model from the author: agy, `gemini-4-argon-high`, briefed with paths to the kit folder and its schemas, in `design` mode with the standards lens.

| Round | Verdict | Findings | Disposition |
|---|---|---|---|
| 1 | publish with these changes; not self-contained | 4 Blocker, 5 Major, 2 Minor, 1 Nit | All fixed in 7d2db33 (text) and b0cb160 (schemas and tools). Round 2 verified each closed. |
| 2 | publish with these changes; self-contained except three edge interactions | 3 Major, 2 Minor | All fixed in 73a9689. Round 3 verified each closed. The three new worked ids in QE2 were recomputed and match. |
| 3 | publish/freeze as-is; self-contained | 1 Minor: whether QT5's source digest has directory entries | Fixed: QT5 says it has an entry for each file and none for a directory. |

Round 1's Blockers were: a `kill_attribution` value with no decision rule; no stated way for a scripted run to receive its recording; `input.fixtures` with no assembly rule; and a baseline schema that rejected placeholder objects in preimages. Each finding was checked against the text before it was acted on.

## The corpus and the acceptance examples

The corpus has 152 cases: 121 judge-checked and 31 derivation-only. Every judge-checked case's expected output was written from the rules and agrees with the judge (`./check.sh`, step `cases`). Every held or not-held outcome a case states was also checked against its form's recording by a separate checker that reads only the case and the recording. It found two cases red on a second check, `case-039` and `case-040`, which were fixed. The four outcomes it cannot evaluate are the ends by a signal, which QO4 and QO5 decide: `case-072`, `case-076`, `case-078` and `case-089`.

| Example | Cases | Note |
|---|---|---|
| CAE1 | case-080 | |
| CAE2 | case-081 | |
| CAE3 | case-077, case-076, case-072 | Self-kill not measured; crash and overrun red. |
| CAE4 | case-139, case-010 | A list that agrees with itself holds without credit; the literal count is red. |
| CAE5 | case-140 | The count disagrees with the list: red. |
| CAE6 | case-050, case-051 | |
| CAE7 | case-001, case-013, case-138 | Journal excluded; another file red; no store gives `empty_target`. |
| CAE8 | case-018, case-019 | |
| CAE9 | case-063 | The unclassified command is a top-level one, not a nested one. |
| CAE10 | case-098, case-099 | |
| CAE11 | case-083 | |
| CAE12 | case-003, case-004 | The system version is not a case input. QO1 gives the same record when confinement is not applied. |
| CAE13 | none | A collector that follows the text never sends a record the judge rejects, so no case input leads to exit 70. QE15 is listed in `CASES.md` as not expressible. |
| CAE14 | case-071, case-143 | The scripted anchor is refused as a test run; the anchor on the real verifier is anchored. |
| CAE15 | case-084 | |
| CAE16 | case-085, case-097 | |
| CAE17 | case-020, case-021 | One declared command missing of five, not 37 of 40; the same rule, QD4, decides. |
| CAE18 | case-136 | The marker is rejected with 2 before collection. Exit 64 needs a malformed command line, which is not a case input. |
| CAE19 | case-003, case-090, case-004 | Nested sandbox (71), broken profile (65), and a self-test that fails. |
| CAE20 | case-110, case-112 | A path that climbs out and a link in the seed. A path holding profile syntax is a parameter value (QH11), and a scripted run applies no profile. |
| CAE21 | case-023, case-024, case-025, case-056 | |
| CAE22 | case-080 to case-087 | |
| CAE23 | case-072, case-073 | |
| CAE24 | case-118 | |
| CAE25 | case-137, case-132, case-130 | Another seed's baseline: every entry missing, re-anchor next. Removed steps missing. A worse binary regressed. |
| CAE26 | case-039, case-030 to case-038 | The secret goes to a declared bookkeeping path in case-039. |
| CAE27 | case-086, case-097 | |
| CAE28 | case-021 | |
| CAE29 | case-088, case-014 | |
| CAE30 | case-001 | The byte-bound and exit-code probes on `list` have different ids, and the judge accepts the set. |
| CAE31 | case-082 | |
| CAE32 | case-040 | |

## Corrections found while building the corpus

Each of these changed the text or the tools, and each is in this pull request:

- QT6: a twin removes one reason a check was not measured, and a chain of twins ends where the check is held and crediting. Cases with two reasons need it (`case-079`, `case-090`, `case-091`, `case-092`). The corpus test follows the chain.
- QE8: a check that would use no invocation still has one target with the check's id, because the judge contract requires one.
- QE1: "three strings" listed four.
- QT5: the source digest of a form that is one file, not a build.
- The cross-check reads a placeholder seed as the placeholder object, as QS5 says.
- A not-measured check still names the invocation it would have used (QE8), so a placeholder case lists every invocation of its kinds (QT6).

## The frozen text: standards review

A Claude standards reviewer read the whole frozen text (digest `c4180a35…e802`, commit de882dd) with its schemas and `CASES.md`. Verdict: publish with these changes. Every finding was checked against the text and the corpus before it was acted on; all are fixed (commits 711457b, ab8735b, 774067a).

| Finding | Disposition |
|---|---|
| B1: discovery's targets when no help request ran | Fixed. QE8: discovery's targets are the help requests its walk made; with nothing run it has one. Cases 003, 004, 005, 060 and 090 now have one target. |
| B2: the recordings' profile and launcher digests cannot be reproduced from the text, and QT1 makes a mismatch exit 2 | Fixed. QT5: those digests, the system and the commit are provenance, and a collector rejects no recording on them. |
| M1: no scripted-run exemption from the system-version check | Fixed in QT3. |
| M2: QE7's catch-all nulls `dimension` | Fixed: QE7 names `dimension`; QE8 names `check_id` and `targets`. |
| M3: QE1 sends binding changes outside the record to errata | Fixed: any binding change is a new version. |
| M4: the judge version is not pinned | Fixed: the rules pin judge 1.0. |
| M5: no conformance clause | Fixed in QE19; QT6's `input.json` is "up to QE19's comparison". |
| M6: twin identity undefined | Fixed in QT6: one input, and a counterpart's selector. |
| M7: tiers of targets that never ran, and of the planted file | Fixed in QE8 and QE9. |
| M8: the freeze digest left out the schemas | Fixed: the second freeze covers the four text files, `CASES.md` and the schemas. |
| m1 to m5, n1, n2 | Fixed: QT1's flags, `bin/subject` in a scripted assembly, a case with no secret (`case-144`), artifact targets only for a path, QE4's tie, and the status wording. m5 is moot after B2. |

Fixing B1 also showed that the reason pair QD4 before QH8 cannot happen in a scripted run, where the private copy is changed for every use, discovery included. QT6's `order:` rows are now the pairs a scripted run can show, and `case-092` shows QH8 before QA6 instead.

## The corpus: review against the recordings

A Claude test reviewer read every form against its recording and sampled about 70 cases. Every recording matched its source and every sampled outcome followed from its recording. Its findings:

| Finding | Disposition |
|---|---|
| Nothing in the repository ties case outcomes to recordings | Fixed: `scripts/check-case-outcomes.py` runs in `check.sh` (4c3182f). Its first run also found that the builder counted only standard output for a byte bound, which QP3 counts on both streams; fixed in the cases. |
| Two violating forms act only beside setup the recorder prepares | Fixed in the text: QT5 names the decoys. |
| Rules that only a recording shows counted as covered | Fixed: `CASES.md` lists QE18, QH10, QH12, QH15, QH16, QH20 and QH22 as rules a scripted record cannot break, and cases no longer cite them. §4 notes that four encodings coincide for the canary's alphabet. |
| Collector paths with no form or case | Fixed after the owner chose to add them (2026-11-24): `help-hidden` and `help-stderr`, an environment value and two offline settings on one command, a secret read by two commands, a predicate and a capture on a cut stream, invalid bytes and a 33-digit number before a predicate, and a subject file without the bare binary (cases 145 to 157). Every form was recorded again. |
| A recorded `null` printed value was ambiguous | Fixed: `printed_value` is `{"value": v}` or `null`. |
| QD4's two sentences unranked | Fixed: the bound or QD5 sentence governs. |

## Blind derivation, round 1

A reader on a different model (agy, `gemini-4-argon-high`) received only the export (`scripts/collection-export.py`) of the frozen text and 210 cases, public and private, and wrote its own derivation program. It matched 161 cases exactly. After the corrections below, 199 of 210 matched; the remaining 11 come from text changed after its export, and round 2 re-derives them.

| Disagreement | Disposition |
|---|---|
| The corpus listed runbook steps of a destructive runbook outside the destructive scope among a case's invocations (35 cases) | The reader was right. QT6 now defines the list from the record; cases fixed. |
| `target_size` for a check not held for its exit code or end | Settled in QE8: a size only when the predicate was evaluated; the reader's reading. |
| The rule a pre-check rejection names (QE5 or QE6) | Settled in QT6: QE6. |
| Invocations of a placeholder case | Settled in QT6: every invocation a target names, a not-observed check naming the one it would have used. |
| QS5's override beside a missing declared command, and a secret command with a placeholder class | Settled in QS5; the corpus already took these readings. |

Of the reader's other two-way readings, it took the corpus's reading each time: the specific rule over the schema for a rejected file, `wrong_exit` for an exit code a check does not accept, and QS6 for an undeclared secret command.

## Blind derivation, rounds 2 and 3

Each round gave a fresh reader on the same model only the export of the frozen text and every case, 224 in all.

| Round | Freeze | Agreement | Disposition |
|---|---|---|---|
| 2 | `d04af121…2c5f` | 223 of 224 | The reader stopped discovery's walk at a help request that exited 1 with a command list (`case-044`). QD2 now says a help request that ran lists its commands whatever its exit code, as QO7's "whatever an earlier one's end" implies; the corpus already took that reading. QA7 now says QS5's last override stops no runbook. |
| 3 | `da13c11c…98ff` | 224 of 224 | No disagreement. Its remaining two-way readings all went the corpus's way; the depth bound and QS5's override are now stated outright. |

## The frozen text: different-model standards review

agy (`gemini-4-argon-high`) read freeze 3 with the schemas, `CASES.md` and this log. Verdict: publish with these changes; self-contained for the corpus and the core contract. Every finding was checked against the text and fixed in 2d77d40; no case changed.

| Finding | Disposition |
|---|---|
| Masking (QE18): no replacement text, and a short declared value such as `1` would be masked everywhere | Fixed: the canary becomes the masked string QT5 uses; declared values of 8 bytes or more become `*`; masking changes no count, digest or outcome; shorter values are not masked and the scorecard says so. |
| `target_size` of a row count with no array, and when `precondition_observed` is reported | Fixed in QE8; QS5's override applies whatever the observed precondition. |
| `json_path_equals` and `row_count` not read under the judge contract's §3 | Fixed in QA9. |
| `determinism` credit read per runbook; `idempotency`'s "same command" | Fixed in `credit-and-routes.md` §2: per check, and the same declared command. |
| QH26 against QO7, and QP9 against QP6 | Fixed: a failed read calibration proves no read but lets a leak or an earlier end decide; QP9's outcomes come first. |
| Minors: QS6 and QP10's secret commands, QS7's engine-set names, QS5's `writes` placeholder and QP7 skip, QP8's regular side files, links inside left-out paths, the walk stopping at a bound, QS11's depth counting, an empty file's digest in QT5 | Fixed. |
