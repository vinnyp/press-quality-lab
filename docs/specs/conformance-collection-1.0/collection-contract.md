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
    - recorded behaviour's `behaviour_version` (QT5);
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
  | `seed` | the seed's digest, as below, or `null` with no seed |
  | `store` | the declared store, or `null` |
  | `bookkeeping` | the declared bookkeeping paths, sorted by their UTF-8 bytes |
  | `parameters` | a probe's parameters, as below; for an authored check, `{"exit_codes": its own exit codes, or null}` |
  | `expected` | an authored check's expected source, as below; `null` for a probe |

  For a check with no invocation or with several (discovery, the planted secret, and a probe check with no command), `words` and `budget_seconds` are `null`, `offline_args` is `[]` and `env` is `{}`; the planted secret carries its per-command values in `parameters`. A placeholder enters the preimage as the parsed placeholder object, in RFC 8785 form like everything else.

  Every digest in this specification is the lowercase hexadecimal SHA-256 of the bytes it names.

  **A probe's parameters:**

  | Probe | Parameters |
  |---|---|
  | discovery | `{"declared_count": n, "commands": [the words of each declared command except the bare binary, sorted by their canonical form's bytes]}` |
  | bare | `{"exit_codes": the bare binary's declared codes, or null}` |
  | byte_bound | `{"bound": n}` |
  | exit_codes | `{"exit_codes": [the declared codes, as the file lists them]}` |
  | planted_secret | `{"path": p, "kind": k, "text_sha256": the SHA-256 of the token's text, "commands": [one object per listed command, in the file's order, with that invocation's `words`, `offline_args`, `env` and `budget_seconds`]}`; with `secret` `null`, `{}`; with `secret` a placeholder, `{"secret": the placeholder object}` |
  | store_unchanged | `{}` |

  A probe check with no command has the parameters `{}`.

  **An expected source:**
  - a literal is `{"literal": v, "precondition": ...}`;
  - a computation is `{"computation": {"argv": [...], "budget_seconds": n, "captures": [...], "files": {p: d}}, "precondition": ...}`, where `files` maps the executable, when it is named by a relative path, and each declared file to its content's SHA-256 in hexadecimal.

  Each expected source also has the key `precondition`: for a check that asserts emptiness, its precondition as `{"computation": {...}}` in the same form; otherwise `null`.

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

  **Three more worked ids.** They share a second seed, which holds four entries:
  - `data.json`, whose thirty bytes are `{"widgets":[{"name":"gear"}]}` followed by a line feed, with the content digest `3a8faa48b8f9b4a4ee9325929bebc6d41692a782586147918ae4b995fba81b8b`;
  - `.config/widgets/token`, whose nineteen bytes are `PLANTED-TOKEN-0001` followed by a line feed, with the content digest `b7a2426b2cad538728ed95180a5323b463b166d48bd1d3270ba46e72fd958d7f`;
  - the directories `.config` and `.config/widgets`.

  The seed's digest is `c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a`. Every invocation has the offline argument `--offline` and the declared environment name `WIDGETS_OFFLINE` with the value `1`, whose digest is `6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b`. The store and the bookkeeping path are as above.

  - **An authored check with a literal.** The runbook `widgets` has a first step `list` that runs `list --json` with a 30-second budget. Its one check, on `output-conformance`, uses `stdout_json_equals` with the literal `{"widgets":[{"name":"gear"}]}`. Its preimage is:

    ```
    {"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"output-conformance","earlier":[],"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"expected":{"literal":{"widgets":[{"name":"gear"}]},"precondition":null},"index":0,"offline_args":["--offline"],"parameters":{"exit_codes":null},"path":null,"predicate":"stdout_json_equals","probe":null,"runbook":"widgets","scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":"list","store":"data.json","words":["list","--json"]}
    ```

    Its id is `c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7`.
  - **A check with a computed precondition.** The runbook's second step `filter` runs `list --tag none` with a 30-second budget. Its one check, on `response-correctness`, uses `stdout_empty`, so it asserts emptiness and has a precondition. The precondition runs `computations/has-widgets.sh` with the argument `{SEED}/data.json`, a 5-second budget and no captures. That script's eighty-four bytes are these two lines, each ending in a line feed, and its content digest is `a1b660c0d3bd81e132520531ee7d97c5467d5c9f3575fda91abd60daf573dbc8`:

    ```
    #!/bin/sh
    if grep -q '"name"' "$1"; then printf 'true\n'; else printf 'false\n'; fi
    ```

    The check's `earlier` holds the first step's id. Its preimage is:

    ```
    {"bookkeeping":[".local/state/widgets"],"budget_seconds":30,"dimension":"response-correctness","earlier":["c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7"],"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"expected":{"literal":null,"precondition":{"computation":{"argv":["computations/has-widgets.sh","{SEED}/data.json"],"budget_seconds":5,"captures":[],"files":{"computations/has-widgets.sh":"a1b660c0d3bd81e132520531ee7d97c5467d5c9f3575fda91abd60daf573dbc8"}}}},"index":0,"offline_args":["--offline"],"parameters":{"exit_codes":null},"path":null,"predicate":"stdout_empty","probe":null,"runbook":"widgets","scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":"filter","store":"data.json","words":["list","--tag","none"]}
    ```

    Its id is `40b08dbef70438992c53f14fee9f5e1297277bc94bb35cb5e2b435758df22993`.
  - **A planted secret.** The secret is the `credential` at `.config/widgets/token` with the token `PLANTED-TOKEN-0001`, whose text digest is `d2ea444a8905d8d4d040c5770edcd4a576b858fe4b357ff9e277241dea36261b`, read by the command `account` with a 30-second budget. Its preimage is:

    ```
    {"bookkeeping":[".local/state/widgets"],"budget_seconds":null,"dimension":"pii-secret-hygiene","earlier":[],"env":{},"expected":null,"index":null,"offline_args":[],"parameters":{"commands":[{"budget_seconds":30,"env":{"WIDGETS_OFFLINE":"6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b"},"offline_args":["--offline"],"words":["account"]}],"kind":"credential","path":".config/widgets/token","text_sha256":"d2ea444a8905d8d4d040c5770edcd4a576b858fe4b357ff9e277241dea36261b"},"path":null,"predicate":null,"probe":"planted_secret","runbook":null,"scheme":"pql-check-id-1","seed":"c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a","step":null,"store":"data.json","words":null}
    ```

    Its id is `181431d07ae5e61a4a5fde89d6a606708476067a3dcdfad263b473f33adb98ba`.
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
  - Each invocation is a target of kind `invocation`. Its id is the check's id, or the check's id followed by `#` and the invocation's position from 1 when the check has more than one invocation. Discovery's help requests are numbered in the order QD2 gives; the planted secret's invocations in the order its commands are listed, counting listed commands only. A control invocation is never a target.
  - The store-unchanged probe adds the declared store as a target of kind `artifact`. The planted-secret probe adds the planted file the same way. Each artifact's id is its path relative to the home, and its tier is the tier of the invocation that used it.

  A check that is not observed has an `outcome` of `not_observed` and carries its reason and the invocation it would have used.
