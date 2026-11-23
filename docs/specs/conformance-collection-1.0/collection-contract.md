# Collection contract

**Draft.** This folder is a draft of the conformance collection specification, version 1.0. It is not sealed, and it holds no `KIT.sha256`.

**What this file covers.** How the engine turns its observations into the judge's input, what it does with the judge's output, which exit code it ends with, and how it keeps and cleans up what it ran. The rules are families QE and QT. How to read the rules, and what binds, is as `collection-rules.md` says. The judge decides every verdict; nothing here decides one.

## 1. Versions

- **QE1. Version and change.** This specification's version string is `1.0`.
  - **When the string changes.** It changes when, and only when, some subject file, scope and subject behaviour can lead to a different record for the judge or a different engine exit code. Every other change is editorial and goes in the dated errata note `docs/specs/conformance-collection-1.0-errata.md`, outside the sealed folder.
  - **Ratification.** The owner ratifies each version.
  - **Reading older files.** A later specification version states which subject-file versions it reads. Version strings take the form `<major>.<minor>`.
  - **Other version strings.** Three strings are versioned on their own, and each changes only when its own format does:
    - the subject file's `subject_file_version`;
    - the stored record's and baseline's `record_version` and `baseline_version`;
    - the id scheme of QE2.

    A new specification version that only adds a supported macOS version (QH1) keeps the id scheme, so every baseline stays comparable.

## 2. The required set and its ids

- **QE2. Check ids.** A check's id is the lowercase hexadecimal SHA-256 of its preimage: a JSON object in the canonical form of RFC 8785 (JSON Canonicalization Scheme), encoded as UTF-8. The preimage has exactly these keys:

  | Key | Value |
  |---|---|
  | `scheme` | `"pql-check-id-1"` |
  | `probe` | for a generated check, one of `discovery`, `bare`, `byte_bound`, `exit_codes`, `planted_secret`, `store_unchanged`; for an authored check, `null` |
  | `runbook`, `step` | the authored check's runbook name and step name; `null` for a probe |
  | `index` | the authored check's position among its step's checks, from 0; `null` for a probe |
  | `earlier` | the ids of every check of every earlier step of the runbook, in step order and then check order; `[]` for a probe |
  | `dimension` | the dimension the check feeds |
  | `predicate`, `path` | the authored check's predicate and its `path`; `null` for a probe |
  | `words` | the invocation's words |
  | `offline_args` | the arguments every applying offline setting appends, in order; `[]` when none applies |
  | `env` | an object mapping each declared environment name that applies to the lowercase hexadecimal SHA-256 of its value's UTF-8 bytes |
  | `budget_seconds` | the invocation's budget |

  For a check with no invocation or with several (discovery, the planted secret, and a probe check with no command), `words` and `budget_seconds` are `null`, `offline_args` is `[]` and `env` is `{}`; the planted secret carries its per-command values in `parameters`. A placeholder enters the preimage as the placeholder object exactly as written.
  | `seed` | the seed's digest, as below, or `null` with no seed |
  | `store` | the declared store, or `null` |
  | `bookkeeping` | the declared bookkeeping paths, sorted by their UTF-8 bytes |
  | `parameters` | a probe's parameters, as below; for an authored check, `{"exit_codes": its own exit codes, or null}` |
  | `expected` | an authored check's expected source, as below; `null` for a probe |

  **A probe's parameters:**

  | Probe | Parameters |
  |---|---|
  | discovery | `{"declared_count": n, "commands": [the words of each declared command except the bare binary, sorted by their canonical form's bytes]}` |
  | bare | `{"exit_codes": the bare binary's declared codes, or null}` |
  | byte_bound | `{"bound": n}` |
  | exit_codes | `{"exit_codes": [the declared codes, as the file lists them]}` |
  | planted_secret | `{"path": p, "kind": k, "token": the token's SHA-256 in hexadecimal, "commands": [one object per listed command, in the file's order, with that invocation's `words`, `offline_args`, `env` and `budget_seconds`]}`; with `secret` `null`, `{}` |
  | store_unchanged | `{}` |

  A probe check with no command has the parameters `{}`.

  **An expected source:**
  - a literal is `{"literal": v}`;
  - a computation is `{"computation": {"argv": [...], "budget_seconds": n, "captures": [...], "files": {p: d}}}`, where `files` maps the executable, when it is named by a relative path, and each declared file to its content's SHA-256 in hexadecimal.

  A check that asserts emptiness adds `{"precondition": ...}` in the same form to its expected source.

  **The seed's digest** is the SHA-256 of the RFC 8785 form of a JSON array, encoded as UTF-8. The array holds one entry for each file and directory below the seed directory, sorted by path bytes. Each entry is `[path, kind, digest]`:
  - `path` is relative to the seed directory and uses `/`;
  - `kind` is `"file"` or `"directory"`;
  - `digest` is the file's content SHA-256 in hexadecimal, or `null` for a directory.

  **What is not in an id.** The binary's own digest is not part of the id, so a changed binary keeps its checks' ids and the judge can compare it with a baseline. Neither is this specification's version string.

  **A worked id.** A seed holds one file, `data.json`, whose fifteen bytes are `{"widgets":[]}` followed by a line feed.
  - Its content digest is `3890b1a054e609ad21a23184fc351862f3aed55501fdf682bc1d2cec95dacff2`.
  - The seed's digest is `d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee`.
  - The byte-bound check of the command `list` has a 30-second budget, a bound of 65,536 bytes, the offline argument `--offline`, the store `data.json` and the bookkeeping path `.local/state/widgets`. Its preimage is:

  ```
  {"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"bounding","earlier":[],"env":{},"expected":null,"index":null,"offline_args":["--offline"],"parameters":{"bound":65536},"path":null,"predicate":null,"probe":"byte_bound","runbook":null,"scheme":"pql-check-id-1","seed":"d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee","step":null,"store":"data.json","words":["list"]}
  ```

  Its id is `46c4884c4195f646969ad14aca889bb614643f7c5e7e88bdec87d1032c0344d7`. The declared-exit-code check on the same command differs in `probe`, `dimension` and `parameters`, so the two probes on one command get two ids.
