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
