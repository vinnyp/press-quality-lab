# Peer-review gate — conformance-collection-spec-plan (2026-11-22)

**Mode:** design. **Reviewers:** peer-staff-software-engineer-reviewer, peer-security-reviewer, peer-privacy-reviewer, peer-architecture-reviewer, peer-standards-reviewer, peer-interface-reviewer, peer-experiment-reviewer. **persona-version:** cache/1.4.0.

**Design round:** Round 1 (design).

**Subject:** `docs/plans/2026-11-21-2309-feat-conformance-collection-spec-plan.md`, reviewed as an uncommitted snapshot after its document review.

**tier-rationale:** Staff engineer is the design baseline. Tier 2: security (runs untrusted code), privacy (stored output can hold personal data) and measurement methodology (the plan defines verdicts). Tier 3: architecture (a new isolation boundary), standards (a second implementer builds from the specification) and interface (file format and exit codes). Plan review waits for implementation planning. Performance, database, reliability, devops and release did not apply to a requirements document.

**Spec gate:** The load-bearing control, the macOS sandbox profile, was checked live before dispatch. That check was flawed: it judged clean runs by exit status, and a failed user lookup exited 0. It also had no process rules. It was redone after the review with output assertions and the hardened profile: 350 of 350 violations killed, 125 of 125 clean runs correct.

**Per-lens runtime:**

- peer-staff-software-engineer-reviewer: runtime=claude (sonnet)
- peer-security-reviewer: runtime=claude (sonnet); also runtime=agy (gemini-4-argon-high requested, rc 0)
- peer-privacy-reviewer: runtime=claude (sonnet)
- peer-architecture-reviewer: runtime=claude (sonnet)
- peer-standards-reviewer: runtime=claude (sonnet); also runtime=agy (gemini-4-argon-high requested, rc 0)
- peer-interface-reviewer: runtime=claude (sonnet)
- peer-experiment-reviewer: runtime=claude (sonnet)

Findings are condensed, with local paths removed, because this repository is public.

## Round 1 (design)

## Findings

### peer-staff-software-engineer-reviewer

- Major: Refusals without a kill become engine-caused red (row 1)
- Major: A sandbox wrapper failure looks like the subject's exit (row 2)
- Major: The read proof cannot attribute its kill (row 3)
- Major: Subject or computation bytes can force exit 70 (row 4)
- Major: Pre-validation duplicates the judge's rules (row 5)
- Major: No route table or false-pass table (row 6)
- Major: Store comparison on SQLite side files (row 7)
- Major: Specification and corpus share one author (row 8)
- Major: No critical path to the seal (row 9)
- Claims 2, 4, 5 and 6 not shown. Minors: child-start reason, one-off runner job, test-entry exit code, ambient conditions, declared paths, wording of the green exclusion, errata E-4.

### peer-security-reviewer

- High: Profile built by interpolation (row 10)
- High: Seed copy follows links (row 11)
- High: A subject can signal other processes (row 12)
- High (agy): Declared paths allow arbitrary reads (row 54)
- High (agy): System services bypass confinement (row 55)
- High (agy): Tripwire, copy and exclusion false greens (row 56)
- High (agy): A killed runbook step leads to a false red (row 57)
- No Critical. Mediums: daemon delegation, differential read proof, exclusion and leak scope, declared files, build-time test entry, env-only route. Lows: help runs an action, metadata, record location, runner pin.

### peer-privacy-reviewer

- High: Captured output stored without minimisation (row 13)
- High: No retention, deletion or cleanup (row 14)
- No Critical. Mediums and Lows: mask declared values, metadata, several read routes, synthetic secret markers, review before publishing a spike result.

### peer-architecture-reviewer