- **QE3. A fixed set.** The engine fixes the required set from the subject file, the judge's rulebook and the requested scope before discovery or any other check runs (R21). It holds every generated check QP1 places, and every authored check whose dimension is in the requested scope and whose runbook may run in it (QA7). Evidence never adds to it or removes from it.
- **QE4. Baselines and missing ids.**
  - **What a baseline holds.** It stores each check's preimage beside its entry.
  - **Which baseline a grade uses.** The baseline the user names. Without one, it uses the baseline with the latest `created_at` stored for the subject file's canonical path.
  - **Missing ids.** When the baseline holds ids the new required set lacks, the engine lists each one. It names the cause from the stored preimage: the command, step or runbook that is gone, or the preimage keys whose values changed. It writes the list as a JSON array of the missing ids, sorted. That file is the form `pql anchor` accepts as its retired list. The engine retires none by itself.

## 3. Before collection

- **QE5. The pre-check.** Before any check runs, the engine validates the subject file (§1 of `collection-rules.md`) and its own generated checks. It then calls the judge once with the run's own request kind, the required set and no evidence.
  - A grade's pre-check carries the baseline. An anchor's also carries the retired checks and accepted regressions.
  - A rejection here stops the run, before any check runs, with the exit code QE16 gives.
- **QE6. Who caused a rejection.** The engine validates its own generated checks first, so a pre-check rejection is the user's except where the table says otherwise. A rejection the engine caused exits 70.

  | Rule | Cause |
  |---|---|
  | X1 | user: the requested scope |
  | X2, X3 | engine: no evidence is sent in the pre-check, so either is an engine fault |
  | X4 | user: a marker in the subject file on a Safety or Steering dimension, or on a record with no crediting check there |
  | X5 | engine: every field X5 checks is set by the engine |
  | X6 to X9 | engine: no trial is sent in this version |
  | X10 | engine: QS10 already rejects duplicate names, so a duplicate id is an engine fault |
  | X11 | user: the retired list, accepted regressions or the baseline named |
  | X12 | engine: the engine writes the document |

## 4. Evidence

- **QE7. Fields of a required check.** For each check the engine sets these fields, and every field not listed to `null`:

  | Field | Value |
  |---|---|
  | `id` | as QE2 says |
  | `kind` | `plain` |
  | `record` | the runbook that holds the check, or `null` for a probe |
  | `crediting` | from QA3, and from the probe table of `credit-and-routes.md` §1 for a probe |
  | `empty_satisfiable` | as QA9 says for an authored check; `true` for byte bound; otherwise `false` |
  | `asserts_emptiness` | as QA9 says |
  | `precondition` | for a check that asserts emptiness, the string `precondition computation printed true` |