- **QE9. Tiers.** A normal run claims `live`. Each target that ran with an offline setting carries the tier `offline`, and every other target carries `live`. A piece of evidence carries `offline` when every one of its targets does, and `live` otherwise.
- **QE10. Bounds.**
  - **Draining.** Captured output is drained as it is produced, so the subject is never blocked by a full pipe. The engine counts every drained byte and keeps the first 1 MiB of each stream for predicates and the leak scan. Past that point it records that the stream was cut and keeps draining.
  - **What a record carries.** In `expected` and `observed`, a stream appears as an object with exactly these keys:
    - `bytes`, the stream's byte count;
    - `sha256`, its SHA-256 over the kept bytes;
    - `cut`, whether it was cut;
    - `head`, its first 64 KiB decoded as UTF-8, with each ill-formed byte replaced by U+FFFD.
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
  | 2 | a well-formed request rejected for its content or scope: a subject file rejected by §1 of `collection-rules.md`, an unreadable subject file, a scripted verifier, host or recording requested from a released build, a scripted run missing its host or recording or given one that is unreadable or invalid (QT1), or a pre-check rejection the user caused (QE6) |
  | 3 | the judge's grade is PROVISIONAL, `known-defects` |
  | 4 | the judge's grade is PROVISIONAL, `gaps` |
  | 5 | the judge refused the anchor |
  | 64 | a malformed command line: an unknown command or flag, or a missing argument |
  | 70 | an engine fault, a recovered panic included: a pre-check rejection the engine caused, a record the judge rejected, a store it cannot write, a judge it cannot run, or running as root (QH3) |

  The specification states the limit that a fatal error of the Go runtime cannot be caught, and exits 2.

