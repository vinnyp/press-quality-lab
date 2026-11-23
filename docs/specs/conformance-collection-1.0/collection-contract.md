# Collection contract

**Draft.** This folder is a draft of the conformance collection specification, version 1.0. It is not sealed, and it holds no `KIT.sha256`.

**What this file covers.** How the engine turns its observations into the judge's input, what it does with the judge's output, which exit code it ends with, and how it keeps and cleans up what it ran. The rules are families QE and QT. How to read the rules, and what binds, is as `collection-rules.md` says. The judge decides every verdict; nothing here decides one.

## 1. Version

- **QE1. Version and change.** This specification's version string is `1.0`. The string changes when, and only when, some subject file, scope and subject behaviour can lead to a different record for the judge or a different engine exit code. Any other change is editorial and goes in the dated errata note `docs/specs/conformance-collection-1.0-errata.md`, outside the sealed folder. The owner ratifies each version.

## 2. The required set and its ids

- **QE2. Check ids.** A check's id is the lowercase hexadecimal SHA-256 of a canonical JSON object holding, and holding nothing else:
  - the check's kind, dimension, predicate and its parameters;
  - the invocation: the command words, the offline arguments applied, and the names and values of the declared environment;
  - the ids of the earlier steps of its runbook, in order;
  - the seed's digest;
  - the bookkeeping exclusions in force;
  - the expected value's source: the literal, or the computation's argument list and the digest of its executable and declared files;
  - this specification's version string.

  The canonical form sorts keys, writes no whitespace and writes each string with only the escapes JSON requires. The binary's own digest is not part of the id, so a changed binary keeps its checks' ids, and the judge can compare it with a baseline. Two probes on one command therefore get two ids.
- **QE3. A fixed set.** The engine fixes the required set from the subject file, the judge's rulebook and the requested scope before discovery or any other check runs (R21). Evidence never adds to it or removes from it.
- **QE4. Missing ids.** When a baseline holds ids the new required set lacks, the engine lists each with the cause it can name: the command or step removed, or the part of the id that changed. It prints the list in the form `pql anchor` accepts as its retired list, and retires none by itself.

## 3. Before collection

- **QE5. The pre-check.** Before any check runs, the engine validates the subject file (§1 of `collection-rules.md`) and its own generated checks, then calls the judge once with the run's own request kind, the required set and no evidence. A grade's pre-check carries the baseline. An anchor's also carries the retired checks and accepted regressions. A rejection here stops the run with the exit code QE16 gives, before any check runs.
- **QE6. Who caused a rejection.** The engine validates its own generated checks first, so a pre-check rejection is the user's except where the table says otherwise. A rejection the engine caused exits 70.

  | Rule | Cause |
  |---|---|
  | X1 | user: the requested scope |
  | X2, X3 | engine: no evidence is sent in the pre-check, so either is an engine fault |
  | X4 | user: a marker in the subject file on a Safety or Steering dimension, on `cli-mcp-parity`, or on a record with no crediting check there |
  | X5 | engine for a generated check; user for an authored check the file shapes this way |
  | X6 to X9 | engine: no trial is sent in this version |
  | X10 | engine: the engine composes ids and routes |
  | X11 | user: the retired list, accepted regressions or the baseline named |
  | X12 | engine: the engine writes the document |

## 4. Evidence

- **QE7. Fields of a required check.** For each check the engine sets `record` to the runbook that holds it, or `null` for a generated probe; `crediting` from §2 of `credit-and-routes.md` and QA3; `empty_satisfiable` when an empty target would satisfy the predicate; `asserts_emptiness` and `precondition` when the predicate asserts that something is empty, naming what must be observed to exist first.
- **QE8. Fields of a piece of evidence.** For each observed check the engine sets:
  - `outcome`, `reason` and `failure_kind` by §2 of `collection-rules.md` and the check's predicate;
  - `target_size` for an empty-satisfiable check, as the number of items the predicate evaluated;
  - `precondition_observed` for a check that asserts emptiness;
  - `expected` and `observed`, bounded as QE10 says;
  - `targets`: each invocation by its check id, and each artifact by its path relative to the home;
  - `evaluator` as `deterministic`.
- **QE9. Tiers.** A normal run claims `live`. Each target that ran with an offline setting carries the tier `offline`, and every other target `live`. Every check that is not observed still reaches the judge with its reason and at least one target: the invocation it would have used.
- **QE10. Bounds.** Captured output is drained as it is produced, so the subject is never blocked by a full pipe. Each stream is kept up to 1 MiB; past that the engine stops keeping it, records that it was cut, and keeps draining. Before any of it enters the record, it is decoded as UTF-8 with each ill-formed byte replaced by U+FFFD. A literal from the subject file is bounded at 64 KiB. These bounds keep every record inside the judge contract's §3, whatever a user or a subject supplies.
- **QE11. Markers.** Each expected-failure marker in the subject file reaches the judge as given, naming its runbook as the record and its dimension.