- **QE8. Fields of a piece of evidence.** For each check in the required set the engine sends one piece of evidence:

  | Field | Value |
  |---|---|
  | `outcome`, `reason`, `failure_kind` | by §2 of `collection-rules.md` and the check's predicate |
  | `target_size` | for an empty-satisfiable check that was observed, the number of bytes or items its predicate evaluated |
  | `precondition_observed` | for a check that asserts emptiness and was observed, the observed precondition (QA9) |
  | `expected`, `observed` | bounded as QE10 says |
  | `tier` | as QE9 says |
  | `evaluator` | `{"kind": "deterministic", "model": null}` |

  **Targets.**
  - Each invocation is a target of kind `invocation`. Its id is the check's id, or the check's id followed by `#` and the invocation's position from 1 when the check has more than one invocation. Discovery's help requests are numbered in the order QD2 gives; the planted secret's invocations in the order its commands are listed. A control invocation is never a target.
  - The store-unchanged probe adds the declared store as a target of kind `artifact`. The planted-secret probe adds the planted file the same way. Each artifact's id is its path relative to the home, and its tier is the tier of the invocation that used it.

  A check that is not observed has an `outcome` of `not_observed` and carries its reason and the invocation it would have used.
- **QE9. Tiers.** A normal run claims `live`. Each target that ran with an offline setting carries the tier `offline`, and every other target carries `live`. A piece of evidence carries `offline` when every one of its targets does, and `live` otherwise.
- **QE10. Bounds.**
  - **Draining.** Captured output is drained as it is produced, so the subject is never blocked by a full pipe. The engine counts every drained byte and keeps the first 1 MiB of each stream for predicates and the leak scan. Past that point it records that the stream was cut and keeps draining.
  - **What a record carries.** In `expected` and `observed`, a stream appears as an object holding:
    - the stream's byte count;
    - its SHA-256 over the kept bytes;
    - whether it was cut;
    - its first 64 KiB decoded as UTF-8, with each ill-formed byte replaced by U+FFFD.
  - **The whole record.** The `expected` and `observed` values of all evidence together are held to 4 MiB, measured on the serialized JSON the engine sends. The cap is applied in the order of check ids. Past it, a value is replaced by an object holding only its byte count and SHA-256.
  - **The subject file.** A literal is bounded at 64 KiB and at a nesting depth of 48 (QS11).
  - **Why this fits.** QS11 allows at most 3,587 required checks: 2,048 authored, three probes with one check per command for at most 512 commands each, and three more. Without its values, each check costs at most about 2 KiB across `required`, its evidence and its baseline entry, names included, which is under 8 MiB in all. Markers add at most 1,024 small objects. With the 4 MiB of values, a record stays under 13 MiB, inside the judge contract's 16 MiB.
  - **A last bound.** A record that would still pass 15 MiB has values replaced by digests in reverse id order until it fits. One that cannot fit is an engine fault, exit 70.
- **QE11. Markers.** Each expected-failure marker in the subject file reaches the judge as given, naming its runbook as the record and its dimension.

## 5. Next actions

- **QE12. Every reason this specification reports has a next action.** The scorecard shows the next action for each reason. For a check that is not observed, it appears beside the check. For a dimension's own reason, `placeholder_unfilled` on an unstated fact or `scope_not_requested`, it appears beside the dimension. The stored record keeps each next action against the check id or the dimension it belongs to.

  | Reason | Next action |
  |---|---|
  | `network_unverifiable` | The confinement ended the subject. Possible causes: a network request, a child process, another refused operation, or an external kill. Check that the command has an offline setting that keeps it off the network, and that it starts no child process. On an unsupported system, grade on one QH1 names. |
  | `could_not_start` | Check that the binary the subject file names is an executable for this machine. |
  | `input_missing` | For the planted secret: name the file the subject reads its secret from, its token, and a command that reads it, and check that the command reads it. For another check: supply what the check needs. |
  | `precondition_unmet` | For a generated check: discovery did not find every declared command, or could not read the help; fix the declared list or the binary. Otherwise: seed the data the check needs to exist. |
  | `empty_target` | Seed data the check can evaluate, or declare the bound, the exit codes or the free-of-writes command the probe needs. |
  | `placeholder_unfilled` | Fill each placeholder the scorecard names, and classify each discovered command the file does not declare. |
  | `computation_failed` | Run the computation by hand on the pristine seed and make it print exactly one JSON value within its budget, without anything its confinement refuses. |
  | `earlier_step_failed` | Fix the first step of the runbook that was not held. |
  | `harness_fault` | Rerun. If it repeats, report it with the stored record: the engine or the host failed, not the subject. When a stream was cut, the output passed the 1 MiB the engine keeps. |
  | `no_evidence` | Report it with the stored record: the engine fixed a check it did not observe. |
  | `scope_not_requested` | Request a scope that includes the dimension. For `cli-mcp-parity`, no scope includes it in this version. |
