# Phase 3 plan: review log

The plan is `docs/plans/2026-11-24-2115-feat-grade-command-plan.md`. It was reviewed at commit `e3520ff` by a document-review team (coherence, feasibility, scope, adversarial and security lenses, runtime claude) and by a different model as `AGENTS.md` requires: the staff-engineer lens on agy, model `gemini-4-argon-high`, briefed with paths to the plan, the engine plan, the sealed collection specification and the code. Each finding was checked against the plan, the specification and the code before it was acted on.

The different-model review's verdict was "ready with changes". Parts of it were not grounded: it cited rule families QC, QB and QK, which the specification does not have; quoted text the plan does not contain (a file count in U1, a verifier interface, an `internal/confinement` package); and misread QE18 as baseline persistence and QH18 as a per-invocation temporary directory. Those parts were rejected. Its grounded findings agreed with the document-review team's.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The corpus runner needs the stored record from the first slice, but the store came in slice f | feasibility (P1) | Confirmed: the stored record is the only place `grade` exposes the judge input and output | Fixed: the store's run records (U17) move into the end-to-end slice. |
| 2 | Per-PR case predictions contradict the two-sided ratchet (`case-061` to `case-066`, the runbook-free and discovery cases) | different model (Blocker) | Confirmed against the cases' runbooks and QS5 | Fixed: the plan names only cases it is sure of; each pull request lists the set its ratchet moves. |
| 3 | Darwin-only verifier code is never compiled or linted in CI | adversarial, feasibility | Confirmed: CI runs on Linux and the gate passed no `GOOS` | Fixed: the gate adds `GOOS=darwin` build and lint passes (KTD12, U2). |
| 4 | Home isolation alone does not isolate the store on Linux | feasibility, different model | Confirmed: Go reads `XDG_CONFIG_HOME` first | Fixed: each run also gets its own `XDG_CONFIG_HOME` (KTD5). |
| 5 | The real observation path is proved only on the known-good form | adversarial (P1) | Confirmed: the three real-verifier cases all use it | Fixed: U16 runs every form's recorded invocations through the real verifier and compares them with the recording. |
| 6 | U16's rule list omits QH6, QH13 and QH23 | security (P1) | Confirmed against `confinement.md` | Fixed: added, and GR15 governs the list. |
| 7 | The shared reader needs each caller's bounds, value access and the canonical form | different model (Major) | Confirmed: QS11, QA6 and QA9 set their own bounds and compare canonical values | Fixed: KTD3 and U3. |
| 8 | KTD3 relies on private judge scoring no unit runs | adversarial | Confirmed | Fixed: U3 re-runs phase 1's private judge scoring. |
| 9 | Private scoring waits behind the owner's decision on E-3 | adversarial | Confirmed: the close waits on slice e | Fixed: a private scoring run after slice f as well. |
| 10 | The necessity test can pass on a malformed one-grant profile | adversarial | Confirmed | Fixed: the granted program must succeed and no other result change. |
| 11 | Masking (QE18) has no building unit | security | Confirmed | Fixed: U7 masks every record and tests each encoding. |
| 12 | The three real-verifier cases cannot satisfy the CI ratchet | coherence | Confirmed | Fixed: they are outside the CI ratchet by name and run in the runner's macOS mode (KTD2). |
| 13 | A rejected record was to be kept before the store existed | coherence, scope | Confirmed | Fixed by row 1. |
| 14 | QO2 had no owning unit | different model (Minor) | Confirmed | Fixed: U12. |
| 15 | A record should be stored only once one exists | different model (Minor) | Confirmed for derivation-only cases; its baseline claim was not grounded | Fixed: KTD5 and U17. |
| 16 | `case-129` passes in U8 and was listed again in U9 | coherence | Confirmed | Fixed. |
| 17 | GR5's "survives CI" reads as a CI mechanism | coherence | Rejected in part: the engine plan measures whether the build's own CI tests notice each seeded mistake, which the private harness runs | Reworded to say so. |
| 18 | KTD12 names a `check.sh` step no unit defines | coherence | Confirmed | Fixed: `check.sh` needs no new step. |
| 19 | U1 embeds a fragile file count | different model (Nit) | Rejected: U1 contains no file count | No change. |

