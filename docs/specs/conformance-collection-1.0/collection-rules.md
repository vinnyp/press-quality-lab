# Collection rules

**Draft.** This folder is a draft of the conformance collection specification, version 1.0. It is not sealed, and it holds no `KIT.sha256`.

**Specification version:** `1.0`. A collector reports this string with every record it writes. It enters every check id (QE2).

**What this file covers.** How an engine turns one subject file and one requested scope into observations: what the subject file holds (§1), what a run of the subject means (§2), how commands are discovered (§3), the six probes the engine generates (§4), and how authored checks get their expected values (§5). `confinement.md` defines how the subject and a computation are confined and how a refusal is seen. `collection-contract.md` defines how observations become a record for the judge, and the engine's exit codes. `credit-and-routes.md` holds the routing tables.

**How to read the rules.** Rules have stable ids made of `Q`, a family letter and a number: QS subject file, QO outcome of a run, QD discovery, QP probes, QA authored checks, QH home and confinement, QE engine to judge, QT test entry. Where a list says "first match", try its rules in the order written; the first whose condition holds decides. A path "inside" a directory is the directory or anything below it, after every link in the path is resolved. Requirement numbers (R16, R42 …) are the requirements document's. Words the judge's rulebook defines, such as dimension, record, crediting and tier, mean what it says they mean.

**What binds.** These opening paragraphs and §1 to §5 bind a collector. §6 records how requirements were read and explains; where it disagrees with a rule, the rule is right.

## 1. The subject file

- **QS1. One file.** A subject is registered by exactly one JSON file. Its schema is `schema/subject-file.schema.json`. The file states its own version in `subject_file_version`, which is `1.0` for this specification. A file with any other version is rejected before collection.
- **QS2. Strict reading.** The file is read under the judge contract's §3: a repeated key, a number written in a form that §3 refuses, nesting past §3's depth, and ill-formed UTF-8 each reject the file. A value the schema does not allow rejects the file. A rejected file stops the run before any check is fixed (QE5).
- **QS3. What it holds.** The file holds:
  - the subject's binary;
  - the judge's five subject facts (QS4);
  - the declared commands, each with its safety class, its declared exit codes and its output bound (QS6);
  - the declared command count;
  - the seed and the declared store (QS9);
  - offline settings (QS7);
  - bookkeeping paths (QP7);
  - the planted secret's location (QP10);
  - runbooks and their authored checks (§5);
  - expected-failure markers;
  - a `prompts` object reserved for steering, which this version reads only for its version string.
- **QS4. Subject facts.** The file states `read_only`, `has_destructive_ops`, `store_backed` and `has_cli`, each true or false, and `bounding`, which is `none` or `declared`. The engine passes them to the judge as given.
- **QS5. Placeholders.** A value the author has not supplied is a placeholder: an object with exactly one key, `placeholder`, whose value is a sentence saying what belongs there and why the grade needs it. A placeholder is never read as false, `none` or empty.
  - `pql create` writes a placeholder for every value it cannot know, so the file explains itself without comments.
  - A subject fact left as a placeholder reaches the judge as unstated.
  - Any placeholder in the file, including a declared or discovered command with no safety class, makes every required check on an always-applicable dimension not observed with `placeholder_unfilled`, so the run cannot exit 0. The scorecard names each placeholder by its location in the file.
- **QS6. Commands.** Each declared command is a list of words naming it from the binary, its safety class, its declared exit codes and its output bound in bytes.
  - The safety class is `safe`, `destructive`, or a placeholder (QS5).
  - A `destructive` command runs only in a scope that includes `destructive` (R16, R12).
  - A command the file declares `safe` is run under the subject's full confinement like any other; the class says only that running it is wanted, not that it writes nothing.
