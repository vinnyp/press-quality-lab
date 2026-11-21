# Engine plan: from the judge to the first real grade

**Date:** 2026-11-21. **Status:** draft for the owner's approval. The milestone, deadline and command name in it are the owner's decisions of 2026-11-21. **Language:** Go, chosen by a trial of six independent judge builds, three in Go and three in Rust.

## 1. Goal and stop rule

The goal is a measurement engine for agent-facing command-line tools that a person can trust: green means everything the run was asked to measure was measured and held, red names a real defect, and anything else says what was not measured and what to do next.

This plan covers the work up to one milestone. The owner set the rule on 2026-11-21:

- **Milestone.** Both of these are true:
  1. The Go judge passes every case of the version 1.0 judge specification, the published cases and the privately kept ones.
  2. The engine grades one real printed CLI end to end, in two scopes: conformance, offline with no model call; and steering, with live trials against a model. In each: subject file in, evidence collected, the judge called, a scorecard and an exit code out.
- **Deadline.** 2027-03-31, the end of the first quarter. The owner first set two months and extended it when steering was confirmed as part of the milestone.
- **If the milestone is not reached by then,** the effort stops. This repository is left as it stands with a note saying so, and the predecessor engine carries on unchanged.

**Early warning.** On 2027-01-25, halfway, the owner is told plainly whether the milestone is on course. The test is in §6.

## 2. What already exists

- **The judge specification, version 1.0,** sealed on 2026-11-19, in `docs/specs/judge-1.0/`: the rulebook, the judge contract, the input and output schemas, a prose description of the evidence record, 383 cases and a runner. More cases and a catalogue of seeded rule-level mistakes are kept privately and used only to score a build. A build never sees them.
- **A scoring harness,** kept privately with those cases. It runs every case twice against a judge binary, classifies each wrong output as a false green, a false red or other, and measures whether a build's own tests notice each catalogued mistake.
- **A Go quality gate:** build, `vet`, `staticcheck`, `exhaustive` and `errcheck` at pinned versions, with the network off.
- **What the trial taught.** All six builds passed every case they were given on the first attempt. All six were wrong at the JSON boundary, in different ways, and nowhere else. Neither language's toolchain stopped any catalogued mistake at compile time. The specification, not the build, took the time: each outside review of it found real problems that same-family checks had agreed were absent.

Nothing is ported from the predecessor engine. Its code is reference only and its outputs are not an oracle.

## 3. What is not yet specified

Version 1.0 specifies the judge and nothing else. The judge takes a finished evidence record. Everything that produces that record is undecided, and none of it can be built well until it is written down:

- the subject file: its format, its placeholders, and how a command is declared safe, destructive or unclassified;
- check identity: how a check gets an id that is stable across runs;
- authored checks: the runbook format, ordered steps and time budgets, and how a computed expected value is supplied;
- generated checks: which probes exist, how commands are discovered, and which dimension each probe feeds;
- how the required set is fixed before collection;
- how the evidence fields the judge relies on are produced: target size, observed precondition, tier, evaluator;
- how the engine verifies that the subject made no network request, on each supported system;
- the store: where run records and baselines live and in what form;
- the engine's own failures: what the grade command does when the judge rejects the record the engine itself built.

And for steering:

- how prompts are authored and named in the subject file, and the fixed framing text sent with each;
- which harness drives the agent under test, who supplies its credentials, and how the model and its sampling settings are recorded;
- how a condition digest is computed from the subject's surface, the model, the prompt, the framing, the declared count and the rulebook version;
- how a trial's first relevant tool call is found, and how a failure of the harness is told apart from a failure of the agent;
- how declared counts, trial indexes and replacement of errored trials are managed across runs, and how every trial is stored;
- how cost is stated before a run, approved, and capped for an unattended run;
- the scripted agent used for tests, and how released builds are kept from exposing it.

These are two **collection specifications**, one for conformance (phase 2) and one for steering (phase 5). They are the largest risk in the plan (§7).

## 4. Shape of the engine

One Go module, `github.com/vinnyp/press-quality-lab`, and one binary. The command is `pql`.