## The peer-review gate, design mode

Reviewed at commit `2344678` by five lenses, runtime claude, each in its own clone with unchanged repository-state snapshots: staff engineer (no Blocker, seven Major), plan (one Blocker, ten Major), architecture (no Blocker, seven Major), security (two High, five Medium) and measurement (no Blocker, eight Major). Each finding was checked against the specification and the code. The owner settled four decisions the gate raised: the launcher and self-test programs are hidden Go modes of `pql` (GKD6), darwin code may use `golang.org/x/sys` and no cgo (GKD7), the command line (GKD8), and macOS evidence on 27.2 only (GKD9).

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 20 | U15 refused an unsupported system; QH1 requires a judged record with every subject-running check `network_unverifiable` | plan (Blocker) | Confirmed from QH1 | Fixed: U14 builds and tests the QH1 path, Linux included. |
| 21 | The launcher and self-test programs have no design, and the real verifier first runs at PR6 | staff, architecture, plan, security | Confirmed: the recorded launcher digest is the spike's C launcher, and engine plan §8 keeps one binary | Fixed: GKD6, KTD14, and the time-boxed launcher spike U21 in PR2. |
| 22 | The seam between the scripted and real paths is unowned, so decision code could sit behind the tag | architecture, measurement | Confirmed from QT4 "decide from them exactly as the released build decides" | Fixed: KTD13, U23 and its guard test. |
| 23 | Profile text and parameter injection have no failing test | security (High) | Confirmed: confinement.md lists refusals no program witnesses | Fixed: U28 compares the template with `confinement.md`'s code fences and tests adversarial names; the recorded profile digests are not used, since QT5 says they record provenance and a collector reads none. |
| 24 | Environment and descriptor inheritance have no detecting test | security (High) | Confirmed: no form shows a leaked variable | Fixed: U28's builder tests in CI and a macOS probe. |
| 25 | The runner can pass vacuously | measurement, plan, staff, architecture | Confirmed | Fixed: KTD2's guards and U8's meta-tests. |
| 26 | Derivation-only cases pass on any exit 2 | measurement, plan, architecture, staff | Confirmed: all 31 expect exit 2 | Fixed: GKD8's rejection line compared with `expect.rule`, and no stored record. |
| 27 | No planted mutation for the QE19 fields the judge cannot see | measurement | Confirmed from the judge contract | Fixed: one per field in U8. |
| 28 | RFC 8785 and the judge's writer were conflated, and canonical bytes would compare two integers that round to one double as equal | architecture, staff, different model | Confirmed against `internal/judge/json.go` and QE2 | Fixed: KTD3 shares the reader and exact equality; KTD4 puts RFC 8785 in `internal/jcs`. |
| 29 | `subject` and `record` would import each other | architecture | Confirmed | Fixed: KTD11's import direction. |
| 30 | U5 owns more required-set rules than it named | staff, plan | Confirmed from `case-057`, `case-093` and others | Fixed: U5 owns all of QE3 and QE7, and the runner checks every `required` array from PR2. |
| 31 | The darwin passes cover only cgo-free code | staff | Confirmed | Fixed: GKD7 and KTD12's cgo check. |
| 32 | Hand-written schema shapes have no independent oracle | staff, architecture, plan | Confirmed: no Go JSON Schema validator | Fixed: KTD15. |
| 33 | The command line was undefined | staff, plan | Confirmed | Fixed: GKD8. |
| 34 | U7, U13, U15 and U16 were too large to dispatch | plan | Agreed | Fixed: split into U7, U24, U25; U13, U26, U27; U15, U28, U29, U30, U31; U16, U32. |
| 35 | The form replay lacked decoys, the full QT5 fields and a driver | measurement, staff, plan | Confirmed from QT5 "needs the same decoys" | Fixed: U32. |
| 36 | Mutated-profile evidence can pass vacuously | security | Confirmed from the spike record | Fixed: U16 step 3. |
| 37 | Evidence on one macOS version only | measurement | Owner decision | GKD9: 27.2 only, 27.0.1 stated unverified at the close. |
| 38 | The confinement evidence is not repeated before phase 4 | security, measurement | Confirmed | Fixed: KTD18 and U20. |
| 39 | GR5's seeded-mistake bar has no procedure | measurement | Confirmed | Recorded as an Open Question with a proposed procedure. |
| 40 | Private scoring after slice f cannot meet zero wrong records on baseline cases | plan, staff | Confirmed | Fixed: scoped to cases that need no baseline. |
| 41 | QE20's per-run directories and sweep are unsafe as written | security | Agreed | Fixed: KTD5. |
| 42 | Masking was tested only in the stored record | security | Agreed | Fixed: U24 scans every byte written. |
| 43 | Goroutine panics exit 2 | plan, architecture | Confirmed from `CODING_STANDARDS.md` | Fixed: KTD17. |
| 44 | U4's "every subject file parses" contradicts `case-100` | plan | Confirmed | Fixed. |
| 45 | `case-136` and the schema-detectable derivation cases move earlier than U9 | staff, plan | Confirmed | Fixed: U9 moves "the derivation-only cases still pending". |
| 46 | AE8 and AE30 cannot prove slice b | staff, architecture | Confirmed | Fixed: GR10 and GR11. |
| 47 | QS7's appending and QO5's exit-code rules are used before their owner | plan | Confirmed | Fixed: U12. |
| 48 | The ratchet's "none back" is not enforced | measurement | Confirmed | Fixed: KTD2's growth check. |
| 49 | Readings the text leaves open: store write after the judge, QE15 against the record schema, QS8's home, QA6's depth, an unbounded home | staff, architecture, security | Confirmed as two-way readings or gaps | Sent to the owner in Open Questions (GR6). |
| 50 | Gate run time | staff, architecture, plan | Not measured | KTD12: measured in PR2. |