- Major: A denial that returns an error becomes red (row 15)
- Major: No per-run verifier health check (row 16)
- Major: Environment and service leaks around file rules (row 17)
- Major: Store probe has no positive control (row 18)
- Major: Id does not follow what the check measures (row 19)
- Major: Corpus covers only record to verdict (row 20)
- Major: A slow host makes a timeout red (row 21)
- Major: Schedule has no cut rule or approval date (row 22)
- Sound with fixes. Minors: seal the verifier's contract apart from its mechanism, end stray computation processes, say what a green speaks for, refusal labels.

### peer-standards-reviewer

- Blocker: Nothing establishes per run that the verifier is active (row 23)
- Blocker: A link in the seed defeats the secrets claim (row 24)
- Major: Corpus validates the judge, not collection (row 25)
- Major: No equality relation for collector output (row 26)
- Major: No subject-file schema (row 27)
- Major: Discovery compares counts only (row 28)
- Major: Read allow-list and metadata unstated (row 29)
- Major: Spike list omits two computation rules (row 30)
- Major: Engine mirrors the judge's rejection rules (row 31)
- Major: Exclusions hollow out a credited check (row 32)
- Major: Ids carry no specification version (row 33)
- Major: Race between a timeout kill and a sandbox kill (row 34)
- Major: Serial chain with no sizing (row 35)
- Blocker (agy): Declared paths read and leak private files (row 58)
- Blocker (agy): Unbounded output triggers a judge rejection (row 59)
- Major (agy): No green for env-only subjects, read proof vague (row 60)
- Major (agy): Targets, tiers and reason codes unspecified (row 61)
- Major (agy): Unclassified commands have no route to the judge (row 62)
- Major (agy): Offline arguments and exclusions give false green (row 63)
- Major (agy): Verdict-source rule conflicts with exit codes (row 64)
- Major (agy): Scripted baselines cannot be anchored (row 65)
- Needs narrow rework. Minors: holder of private cases, skipped cases, child attribution, definitions of observed and released build, the overloaded word record.

### peer-interface-reviewer

- Major: No per-grade positive control (row 36)
- Major: Profile is not parameterised (row 37)
- Major: Declared paths are not contained (row 38)
- Major: Exclusions give a false green (row 39)
- Major: Read proof has no attribution rule (row 40)
- Major: No closed table of engine exit codes (row 41)
- Major: Discovery has no defined algorithm (row 42)
- Major: Route to green assumed for real CLIs (row 43)
- Major: Corpus validated on the judge half only (row 44)
- Contract sound at the judge boundary. Minors: test-entry code, timeout sensitivity, CR43's claim, two CI jobs, canonical seed digest.

### peer-experiment-reviewer

- Blocker: Verifier attested once, never checked at run time (row 45)
- Major: Injection, seed links and a copy escape defeat the secrets claim (row 46)
- Major: Planted-secret probe does not say what it asserts (row 47)
- Major: Route to green unproven (row 48)
- Major: Declarations can reach green (row 49)
- Major: Discovery count hides missing commands (row 50)
- Major: Corpus cannot test collection rules (row 51)
- Major: Engine-caused red routes remain (row 52)
- Major: Date cannot be confirmed (row 53)
- Not yet sound as written. Minors: wording of CAE1, judge-based pre-checks, errata E-4.