## 7. Records

- **QE17. Where records live.** Run records, baselines and their trials are kept only in the engine's store, never inside a repository. The store is a directory under the user's application-support directory. Directories are created with mode 0700 and files with 0600. The engine holds no other state between runs that can change a verdict (R27).
- **QE18. Masking.** Every record, a rejected one included, and every scorecard mask the planted canary and each declared environment value, in every encoding `credit-and-routes.md` §4 lists.
- **QE19. Schemas and conformance.** The subject file, the stored record, the baseline, the case format and recorded behaviour (QT5) each have a JSON Schema in `schema/`. Two collectors conform alike when, for the same subject file, scope, binary and host, their records to the judge agree after both are sorted by check id:
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

- **QT1. A scripted verifier and a scripted host.** `--verifier` takes `real`, the default, or `scripted`. Only a build made with the `scripted` build tag contains a scripted network verifier, selected by `--verifier scripted`, which reads a recording named by `--behaviour <file>` (QT5) and a scripted host named by `--host <file>` (QT4).
  - **The released build** parses all three flags. It accepts `--verifier real`. It rejects `--verifier scripted`, any `--host` and any `--behaviour` with exit 2, and every other value of `--verifier` as a malformed command line with exit 64.
  - **A scripted build** runs a scripted run only with all of `--verifier scripted`, `--host` and `--behaviour`. Any one of them without the other two exits 2, and so does a host or recording file that cannot be read, that the judge contract's §3 would refuse as JSON, that its schema rejects, or that breaks a rule of QT4 or QT5.
- **QT2. A test run is scripted.** A run with the scripted verifier claims the tier `scripted`, and so do its targets and evidence. It cannot be anchored, and the judge refuses an anchor of it. The corpus's baseline cases for it use scripted baselines supplied as files.
- **QT3. What it is for.** The scripted verifier lets the corpus run without a sandbox. It shows that the rules turn observations into the right record. It shows nothing about a real subject, and a record from it is never evidence that a subject made no network request.
  - **What a scripted run observes.** It takes each invocation's result from the recording `--behaviour` names (QT5), in place of running the subject or a computation. It uses the recording's first confined run, and the unconfined run never. An invocation the recording lacks is an engine fault, exit 70.
  - **The engine's own controls.** It takes their raw results from the scripted host (QT4).
- **QT4. The scripted host.** The host file holds one JSON object, as the `host` subschema of `schema/case.schema.json` defines it. Each field forces the raw result of one control the engine runs, and every field is required:

  | Field | Forces | Values |
  |---|---|---|
  | `confinement` | whether confinement was applied (QH6). Not applied, with `sandbox-exec`'s own exit code, holds for every invocation, the subject's and each computation's. Applied leaves each invocation's markers and end as recorded | `{"applied": true, "sandbox_exec_exit": null}` or `{"applied": false, "sandbox_exec_exit": n}` |
  | `self_test` | the self-test's result (QH9) | `passed`, `failed` |
  | `known_fast` | the known-fast control (QH24) | `within`, `over` |
  | `read_calibration` | the volume's read calibration (QH26) | `moved`, `not_moved` |
  | `store_control` | the store comparator's control copy (QP9) | `detected`, `not_detected` |
  | `read_control` | the read proof's control invocation (QH27), for every listed command whose control runs | `killed`, `not_killed` |
  | `private_copy` | the check of the private copy before each use (QH8) | `unchanged`, `changed` |
  | `kill_attribution` | for each invocation it names, what ended a recorded `SIGKILL` that the recording attributes to the confinement | a list of `{"invocation": key, "by": b}`, where `key` is as QT5 defines and `b` is `memory` or `code_signing` (an exit detail the system reported) or `engine` (a kill the engine sent itself outside the time budget) |

  - **Attributions.** Each entry of `kill_attribution` names an invocation, at most once, whose first confined run ended by `SIGKILL` attributed to `confinement`. Any other entry makes the host file invalid. The entry replaces that attribution, and QH21 and QO5 then decide.
  - **Precedence.** In a scripted run, the scripted host's results replace any result a recording holds for the same control. The recording supplies only what the subject and the computations did.
  - **What stays the same.** The scripted build takes only these raw results from the host. QO1, QO4, QP9, QP11 and QH21 decide from them exactly as the released build decides from the controls it ran. A collector that ignores a failed control therefore gives a wrong record for a case that forces one.
  - **A passing host.** Every control passes when `confinement` is applied, `self_test` is `passed`, `known_fast` is `within`, `read_calibration` is `moved`, `store_control` is `detected`, `read_control` is `killed`, `private_copy` is `unchanged` and `kill_attribution` is empty.