## The peer-review gate, second pass

Reviewed at commit `4940a80` by the same five lenses, runtime claude, each in its own clone with unchanged repository-state snapshots: staff engineer (no Blocker, eight Major), plan (no Blocker, eight Major), architecture (no Blocker, six Major), security (one High, five Medium) and measurement (no Blocker, six Major). The first pass's fixes were confirmed present; these rows record what was still wrong or newly wrong. Two claims were reproduced before acting: the judge's writer panics on an integer beyond 64 bits (`internal/judge/json.go`), and a Linux cross-build drops a darwin cgo file instead of listing it.

Rows 11 and 18 above are superseded: masking is now U24, and `check.sh` gains the validation and growth-check steps (KTD2, KTD15).

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 51 | The could-not-start marker cannot be on the first line | staff, plan, architecture, security | Confirmed from QH5, QH7 and `behaviour/not-executable.json` | Fixed: KTD14.4; it carries the per-invocation value and is honoured only directly after the marker. |
| 52 | Go runs every package's initialisation before `main` | staff, plan, architecture, security | Confirmed | Fixed: KTD14.2's initialisation list and dependency guard, and a launcher re-check on import changes. |
| 53 | Unconfined cleanup can follow a link a subject or a surviving child made | security (High) | Confirmed from the profile's `file-write*` on the home | Fixed: GR16, KTD20 and U17's tests. |
| 54 | The seam leaves marker, attribution and control logic behind the darwin tag | architecture, staff, measurement | Confirmed | Fixed: KTD13's untagged driver and `Invoker`, raw end facts, a type-based guard and a projection test. |
| 55 | The judge's writer panics on integers beyond 64 bits | staff | Reproduced from the code | Fixed: KTD3's total writer and U3's test. |
| 56 | The cgo check passes vacuously on Linux | plan, staff | Reproduced | Fixed: KTD12 sets cgo on and searches darwin files. |
| 57 | The import guard never sees tagged or darwin code | staff, security | Confirmed | Fixed: the guard enumerates systems and tag sets itself. |
| 58 | The pending-list growth check has no base in the `cases` job | staff, plan, architecture, measurement | Confirmed from `ci.yml` | Fixed: KTD2.5, U8's `ci.yml` change, a missing base a failure. |
| 59 | The judge's tables are switches no unit exports | architecture, plan, staff | Confirmed from `requests.go` and `results.go` | Fixed: U3 exports `ScopeIncludes` and `AlwaysApplicable`. |
| 60 | The darwin work all lands in slice d | architecture | Agreed | Fixed: the darwin track, PR3 and PR7 to PR9. |
| 61 | The launcher spike shares a pull request with the pipeline | staff, architecture, plan | Agreed | Fixed: PR3 on its own. |
| 62 | Go's `net` package sets socket options the profile kills | staff | Not verified; needs a run | Fixed by design: KTD14.5's raw calls, and U21 checks three programs' necessity. |
| 63 | The real path is undefined before slice d | security, architecture | Confirmed | Fixed: KTD21 from PR2. |
| 64 | The form replay cannot fail and lacks context | measurement | Confirmed: half the invocations need runbook context | Fixed: U32's context from the cases, five runs, and no escape to the owner without a diagnosis. |
| 65 | U16's not-evaluated rule has no failing aggregate and conflicts with its own steps | measurement | Confirmed | Fixed: U16 steps 2 to 6. |
| 66 | No test sees whether the subject ran | measurement | Confirmed | Fixed: KTD2.4's invocation count and U14's sentinel subject. |
| 67 | The private scoring can shrink its own denominator | measurement | Confirmed | Fixed: Open Questions, a mistake not evaluated blocks GR5. |
| 68 | A resolver step not yet built has no stated output | plan | Confirmed | Fixed: KTD11, it gives `harness_fault`. |
| 69 | U5 needs case assembly and is too large | plan | Confirmed | Fixed: U34, and U5 split into U5 and U33. |
| 70 | Coding agents cannot run macOS tests inside their sandbox | plan | Confirmed from QH6's exit 71 | Fixed: KTD19. |
| 71 | The masking test can pass vacuously and contradicts the planted file | security, staff | Confirmed: the recordings are already masked | Fixed: U24's injected values, positive control and scope. |
| 72 | The environment and descriptor probe can pass vacuously | security | Confirmed | Fixed: U28's inherited descriptor, real sentinels and knock-out. |
| 73 | Subject-controlled text reaches the terminal unescaped | security | Agreed | Fixed: KTD16's escaping and U18's fixtures. |
| 74 | A forged could-not-start through the launcher copy | security | Confirmed from GKD6 | Fixed: U15's test. |
| 75 | The rejection line's rule for `case-136` and for schema-detectable files | measurement, plan | Confirmed from QT6 | Fixed: QE6 with the judge's rule in the cause; a named rule wins over QS2; a two-rule tie goes to the owner. |
| 76 | Workspace and store mixed; sweep race; root before the sweep | architecture, security | Agreed | Fixed: KTD5. |
| 77 | Main-goroutine panics and interruptions | staff, security | Confirmed | Fixed: KTD17. |
| 78 | Slice e's scope contradicted itself | staff | Confirmed | Fixed: a 1.1 is the owner's work; if adopted, slice e waits for it and the date moves. |
| 79 | Real-path draining, budget timing, QA4 substitution, QP9's control and QP10's canary had no owner | plan, staff | Confirmed | Fixed: U30, U26 and U12. |
| 80 | The acceptance examples' naming halves are scorecard behaviour | staff | Confirmed | Fixed: GR13 and U18. |
| 81 | Case predictions off by two units | plan | Confirmed from the corpus | Fixed: those cases pass from U27. |
| 82 | Record field owners | plan | Confirmed | Fixed: U7 and U17. |
| 83 | Smaller items: QA5 on `anchor`, test seams, the hidden-mode selector, `x/sys` in `go.mod`, absolute `sandbox-exec`, an over-long argument list, decoy directory safety, host name in logs, 25 of 25, ratchet divergence, the manifest count, the assembled directory's contents, Python-free vectors, stated limits | all lenses | Confirmed or agreed | Fixed in the units and KTDs named in each. |