## Verify-the-reviewer dispositions

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | Major: Refusals without a kill become engine-caused red | staff | Reproduced: a refusal without a kill left a failed lookup exiting 0 | Accepted: CR67 covers every operation, CR97 judges clean runs by output |
| 2 | Major: A sandbox wrapper failure looks like the subject's exit | staff | Reproduced: nested sandbox exit 71, broken profile exit 65 | Accepted: CR23, CR74, CR75, CAE19 |
| 3 | Major: The read proof cannot attribute its kill | staff | Confirmed from CR20 and CR31: a kill in the control alone proves no read | Accepted: CR31, CR107 |
| 4 | Major: Subject or computation bytes can force exit 70 | staff | Reproduced: 33-digit number, deep nesting and 17 MB each rejected by X12 | Accepted: CR38, CR79, CAE21 |
| 5 | Major: Pre-validation duplicates the judge's rules | staff | Reproduced: a Go panic exits 2; the judge's rejection rules would have been written twice | Accepted: CR69, CR71, CR88 |
| 6 | Major: No route table or false-pass table | staff | Confirmed: routes were assumed, and a subject with no secret file has none | Accepted: CR92, CR94, stated limit; env-only proof stays deferred by the owner |
| 7 | Major: Store comparison on SQLite side files | staff | Reproduced: a plain read created two side files and left the main file unchanged | Accepted: CR101; the exact rule is the specification's |
| 8 | Major: Specification and corpus share one author | staff | Confirmed: CR51 checks record to verdict only | Owner chose blind derivation: CR104; CR52 now records behaviour under the profile |
| 9 | Major: No critical path to the seal | staff | Confirmed: no checkpoint and no rule for slipping | Owner chose moving the date and cutting nothing: CR105, CR106 |
| 10 | High: Profile built by interpolation | security | Reproduced: a path spliced into profile text read a decoy; a parameter did not | Accepted: CR78, CAE20 |
| 11 | High: Seed copy follows links | security | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 12 | High: A subject can signal other processes | security | Reproduced: another process's environment read and a process killed; both closed by process rules | Accepted: CR80, CR81, CAE27 |
| 13 | High: Captured output stored without minimisation | privacy | Confirmed from CR48 and CR69: no minimisation | Owner chose minimisation: CR79, CR102 |
| 14 | High: No retention, deletion or cleanup | privacy | Confirmed: no cleanup rule. The legal framing does not apply to a local single-user tool | Owner chose cleanup: CR103; pruning and retention deferred |
| 15 | Major: A denial that returns an error becomes red | architecture | Reproduced: a refusal without a kill left a failed lookup exiting 0 | Accepted: CR67 covers every operation, CR97 judges clean runs by output |
| 16 | Major: No per-run verifier health check | architecture | Reproduced: nested sandbox exit 71, broken profile exit 65 | Accepted: CR23, CR74, CR75, CAE19 |
| 17 | Major: Environment and service leaks around file rules | architecture | Reproduced: another process's environment read and a process killed; both closed by process rules | Accepted: CR80, CR81, CAE27 |
| 18 | Major: Store probe has no positive control | architecture | Confirmed from CR29 and CR30: an excluded store with one other file left holds with credit | Owner chose bounded exclusions: CR100, CR101, CAE24 |
| 19 | Major: Id does not follow what the check measures | architecture | Confirmed: rulebook E1 to E5 match plain checks by id and carry no condition | Owner chose ids that follow meaning: CR99, CR91, CAE25 |
| 20 | Major: Corpus covers only record to verdict | architecture | Confirmed: CR51 checks record to verdict only | Owner chose blind derivation: CR104; CR52 now records behaviour under the profile |
| 21 | Major: A slow host makes a timeout red | architecture | Confirmed from CR21: an overrun on a slow host was always red | Owner chose a host control: CR98, CAE23 |
| 22 | Major: Schedule has no cut rule or approval date | architecture | Confirmed: no checkpoint and no rule for slipping | Owner chose moving the date and cutting nothing: CR105, CR106 |
| 23 | Blocker: Nothing establishes per run that the verifier is active | standards | Reproduced: nested sandbox exit 71, broken profile exit 65 | Accepted: CR23, CR74, CR75, CAE19 |
| 24 | Blocker: A link in the seed defeats the secrets claim | standards | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 25 | Major: Corpus validates the judge, not collection | standards | Confirmed: CR51 checks record to verdict only | Owner chose blind derivation: CR104; CR52 now records behaviour under the profile |
| 26 | Major: No equality relation for collector output | standards | Confirmed: no schema or equality rule was required for the collector | Accepted: CR89 |
| 27 | Major: No subject-file schema | standards | Confirmed: no schema or equality rule was required for the collector | Accepted: CR89 |
| 28 | Major: Discovery compares counts only | standards | Confirmed: equal counts can hide a different name | Accepted: CR70, CR85, CR86, CAE28 |
| 29 | Major: Read allow-list and metadata unstated | standards | Confirmed: the allow-list was unstated, and file metadata was visible in a test | Accepted: CR83 |
| 30 | Major: Spike list omits two computation rules | standards | Confirmed: CR3 omitted CR35 and CR37 | Accepted: CR3 |
| 31 | Major: Engine mirrors the judge's rejection rules | standards | Reproduced: a Go panic exits 2; the judge's rejection rules would have been written twice | Accepted: CR69, CR71, CR88 |
| 32 | Major: Exclusions hollow out a credited check | standards | Confirmed from CR29 and CR30: an excluded store with one other file left holds with credit | Owner chose bounded exclusions: CR100, CR101, CAE24 |
| 33 | Major: Ids carry no specification version | standards | Confirmed: rulebook E1 to E5 match plain checks by id and carry no condition | Owner chose ids that follow meaning: CR99, CR91, CAE25 |
| 34 | Major: Race between a timeout kill and a sandbox kill | standards | Confirmed from CR21: an overrun on a slow host was always red | Owner chose a host control: CR98, CAE23 |
| 35 | Major: Serial chain with no sizing | standards | Confirmed: no checkpoint and no rule for slipping | Owner chose moving the date and cutting nothing: CR105, CR106 |
| 36 | Major: No per-grade positive control | interface | Reproduced: nested sandbox exit 71, broken profile exit 65 | Accepted: CR23, CR74, CR75, CAE19 |
| 37 | Major: Profile is not parameterised | interface | Reproduced: a path spliced into profile text read a decoy; a parameter did not | Accepted: CR78, CAE20 |
| 38 | Major: Declared paths are not contained | interface | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 39 | Major: Exclusions give a false green | interface | Confirmed from CR29 and CR30: an excluded store with one other file left holds with credit | Owner chose bounded exclusions: CR100, CR101, CAE24 |
| 40 | Major: Read proof has no attribution rule | interface | Confirmed from CR20 and CR31: a kill in the control alone proves no read | Accepted: CR31, CR107 |
| 41 | Major: No closed table of engine exit codes | interface | Reproduced: a Go panic exits 2; the judge's rejection rules would have been written twice | Accepted: CR69, CR71, CR88 |
| 42 | Major: Discovery has no defined algorithm | interface | Confirmed: equal counts can hide a different name | Accepted: CR70, CR85, CR86, CAE28 |
| 43 | Major: Route to green assumed for real CLIs | interface | Confirmed: routes were assumed, and a subject with no secret file has none | Accepted: CR92, CR94, stated limit; env-only proof stays deferred by the owner |
| 44 | Major: Corpus validated on the judge half only | interface | Confirmed: CR51 checks record to verdict only | Owner chose blind derivation: CR104; CR52 now records behaviour under the profile |
| 45 | Blocker: Verifier attested once, never checked at run time | experiment | Reproduced: nested sandbox exit 71, broken profile exit 65 | Accepted: CR23, CR74, CR75, CAE19 |
| 46 | Major: Injection, seed links and a copy escape defeat the secrets claim | experiment | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 47 | Major: Planted-secret probe does not say what it asserts | experiment | Confirmed: the plan defined neither where a leak is sought nor what matches | Accepted: CR84, CAE26 |
| 48 | Major: Route to green unproven | experiment | Confirmed: routes were assumed, and a subject with no secret file has none | Accepted: CR92, CR94, stated limit; env-only proof stays deferred by the owner |
| 49 | Major: Declarations can reach green | experiment | Confirmed from CR29 and CR30: an excluded store with one other file left holds with credit | Owner chose bounded exclusions: CR100, CR101, CAE24 |
| 50 | Major: Discovery count hides missing commands | experiment | Confirmed: equal counts can hide a different name | Accepted: CR70, CR85, CR86, CAE28 |
| 51 | Major: Corpus cannot test collection rules | experiment | Confirmed: CR51 checks record to verdict only | Owner chose blind derivation: CR104; CR52 now records behaviour under the profile |
| 52 | Major: Engine-caused red routes remain | experiment | Confirmed from the text: environment, terminal, pipe and comparison inputs were open | Accepted: CR79, CR82, CR98, CR101 |
| 53 | Major: Date cannot be confirmed | experiment | Confirmed: no checkpoint and no rule for slipping | Owner chose moving the date and cutting nothing: CR105, CR106 |
| 54 | High: Declared paths allow arbitrary reads | security (agy) | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 55 | High: System services bypass confinement | security (agy) | Reproduced: named services reachable; an allow-list with kill closed the five tested and kept user lookup correct | Accepted: CR80, CR81, CR83 |
| 56 | High: Tripwire, copy and exclusion false greens | security (agy) | Confirmed: CR36 names the copy escape | Corrected: same-content declared files refused by CR36; other copies remain a stated limit |
| 57 | High: A killed runbook step leads to a false red | security (agy) | Confirmed: R33 covers only steps after a failed step | Owner's reading: CR40, CAE22 |
| 58 | Blocker: Declared paths read and leak private files | standards (agy) | Reproduced: a link-following seed copy exposed a decoy; no plan rule contained declared paths | Accepted: CR76, CR77, CAE20 |
| 59 | Blocker: Unbounded output triggers a judge rejection | standards (agy) | Reproduced: 33-digit number, deep nesting and 17 MB each rejected by X12 | Accepted: CR38, CR79, CAE21 |
| 60 | Major: No green for env-only subjects, read proof vague | standards (agy) | Confirmed: routes were assumed, and a subject with no secret file has none | Accepted: CR92, CR94, stated limit; env-only proof stays deferred by the owner |
| 61 | Major: Targets, tiers and reason codes unspecified | standards (agy) | Confirmed against the judge's input schema | Accepted: CR90, CR70 |
| 62 | Major: Unclassified commands have no route to the judge | standards (agy) | Confirmed: the judge's input carries only five facts, so an unclassified command needs a check | Accepted: CR86 |
| 63 | Major: Offline arguments and exclusions give false green | standards (agy) | Confirmed from CR13: scope and help arguments were open | Accepted: CR87, CR94 |
| 64 | Major: Verdict-source rule conflicts with exit codes | standards (agy) | Reproduced: a Go panic exits 2; the judge's rejection rules would have been written twice | Accepted: CR69, CR71, CR88 |
| 65 | Major: Scripted baselines cannot be anchored | standards (agy) | Confirmed from rulebook N1 and E2 | Accepted: CR73, baselines supplied as files |

