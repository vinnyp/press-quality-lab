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