## The peer-review gate, third pass

Reviewed at commit `2183ef4` by the same five lenses, runtime claude, each in its own clone with unchanged repository-state snapshots: staff engineer (no Blocker, six Major), plan (no Blocker, seven Major), architecture (no Blocker, three Major), security (no Critical or High, four Medium) and measurement (no Blocker, two Major). Every lens confirmed the second pass's fixes present. These claims were reproduced on go1.27.4 before acting:

- `os.RemoveAll` fails on a mode-0000 directory, plain and through `os.Root`.
- A walk with `Root.Lstat` and `Root.Chmod`, followed by `Root.RemoveAll` on each child entry, empties such a directory and leaves an outside directory reached by a link unchanged. `Root.RemoveAll(".")` is refused, so the directory itself is removed from its parent.
- darwin's standard `syscall` package has no `Openat` or `Unlinkat`.
- The judge's writer writes an input `-0` integer as `0`, and the baseline echo in `internal/judge/baseline.go` passes untouched entries through it.

The owner chose to split the end-to-end slice and keep it thin (row 95).

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 84 | The darwin track builds spawn, waits and draining last, after the units that need them | architecture | Confirmed from the units' tests | Fixed: U30 moves into PR8 before U28; U29 is the last darwin unit, depending on U30, U31 and U41. |
| 85 | `strictjson` cannot take the judge's shape-guided reader as it is | architecture | Confirmed from `json.go` and `review_memory_test.go` | Fixed: KTD3 states the leaf's exports and leaves `shape` and the retention table in the judge; any change under `strictjson` re-runs private judge scoring. |
| 86 | KTD20's mechanism cannot remove a mode-0000 directory, and darwin lacks the `*at` calls | architecture, security, staff | Reproduced | Fixed: KTD20 is built on `os.Root`; U40's tests include a swap between `Lstat` and the mode change, and a mutation to `os.Chmod(path)`. |
| 87 | The total writer would write `-0` | staff, architecture | Reproduced | Fixed: KTD3 keeps 64-bit integers on today's path; U3's `k: -0` test. |
| 88 | U2 and U21 both claim `hidden.go`; the spike lands darwin code before the darwin gate | architecture, plan, staff | Confirmed | Fixed: U2 creates the dispatch; PR4 merges the spike's record and evidence only; `x/sys` enters `go.mod` with U15; the import guard's `x/sys` rule is tested on synthetic output first. |
| 89 | U31 and U30 depend on slice b and c code that PR9 does not wait for | plan, staff | Confirmed | Fixed: U31 tests with a fixture scratch directory and joins QA4 in U35; U41 records the read proof's raw facts and U37 decides. |
| 90 | Tests that start `sandbox-exec` run in a default `go test` on a Mac | plan, staff | Confirmed: only three units were tagged | Fixed: KTD19's `macos_evidence` tag on every such test; an evidence run fails on a skip. |
| 91 | An unbuilt resolver step, read literally, gives `harness_fault` for every check | plan | Confirmed against U12's and U27's predictions | Fixed: KTD11, an unbuilt step acts only when its condition is in the raw facts; the QH8 step and QO3 move to U12. |
| 92 | U12 is too large for one brief | plan | Agreed | Fixed: U12, U36, U37 and U38. |
| 93 | "No invocation requested" cannot be observed after cleanup | plan, measurement | Confirmed | Fixed: KTD2.4-5's scripted-only trace and U8's planted cases. |
| 94 | The growth check has no rule for the first list or a local run | plan, staff, measurement | Confirmed | Fixed: KTD2.6. |
| 95 | PR2 is no longer the thin slice GKD1 approved | staff | Confirmed: fourteen units | Owner decided: split and stay thin. PR2 is the strict JSON leaf; PR3 is the end-to-end slice for `case-003`, `case-004` and `case-129`; U39 completes the required set in slice a; masking and unconfined safety move to slice b. |
| 96 | A seeded mistake caught only by a lint counts as caught | measurement | Confirmed from KTD12 | Fixed: the mutant must build and pass every lint, and a catch is a named test failure. |
| 97 | U3's re-run of the judge catalogue has no not-evaluated rule | measurement | Agreed | Fixed: U3 reports evaluated out of total, and a mistake not placed blocks unless waived. |
| 98 | Private-case denominators are unstated | measurement | Agreed | Fixed: U18's subset criterion and U20's scored out of 72. |
| 99 | KTD15's validation count comes from the runner it checks | measurement | Agreed | Fixed: a fresh directory, `-count=1`, and a count `check.sh` takes from the corpus and the list. |
| 100 | The comparator is tested only for false passes; input lists have no order | measurement | Confirmed from the judge contract | Fixed: KTD2.3 compares input lists as multisets; U8's allowed-variation tests. |
| 101 | The form replay drops unnamed invocations and lacks a context rule | measurement | Confirmed: 209 of 1,415 recorded invocations are named by no case | Fixed: U32 replays each context, drives the KTD13 driver, compares through the projection serialiser and reports coverage for the owner. |
| 102 | The real-verifier cases run once | measurement | Agreed | Fixed: 25 of 25 under KTD18. |
| 103 | A subject can make files undeletable with flags or access-control entries | security | Not verified on 27.2; needs a run | Fixed: KTD20's darwin half and U40's macOS tests; a failed cleanup goes to the owner (Open Questions). |
| 104 | The leak scan and inspection can skip an unreadable file | security | Agreed | Fixed: KTD20's file mode change, U36's and U37's fixtures; the outcome goes to the owner. |
| 105 | base64 forms miss a canary at an unaligned offset | security | Confirmed from §4 | To the owner (Open Questions); the engine does not work around it. |
| 106 | A subject keeps running after `pql` dies, and the sweep races it | security | Agreed | Fixed: KTD5's recorded groups, KTD17's SIGHUP and SIGQUIT, U40's tests; the exit code goes to the owner. |
| 107 | Escaping misses error paths and masking runs after escaping and head cuts | security | Agreed | Fixed: KTD16 and U24's edge cases. |
| 108 | The macOS re-check trigger misses transitive imports | staff, architecture, security | Confirmed | Fixed: KTD14.2's checked-in darwin dependency list, owned by U2. |
| 109 | The scripted invoker must read attribution strings the guard forbids | architecture | Confirmed | Fixed: KTD13's guard excludes the scripted reader; the projection test proves the mapping. |
| 110 | The driver could assign `earlier_step_failed` ahead of earlier §2 steps | architecture | Agreed | Fixed: KTD11, the driver records facts and the resolver assigns reasons. |
| 111 | Stored-record fields without an owner | plan, staff | Confirmed from `record.schema.json` | Fixed: U17. |
| 112 | Real-path evidence lands last | staff | Agreed | Fixed: U16 and U32 move into the darwin track before slice d. |
| 113 | Smaller items: U4's schema-detectable rules, U10's build, QO2's two owners, U23's verification, U25's dependency on U2, U18's agreement wording, the descriptor loop, the random copy folder, exact version matching, U32's decoy and environment controls, U31's survivor assertion, the two-rule question at run time, the main session's sandbox, and test commands with `-tags scripted` | all lenses | Confirmed or agreed | Fixed in the units, KTDs and Open Questions named in each. |