### Rejected or corrected claims

- Rejected: that 13 rather than 14 dimensions always apply (standards, Nit). `flag-conformance` applies to every CLI, and running the judge with 14 held and 3 excluded exits 0.
- Rejected: that the date is confirmed (standards on agy). Four other lenses showed it was not, and the owner set the slip rule.
- Rejected: that refusing process operations in general closes the environment leak. It killed clean programs. Refusing them toward other processes, with a kill and self allowed, closed it.
- Deferred Minors: sealing the mechanism as a separate annex, synthetic markers for planted secrets, the canonical form of the seed digest, rejecting a false `has_cli`, and naming who holds the private cases. Planning takes these up.
- The specification is documentation, so no regression test accompanies these fixes. Each reproduced finding has an acceptance example or a required spike test in its place.
## Round 2 (design): delta verification

**Reviewers:** peer-standards-reviewer, runtime=agy (gemini-4-argon-high requested, rc 0). The author wrote the fixes, so a different model checked each one against the revised plan, the requirements and the sealed judge.

**Result:** all 65 rows closed and no new Blocker or Major.

This verifies the plan's text. The spike still has to prove each control on a real printed CLI, on the CI runner image and under a sandboxed parent.

**Gate outcome:** passed on round 2. Round 1 had 5 Blocker findings in three themes, 9 High and 51 Major.