- **QS7. Offline settings.** The file may declare arguments and environment that make the subject run offline. Each setting names the commands it applies to.
  - A setting applies only to the commands it names, is appended after the command's own words, and is never applied to discovery (§3).
  - A setting that is or contains a help or version request, such as `--help`, `-h`, `help` or `--version`, rejects the file.
  - A declared environment value is a literal string. The engine adds it to the allow-listed environment of QH12 and masks it in every record and scorecard (QE18).
  - Every scorecard shows each offline setting and the commands it applied to.
- **QS8. Files the file names.** Every path the file names is a regular file inside the directory that holds the subject file, after every link is resolved. That directory is neither the user's home directory nor a directory above it. A path that leaves it, a directory, a link left unresolved, or a special file rejects the file.
- **QS9. The seed and the store.** The seed is one directory inside the subject file's directory, or absent.
  - The engine copies the seed by content into each throwaway home (QH14). A seed that holds a link or a special file rejects the file before collection.
  - The declared store is a path relative to the throwaway home, naming the file the subject keeps its data in. It must be a path the seed supplies, or the file is rejected.
  - With no seed, every throwaway home starts empty.
- **QS10. Executables the file names.** An executable the file names, for a computation (QA5), is a regular file inside the subject file's directory. Naming it grants a computation a read of that one file and of the private directory its copy runs from (QH8), and nothing beside it.

## 2. What a run of the subject means

These rules turn one invocation of the subject into an outcome for the check it serves. `confinement.md` says how each condition below is seen.

**First match.**

- **QO1. Confinement not applied.** The engine could not apply confinement to the invocation (QH6), or the grade's self-test failed (QH9), or the system is not one the spike's record names as passed (QH1). The check is not observed with `network_unverifiable`. The subject is never run without confinement instead.
- **QO2. Could not start.** The launcher reported that the binary could not be started (QH7). The check is not observed with `could_not_start` (R22).
- **QO3. Killed by the confinement.** The invocation ended by `SIGKILL` that QH21 attributes to the confinement. The check is not observed with `network_unverifiable`. Its next action names the possible causes: a network request, a child process, another refused operation, or an external kill (QE12).
  - A child process the subject starts is refused with a kill, so it falls here (R16).
  - A bind or a listen on any socket, loopback included, is a network attempt and falls here.
  - In this version no invocation is ever not observed with `network_attempted`, because the confinement cannot tell which refused operation ended the process. `network_attempted` is reserved for a verifier that evidences the attempt itself.
- **QO4. Timed out.** The engine ended the invocation at its time budget.
  - When the engine saw the process still running as it ended it, and the grade's known-fast control kept within its own bound on that host (QH24), the subject ran past its budget: the outcome is `not_held` with `over_budget` (R22).
  - Otherwise the check is not observed with `harness_fault`.
- **QO5. Other ends.** Otherwise the invocation ran to its end under confinement. A crash is `not_held` with `crashed`, and an exit code the check does not accept is `not_held` with `wrong_exit` (R22). Every other result is judged by the check's own predicate.

## 3. Discovery

- **QD1. Discovery is a required check.** Discovery is always in the required set. It reports how many commands it found against the declared count, and the names it found and missed (R42).
- **QD2. How help is asked.** Discovery asks the binary for help with exactly one form, `<command words> --help`, starting from the bare binary and descending into each command it lists.
  - Each request runs under the subject's full confinement, in a throwaway home that starts empty whatever the seed, with no offline setting and no other argument (QS7).
  - Help output is read from standard output, and from standard error when standard output is empty. A command is listed when it appears in the help's command list in the form the help text uses for commands. This specification reads Cobra's `Available Commands:` section and the equivalent section of other generators, as `credit-and-routes.md` lists.
  - The commands `help` and `completion`, and any command the help marks as hidden, are left out of the count.
