---
title: Conformance Collection Specification - Plan
type: feat
date: 2026-11-21
topic: conformance-collection-spec
artifact_contract: ce-unified-plan/v1
product_contract_source: ce-brainstorm
execution: code
deepened: 2026-11-22
---

# Conformance Collection Specification - Plan

This plan covers phase 2 of the [engine plan](2026-11-21-engine-plan.md).
Its own ids start with C: CR for requirements, CF for flows, CAE for acceptance examples.
A bare R, AE, A or F number is an entry in [the requirements](../requirements.md).
The prefix is needed because the citation test reads every bare R or AE number under `docs/` as one of those entries, including one written after a hyphen.

## Goal Capsule

- **Objective:** A person who grades a printed CLI can trust that the verdict rests on what that CLI did: no green from evidence that was never observed, and no red that the engine caused.
- **Means:** A sealed conformance collection specification, gated by a no-network spike and proved by a synthetic subject and a two-sided corpus. A builder with only this repository can then build the grade command without inventing a rule (KTD1, KTD7, KTD9).
- **Authority:** The sealed judge 1.0 first, then this Product Contract, then the Planning Contract, then a unit. A requirement wins over a decision of planning. An input with two honest readings goes to the owner.
- **Stop conditions:** A failed spike gate (KTD6), a need to download, use a key or use the network for the real subject (CR4), or any change that would need judge 1.1.
- **Execution profile:** Deep. Three pull requests: the spike, the draft specification, then the corpus and seal. The main session writes the specification and a coding agent writes the code (KTD16).
- **Who finishes:** The executing session opens each pull request and stops. The owner merges and ratifies.
- **Product authority:** The owner, who ratifies and seals the specification. This plan covers phase 2 only. The grade command, the first real grade and steering are not active scope.
- **Open blockers:** None for planning. The spike gates the specification work (CR1, CR6). Building and running the real subject needs the owner's separate approval (CR4).
- **Dates:** Spike answered by 2026-11-28, specification draft by 2026-12-05, corpus by 2026-12-12, seal by 2026-12-19. A late checkpoint moves the date and cuts nothing (CR106).

---

## Product Contract

### Summary

Specify and seal how `pql` turns a subject file and a real CLI into evidence for judge 1.0.
The CLI runs confined in a seeded throwaway home, expected answers come from known inputs, six probes are generated, and whatever cannot be verified is not-measured.

### Problem Frame

Judge 1.0 decides every verdict from a finished evidence record, and nothing specifies how that record is produced.
A collector built by guesswork would hand the judge wrong evidence, and the judge would grade it faithfully.

The engine plan's §3 lists what is missing, and that list is incomplete.
The judge also reads `crediting`, `record`, expected-failure markers and five subject facts, and §3 names a source for none of them.
Crediting matters most: the default scope has 17 dimensions, at least 14 always apply to a CLI, and each applicable one needs a held crediting check before a run exits 0.

Printed CLIs also make offline measurement harder than it looks.
The source of current ones, read for this plan, shows reads that try the network before local data, state written on read commands and fallbacks to a cache.
Blocking the network is therefore not evidence that no request was attempted.

### Key Decisions

