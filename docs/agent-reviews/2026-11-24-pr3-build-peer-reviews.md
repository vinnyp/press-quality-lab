# PR3 build: review log

The plan is `docs/plans/2026-11-21-2309-feat-conformance-collection-spec-plan.md`, PR3 group. This log records the review of the test entry's specification text and the corpus's map to the plan's acceptance examples. The blind derivation of the corpus by a different model is its own step, and its results are added here when it runs.

## The test entry's text

The reviewer was a different model from the author: agy, `gemini-4-argon-high`, briefed with paths to the kit folder and its schemas, in `design` mode with the standards lens.

| Round | Verdict | Findings | Disposition |
|---|---|---|---|
| 1 | publish with these changes; not self-contained | 4 Blocker, 5 Major, 2 Minor, 1 Nit | All fixed in 7d2db33 (text) and b0cb160 (schemas and tools). Round 2 verified each closed. |
| 2 | publish with these changes; self-contained except three edge interactions | 3 Major, 2 Minor | All fixed in 73a9689. Round 3 verified each closed. The three new worked ids in QE2 were recomputed and match. |
| 3 | publish/freeze as-is; self-contained | 1 Minor: whether QT5's source digest has directory entries | Fixed: QT5 says it has an entry for each file and none for a directory. |

Round 1's Blockers were: a `kill_attribution` value with no decision rule; no stated way for a scripted run to receive its recording; `input.fixtures` with no assembly rule; and a baseline schema that rejected placeholder objects in preimages. Each finding was checked against the text before it was acted on.

## The corpus and the acceptance examples

The corpus has 138 cases: 107 judge-checked and 31 derivation-only. Every judge-checked case's expected output was written from the rules and agrees with the judge (`./check.sh`, step `cases`). Every held or not-held outcome a case states was also checked against its form's recording by a separate checker that reads only the case and the recording. It found two cases red on a second check, `case-039` and `case-040`, which were fixed. The four outcomes it cannot evaluate are the ends by a signal, which QO4 and QO5 decide: `case-072`, `case-076`, `case-078` and `case-089`.

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