- **QD3. What discovery compares.** Discovery compares names as well as the count. It holds when every declared command was found and the count found equals the declared count.
- **QD4. A command discovery did not find.** When a declared command was not found, discovery is `not_held`, and every other generated check is not observed with `precondition_unmet`, never red. The scorecard shows both counts and the missing names.
- **QD5. A stated limit.** A CLI that ignores a help request runs that command's action. It does so inside the subject's full confinement, in an empty home, so it can write only there and can reach nothing outside it.

## 4. The generated probes

- **QP1. Six probes.** This version generates exactly six probes: discovery (§3), bare invocation, byte bound, declared exit codes, planted secret and store unchanged. Each probe names the one dimension it feeds in the table in `credit-and-routes.md`, says whether it credits, and says when it is not observed. At least one dimension can be judged from generated checks alone.
- **QP2. Bare invocation.** The binary is run with no arguments in a throwaway home with a fresh copy of the seed. It holds when the process exits with a code the file declares for the bare binary, or 0 when none is declared.
- **QP3. Byte bound.** Each declared `safe` command with an output bound is run with its offline settings. It holds when standard output and standard error together stay within the bound. The bound is enforced by draining the streams (QE10), never by trusting the subject.
- **QP4. Declared exit codes.** Each declared `safe` command with declared exit codes is run with its offline settings. It holds when the exit code is one the file declares for that command.
- **QP5. Fresh state for every probe.** Each probe gets a fresh copy of the seed in its own throwaway home. The engine records the seed's digest with the run, and with no seed the home is empty.

### The store-unchanged probe

- **QP6. What it compares.** The engine snapshots the throwaway home before and after each declared `safe` command it runs for this probe, and compares the two by names and contents only: which paths exist, their kinds, and the bytes of each regular file. Times, modes and owners are not compared.
- **QP7. Bookkeeping paths.** The file may declare bookkeeping paths: paths relative to the home that the subject writes as a matter of course, such as a journal. Changes under a declared bookkeeping path are left out of the comparison, and every scorecard lists the paths that were left out.
  - An exclusion cannot cover the declared store, or any path the seed supplied. A seed is therefore prepared without bookkeeping files.
- **QP8. Database side files.** A side file of the declared store that SQLite creates and removes, such as `-journal`, `-wal` or `-shm` beside the store, is compared as follows: its existence after the command is a change only when it holds bytes, and a store whose bytes changed is a change whatever the side files say.
- **QP9. When it credits.** The probe credits only after the engine has shown, on a separate control copy of the same seed, that its comparison detects a change: it writes one byte to the declared store of the control copy and confirms the comparison reports it. Without that control the probe is not observed with `harness_fault`. When nothing non-empty is left to compare after the exclusions, the probe is not observed with `empty_target`.
  - A link or a special file the subject created anywhere in the home is a change, and the engine never follows or opens one (QH25).

### The planted-secret probe

- **QP10. Where the secret lives.** The file names one file inside the throwaway home where a real secret would be read: a credential file the subject reads, or a seeded-state file for which the file says why reading it proves the secret was read. A multi-record database never qualifies. The engine writes a random canary into that file before the run and never prints the canary.
- **QP11. A proved read.** The probe is observed only when an invocation finishes with its declared successful exit, the planted file's access time moved, the file's device and inode are unchanged, and an otherwise identical control run, whose profile kills on opening that file for reading, is killed (QH27). Otherwise the probe is not observed with `input_missing`.
  - A secret that only an environment variable can carry is also not observed with `input_missing`.
- **QP12. What a leak is.** A leak is the canary, plain or in any encoding `credit-and-routes.md` lists, in a captured stream or in any file of the used home other than the planted one, bookkeeping paths included.
- **QP13. A scan stopped by a bound.** A leak scan that reached any bound, on a stream or in the home, is not observed with `harness_fault`, and is never held.
- **QP14. Stated limits.** The specification states that:
  - another process reading the planted file during the invocation looks like the subject's read;
  - a read that returns no bytes at the end of the file moves its access time and counts as a read;
  - a leak through the system's unified log is not seen, because the logging service the subject may reach writes outside its home;
  - on a Mac bound to a directory server, a user or group lookup can reach that server.