## The bake-off: four approaches to the whole build

After the third pass, the owner asked for competing approaches to phase 3's whole build, within GR1 to GR16 and GKD1 to GKD9. Three bakers each wrote an approach from a common brief, without seeing the plan or this log:
- Baker A, a fresh Claude agent;
- Baker B, Gemini 4 Argon High through agy;
- Baker C, GPT-6.1 Sol through codex.

The brief held the requirements, the owner's decisions and the facts the three passes had verified. Each baker read a copy of the repository without the plan or this log. A fresh Claude judge ranked the three approaches and the plan at commit `e80c7bd` under neutral labels, without the coordinator's ranking. A judge from another model family could not run the comparison skill, and the review records that fallback.

The judge ranked Baker A first, the plan second, Baker C third and Baker B last.
- **Why B ranks last.** Baker B builds on one reading of a two-way question before the owner rules, which GR6 forbids. It also treats `gate.sh` as already compiling darwin code, which it does not.
- **Why A beats the plan.** The plan left slices f and e, full private scoring and the close for the last week. Baker A front-loads the macOS work, lands slice d by 2026-12-23, and scores twice.
- **Two judge findings were dismissed.** "U1 is stale" was an artefact of the bake-off's copy. "Narrows GKD8" compared the plan with the brief's paraphrase.