- **The subject is strict and a doubt is not-measured.** This is what R16 asks. Governs CR1, CR18, CR19, CR20. (session-settled: user-directed — chosen over counting a blocked run as verified: a swallowed attempt that falls back to a cache would grade green.)
- **macOS only in 1.0.** The engine plan's §7 allows one supported system. Governs CR1, CR5, CR6, CR56. (session-settled: user-directed — chosen over a Linux time box, a network-only measurement or a ptrace supervisor: a seccomp filter cannot decide a file rule by path, so the time box could only end unsupported.)
- **An unexplained SIGKILL is not-measured.** This reads R16 over R22 for that one input. Governs CR20, CR21. (session-settled: user-directed — chosen over red or a diagnostic re-run: the sandbox's kill cannot be told from any other.)
- **Help is not execution when confined.** This settles how R42 and AE24 meet. Governs CR25, CR26. (session-settled: user-directed — chosen over root help only or the subject's own manifest: nested commands must be counted without trusting the subject's report.)
- **One strict JSON subject file.** It needs no dependency. Governs CR8, CR9. (session-settled: user-directed — chosen over TOML, YAML and Markdown: the judge's input rules apply directly and no second grammar is needed.)
- **A computation is any executable.** It keeps `pql` a single binary. Governs CR34, CR35, CR39. (session-settled: user-directed — chosen over built-in operations, a values file and a Python contract: users bring any language without adding a runtime to the engine.)
- **A computation has its own confinement.** R16 governs the subject, and ordinary scripts need child processes. Governs CR35, CR36, CR37. (session-settled: user-approved — chosen over the subject's no-child rule: that rule kills an ordinary shell pipeline.)
- **A computation's process group is ended, and the escape is a stated limit.** macOS cannot refuse a child that leaves its group through `posix_spawn` without also refusing that call. Governs CR35. (session-settled: user-directed — chosen over tracking processes by their sandbox or forbidding `posix_spawn`: the first relies on a private call that can kill unrelated processes, and the second breaks Python, Node, `xargs` and `system()`.)
- **Two operations are refused without a kill.** macOS rejects the kill on them. Governs CR67. (session-settled: user-directed — chosen over leaving both allowed: an untested entitlement could widen the confinement.)
- **The CLI agreeing with itself can fail but cannot earn green.** Green needs a known answer. Governs CR32, CR33, CR37. (session-settled: user-directed — chosen over crediting agreement or banning self-checks: two commands can share a bug, and a disagreement is still worth reporting.)
- **Every grade runs on a seeded throwaway copy.** Known answers need known data. Governs CR14, CR15, CR16, CR17. (session-settled: user-directed — chosen over a snapshot or the real store: results must repeat and real data must not be touched.)
- **The subject file can declare how to run offline.** Live-first CLIs would otherwise have no measurable reads. Governs CR13. (session-settled: user-approved — chosen over default invocations only: those reads attempt the network and are all not-measured.)
- **Six generated probes.** They are what the requirements and restricted credit need. Governs CR27, CR28, CR31. (session-settled: user-directed — chosen over every mechanical probe or green without authoring: each probe costs cases, and proxy probes invite false greens.)
- **Bookkeeping exclusions are declared and shown.** Governs CR29, CR30. (session-settled: user-directed — chosen over any write being red or switching bookkeeping off: invocation journals are by design, and the default behaviour should be the one graded.)
- **A planted secret counts only where it would be read.** Governs CR31. (session-settled: user-approved — chosen over planting it anywhere: a secret the CLI never reads proves nothing by its absence.)
- **The needed crediting rules are restated here.** CDR spec 0.9.1 is their starting point. Governs CR43, CR44. (session-settled: user-directed — chosen over waiting for that spec's publication or writing new rules: the specification must be buildable from this repository and judge 1.0 keeps those rules unchanged.)
- **The real subject is chosen and not named here.** The engine plan's §8 item 1 is settled. Governs CR4, CR61. (session-settled: user-directed — chosen over three other candidates: it best represents current generator output with low-sensitivity data.)
- **Some cases stay private.** Governs CR54. (session-settled: user-approved — chosen over publishing every case: builds are scored against cases they have not seen.)
- **The corpus is checked before a collector exists.** Governs CR51, CR52. (session-settled: user-approved — chosen over waiting for the grade command: a wrong corpus would train a wrong collector.)
- **A rejected record is the engine's failure.** Governs CR47. (session-settled: user-approved — chosen over reporting a verdict: the judge gave none.)
- **A confinement refusal is never red.** The objective rules out a red the engine caused. Governs CR20, CR67. (session-settled: user-approved — chosen over letting the subject carry on after a refusal: its failure would look like its own defect.)
- **A planted secret's read is proved.** An author's declaration is not an observation. Governs CR31, CR68. (session-settled: user-approved — chosen over trusting the declared location: a place never read gives a green that proves nothing.)
- **A read takes two signals.** macOS kills at open, so the control run alone counts a file that was opened and never read. Governs CR31. (session-settled: user-directed — chosen over calling an open a read or using the access time alone: the first credits an unread secret, and the second has nothing to catch another reader.)
- **Synthetic programs decide the spike's pass.** The real subject's run gates the seal. Governs CR6, CR66. (session-settled: user-approved — chosen over making that run part of the 2026-11-28 pass: a late approval would stop the work for a reason that is not evidence.)
- **A test run is scripted at every level.** Nothing verified that it was offline. Governs CR49, CR73. (session-settled: user-approved — chosen over labelling its evidence offline: that would claim what a fake verifier cannot show.)
- **A malformed command line and a rejected request have different codes.** Both existing contracts are kept. Governs CR71, CR72. (session-settled: user-approved — chosen over one code for both: the rulebook gives a rejected request 2 and the command already gives a malformed line 64.)
- **The store cannot be excluded.** This bounds the declared exclusions. Governs CR100, CR101. (session-settled: user-directed — chosen over unbounded exclusions or no credit with any exclusion: an excluded store gave a false green, and CLIs with journals must still be able to reach green.)
- **A runbook stops at the first step not held.** This reads R33 for a step that was not observed. Governs CR40. (session-settled: user-directed — chosen over stopping only after a failure: later steps would go red on state the sandbox left half written.)
- **A timeout is red only on a healthy host.** R22 stands where the host kept up. Governs CR98. (session-settled: user-directed — chosen over always red: a loaded host would cause reds that are not the subject's.)
- **A check's id follows its meaning.** The judge compares by id alone. Governs CR99. (session-settled: user-directed — chosen over fixed author names: two different measurements must not compare as the same.)
- **Records are minimized and cleaned up.** Captured output can hold secrets and personal data. Governs CR102, CR103. (session-settled: user-directed — chosen over location only or adding retention and pruning now: it closes the leak without another command to specify.)
- **An independent reader derives evidence before the seal.** The specification and corpus have one author. Governs CR104. (session-settled: user-directed — chosen over reviews alone: the judge's ambiguities were found by independent case writers.)
- **A late phase 2 moves its date.** Quality is kept and the risk to the 2027-01-25 halfway check is accepted. Governs CR105, CR106. (session-settled: user-directed — chosen over cutting breadth first or deciding at the time: nothing is cut.)
- **A local bind or listen is a network attempt.** This reads R16 strictly. Governs CR109. (session-settled: user-directed — chosen over allowing a loopback bind: a local relay could carry requests out of the sandbox.)
- **The user retires ids, from the engine's list.** This keeps R26 when a change of meaning renames checks. Governs CR114. (session-settled: user-directed — chosen over retiring automatically: a dropped check must not vanish silently.)

<!-- ce-section: work-relationships -->
### How This Work Fits Together

This plan covers phase 2. The rest is the engine plan's current understanding and is not scope here.

- Phase 3, the grade command. Depends on this plan's sealed specification and reviewed corpus.
- Phase 4, the first real conformance grade. Depends on phase 3, and shares the real subject chosen here.
- Phase 5, the steering collection specification. Shares the subject file, and extends the test entry this plan specifies.
- Phases 6 and 7, steering built and run. Depend on phase 5.

### Actors

The requirements' A1 (user), A2 (operator agent) and A4 (subject CLI) apply unchanged. Three roles act in this phase:

- **Owner:** decides readings, approves runs of the real subject, ratifies and seals.
- **Specification author:** writes the specification and the expected evidence, and does not build the collector.
- **Builder and reviewers:** the builder sees only this repository. A reviewer did not write what it reviews.

### Key Flows

- CF1. Phase 2, from spike to seal
  - **Trigger:** This plan is approved.
  - **Steps:** Answer the spike. Write the specification. Build the synthetic subject. Write the corpus and check it against the judge. Review, ratify and seal.
  - **Outcome:** Phase 3 can start, or the owner has been told why it cannot.
  - **Covered by:** CR1 to CR7, CR49 to CR61, CR65, CR66, CR73, CR97, CR104 to CR106
- CF2. Collection, as the specification must describe it
  - **Trigger:** A1 or A2 asks for a grade.
  - **Steps:** Read and validate the subject file. Fix the required set. Test the confinement. Discover commands by confined help, as a check. Run the subject on fresh seeded copies. Run computations separately. Build the record and call the judge.
  - **Outcome:** One judge document gives the exit code and every verdict. The scorecard and stored record add the run's evidence and details, per CR69.
  - **Covered by:** CR8 to CR48, CR109 to CR114, CR62 to CR64, CR67 to CR72, CR74 to CR92, CR94, CR98 to CR103, CR107, CR108

```mermaid
flowchart TB
  F[Subject file, validated] --> Q[Required set fixed before collection]
  Q --> V[Confinement self-test, every grade]
  V --> D[Discovery by confined help]
  D --> S[Subject: no network, no children, throwaway home]
  I[Seed] --> S
  I --> C[Computation: no network, children allowed]
  S -->|captured output: self-check, no credit| C
  S -->|observed| E[Evidence record]
  C -->|expected| E
  E --> J[Judge 1.0]
  J -->|verdicts and exit code| O[Exit code, scorecard, stored record]
  E -->|evidence and run details| O
```

### Requirements

**The no-network spike**

- CR1. On macOS, the spike shows that a run with no attempt is verified as such, and that a run with an attempt or one that cannot be verified never is.
- CR2. The spike tests swallowed errors, DNS, child processes, replacing the process image, local sockets, and delegation to system daemons.
- CR3. The spike tests that the confinement of files and computations required by CR16, CR17, CR35 to CR37, CR62 to CR64, CR67, CR78 and CR80 to CR82 can be enforced, and that the read proof in CR31 works.
- CR4. The spike includes a confined offline run of the real subject, after the owner separately approves its build and that run.
- CR5. Linux is unsupported in version 1.0, and the spike's record gives the reason: a seccomp filter cannot decide by path, Landlock refuses without ending the process, and a ptrace supervisor was not attempted.
- CR6. macOS passes when CR1, CR2 and CR3 hold on synthetic programs, and if it has not passed by 2026-11-28, specification work that depends on confinement stops and the owner is told that the milestone is in doubt.
- CR7. The spike's answer is recorded in this repository with what was tested, on which system versions, and each limit left open, and holds no captured output, local path or name of the real subject.
- CR65. The spike's synthetic programs also run once on a pinned GitHub macOS runner image, whose version is recorded.
- CR97. The spike also runs under an already-sandboxed parent, tests delegation from a computation, and judges each clean program by its output as well as its exit status.
- CR66. The results of CR4 and CR65 are required before the seal, and a failure in either goes to the owner.

**The subject file**

- CR8. One versioned strict JSON file registers a subject and holds its facts, command classes, declared commands and bounds, seed, offline settings, bookkeeping paths, secret location, runbooks and expected-failure markers, with a versioned place for phase 5's prompts.
- CR9. The file follows the input rules of the judge contract's §3 for repeated keys, number forms, depth and encoding.
- CR10. The file supplies the judge's five subject facts, and an unstated fact is an unfilled placeholder, never a "no".
- CR11. `create` writes placeholders that explain themselves without comments, and a grade of an unfilled file names each one.
- CR12. Each command is declared safe or destructive or is unclassified, with the consequences R16 states.
- CR13. The file can declare arguments and environment for offline invocation, and every scorecard shows them, with environment values masked.
- CR87. Offline settings state which invocations they apply to, never apply to discovery, and cannot be a help or version request.
- CR86. Any unfilled placeholder, including a declared or discovered command with no safety class, makes a required check on an always-applicable dimension `placeholder_unfilled`, so the run cannot exit 0.
- CR76. The seed and every declared file resolve inside the subject file's directory, which is neither the user's home nor above it, and a declared executable grants a read of that one executable file only.
- CR112. Each declared file and executable is a regular file, and a grant covers that one file and never a directory.
- CR77. The seed is copied by content, and a seed holding a link or special file is rejected before collection.

**State and confinement**

- CR14. Each runbook and each probe gets a fresh copy of the declared seed in a throwaway home, and an ordered runbook's steps share that copy.
- CR15. The run records the seed's digest, and with no seed the home is empty.
- CR16. The subject cannot write outside its throwaway home.
- CR17. The subject cannot read the user's real state for that CLI.
- CR62. The subject can read file contents only from its binary, the system paths a process needs to run, and its throwaway home.
- CR63. The engine builds the environment of the subject and of each computation from an allow-list and the subject file's literal declarations, and passes no other variable through.
- CR67. Any operation the confinement refuses ends the subject's process, so the check is not-measured and never red. The specification names the only two exceptions, `exception-entitlement` and `process-exec-update-label`, which macOS cannot refuse with a kill: both are refused without one, and it states that such a refusal is silent.
- CR78. The profile is a fixed template that receives every path as a canonical parameter, never as profile text.
- CR80. The subject can reach only an allow-list of system services, and can neither read another process's environment or arguments nor signal another process.
- CR81. CR80 binds a computation too, except toward processes in its own sandbox.
- CR82. The engine fixes home, temporary directory, search path, locale and time zone, gives no terminal, input or inherited descriptor, and rejects a declaration that overrides these or sets a loader variable.
- CR83. The specification lists the allowed system paths and services, and states which file metadata stays visible.

**What the subject's run means**

- CR18. A check is observed only when the engine verified that the subject attempted no network request during it.
- CR109. Binding or listening on any socket, loopback included, is a network attempt.
- CR19. A check in which the subject starts a child process is not-measured under CR20.
- CR20. A SIGKILL the engine cannot attribute is `network_unverifiable`, and its next action names the possible causes: network, child process, confinement refusal and external kill.
- CR21. A timeout under CR98 and every other crash remain subject failures and red, as R22 says.
- CR98. A timeout is red when the engine saw the process still running as it ended it and a known-fast control kept within its bound on that host; with an overrun control it is `harness_fault`.
- CR22. `network_attempted` is reported only when the attempt itself was evidenced.
- CR74. Before any check runs the subject, each grade runs self-test programs under that grade's confinement: one for each kind of refused operation must be killed, and a clean one must finish with the right output.
- CR111. The self-test's program takes the subject's place in the profile, and the computation's confinement is self-tested and never bypassed in the same way.
- CR75. A confinement that cannot be applied is detected apart from the subject's exit status, and the subject is never then run unconfined.
- CR23. When CR74 fails, CR75 applies, or the system has no verifier that passed the spike, every check that runs the subject is `network_unverifiable` and never red.
- CR24. A binary that cannot be started is `could_not_start`.

**Discovery and generated probes**

- CR25. Discovery may ask any command for help, only under the subject's full confinement with an empty home and with no other arguments.
- CR26. The specification states the limit that a CLI which ignores a help request runs that command's action inside the confinement.
- CR70. A declared command that discovery does not find leaves discovery and the other generated checks not-measured, never red, with a reason from the rulebook's §13 and both counts and the missing names shown.
- CR85. The specification defines how help is requested and read and which built-in commands are left out, and discovery compares command names as well as the count R42 asks for.
- CR27. Version 1.0 generates exactly six probes: discovery, bare invocation, byte bound, declared exit codes, planted secret and store unchanged.
- CR28. Each probe names the one dimension it feeds, whether it credits and when it is not-measured, and at least one dimension can be judged from generated checks alone.
- CR29. The store-unchanged probe compares the whole throwaway home except declared bookkeeping paths, which every scorecard lists.
- CR30. The store-unchanged probe is not-measured as `empty_target` when nothing non-empty is left to compare.
- CR100. An exclusion cannot cover the declared store or any path the seed supplied, so a seed is prepared without bookkeeping files.
- CR101. The store-unchanged probe credits only after its comparison detected a change made in a control copy, compares names and contents only, and follows a stated rule for database side files.
- CR31. The planted-secret probe is observed only when an invocation finishes with its declared successful exit and moved the planted file's access time, which it cannot set, and an otherwise identical control run, which kills on opening that file for reading, is killed. The self-test first shows that the volume records a read's access time, or the probe is not-measured. The specification states the limit that another process reading the file during such an invocation looks like the subject's read.
- CR107. The planted file is a credential file, or a seeded-state file for which the specification says why reading it proves the secret was read, which a multi-record database never is.
- CR84. A leak is the planted secret, in any listed encoding, in a captured stream or in any file of the used home other than the planted one, excluded paths included.
- CR110. The engine inspects a used home without following a link, opening a special file or reading past a bound, and reports a link or special file the subject created as a change.
- CR113. A leak scan stopped by any bound, on a stream or in the home, is not-measured and never held.
- CR68. With no proved read the planted-secret probe is not-measured as `input_missing`, as it is for a secret that only an environment variable can carry.

**Authored checks and expected values**

- CR32. Every observed value comes from the engine running the subject's binary under its confinement.
- CR33. A check whose expected value uses the subject's captured output is non-crediting, as fixed from its declared inputs before collection.
- CR34. An expected value is a literal or the one JSON value printed by a user's executable, which is named as an argument list and run without a shell.
- CR35. A computation runs with no network and a time budget, may start child processes, is killed on `setsid` or `setpgid` in the process that calls it, and has its whole process group ended with it. The specification states that a child which leaves the group through `posix_spawn` escapes that ending and stays in the computation's confinement.
- CR36. A computation cannot execute or read the subject's binary or be given a file with the same content, and the specification states that any other copy escapes this.
- CR37. A computation receives only the pristine seed and the captured outputs its check declares, never the subject's used home.
- CR64. A computation can read file contents only from its executable and declared files, system paths and what CR37 gives it, and can write only to its own scratch directory, never to the seed.
- CR38. A computation that fails, is absent, runs out of time or prints anything other than one JSON value that the judge contract's §3 accepts inside a record is `computation_failed`.
- CR39. The specification and the engine's documentation warn that grading someone else's subject file runs their code.
- CR40. Runbook steps run in order with time budgets, as R33 states, and every step after the first one not held is `earlier_step_failed`.
- CR41. The destructive-refusal check is authored and runs only in a scope that includes `destructive`, as R12 states.

**What the engine gives the judge**

- CR42. Check ids follow CR99, and the required set is fixed from the subject file, the rulebook and the requested scope before discovery or any other check runs.
- CR99. A check's id is made from its kind, dimension, predicate and parameters, invocation and declared environment, earlier runbook steps, seed digest, exclusions, expected-value source and collection specification version, and from nothing else, so a changed subject binary keeps it.
- CR114. The engine lists a baseline's missing ids with the cause of each, in a form the user passes back whole to `anchor` as retired, and retires none by itself.
- CR91. The collection specification has a version string, a definition of a breaking change and a dated errata note outside the sealed folder.
- CR92. The specification holds a table of every conformance dimension with its route to credit, the independent source of its expected value, and the subjects that have no route.
- CR94. The specification holds a table of each known false-pass and false-fail route with its positive control, including that a green speaks only for the declared invocation and exclusions.
- CR43. The specification restates, in its own words and under its own rule ids, the crediting and routing rules it uses, with no text or fixtures copied from CDR spec 0.9.1.
- CR44. Only the planted-secret probe credits `pii-secret-hygiene`, and only the store-unchanged probe credits `side-effect-integrity`.
- CR45. A rule defines how each required-check and evidence field is produced, including `record`, target size, observed precondition, tier and evaluator, and how markers reach the judge.
- CR90. Every not-observed check reaches the judge with its reason and at least one target, and a normal run claims `live` while its offline evidence carries `offline`.
- CR79. Captured output is drained, bounded and encoded, and literals are bounded, so that nothing a user or subject supplies can make the engine's record fail the judge contract's §3.
- CR89. JSON Schemas for the subject file, stored record, baseline and case format are sealed with the specification, which also says which record fields must be equal for a collector to conform.
- CR46. Every not-measured reason conformance can produce has a next action.
- CR71. Before collection the engine validates the subject file and has the judge check the request and required set without evidence, and the specification states for each rejection rule whether user or engine caused it.
- CR88. The specification has a closed table of engine exit codes, in which every engine fault, a panic included, is 70 and none can exit 1, and which states the limit that a fatal error of the Go runtime cannot be caught and exits 2.
- CR72. A malformed command line exits 64, and a well-formed request rejected for its content or scope exits 2, as the rulebook's §8 says.
- CR47. When the judge rejects the engine's record, the engine exits 70, gives no verdict and keeps that record for diagnosis.
- CR48. The specification defines where run records and baselines live and in what form, consistent with R27.
- CR69. For every record the judge decided, the exit code and every verdict are read only from the judge's output, while the scorecard and stored record add the evidence and run details the engine recorded.
- CR102. Records are readable only by the user and live outside any repository, and every record, a rejected one included, and every scorecard mask the planted secret and declared environment values in every encoding CR84 lists.
- CR103. Throwaway homes and scratch directories are removed on every exit, a killed check included.
- CR108. The scorecard and documentation meet the errata's E-4, so a successful anchor is never shown as a green run.

**The test entry, synthetic subject and corpus**

- CR49. The specification defines the test entry for a scripted network verifier: the default build is the released one and rejects it with exit 2, only a non-default build contains the scripted verifier, and its runs cannot be anchored.
- CR73. A test run is `scripted` in its claimed tier, targets and evidence, and the corpus's baseline cases for it use scripted baselines supplied as files.
- CR50. The synthetic subject is a small CLI in test data, with a known-good form that can make all 17 default-scope dimensions green, and `destructive-guard` in its scope, and other forms that each carry one seeded defect.
- CR51. Expected evidence is written from the specification, and each case's record is run through the real judge to confirm the stated verdict and exit code.
- CR52. Each form of the synthetic subject is run directly and under the spike's profile without collector code, and what it did is recorded as the source of its expected evidence.
- CR104. Before the seal, a reader who did not write the specification, on a different model, derives expected evidence for a sample of cases from the text alone, and each disagreement is treated as an ambiguity.
- CR53. The corpus is language-neutral and two-sided: every rule has a case, every not-measured case has a repaired twin that grades green, and every defect is red on its named check.
- CR54. A privately kept set of cases scores builds, and nothing from it reaches a builder except a rule id.
- CR55. The corpus states which cases need the real verifier, including every anchor case except the refusal as a test run, and which run through the test entry, and a skipped case is reported as skipped, never passed.

**CI, review and seal**

- CR56. The specification and the engine plan assign phase 3 a macOS CI job for the real verifier, required on `main` once green, with the scripted verifier on Linux.
- CR57. CI verifies the new sealed folder's digest and file set as it does judge 1.0's.
- CR58. An input that can honestly be read two ways goes to the owner with both readings.
- CR59. The specification lists each reading or departure from the requirements, including those in CR20, CR25, CR40, CR67, CR70, CR72, CR73 and CR98.
- CR60. A standards reviewer and a different model review the specification, the synthetic subject and corpus are reviewed, and the owner then ratifies and seals `docs/specs/conformance-collection-1.0/`.
- CR105. Progress is reported to the owner at each checkpoint in the Goal Capsule.
- CR106. A missed checkpoint moves the phase's date and cuts no scope, case or review, and the owner is told what that does to the 2027-01-25 halfway check.
- CR61. The engine plan is updated for the decisions settled here, the items missing from its §3, the earlier test entry, the phase 3 macOS job §4's rule on what is read from the judge's output and any date moved under CR106, without naming the real subject.

### Acceptance Examples

- CAE1. **Covers CR18, CR20.** Given a subject that tries the network, ignores the error and would serve its cache, when the check runs on macOS, then it is not-measured and the network is named among the possible causes.
- CAE2. **Covers CR19, CR20.** Given a subject that starts a harmless child, when the check runs, then it is not-measured and the next action names child processes as a possible cause.
- CAE3. **Covers CR20, CR21.** Given three subjects, one killed by an unexplained SIGKILL, one that segfaults and one past its budget on a host where the control kept up, when each is checked, then the first is not-measured and the other two are red.
- CAE4. **Covers CR33.** Given a subject whose `list` and `count` both report 9 of 10 seeded items, when a self-check compares them, then it holds without credit. When a check expects the literal 10, it is red.
- CAE5. **Covers CR33.** Given `list` reporting 10 and `count` reporting 9, when a self-check compares them, then the dimension is red.
- CAE6. **Covers CR35, CR36.** Given a computation that pipes its declared input through another program, then it returns its value. Given one that invokes the subject's binary, then the subject does not run.
- CAE7. **Covers CR29, CR30.** Given a read command that appends only to an excluded journal, then the probe holds and the exclusion is shown. Given a write to any other file, then it is red. Given a home with nothing non-empty outside the excluded paths, then it is not-measured as `empty_target`.
- CAE8. **Covers CR31, CR68.** Given a subject with no file it reads a secret from, or a declared file that the proof shows it never reads, a file it only opens included, when it is graded, then the probe is not-measured and the run does not exit zero.
- CAE9. **Covers CR12, CR25.** Given a nested command with no safety class, when the subject is graded, then its help may be read, its action is not requested, and the run is not-measured naming that command.
- CAE10. **Covers CR13, CR18.** Given a live-first read, when it runs with no offline declaration and attempts the network, then it is not-measured. When a declared offline invocation avoids the attempt, then it is judged and the declaration is shown.
- CAE11. **Covers CR16, CR17.** Given a subject that ignores the redirected home, when it runs, then the user's real state is neither read nor changed.
- CAE12. **Covers CR23.** Given a grade on an unsupported system, then every check that runs the subject is not-measured as `network_unverifiable` and the run does not exit zero.
- CAE13. **Covers CR47.** Given a record the judge rejects, when `grade` ends, then it exits 70, shows no verdict and keeps the record.
- CAE14. **Covers CR49, CR55.** Given a green run through the test entry, when it is anchored, then it is refused as a test run. The successful-anchor case runs on the real verifier.
- CAE15. **Covers CR67.** Given a subject that writes a cache outside its home and would carry on or crash when refused, when the write is refused, then the process ends and the check is not-measured.
- CAE16. **Covers CR62, CR63, CR64.** Given a real API key in the engine's environment and a private file in the user's home, when a subject or computation tries to read either, then it gets neither.
- CAE17. **Covers CR70.** Given 40 declared commands and 3 found, when the subject is graded, then the generated checks are not-measured and both counts are shown.
- CAE18. **Covers CR71, CR72.** Given an unknown flag, then the exit code is 64. Given a marker on a Safety dimension in the subject file, then the request is rejected with 2 before collection and is not reported as an engine failure.
- CAE19. **Covers CR23, CR74, CR75.** Given a nested sandbox, a broken profile or a profile whose kill does not fire, when a grade runs, then every check that runs the subject is not-measured and none is red.
- CAE20. **Covers CR76, CR77, CR78.** Given a declared path that holds profile syntax, climbs out of its directory or is a link to a private file, when the subject is graded, then that file is never read or copied.
- CAE21. **Covers CR38, CR79.** Given a subject that prints 17 MB, invalid UTF-8 or a 33-digit number, then the check is judged and the exit is not 70. Given a computation that prints such a number, then it is `computation_failed`.
- CAE22. **Covers CR40.** Given a first step killed by the confinement, when the runbook continues, then each later step is `earlier_step_failed` and none is red.
- CAE23. **Covers CR98.** Given a subject past its budget, then it is red. Given the same on a host where the control also overran, then it is `harness_fault`.
- CAE24. **Covers CR100.** Given a subject file that lists its store as bookkeeping, when it is graded, then it is rejected before collection.
- CAE25. **Covers CR99, CR114.** Given a baseline and an edited seed, then the old checks are missing, they are listed for retirement, and re-anchoring with that list is the next action. Given the same file and a worse binary, then the check is regressed.
- CAE26. **Covers CR84.** Given a subject that writes the planted secret, plain or in a listed encoding, to its excluded journal, when it is graded, then the probe is red.
- CAE27. **Covers CR80, CR81.** Given another process with a key in its environment, when a subject or computation tries to read or signal it, then it gets nothing and that process is unharmed.
- CAE28. **Covers CR70, CR85, CR86.** Given as many discovered commands as declared but one different name, when the subject is graded, then the missing and the unclassified command are named and the run is not-measured.
- CAE29. **Covers CR110.** Given a subject that leaves a link to a private file in its home, when the engine inspects that home, then the private file is never read and the link is reported as a change.
- CAE30. **Covers CR99.** Given two probes on one command, when the required set is built, then their ids differ and the judge accepts the set.
- CAE32. **Covers CR113.** Given a subject that writes the planted secret past the scan's bound, when it is graded, then the probe is not-measured and never held.
- CAE31. **Covers CR109.** Given a subject that listens on a loopback port, when the check runs, then it is killed and not-measured.

### Success Criteria

- The engine plan's phase 2 "Done when" holds: the spike is answered, the specification is sealed, and the synthetic subject and corpus exist and are reviewed.
- A builder given only this repository can implement each slice of phase 3 without asking what a rule means.
- No collector code was written before the corpus and synthetic subject were reviewed.
- The owner hears of a late checkpoint when it is missed, and nothing was cut to meet a date.

### Scope Boundaries

**Deferred for later**

- All collector and grade-command code, which is phase 3.
- The macOS CI job for the real verifier, which phase 3 adds under CR56.
- Proving that a secret was read from an environment variable.
- More generated probes, including help on every command as a check, unknown flags, error text, JSON parsing and repeat-run determinism.
- The MCP surface.
- Steering, including its part of the test entry.
- Linux verification, which CR5 puts outside version 1.0.
- Verifying subjects that need child processes.
- A command to prune stored records, and retention rules.

**Not in this product**

- A green run without authored checks in version 1.0. A single dimension can be green from a probe alone.
- Reading a subject's store in place of running the subject.
- Compatibility with the predecessor's runbook or manifest files.

### Dependencies / Assumptions

- The trust in CR16 to CR20, CR62, CR64, CR67, CR80 and CR81 rests on one platform control: a kill-on-refusal sandbox profile, enforced on macOS by `sandbox-exec`. The hardened profile passed a live check on test programs on 2026-11-22, with clean runs judged by their output.
- `sandbox-exec` is deprecated. It worked on macOS 27.2, and other versions, including CI's, are untested.
- A kill-on-attempt sandbox is assumed to work for a real printed CLI with a local SQLite store. No printed CLI has been run under it.
- Every default-scope dimension other than `side-effect-integrity` and `pii-secret-hygiene`, so including `idempotency`, is assumed to have an authored route to credit. A review of the routing tables supports this, and the specification's author confirms it dimension by dimension.
- A subject that reads no secret from a file has no route to a green run in version 1.0, because `pii-secret-hygiene` can be neither N/A nor credited.
- Someone prepares a seed for the real subject before phase 4. A seed made by the subject is only as independent as its author's check of it.
- The schedule assumes the owner is available for readings and ratification before 2026-12-19.
- No test, probe or subject run in this phase calls a model or a subject's live API.

### Outstanding Questions

**Resolve Before Planning**

None.

**Deferred to Planning**

Planning schedules each of these. Any that changes a verdict reaches the owner under CR58 before the seal.

- Which system and device paths must be allowed so that clean subjects are not killed, and can every kind of refusal be made to end the process?
- Which system services must be allowed for a real printed CLI to run, and does any of them delegate network or file access?
- Can a network kill be told from a child-process or confinement kill, so that `network_attempted` can ever be reported on macOS?
- Does binding or listening on a local port count as a network request?
- Which dimension does each of discovery, bare invocation, byte bound and declared exit codes feed?
- Which surface and which commands does the declared count in R42 count: groups, hidden commands, built-in help and completion?
- How does a subject file mark a command as free of writes, which is not the same as safe to run?
- How are database side files and first-open initialisation compared by the store-unchanged probe?
- Where does the store live by default?
- How large is the private set of cases, and is a catalogue of seeded collection mistakes kept with it?

### Sources / Research

- [Engine plan](2026-11-21-engine-plan.md): §3, phase 2, §7 and §8.
- [Requirements](../requirements.md): R10, R12, R16, R21, R22, R32, R33, R34, R42, AE8, AE24.
- [Rulebook](../specs/judge-1.0/rulebook.md): §3 scopes, §6 exclusions and credit, §12 anchoring, §13 closed lists, §14 what is kept from CDR spec 0.9.1.
- [Evidence record](../specs/judge-1.0/evidence-record.md), [judge contract](../specs/judge-1.0/judge-contract.md) and [errata](../specs/judge-1.0-errata.md), whose E-4 binds the engine.
- [A test's expected value must not come from the code under test](../solutions/best-practices/test-oracle-must-not-come-from-code-under-test.md).
- `check.sh` checks only judge 1.0's folder today, and `.github/workflows/ci.yml` has only Linux jobs.

Feasibility probes on 2026-11-21 and 2026-11-22 used throwaway test programs on macOS 27.2 as an ordinary user. They orient the spike and do not answer it.

| Probe | Result |
|---|---|
| Kill on any network operation or fork: nine network scenarios, 25 runs each | 225 of 225 killed |
| The same profile: two ways of starting a child | 50 of 50 killed |
| The same profile: two scenarios with no network or child | 50 of 50 finished normally |
| SIGSTOP as the signal for a network denial | No signal seen. Subjects exited 0 after their attempts, so it is unusable |
| Plain network denial, then searching the system log | No denial found, so a swallowed attempt left no trace |
| Killing on every system-daemon lookup, with no allow-list | Also killed a harmless current-user lookup |
| Shell pipeline under the subject's profile, then with the network blocked only | Killed, then worked |
| Computation denied the subject's binary | Direct run, symlink and copying refused. An existing copy or hardlink ran |
| File writes confined to a throwaway home, with a protected store | Outside write and protected read both refused |
| Kill on a refused write or protected read, with device paths allowed | 25 of 25 killed in each case, clean runs untouched |
| The same, without allowing device paths | Every run killed, clean ones included |
| Secret file as a tripwire in a control run | 25 of 25 readers killed, 0 of 25 non-readers |
| Inherited environment under the sandbox | The subject printed the inherited key in 25 of 25 runs |
| Refusing daemon lookups without a kill | A user lookup failed and the program still exited 0, so exit status alone does not show a clean run |
| A profile without process rules | A subject read another process's environment and killed another process |
| Hardened profile: 14 kinds of violation, 25 runs each | 350 of 350 killed |
| The same profile: file write, two kinds of user lookup, a Go CLI's help, SQLite in WAL mode, each judged by output | 125 of 125 correct |
| The same rules for a computation | Pipeline correct 25 of 25. Reading another process's environment and a daemon lookup were killed |
| A path spliced into profile text, and a seed link followed on copy | Each exposed a protected file. A path parameter and a link-preserving copy did not |
| Nested sandbox, broken profile, Go panic | Exit 71, 65 and 2, none of them from the subject's check |
| A plain read of a WAL database, process ended before close | Main file unchanged, two side files created |
| A 33-digit number, deep nesting or 17 MB in `observed` | The judge rejected the record by X12 |
| Linux | Not tested |

---

## Planning Contract

**Product Contract preservation:** changed: CR31 and CAE8 amended on 2026-11-22, after the spike's review found that the control run kills at open and the owner required both signals. CR67 amended on 2026-11-22, after the spike found two operations that cannot carry a kill and the owner chose to refuse both and state the limit. CR35 amended on 2026-11-22, after the spike showed that not every descendant can be ended and the owner chose the stated limit. Earlier: CR5, CR6, CR56, CR88 and CAE25 amended, CR113, CR114 and CAE32 added, and Linux taken out of 1.0, after the owner's answers to the review before publication. Earlier: CR31, CR55, CR99, CR100, CR102, CR107, CAE3 and CAE7 corrected, and CR109 to CR112 and CAE29 to CAE31 added, after the owner approved the plan review's fixes and settled local ports on 2026-11-22. Also one line under Dependencies / Assumptions, reworded to cover what it meant, the phase's tests, probes and subject runs. The owner decided on 2026-11-22 that agents' model calls are not tracked as part of the work. The Linux time box is the one earlier decision the owner reversed.

### Key Technical Decisions

**The spike**

- KTD1. **The profile denies by default and kills on refusal.** The surveyed sandboxes deny by default, and an allow-by-default profile cannot kill what it does not list. Each allowance is added for a named clean program that needs it. Implements CR62, CR67, CR80, CR83. (session-settled: user-approved — chosen over extending the allow-by-default profile of the feasibility probes: unlisted operations would pass unseen.)
- KTD2. **Spike code is throwaway.** It lives in `spike/confinement/`, serves the spike and the recording of the synthetic subject's behaviour, is never imported by `cmd/` or `internal/`, is not given to the phase 3 builder, and is deleted in the seal's pull request. The profile's authority is the specification's text. (session-settled: user-approved — chosen over keeping the harness: it would become a collector built before the corpus was reviewed.)
- KTD3. **The real subject runs by 2026-12-03 and the confinement rules freeze on 2026-12-04.** A later change to the rules re-records every behaviour that depends on them. Implements CR4, CR66. (session-settled: user-approved — chosen over running it any time before the seal: a late surprise would invalidate the recorded corpus.)
- KTD5. **Supported means tested.** A macOS version is supported when the spike's record names it as passed, and CR23 covers every other.
- KTD6. **A failed gate brings options within a day.** The owner gets the options and a recommendation, and only chapters that do not depend on confinement continue. Implements CR6, CR66. (session-settled: user-approved — chosen over stopping all work without a next step: the Product Contract names no choice for the owner at that point.)

**The specification and its corpus**

- KTD7. **The text lands as a draft, the seal comes last.** The specification is merged at its final path with a draft notice and no `KIT.sha256`. Reviews finish, the text freezes, the owner ratifies the manifest's digest, and only then is the manifest committed. An edit after ratification needs a new ratification. Implements CR60. (session-settled: user-approved — chosen over one pull request at the seal: the owner could not read the text apart from several hundred case files.)
- KTD8. **Rule ids begin with Q and a family letter.** The families are QS subject file, QH home and confinement, QO outcome, QD discovery, QP probes, QA authored checks, QE engine to judge and QT test entry. The judge uses single letters, and the citation test claims every bare R and AE number.
- KTD9. **Each case says what checks it.** Every case carries its input: subject file, request and scope, form or recorded behaviour, baseline and fixture descriptions. A case that yields a record is `judge-checked` from record to verdict. A case that ends before any record is `derivation-only`. The runner also counts the behaviour-to-record step of every case as unchecked, and never adds it to passes. Implements CR51, CR55. (session-settled: user-approved — chosen over one kind of case: the judge can confirm record to verdict and nothing before it.)
- KTD11. **The blind reader works from an export.** It holds the frozen text, the schemas, and each sampled case's input with its recorded behaviour, and no expected evidence, verdict, cited rule or worked example. The sample is drawn once, with the first frozen text's digest as the random seed, with at least one case per rule family and 20 in all. Later rounds keep that sample, add a case for any changed rule it lacks, and derive again only cases of changed rules. Implements CR104. (session-settled: user-approved — chosen over a reader in the repository: committed cases would give it the answers.)
- KTD12. **The private half is one case per rule and a catalogue of seeded collection mistakes.** It stays outside the repository, is checked through the judge locally, and the seal note publishes only its count and digest. Implements CR54. (session-settled: user-approved — chosen over an unsized set: a count and digest let the seal name what it was scored against.)
- KTD13. **The kit copies the judge kit's pattern and its runner.** `runner.py` and `common.py` are copied and extended, because one kit cannot import another's modules. The judge's folder is not touched.
- KTD14. **Fixtures that cannot be tracked are described and built at run time.** The case format describes links, special files, oversized output, invalid bytes and planted secrets. No symlink, secret-shaped string or home path is committed, and no gitleaks allowlist is added.
- KTD15. **The synthetic subject is standard-library Go under test data, one build per form.** A test builds every form explicitly, because the gate's wildcards skip test data. A form is fixed at build time, so no argument or variable reveals it to a check.
- KTD16. **The author of the text does not write the code.** The main session writes the specification. A coding agent writes spike programs, the synthetic subject and the runner, from briefs that hold nothing private. Someone else reviews each.
- KTD18. **The spike's tests run only when asked.** They carry the `confinementspike` build tag, so the gate neither runs a sandbox nor depends on the operating system. Violators and twins live under the spike's `testdata/`, because they ignore errors on purpose and `errcheck` would fail the gate. A run in which no scenario executed fails.
- KTD20. **A violator can reach only a decoy.** Whether confined, unconfined or under a deliberately broken profile, its file targets are in a temporary root, its process target is a victim the harness started, its network target is loopback or a `.invalid` name, and its secrets are random canaries whose values are never printed. Binds U1, U3, U4 and U7, and the specification's self-test.
- KTD21. **The real subject's name is checked mechanically.** Before each push, its identifiers, kept outside the repository, are searched for in tracked files, the commit range, the branch name and the pull request's text.
- KTD19. **A draft kit can be checked and says so.** The copied runner has a draft mode that needs no manifest, prints `UNSEALED`, and is refused once a manifest exists.
- KTD17. **The judge's pre-check is an ordinary call with no evidence.** It uses the run's own request kind and carries the baseline. Only an anchor's carries retirements and accepted regressions, because the judge rejects them on a grade. Where one rejection rule covers both causes, the engine first excludes its own by validating the store and its generated checks, so that a pre-check rejection is the user's. Only whether it rejects, and by which rule, is read. Implements CR71.

### High-Level Technical Design

**Order of work and its gates**

```mermaid
flowchart TB
  U1[U1 macOS spike] --> U3[U3 runner image and spike record]
  U3 -->|pass by 11-28| U4[U4 real subject by 12-03, freeze 12-04]
  U3 -->|pass by 11-28| U5[U5 rules for collection]
  U4 -->|confinement and routes only| U6
  U5 --> U6[U6 rules toward the judge]
  U6 -->|draft read by owner, 12-05| U7[U7 synthetic subject]
  U7 --> U8[U8 corpus by 12-12]
  U8 --> U9[U9 blind derivation and reviews]
  U9 -->|ratified digest| U10[U10 seal by 12-19]
```

**What produces and checks a case**

```mermaid
flowchart TB
  T[Specification text] --> E[Expected evidence, by the author]
  F[Synthetic form] --> H[Spike harness]
  H --> B[Recorded behaviour]
  B --> E
  E --> J[Real judge]
  J --> V[Stated verdict confirmed]
  T --> X[Export without cases]
  B --> X
  X --> D[Blind derivation]
  D --> C{Same evidence?}
  E --> C
  C -->|no| O[Ambiguity, to the owner]
```

**Where each part lives**

```mermaid
flowchart TB
  S[spike/confinement, deleted at seal] -->|behaviour| K[docs/specs/conformance-collection-1.0]
  G[testdata/collection-subject] -->|built forms| S
  T[internal/collectionkit, tests only] -->|builds and checks| G
  T -->|checks cases and schemas| K
  K -->|judge inputs| J[pql judge]
  C[check.sh] -->|digest and runner| K
  P[Private half, outside the repository] -->|judge inputs| J
```

**States of the specification folder**

```mermaid
stateDiagram-v2
  [*] --> Draft
  Draft --> Frozen: reviews and derivation have no open finding
  Frozen --> Draft: a reading or fix changes text
  Frozen --> Ratified: owner approves the digest
  Ratified --> Draft: any edit
  Ratified --> Sealed: manifest merged and CI verifies it
  Sealed --> [*]
```

**The spike's decisions**

```mermaid
flowchart TB
  A{macOS violators killed and twins correct?} -->|no on 11-28| O[Options to the owner]
  A -->|yes| B{Same on the runner image?}
  B -->|no| O
  B -->|yes| C{Real subject runs confined?}
  C -->|no| O
  C -->|yes| F[Freeze]
```

### Output Structure

```text
spike/confinement/                  deleted at the seal
  profile/  harness
  testdata/programs/
testdata/collection-subject/        one program, one build per form
internal/collectionkit/             tests only
docs/spikes/
  2026-11-28-confinement.md
docs/specs/
  conformance-collection-1.0/
    collection-rules.md
    collection-contract.md
    confinement.md
    credit-and-routes.md
    CASES.md
    schema/
    examples/
    behaviour/<form>.json
    cases/<id>/
    runner.py  common.py
    KIT.sha256                      added at the seal
  conformance-collection-1.0-errata.md
  conformance-collection-1.0-seal.md
```

### Where each deferred question is answered

| Question from the Product Contract | Answered in | Route |
|---|---|---|
| Paths and devices to allow, and whether every refusal can kill | U1, U4 | Measured, then frozen |
| System services a real CLI needs, and whether any delegates | U1, U4 | Measured, each allowance named |
| Can a network kill be told from other kills | U1 | Measured. If not, CR22 is stated unreachable on that system |
| Is a local bind or listen a network request | Settled | CR109 |
| The dimension each generated probe feeds | U6 | Proposed in the credit table, ratified |
| What the declared command count counts | U5 | Owner's reading under CR58 |
| How a command is declared free of writes | U5 | A field separate from the safety class |
| Database side files and first-open writes | U1, U5 | Measured, then a rule under CR101 |
| The store's default location | U6 | Chosen within CR48 and CR102 |
| The private half's size and catalogue | U8 | KTD12 |

### System-Wide Impact

- **CI:** a temporary macOS workflow for the spike, and a second kit in the `cases` job. The job names stay, because the ruleset requires them by name.
- **Citation test:** it walks every file under `docs/`, so the draft folder is checked from its first commit and gets a coverage floor.
- **Phase 3:** its builder receives the sealed folder and never the spike, the private half or the review briefs.
- **Phase 5:** the subject file keeps a versioned place for prompts, and the test entry is extended there.

### Risks

| Risk | Response |
|---|---|
| Deny by default needs allowances that reopen delegation | Each allowance gets its own violator. One that cannot be closed is a stated limit or a failed gate |
| The runner is macOS 26 and the developer machine 27.2 | Both are tested by 2026-11-28 and KTD5 states what is supported |
| A workflow started by hand must already be on `main` | The spike's workflow runs on its pull request and is deleted before that merge |
| A spike or test is silently skipped on Linux | macOS-only results come only from a macOS run, and a skip is reported as a skip |
| An operator agent's own sandbox stops nesting, so its grades are all not-measured | U3 records it, and the next action names a sandboxed parent |
| Inherited descriptors bypass the profile | A violator with an inherited descriptor is part of U1 |
| The real subject's build needs downloads | Stop and ask. The approval covers an offline build only |
| Review changes outdate earlier derivations | KTD7 freezes the text and KTD11 repeats only changed rules |

### Planning Assumptions

- `macos-26` is available free to this public repository and has Go 1.27.4. Whether the sandbox works there is what U3 finds out.
- The ruleset's required checks are still `gate`, `cases` and `scan`.
- The owner reads the draft within two days of 2026-12-05 and can ratify on 2026-12-19.

### Research that shaped these decisions

- [A clean control must assert on its output](../solutions/best-practices/clean-control-must-assert-on-output.md) shapes every spike scenario.
- [A test's expected value must not come from the code under test](../solutions/best-practices/test-oracle-must-not-come-from-code-under-test.md) shapes KTD2, KTD11 and KTD16.
- `docs/specs/judge-1.0/` is the kit pattern. Its `common.py` refuses symlinks and unlisted files.
- [GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) and [manually run workflows](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow) shape U3.
- [seccomp filters](https://www.kernel.org/doc/html/latest/userspace-api/seccomp_filter.html) cannot dereference pointers, so they never see a path. This is why CR5 leaves Linux out.
- Deny by default is the practice of the sandboxes in Chromium, Nix and current agent harnesses, which is the ground for KTD1.

---

## Implementation Units

The work ships as three pull requests. Each merge is the owner's.

| Unit | Title | Key paths | Depends on |
|---|---|---|---|
| U1 | macOS confinement spike | `spike/confinement/` | none |
| U3 | Runner image and spike record | `.github/workflows/`, `docs/spikes/` | U1 |
| U4 | Real subject run and freeze | `docs/spikes/` | U3 |
| U5 | Collection rules | `docs/specs/conformance-collection-1.0/` | U3, and U4 for confinement |
| U6 | Rules toward the judge, schemas and tables | `docs/specs/conformance-collection-1.0/` | U5 |
| U7 | Synthetic subject and behaviour | `testdata/collection-subject/` | U6 |
| U8 | Corpus, runner and private half | `docs/specs/conformance-collection-1.0/cases/` | U7 |
| U9 | Blind derivation and reviews | `docs/agent-reviews/` | U8 |
| U10 | Seal, CI and plan update | `check.sh`, `docs/specs/` | U9 |

### PR1: The spike, by 2026-11-28

Waits on: nothing.

### U1. macOS confinement spike

- **Goal:** Show on synthetic programs that each refusal is killed and each clean twin works.
- **Requirements:** CR1, CR2, CR3, CR16, CR17, CR18, CR19, CR31, CR35 to CR37, CR62 to CR64, CR67, CR74, CR75, CR78, CR80 to CR82, CR97, CR109, CR111. KTD1, KTD2.
- **Dependencies:** None.
- **Files:** Create `spike/confinement/` with the profile template, the harness, the programs under `spike/confinement/testdata/programs/` and `spike/confinement/confinement_darwin_test.go`.
- **Approach:**
  1. Start from a profile that refuses everything with a kill.
  2. Add an allowance only when a named clean program fails without it.
  3. Give every violator a clean twin that differs by that one operation.
  4. Build a second profile for computations from the same template.
  5. Read the wait status for the signal, and run each scenario 25 times.
  6. Use the system's SQLite for that clean program, so no dependency is added.
  7. Tag the tests as KTD18 says, and give every violator the targets KTD20 allows.
- **Execution note:** Write each violator and its twin before the allowance that concerns them, and watch the assertion fail under a broken profile.
- **Patterns to follow:** The plan's probe table, and the clean-control learning.
- **Test file:** `spike/confinement/confinement_darwin_test.go` catches a wrong profile on the machine it runs on. It proves nothing about Linux or an untested macOS.
- **Test scenarios:**
  - Covers CAE1. A TCP, UDP, HTTP, DNS, Unix-socket or local-bind attempt that ignores its error is killed before it prints its fallback.
  - Covers CAE2. A child start and a replaced process image that then uses the network are killed.
  - Covers CAE11 and CAE15. A read or write outside the home, including by a program that ignores the redirected home, is killed and changes nothing.
  - Covers CAE16 and CAE27. Reading another process's environment or signalling it is killed, each tested by its own route, and an inherited variable is absent.
  - Covers CAE19. A nested parent, an unparsable profile and a profile with its kill removed each fail the self-test and are never counted as a subject exit.
  - Covers CAE20. A path holding profile syntax grants nothing.
  - Covers CAE6. A computation's pipeline returns the right value, and its attempt to execute or read the subject fails.
  - A computation's `setsid` or `setpgid` is killed, a child left in its group is ended with it, and a child that left through `posix_spawn` is recorded as the limit and is still confined.
  - A lookup of each system service outside the allow-list is killed, from a subject and from a computation.
  - Covers CAE31. A loopback bind and a listen are each killed.
  - The self-test's programs fail only for the operation each tests, under the subject's profile and the computation's.
  - An inherited descriptor and an attempt at delegation through an allowed service are refused, or recorded as an open limit.
  - The read proof holds for a read, a mapping and a copy, and not for an open without a read, a stat or an attempt to set the access time.
  - File write, user lookup, Go help and SQLite with a write-ahead log each finish with the known output.
  - A plain database read records which side files it creates.
- **Verification:** Every violator is killed and every twin is correct in 25 of 25 runs, and deleting any one deny or kill makes a named test fail.

### U3. Runner image and spike record

- **Goal:** Answer the spike in the repository.
- **Requirements:** CR5, CR6, CR7, CR23, CR65, CR105. KTD5, KTD6.
- **Dependencies:** U1.
- **Files:** Create `docs/spikes/2026-11-28-confinement.md`. Create `.github/workflows/confinement-spike.yml` on the branch and delete it before merge.
- **Approach:**
  1. Run U1 once on a pinned macOS image from the pull request, without `gate.sh`. The workflow copies `ci.yml`'s posture: the `pull_request` trigger only, read-only contents, no persisted credentials, actions pinned by SHA, no secrets.
  2. Record versions, scenario counts, allowances, limits and the run's link, with Linux unsupported and CR5's reason.
  3. Delete the workflow, then report the checkpoint to the owner.
- **Test scenarios:**
  - The run fails when one violator's kill is removed.
  - The record passes `scan.sh` and holds no output, path or subject name.
- **Verification:** The record states pass or fail against CR1 to CR3 for each system tested.

### PR2: The draft specification, by 2026-12-05

Waits on: PR1, needed before this group can start, because its rules rest on the recorded spike.

This is separate because the owner must read and ratify the text, and one pull request with the code and several hundred cases is too large to read well.

### U4. Real subject run and freeze

- **Goal:** Learn what a real printed CLI needs, then freeze the confinement rules.
- **Requirements:** CR4, CR66. KTD3, KTD6.
- **Dependencies:** U3.
- **Files:** Modify `docs/spikes/2026-11-28-confinement.md`.
- **Approach:**
  1. Build from a scratch copy with an allow-listed environment, a throwaway home, the module proxy and checksum lookups off, the local toolchain only and the module cache read-only.
  2. Pass the self-test before every run, including each run after a new allowance.
  3. Run with no network, key or real store, in a throwaway home.
  4. Add an allowance only with a new violator and twin in U1's suite.
  5. Run U1 again on the runner image under the final rules, with U3's workflow restored on the branch and deleted before merge.
  6. Record counts and allowances, then freeze on 2026-12-04.
- **Execution note:** The owner's approval covers this one offline build and run. A download, key or network need stops the unit.
- **Test scenarios:**
  - Help and at least one declared offline read return output judged correct against a known seed.
  - A live-first read without offline settings is killed.
  - After any new allowance, all of U1 still passes on both tested macOS systems.
  - The subject's library checkout and the user's real state are unchanged.
- **Verification:** The record marks the rules frozen without naming the subject or holding its output.

### U5. Collection rules

- **Goal:** Write the rules from the subject file to each observation.
- **Requirements:** CF2. CR8 to CR41, CR58, CR62 to CR64, CR67, CR68, CR70, CR74 to CR78, CR80 to CR87, CR98, CR100, CR101, CR107, CR109 to CR113. KTD1, KTD8.
- **Dependencies:** U3. Only `confinement.md` waits for U4's freeze, so the rest is drafted beside U4 from 2026-11-28.
- **Files:** Create `collection-rules.md` and `confinement.md` in `docs/specs/conformance-collection-1.0/`.
- **Approach:**
  1. Give every requirement above a rule id and closed lists, with the order in which rules are tried.
  2. State when each control runs and what its absence yields: self-test, read proof, comparator and timeout.
  3. Map a failed computation confinement as well as a subject's.
  4. Say which required check carries a command found only by discovery.
  5. Take each two-way reading to the owner before writing it as settled.
- **Patterns to follow:** The opening paragraphs, rule lists and closed lists of `docs/specs/judge-1.0/rulebook.md`.
- **Test expectation:** none -- specification text. U8 gives each rule a case and U9 tests the text by blind derivation.
- **Verification:** Every cited requirement maps to a rule, every reason is in the rulebook's §13 with a next action, and the profile needs nothing from the spike's code.

### U6. Rules toward the judge, schemas and tables

- **Goal:** Write how observations become a record, an exit code and stored state.
- **Requirements:** CF2. CR42 to CR49, CR59, CR69, CR71 to CR73, CR79, CR88 to CR92, CR94, CR99, CR102, CR103, CR108, CR114. KTD7, KTD8, KTD17.
- **Dependencies:** U5, drafted alongside it. Only the routes that depend on an allowance wait for U4.
- **Files:** Create `collection-contract.md`, `credit-and-routes.md`, `schema/` and `examples/` in the new folder, and `docs/specs/conformance-collection-1.0-errata.md`. Create `runner.py` and `common.py` in the new folder, `internal/collectionkit/doc.go` and `internal/collectionkit/schema_test.go`. Modify `check.sh`, `internal/docrefs/docrefs_test.go` and `docs/specs/README.md`.
- **Approach:**
  1. Place every engine ending in the closed exit table, including an unreadable subject file and an unwritable store.
  2. Assign each clause of each judge rejection rule to user or engine, as KTD17 separates them.
  3. Give the 18 dimensions of the conformance scopes a route, the independent expected value and who has no route, and list `cli-mcp-parity` as in no scope.
  4. Give each false-pass and false-fail route a positive control.
  5. Define cleanup after the engine itself is killed.
  6. Mark the folder a draft in the specifications' README.
  7. Copy the runner and give it KTD19's draft mode and a check of schemas against examples, using the validator it already pins.
  8. Run that draft mode from `check.sh`, so the `cases` job, which has uv, tests the runner and the Go gate never calls Python.
- **Test file:** `internal/collectionkit/schema_test.go` catches drift between schemas, examples and the judge's enumerations. It cannot tell whether a rule is right. The runner's own scenarios run in `check.sh`.
- **Test scenarios:**
  - The runner rejects an unknown field, a wrong type and a missing required field at every schema location.
  - The runner's draft mode prints `UNSEALED` and refuses to run once a manifest exists.
  - Every reason, failure kind and tier in the new schemas is in the judge's input schema, read from that file.
  - Covers CAE30. Two probes on one command give two ids in the example required set.
  - Each example record is accepted by the real judge and yields its stated exit code.
  - Covers CAE13 and CAE18. The exit table gives 70, 64 and 2 for those three inputs and holds no path to 1 except a judged red.
  - The citation test fails on an undefined requirement number planted in the new folder.
- **Verification:** The owner has the draft to read, with each open reading listed with both readings.

### PR3: Corpus and seal, by 2026-12-19

Waits on: PR2, merge order only.

This is separate because ratification reviews a large corpus and a digest, which one reviewer cannot read well together with the specification's first draft.

### U7. Synthetic subject and behaviour

- **Goal:** Build the known-good subject and its defect forms, and record what each does.
- **Requirements:** CR50, CR52. KTD2, KTD15, KTD16.
- **Dependencies:** U6.
- **Files:** Create `testdata/collection-subject/`, `internal/collectionkit/subject_test.go` and `behaviour/<form>.json` in the new folder.
- **Approach:** Derive one defect form per named check from the credit table. Run every form directly, against KTD20's decoys, and under the frozen profile through the spike's harness, and store normalised behaviour with no path.
- **Execution note:** Write each form's failing behaviour assertion before the form.
- **Test file:** `internal/collectionkit/subject_test.go` catches a form that does not build or does not show its defect. It does not test a collector. In the gate it builds every form and runs only those that attempt nothing outside their temporary directory. Forms that violate run only in the tagged spike tests.
- **Test scenarios:**
  - The known-good form gives the known output for a route to each of the 17 default dimensions and refuses its destructive command.
  - Covers CAE4 and CAE5. One form reports 9 of 10 from two commands, and another reports 10 and 9.
  - Covers CAE3. Forms that kill themselves, crash by another signal and overrun end in those three distinct ways. In Go the second sets the traceback mode to crash, because a plain fault exits 2 with no signal.
  - Covers CAE7 and CAE26. Forms write only a journal, rewrite the store on a read, and leak the secret plain and encoded.
  - Covers CAE10. A live-first form attempts the network and its offline invocation does not.
  - Covers CAE32. A form writes the secret past the scan's bound.
  - Covers CAE29. A form leaves a link to a decoy in its home.
  - Covers CAE9 and CAE28. Forms hide, rename and add a command without changing the count.
  - Every defect form differs from the known-good form in its named behaviour and no other.
- **Verification:** Every form builds in the gate, and a recorded behaviour exists for each on macOS.

### U8. Corpus, runner and private half

- **Goal:** Give every rule a case and check what can be checked before a collector exists.
- **Requirements:** CR51, CR53 to CR55. KTD9, KTD12, KTD13, KTD14.
- **Dependencies:** U7.
- **Files:** Create `cases/` and `CASES.md` in the new folder and `internal/collectionkit/cases_test.go`. Modify the new folder's `runner.py` and `common.py`.
- **Approach:**
  1. Write expected evidence from the text and the recorded behaviour.
  2. Give each case its input as KTD9 says, and tag its rules, kind, entry and twin.
  3. Confirm every judge-checked case through the real judge.
  4. Write the private half outside the repository and check it the same way.
- **Test file:** `internal/collectionkit/cases_test.go` catches an uncovered rule, a missing twin and a wrong record-to-verdict claim. It cannot check behaviour to record.
- **Test scenarios:**
  - A rule id in the text with no case fails, as does a case naming an unknown rule.
  - A not-measured case without a twin that grades green fails.
  - The known-good record exits 0 in the default scope and with the destructive scope.
  - Each defect's record is red on its named check.
  - Covers CAE8, CAE12, CAE17 and CAE22. Those records give the stated not-measured results through the judge.
  - Covers CAE14. A scripted anchor is refused as a test run, and every other anchor case, refusals included, is tagged for the real verifier.
  - Covers CAE21, CAE23, CAE24 and CAE25. Each has a case of the kind that can check it.
  - A derivation-only case is never counted as passed, and a skip set that differs from the declared one fails.
  - Changing one expected exit code makes the runner fail.
- **Verification:** Every rule and all 32 acceptance examples have cases, and the two counts are reported apart.

### U9. Blind derivation and reviews

- **Goal:** Show that a reader with only the text reaches the same evidence.
- **Requirements:** CR58 to CR60, CR104. KTD11, KTD16.
- **Dependencies:** U8.
- **Files:** Create a dated log in `docs/agent-reviews/`. Modify the new folder's `runner.py` with the export and sampling, whose scenarios run in `check.sh`.
- **Approach:**
  1. Freeze the text and export it for the blind reader on a different model.
  2. Route each disagreement to the owner with both readings.
  3. Run the standards, different-model and corpus reviews.
  4. Verify each finding before acting, and cite private cases by rule id only.
- **Test scenarios:**
  - The export holds the sampled cases' inputs and no expected evidence, verdict, cited rule or worked example.
  - The same digest draws the same sample.
  - A changed rule selects exactly the cases that cite it for another derivation.
- **Verification:** The last derivation round has no disagreement and the reviews have no open Blocker or Major.

### U10. Seal, CI and plan update

- **Goal:** Seal what the owner ratified and leave nothing of the spike behind.
- **Requirements:** CF1. CR56, CR57, CR60, CR61, CR106. KTD2, KTD7, KTD12.
- **Dependencies:** U9.
- **Files:** Create `KIT.sha256` in the new folder and `docs/specs/conformance-collection-1.0-seal.md`. Modify `check.sh`, `.github/workflows/ci.yml` if the job needs it, `docs/specs/README.md`, `docs/plans/2026-11-21-engine-plan.md` and the stale `cases` sentence in `AGENTS.md`. Delete `spike/confinement/`.
- **Approach:**
  1. Put the manifest's digest to the owner.
  2. Commit the manifest after ratification, with the private half's count and digest in the seal note.
  3. Make `check.sh` verify both kits and run both runners.
  4. Update the engine plan as CR61 says, with phase 3's macOS job.
- **Test scenarios:**
  - Changing, adding or removing one file in the sealed folder fails `check.sh`.
  - Judge 1.0's digest still verifies and all 383 of its cases pass.
  - No file imports or names the deleted spike as a source.
- **Verification:** CI is green on the pull request and the sealed digest is the one the owner ratified.

---

## Verification Contract

| Gate | Command or evidence | Applies to | Done when |
|---|---|---|---|
| Go gate | `./gate.sh` | every pull request | exits 0 with go1.27.4 |
| Kits and cases | `./check.sh` | every pull request | judge 1.0 passes 383 of 383, and from U10 the collection kit verifies |
| Secrets and paths | `./scan.sh` on tracked files and the commit range | every pull request | exits 0 with no allowlist |
| macOS spike | `go test -count=1 -tags confinementspike ./spike/confinement/...` on each tested macOS | U1, U3, U4 | 25 of 25 in every scenario, judged by output |
| Subject's name | KTD21's search | every push | no hit |
| Sealed judge | `shasum -a 256 -c KIT.sha256` in `docs/specs/judge-1.0` | every pull request | unchanged |
| Collection cases | `uv run runner.py` in the new folder, against the built judge, from `check.sh`, in draft mode until U10 | U6 to U10 | every judge-checked case passes and the other count is shown apart |
| Mutation by hand | break the guarded rule in a scratch copy | every test added | the named test fails |
| Review | a reviewer who did not write it, and one different model for the specification | each pull request | no open Blocker or Major |
| CI | the pull request's own `gate`, `cases` and `scan` checks | every merge | green |

---

## Definition of Done

- The Product Contract's Success Criteria hold.
- U1 and U3 to U10 each meet their Verification line.
- The spike record says pass or fail for each system, and the real subject's run is recorded without its name or output.
- The sealed folder's digest is the ratified one, and CI verifies it beside judge 1.0.
- Every rule and acceptance example has a case, and no derivation-only case is reported as passed.
- The private half's count and digest are in the seal note, and none of its content is in the repository.
- The engine plan is updated and names no real subject.
- `spike/confinement/`, the temporary workflow and any abandoned attempt are gone from the tree.
- Each lesson that corrected a wrong belief went through `ce-compound` in the pull request that produced it.