- **QT5. Recorded behaviour.** Each form of the corpus's synthetic subject has one recording, `behaviour/<form>.json`, whose schema is `schema/behaviour.schema.json`. A recording is what that form did, and the only source of what a scripted run observes.
  - **What the whole recording names:**
    - its own format version, `behaviour_version`, which is `1.0`;
    - the form's name, and whether it is a violating form, one built to attempt an operation the confinement refuses;
    - the digest of the form's source: QE2's seed digest computed over the source files the form's build compiles, as paths relative to the synthetic subject's source root, with an entry for each file and none for a directory;
    - the digests of the subject profile and the computation profile, each as the template renders with its variant's fixed text and before any line the engine adds for one invocation, and of the launcher's source;
    - the macOS version and build it was recorded on, and the commit.
  - **An invocation's key.** Each invocation is keyed by an object with these fields. `home` names the home it runs in: `seed` for a fresh copy of the seed, `empty` for an empty home (discovery's), `planted` for a fresh copy with the canary planted (the planted-secret probe's), `runbook` for its runbook's shared home, and `scratch` for a computation's scratch directory. The other fields: for a subject invocation, `words`, `offline_args`, `env` (the declared environment names that apply, sorted by their bytes), `runbook` and `step` (both strings for a runbook step, both `null` for a probe's invocation), and `computation` `null`; for a computation's, `computation` (its argument list as the subject file gives it), `runbook` and `step` (those of the check it serves, because a capture makes its result depend on them), `words` `null`, and `offline_args` and `env` empty. No two invocations of one recording have equal keys.
  - **The runs.** Every invocation has five confined runs. An invocation of a violating form also has one unconfined run first, the positive control that the form's act happens. A non-violating form's invocations have no unconfined run.
  - **One run records:**
    - **the markers:** whether the launcher wrote its marker (QH5), and whether it wrote the could-not-start marker (QH7);
    - **the end:** an exit with its code, or a signal with its name; for `SIGKILL`, what QH21 attributes it to: `confinement`, `budget` (the engine ended the process at its time budget while it still ran), `memory` or `code_signing` (an exit detail), or `before_marker`;
    - **each stream,** standard output and standard error without the launcher's markers, with the keys QE10 gives (`bytes`, `sha256`, `cut`, `head`) and `kept`: every kept byte, in base64, or `null` when the stream was cut;
    - **the home's changes,** `home_changes`: every change the run made, before any exclusion QP7 or QP8 applies, each as its path relative to the home, its kind (`file`, `directory`, `link` or `special`), whether it was created, modified or removed, and a file's content digest; with whether the store comparator's control copy showed its written change, or `null` where the store-unchanged probe does not run the invocation;
    - **the planted file,** for an invocation the planted-secret probe makes: whether its access time moved, whether its device and inode were unchanged, and the control invocation's end, or `null` where the control did not run. For any other invocation it is `null`;
    - **the leak scan:** whether it reached a bound (QP13), and each leak found, as its place (`stdout`, `stderr`, or `home/` followed by a path relative to the home) and its encoding, named as `credit-and-routes.md` §4 names it;
    - **each decoy's state:** for each decoy the run set up, its name and whether it was touched;
    - **for a computation,** the one JSON value it printed, or `null` when it printed anything else.
  - **What a recording never holds.** No absolute path, except an executable under `/bin` or `/usr/bin` that a computation's argument list names (QA4); and no canary. Before anything is recorded, every occurrence of the run's canary, in every encoding §4 lists, is replaced by the same encoding of `pqlcanary` followed by 32 zeros. The two strings have the same length, and every count, digest and head in a recording is taken after the replacement, so every count and every leak's place survive.
  - **Agreement.** The five confined runs of an invocation agree on the markers, the end, the leak scan, the planted file, the decoys, the home's change list, a computation's printed value, and every stream's byte count, digest and cut. A recording whose runs disagree is invalid.