## 5. Authored checks and expected values

- **QA1. Observed values.** Every observed value comes from the engine running the subject's binary, as the private copy QH8 describes, under the subject's confinement. Nothing is read from the subject's store in place of running it.
- **QA2. Expected values.** An expected value is either a literal in the file, or the one JSON value printed by a computation: an executable the file names, given as an argument list and run without a shell.
- **QA3. Self-agreement.** A check whose expected value uses any of the subject's captured output is non-crediting. Whether a check is crediting is fixed from its declared inputs before collection.
- **QA4. A computation's inputs.** A computation receives only the pristine seed and the captured outputs its check declares, never the subject's used home.
- **QA5. A computation's confinement.** A computation runs under the computation's confinement (QH16 to QH20) with no network and a time budget. It may start child processes.
- **QA6. A computation that fails.** A computation that fails, is absent, runs out of time, or prints anything other than exactly one JSON value that the judge contract's §3 accepts inside a record, makes the check not observed with `computation_failed`.
- **QA7. Someone else's file.** The documentation and every `pql` command that grades a subject file warn that grading another person's subject file runs their computations on this machine.
- **QA8. Runbooks.** A runbook's steps run in order, each with its time budget, and share one copy of the seed. A step past its budget is `not_held` with `over_budget` (R33). Every step after the first step that is not held, whether it failed or was not observed, is not observed with `earlier_step_failed`.
- **QA9. The destructive-refusal check.** The check that a destructive command refuses without its confirmation is authored, and runs only in a scope that includes `destructive` (R12).
- **QA10. Predicates.** An authored check uses one of these predicates on one step's invocation. "Output" is standard output after the bounds of QE10.

  | Predicate | Holds when | Empty target satisfies it | Asserts emptiness |
  |---|---|---|---|
  | `exit_code` | the exit code equals the expected integer | no | no |
  | `stdout_equals` | the output's bytes equal the expected string | no | no |
  | `stdout_json_equals` | the output is one JSON value, read under the judge contract's §3, equal to the expected value, numbers compared exactly | no | no |
  | `stdout_contains` | the output contains the expected string | no | no |
  | `stdout_not_contains` | the output does not contain the expected string | yes | no |
  | `stderr_contains` | standard error contains the expected string | no | no |
  | `stdout_bytes_at_most` | the output is at most the expected number of bytes | yes | no |
  | `json_path_equals` | the output is one JSON value and the value at `path` equals the expected value | no | no |
  | `row_count` | the array at `path` has the expected number of items | no | when expected is 0 |
  | `stdout_empty` | the output is empty | no | yes |

  `path` is a JSON Pointer (RFC 6901), and is `null` for a predicate that takes none. A predicate an empty target would satisfy reports the target's size: the number of bytes, or of array items, it evaluated. A check that asserts emptiness names a `precondition`, a computation over the pristine seed that must print `true` to show the data exists; the engine reports its result as the observed precondition.

## 6. Readings of the requirements

| Input | Read as | Rule |
|---|---|---|
| R16 asks for no network request, and macOS can only refuse one with a kill | A kill the engine cannot attribute is not observed with `network_unverifiable`, never red; `network_attempted` is not reported in this version | QO3 |
| R22 makes a crash red, and the confinement's kill is a crash to the process | A kill by the confinement is not the subject's failure | QO3 |
| R22 makes a timeout red | Only when the host kept up, shown by a known-fast control | QO4 |
| R42 asks for a count, and a CLI can ignore a help request | Help is asked under full confinement in an empty home, and the risk is a stated limit | QD2, QD5 |
| R33 stops a runbook after a failed step | A step that was not observed stops it too | QA8 |
| R16 asks that the subject makes no network request, and a loopback bind reaches no network | A bind or listen is an attempt | QO3 |