| Package | Does | Does not |
|---|---|---|
| `internal/judge` | Turns input bytes into output bytes and an exit code, by the rulebook. Pure: no file, clock, environment or network. | Collect anything. |
| `internal/record` | The evidence record's types and the one builder the engine uses to write it. | Decide any outcome above a single check. |
| `internal/subject` | Reads and creates the subject file; reports unfilled placeholders. | Run anything. |
| `internal/collect` | Runs authored and generated checks against the subject, and steering trials against the agent under test, and reports per check or trial what it saw. | Decide requiredness, routing, rollups, comparison or verdicts. Those are the judge's. |
| `internal/store` | Keeps run records and baselines. | Hold state that a family can use to change a verdict. |
| `internal/scorecard` | Renders the judge's output for a person. | Recompute anything. |
| `cmd/pql` | `judge` (standard input to standard output, the contract's form), `create`, `grade`, `anchor`. | |

Three rules hold the shape together:

1. **The grade command calls the judge through the same bytes-in, bytes-out function the `judge` subcommand exposes.** The exit code, the scorecard and the stored record are all read from that one output document. There is no second place where a verdict is decided.
2. **The judge owns its own JSON reading and writing.** Go's standard decoder does not report repeated keys, does not keep a number's written form, and has no depth limit of the kind the contract sets. The judge has a small strict reader for the contract's input rules and a writer for its exact output form. Number comparison is exact decimal, as the rulebook requires, with no floating-point step.
3. **Every closed list in the rulebook is a Go type with named constants,** so the `exhaustive` check in the gate flags a `switch` that misses one.

Cobra (`github.com/spf13/cobra`) is the approved command framework, under `cmd/` only. Everything else is kept to the standard library unless the collection specification shows a need. The gate runs with the network off, so every dependency is a deliberate, reviewed addition.

## 5. Phases

Items in each phase are numbered and cited as "phase.item": 0.2 is the second item of phase 0. The slices in the tables of phases 3 and 6 are cited by their letters.

### Phase 0: foundation

**Status:** done. Its last item, 0.2, landed with the pull request that added `docs/requirements.md`.

1. This repository, local first: licence, the published specification, this plan.
2. The requirements this engine answers are brought across from the predecessor project, edited for this repository, so that the requirement and acceptance-example numbers the rulebook cites can be read here. They are in [`docs/requirements.md`](../requirements.md).
3. The Go gate, a script that runs the published cases against the built judge, and a scan for secrets and personal paths. One CI workflow runs all three on every pull request.
4. Changes land by pull request only. A ruleset on `main` requires a pull request, the `gate` and `scan` checks, and an up-to-date branch; the `cases` check becomes required when the judge turns it green.

**Done when:** the repository is published; the `gate` and `scan` checks are green and required on `main`; the `cases` check is red only because the judge is not built; and the requirements can be read in this repository.

### Phase 1: the judge

1. Built by a coding agent from a written brief, given only the published specification. The brief says to implement the rules for every input the schemas allow, not only for the cases given, and to write tests of the rules themselves.
2. CI runs the build's own tests and all 383 published cases.
3. Scored privately against every case, published and private. The bar: zero wrong outputs.
4. Measured privately against the catalogue of seeded mistakes. The bar: no catalogued mistake survives CI. A survivor is fed back to the builder as the rule id whose tests are missing, never as the private input that would catch it.
5. Reviewed twice by reviewers who did not build it, one reading the code against the rulebook and one judging whether the tests prove the rules, and once by a different model.
6. Any input found on which the judge is wrong and the specification is clear becomes a new private case. Any input on which the specification is unclear goes to the owner, because settling it may mean a version 1.1.

**Done when:** part 1 of the milestone holds, and the reviews have no open blocker.

### Phase 2: the conformance collection specification

1. A short spike first, in week 1: can the engine verify, on macOS and on Linux, that a child process made no network request? The requirements say a check is not measured when this cannot be verified, so without a mechanism every offline grade would be not-measured.
2. The specification of the conformance items in §3, written by the plan's author with the owner's decisions, in the same form as the judge specification: stable rule ids, closed lists, worked cases.
3. A synthetic subject: a small CLI kept in this repository's test data, in a known-good form and in forms each carrying one seeded defect.
4. A collection corpus built on it, two-sided as the requirements demand: every not-measured case has a repaired twin that grades green, the known-good subject grades green, and each seeded defect grades red on the named check.
5. Reviewed by a standards reviewer and by a different model, then ratified by the owner and sealed as `docs/specs/conformance-collection-1.0/`.

**Done when:** the specification is sealed and the synthetic subject and its corpus exist and are reviewed.

### Phase 3: the grade command

Built in slices. Each slice lands with its corpus cases passing through the real `grade` command, and each is a separate pull request.

| Slice | Delivers | Proved by (requirements' acceptance examples) |
|---|---|---|
| a | Subject file, `create`, placeholders | AE20 |
| b | Command discovery and generated probes on the synthetic subject | AE8, AE27, AE30 |
| c | Authored checks, ordered runbooks, time budgets, computed expected values | AE22, AE23 |
| d | Safety classes and the no-network check | AE9, AE24 |
| e | The store; saving, comparing and re-anchoring a baseline | AE16 to AE19, AE31, AE33 |
| f | The scorecard, and agreement of exit code, scorecard and stored record | AE21 |

**Done when:** every collection-corpus case passes through `grade`, and the known-good synthetic subject is green and exits zero.

### Phase 4: the first real conformance grade

1. One real printed CLI, chosen by the owner. Its subject file stays outside this repository.
2. Graded in the default scope.
3. Every red is checked by hand: is it a real defect in the CLI? Every not-measured reason's next action is followed: does it turn into a measurement?
4. What the real subject shows the specification got wrong is recorded as findings. A fix that changes a verdict is a new specification version, not an edit.

**Done when:** the real CLI has a conformance scorecard, at least one dimension is judged green or red from generated checks alone, and no red has been shown to be the engine's mistake.

### Phase 5: the steering collection specification

1. The steering items in §3, specified in the same form and reviewed the same way, with the owner's decisions on the harness, the model, the credentials and the unit of cost.
2. A scripted agent behind the same interface as the live one, reachable only through a test entry to the grade command that released builds reject.
3. A steering collection corpus run through that entry: the prompt sent is the authored prompt plus the framing and nothing else; scripted trial sequences reach each of the rulebook's results; errored trials are replaced; pools change when a condition changes.
4. Ratified by the owner and sealed as `docs/specs/steering-collection-1.0/`.

**Done when:** the specification is sealed and its corpus exists and is reviewed.

### Phase 6: steering in the grade command

| Slice | Delivers | Proved by |
|---|---|---|
| g | Prompts in the subject file, the framing, the scripted agent and the test entry | AE11, AE35 |
| h | Trials, declared counts, replacement, storage and pooling by condition | AE12 to AE14, AE29, AE36 |
| i | Two-arm comparison of two versions of the subject | AE15 |
| j | Cost statement, approval and the budget cap | AE10, AE25 |
| k | The live harness behind the agent interface | a counted, capped smoke run |

**Done when:** every steering-corpus case passes through the test entry, and a released build rejects that entry.

### Phase 7: the first real steering grade

1. The same real CLI, with prompts the owner has read.
2. The planned trial count and expected cost are stated and the owner approves them before the first live trial. Nothing in this plan spends on a model before that.
3. A sample of trials is read by hand: did the agent make the call the record says, and is each errored trial the harness's failure?

**Done when:** part 2 of the milestone holds in both scopes.

### After the milestone, outside this plan

An operator agent grading an unseen CLI from the documentation alone; a third family; moving the real fleet.

## 6. Schedule

These are estimates. The trial showed that a build takes hours and a specification takes days, so the dates are driven by the two collection specifications and the two real grades.

| By | Reached |
|---|---|
| 2026-11-28 | Phase 0 done. Judge built and scored. Network spike answered. |
| 2026-12-05 | Phase 1 done: milestone part 1. |
| 2026-12-19 | Phase 2 done. |
| 2027-01-16 | Phase 3 done. |
| 2027-01-25 | **Halfway check.** On course if phase 3 is done and the real CLI's first conformance grade has run. |
| 2027-01-30 | Phase 4 done. |
| 2027-02-13 | Phase 5 done. |
| 2027-03-06 | Phase 6 done. |
| 2027-03-17 | Phase 7 done: the milestone. |
| 2027-03-31 | Deadline. Two weeks of slack. |

## 7. Risks

| Risk | What happens | Response |
|---|---|---|
| The no-network check cannot be made on one system | Offline grades there are all not-measured | Spike in week 1. If only one system works, the milestone is reached on that system and the other is stated as unsupported. |
| The collection specification is wrong in ways only a real CLI shows | Phase 4 turns into specification work | Phase 4 has two weeks. The real subject is looked at during phase 2, read-only, to find surprises early. |
| A real subject exposes a gap in judge 1.0 | A version 1.1, a re-seal, and every case re-derived where it changes | Version policy already written. Settle with the owner the same day; do not work around it in the engine. |
| The collection build is weaker than the judge build, because its corpus is new | Wrong evidence that the judge then grades faithfully | The corpus and synthetic subject are finished and reviewed before any collection code is written. |
| Private cases leak into builds through repeated feedback | The private half stops being an independent check | Feedback names a rule id only. New private cases are added from review findings, so the set grows. |
| Live steering costs more than expected | The owner declines the run, or the cap stops it short and the dimension is not-measured | The rulebook's minimum trial count and the number of prompts fix the cost in advance. It is put to the owner in phase 5, before anything is built around it. |
| A harness failure is scored as the agent's failure, or the reverse | A steering verdict that is wrong in either direction | The distinction is specified in phase 5 with cases for each, and a sample of live trials is read by hand in phase 7. |
| Steering starts late because conformance overruns | The two weeks of slack are spent before phase 5 | The halfway check. If phase 4 is not done by 2027-02-06 the owner is told the deadline is at risk, with what could be cut. |
| Same-model reviews agree and are wrong | A defect ships that every check passed | Every specification and every phase gets one different-model review. |

## 8. Decisions for the owner

Settled on 2026-11-21: the repository, its licence, where specifications live, the milestone and its deadline, and the command name `pql`.

Still to come, each put with a recommendation at the phase named:

1. Phase 2: which real CLI is the subject of phases 4 and 7.
2. Phase 2: the subject file's format.
3. Phase 2: how a computed expected value is supplied, and with it whether the engine must be a single binary with no other runtime.
4. Phase 5: the harness that drives the agent under test, the model, and who supplies credentials.
5. Phase 5: the unit cost is stated in, and the budget for the first live run.
6. After the milestone: whether the engine also ships as a Claude Code plugin.