## 5. Next actions

- **QE12. Every reason has a next action.** The scorecard shows, beside each check that is not observed, the next action for its reason:

  | Reason | Next action |
  |---|---|
  | `network_unverifiable` | The confinement ended the subject. Possible causes: a network request, a child process, another refused operation, or an external kill. Check that the command has an offline setting that keeps it off the network, and that it starts no child process. On an unsupported system, grade on one QH1 names. |
  | `could_not_start` | Check that the binary the subject file names is an executable for this machine. |
  | `input_missing` | For the planted secret: name, in the subject file, the file the subject reads its secret from, and check that the command under test reads it. For a check without its input: supply what the check needs. |
  | `precondition_unmet` | For a generated check: discovery did not find every declared command, so fix the declared list or the binary. Otherwise: seed the data the check needs to exist. |
  | `empty_target` | Seed data the check can evaluate, or declare the bound or the exit codes the probe needs. |
  | `placeholder_unfilled` | Fill each placeholder the scorecard names. |
  | `computation_failed` | Run the computation by hand on the pristine seed and make it print exactly one JSON value. |
  | `earlier_step_failed` | Fix the first step of the runbook that was not held. |
  | `harness_fault` | Rerun. If it repeats, report it with the stored record: the engine or the host failed, not the subject. |
  | `no_evidence` | Report it with the stored record: the engine fixed a check it did not observe. |
  | `scope_not_requested` | Request a scope that includes the dimension. |
- **QE13. `network_attempted`.** This version never reports it (QO3).

## 6. Exit codes

- **QE14. What the judge decides.** For every record the judge decided, the engine's exit code is the judge's, and every verdict, state and count it shows is read only from the judge's output. The scorecard and the stored record add the evidence and the run's details the engine recorded.
- **QE15. A rejected record.** When the judge rejects the engine's record after collection, the engine exits 70, shows no verdict, and keeps the rejected record for diagnosis.
- **QE16. The closed table.** The engine exits with exactly one of these codes.

  | Code | When |
  |---|---|
  | 0 | the judge's grade is PASSED, or the anchor was made |
  | 1 | the judge's grade is FAILED; nothing else exits 1 |
  | 2 | a well-formed request is rejected for its content or scope: a subject file rejected by §1 of `collection-rules.md`, an unreadable subject file, or a pre-check rejection the user caused (QE6) |
  | 3 | the judge's grade is PROVISIONAL, `known-defects` |
  | 4 | the judge's grade is PROVISIONAL, `gaps` |
  | 5 | the judge refused the anchor |
  | 64 | a malformed command line: an unknown command or flag, or a missing argument |
  | 70 | an engine fault, a recovered panic included: a pre-check rejection the engine caused, a record the judge rejected, a store it cannot write, a judge it cannot run, or running as root (QH3) |

  The specification states the limit that a fatal error of the Go runtime cannot be caught, and exits 2.

## 7. Records

- **QE17. Where records live.** Run records, baselines and their trials are kept only in the engine's store, a directory under the user's application-support directory, never inside a repository. Directories are created with mode 0700 and files with 0600. The engine holds no other state between runs that can change a verdict (R27).
- **QE18. Masking.** Every record, a rejected one included, and every scorecard mask the planted canary and each declared environment value, in every encoding §4 of `credit-and-routes.md` lists.
- **QE19. Schemas.** The subject file, the stored record, the baseline and the case format each have a JSON Schema in `schema/`. Two collectors conform alike when, for the same subject file, scope, binary and host, their records to the judge have equal `required` and `evidence` arrays after both are sorted by check id, ignoring `observed` and the order of `targets`.
- **QE20. Cleanup.** Throwaway homes, scratch directories and private copies are removed on every exit, a check killed by the confinement included. Each is created under one engine-owned directory per run, which holds a file naming the engine's process id. At start, the engine removes every such directory whose process no longer exists, so a run that was itself killed is cleaned up by the next.
- **QE21. An anchor is not a green.** The scorecard and the documentation never present exit code 0 from an anchor as a green run.

## 8. The test entry

- **QT1. A scripted verifier.** The released build of `pql` has no scripted network verifier, and a request for one exits 2. Only a build made with the `scripted` build tag contains it.
- **QT2. A test run is scripted.** A run with the scripted verifier claims the tier `scripted`, and so do its targets and evidence. It cannot be anchored, and the judge refuses an anchor of it. The corpus's baseline cases for it use scripted baselines supplied as files.
- **QT3. What it is for.** The scripted verifier lets the corpus run without a sandbox. It shows that the rules turn observations into the right record; it shows nothing about a real subject, and a record from it is never evidence that a subject made no network request.

## 9. Readings of the requirements

| Input | Read as | Rule |
|---|---|---|
| A malformed command line and a rejected request could share a code | 64 for the command line, 2 for the request, keeping both existing contracts | QE16 |
| A test run's verifier is scripted, so nothing showed it was offline | The whole run is scripted, at every level | QT2 |