The owner chose to rewrite the plan around Baker A's approach (GKD10).

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 114 | The order leaves no slack for owner rulings or macOS diagnosis | judge | Confirmed from the dates | Fixed: KTD1's risk-first order, eighteen pull requests, slice d by 2026-12-23, two private scoring rounds, 2027-01-16 spare. |
| 115 | A served-invocation check would show every case requested only what it implies | Baker A | Confirmed: QT6 defines `invocations` as every invocation the record names or implies, and none for a derivation-only case | Added: KTD2.6, on the binary through the scripted trace. |
| 116 | An unbuilt rule should be a loud engine fault, not a quiet record | Baker A | Agreed; `case-003` and `case-004` resolve at level 1, so the thin slice still passes | Adopted: KTD11's exit-70 level, replacing row 91's conditional `harness_fault`, with a close-time test that none remains. |
| 117 | Every binding rule should map to its proof | Bakers A and C | Agreed | Added: KTD22 and U45. |
| 118 | All unconfined file work in one package with a guard | Baker A | Agreed | Added: KTD20's `internal/throwaway` and its AST guard (U46). |
| 119 | Measure what a subject can leave in its home before designing cleanup | Baker A | Agreed: earlier rows rested on "probably" | Added: U43, the hostile-home spike. |
| 120 | The comparator should not use the engine's reader | Baker A | Agreed | Added: KTD2.3. |
| 121 | The judge output's `exit_code` field should be cross-checked against the returned code | Baker A | Agreed | Added: KTD16 and U25. |
| 122 | An unknown `--scope` token is a two-way reading | Baker A | Confirmed: GKD8's exit 64 against QE6's X1 user rejection | To the owner (Open Questions). |
| 123 | The standard error line for a judge-rejected record is not fixed | judge | Confirmed from GKD8's wording | To the owner (Open Questions). |
| 124 | Steps 5 and 7 of §2 apply to every invocation, so they cannot wait for slice d's scripted half | coordinator | Confirmed from `collection-rules.md` §2 | Fixed: U44 in slice b1. |
| 125 | A collection 1.1 changes the record schema's fixed `spec_version` | Baker A | Confirmed: `record.schema.json` fixes it at `"1.0"` | Noted in Scope Boundaries and Open Questions. |