- **QE13. `network_attempted`.** This version never reports it (QO3).

## 6. Exit codes

- **QE14. What the judge decides.** For every record the judge decided, the engine's exit code is the judge's. Every verdict, state and count the engine shows is read only from the judge's output. The scorecard and the stored record add the evidence and the run's details the engine recorded.
- **QE15. A rejected record.** When the judge rejects the engine's record after collection, the engine exits 70, shows no verdict, and keeps the rejected record for diagnosis.
- **QE16. The closed table.** The engine exits with exactly one of these codes.

  | Code | When |
  |---|---|
  | 0 | the judge's grade is PASSED, or the anchor was made |
  | 1 | the judge's grade is FAILED; nothing else exits 1 |
  | 2 | a well-formed request rejected for its content or scope: a subject file rejected by §1 of `collection-rules.md`, an unreadable subject file, a scripted verifier requested from a released build (QT1), or a pre-check rejection the user caused (QE6) |
  | 3 | the judge's grade is PROVISIONAL, `known-defects` |
  | 4 | the judge's grade is PROVISIONAL, `gaps` |
  | 5 | the judge refused the anchor |
  | 64 | a malformed command line: an unknown command or flag, or a missing argument |
  | 70 | an engine fault, a recovered panic included: a pre-check rejection the engine caused, a record the judge rejected, a store it cannot write, a judge it cannot run, or running as root (QH3) |

  The specification states the limit that a fatal error of the Go runtime cannot be caught, and exits 2.

## 7. Records

- **QE17. Where records live.** Run records, baselines and their trials are kept only in the engine's store, never inside a repository. The store is a directory under the user's application-support directory. Directories are created with mode 0700 and files with 0600. The engine holds no other state between runs that can change a verdict (R27).
- **QE18. Masking.** Every record, a rejected one included, and every scorecard mask the planted canary and each declared environment value, in every encoding `credit-and-routes.md` §4 lists.
- **QE19. Schemas and conformance.** The subject file, the stored record, the baseline and the case format each have a JSON Schema in `schema/`. Two collectors conform alike when, for the same subject file, scope, binary and host, their records to the judge agree after both are sorted by check id:
  - the required arrays are equal;
  - each piece of evidence has equal `check_id`, `outcome`, `reason`, `failure_kind`, `target_size`, `precondition_observed` and `tier`;
  - each piece of evidence has the same set of target kinds, ids and tiers.

  `expected` and `observed` are left out of the comparison.
- **QE20. Cleanup.**
  - **When.** Throwaway homes, scratch directories and private copies are removed on every exit, a check killed by the confinement included.
  - **Where they live.** Each is created under one engine-owned directory per run. That directory holds a file naming the engine's process id and that process's start time.
  - **After a killed run.** At start, the engine removes every such directory whose process no longer exists, or whose process id now belongs to a process with a different start time. A run that was itself killed is therefore cleaned up by the next.
- **QE21. An anchor is not a green.** The scorecard and the documentation never present exit code 0 from an anchor as a green run.

## 8. The test entry

- **QT1. A scripted verifier.** Only a build made with the `scripted` build tag contains a scripted network verifier, selected by `--verifier scripted`. The released build parses `--verifier`. It rejects the value `scripted` with exit 2, and every other value as a malformed command line with exit 64.
- **QT2. A test run is scripted.** A run with the scripted verifier claims the tier `scripted`, and so do its targets and evidence. It cannot be anchored, and the judge refuses an anchor of it. The corpus's baseline cases for it use scripted baselines supplied as files.
- **QT3. What it is for.** The scripted verifier lets the corpus run without a sandbox. It shows that the rules turn observations into the right record. It shows nothing about a real subject, and a record from it is never evidence that a subject made no network request.

## 9. Readings of the requirements

| Input | Read as | Rule |
|---|---|---|
| A malformed command line and a rejected request could share a code | 64 for the command line, 2 for the request, keeping both existing contracts | QE16 |
| A test run's verifier is scripted, so nothing showed it was offline | The whole run is scripted, at every level | QT2 |
| A baseline must survive a new supported system | The id carries its own scheme string, not the specification's version | QE1, QE2 |