- **QT6. The corpus's cases.** Each case is a directory `cases/<id>/` holding `case.json`, whose schema is `schema/case.schema.json`, its subject file, and the files below.
  - **`input`** holds what a collector is given: the subject file's name, the form, the request and scope, the verifier (`real` or `scripted`), the scripted host (QT4) for a scripted case or `null` for a real one, the baseline file's name or `null`, and the fixture descriptions.
  - **Fixtures** describe what the repository cannot hold. Each has a `kind`, a `path` and a `detail`:
    - `link`: assembly creates a symbolic link at `path`, relative to the assembled directory, whose target is the text of `detail`, which may name nothing;
    - `special_file`: assembly creates a named pipe at `path`, relative to the assembled directory;
    - `oversized_output`, `invalid_bytes` and `planted_secret` describe what the form or its seed already does, and assembly creates nothing. Their `path` is `stdout`, `stderr`, or a path relative to the home.
  - **A judge-checked case** also holds `input.json`, the judge input the collector must send, and `preimages.json`, which maps each check id in it to that check's preimage (QE2). Its `expect` holds the judge's whole output and the engine's exit code.
  - **A derivation-only case** ends before any record exists. Its `expect` holds the engine's exit code and the id of the rule that decides it.
  - **Assembly.** A case names a form, and the form's canonical directory `subjects/<form>/` holds its seed and its computations. Before applying §1 of `collection-rules.md`, a collector assembles one temporary directory holding:
    - the case's subject file;
    - a copy of the form's seed, at `seed/`;
    - the built form, at `bin/subject`;
    - the form's computations, under `computations/`;
    - each `link` and `special_file` fixture.

    A case's subject file names those fixed paths, so QS8 and QS9 hold. A scripted run never executes `bin/subject`. A real run needs the form built from the source whose digest its recording names.
  - **Fields outside `input`** say what the case is for, and a collector reads none of them: `rules`, the rule ids it exercises; `named_check`, the check a defect case is red on, or `null`; `twin`, the case that differs from it in one input and repairs it, or `null`; `invocations`, the keys (QT5), each once, of every invocation of these kinds for the case's request and scope: each probe's, each help request QD2 reaches in the form's recording, the bare binary's, every runbook step's whether or not an earlier failure would stop it, and every computation's, or none for a derivation-only case that ends before collection; and `covers`.
  - **`covers`** names the table rows, credit-condition sides and reason-order pairs the case exercises. Each tag is `<table>:<row>` or `<table>:<row>:<side>`:

    | Table | Tag | Rows |
    |---|---|---|
    | `credit-and-routes.md` §2 | `route:<dimension>` | each dimension |
    | the credit conditions of §2 | `credit:<dimension>:holds`, `credit:<dimension>:fails` | each dimension whose condition is neither "none" nor "never" |
    | `credit-and-routes.md` §4 | `encoding:<name>` | each encoding |
    | `credit-and-routes.md` §5 | `help:<generator>` | `cobra`, `urfave-cli`, `clap` |
    | QA9 | `predicate:<predicate>` | each predicate |
    | the reason order of `collection-rules.md` §2 | `order:<earlier>><later>` | each adjacent pair of QO1, QS5, QD4, QA7, QH8, QA6, QO2, QO3, QO4, QO5 |

    A tag that names no row rejects the case.

- **An informative note on phase 3.** This note binds no collector. The engine's plan assigns phase 3's real verifier to local macOS runs on a machine the owner controls, each stated in its pull request as the repository's contributor guide requires. The scripted verifier runs in continuous integration on Linux, which stays required on the main branch.

## 9. Readings of the requirements

| Input | Read as | Rule |
|---|---|---|
| A malformed command line and a rejected request could share a code | 64 for the command line, 2 for the request, keeping both existing contracts | QE16 |
| A test run's verifier is scripted, so nothing showed it was offline | The whole run is scripted, at every level | QT2 |
| A baseline must survive a new supported system | The id carries its own scheme string, not the specification's version | QE1, QE2 |
| A scripted host could score logic other than the released build's | Only each control's raw result is scripted; the released decision logic reads it | QT4 |
| A recording could carry the canary it leaked | Every occurrence is replaced by a fixed string of the same length before recording | QT5 |
