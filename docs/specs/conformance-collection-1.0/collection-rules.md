# Collection rules

**Draft.** This folder is a draft of the conformance collection specification, version 1.0. It is not sealed, and it holds no `KIT.sha256`.

**Specification version:** `1.0`. A collector reports this string with every record it writes. Check ids carry a separate id-scheme string (QE2), so a new specification version keeps every check's id unless it changes what an id is made of.

**What this file covers.** How an engine turns one subject file and one requested scope into observations:
- §1: what the subject file holds;
- §2: what a run of the subject means;
- §3: how commands are discovered;
- §4: the six probes the engine generates;
- §5: how authored checks get their expected values.

`confinement.md` defines how the subject and a computation are confined and how a refusal is seen. `collection-contract.md` defines how observations become a record for the judge, and the engine's exit codes. `credit-and-routes.md` holds the routing and crediting tables.

**How to read the rules.** Rules have stable ids made of `Q`, a family letter and a number:

| Family | Covers |
|---|---|
| QS | the subject file |
| QO | the outcome of a run |
| QD | discovery |
| QP | the probes |
| QA | authored checks |
| QH | the home and confinement |
| QE | the engine and the judge |
| QT | the test entry |

- **First match.** Where a list says "first match", try its rules in the order written. The first whose condition holds decides.
- **Inside.** A path "inside" a directory is the directory or anything below it, after every link in the path is resolved.
- **A path relative to the home.** It is a relative path with no empty, `.` or `..` component.
- **Citations.** Requirement numbers (R16, R42 …) are the requirements document's.
- **The judge's words.** Words the judge's rulebook defines, such as dimension, record, crediting and tier, mean what it says they mean.
- **Always-applicable dimension.** A dimension that rulebook §6 excludes on no subject fact.
- **An invocation's command.** It is the declared command whose words are the longest prefix of the invocation's words. The empty word list, the bare binary, is a prefix of every invocation.

**What binds.** These opening paragraphs and §1 to §5 bind a collector. §6 records how requirements were read and only explains. Where it disagrees with a rule, the rule is right. In `confinement.md` and `collection-contract.md`, every numbered section binds except the readings tables. A comment line in the profile template is part of the template's text, though it grants nothing.

## 1. The subject file

- **QS1. One file.** A subject is registered by exactly one JSON file, whose schema is `schema/subject-file.schema.json`. The file states its own version in `subject_file_version`, which is `1.0` for this specification. A file with any other version is rejected before collection.
- **QS2. Strict reading.** The file is read under the judge contract's §3. Each of these rejects the file:
  - a repeated key;
  - a number written in a form that §3 refuses;
  - nesting past §3's depth;
  - ill-formed UTF-8.
  A value the schema does not allow also rejects the file. A rejected file stops the run before any check is fixed (QE5).
- **QS3. What it holds.** The file holds:
  - the subject's binary;
  - the judge's five subject facts (QS4);
  - the declared commands (QS6);
  - the declared command count (QD3);
  - the seed and the declared store (QS9);
  - offline settings (QS7);
  - bookkeeping paths (QP7);
  - the planted secret (QP10);
  - runbooks and their authored checks (§5);
  - expected-failure markers;
  - a `prompts` object reserved for steering, which this version reads only for its version string.
- **QS4. Subject facts.** The file states `read_only`, `has_destructive_ops`, `store_backed` and `has_cli`, each true or false, and `bounding`, which is `none` or `declared`. The engine passes them to the judge as given.
- **QS5. Placeholders.** A value the author has not supplied is a placeholder: an object with exactly one key, `placeholder`, whose value is a sentence saying what belongs there and why the grade needs it.
  - A placeholder is never read as false, `none` or empty.
  - `pql create` writes a placeholder for every value it cannot know, so the file explains itself without comments.
  - The rules of §1 that check a value's content (QS8, QS9) skip a placeholder.
  - A subject fact left as a placeholder reaches the judge as unstated.
  - Nothing that depends on a placeholder runs, and the check that needed it is not observed with `placeholder_unfilled`:
    - with `binary` a placeholder, nothing runs;
    - with `seed` or `store` a placeholder, nothing runs except discovery, whose home is empty anyway;
    - a command whose safety class or `writes` is a placeholder is never invoked, except for its help during discovery;
    - with `declared_count` a placeholder, discovery is not observed;
    - with `secret` a placeholder, the planted-secret probe is not observed.
  - In addition, any placeholder in the file makes every required check on an always-applicable dimension not observed with `placeholder_unfilled`, so the run cannot exit 0. A discovered command that the file does not declare does the same (QD3).
  - The scorecard names each placeholder by its location in the file.
- **QS6. Commands.** Each declared command gives:
  - `words`: the list of words naming it from the binary. The empty list declares the bare binary.
  - `class`: `safe`, `destructive`, or a placeholder.
    - A `destructive` command runs only in a scope that includes `destructive` (R16, R12).
    - A `safe` command runs under the subject's full confinement like any other. The class says only that running it is wanted.
  - `writes`: `false` when running the command with its offline settings changes nothing in its home outside the bookkeeping paths, `true` when it may, or a placeholder. Only a command with `writes: false` is run by the store-unchanged probe (QP6).
  - `exit_codes`: the exit codes that end a successful run, or `null`. When it is `null`, a run of the command succeeds only by exiting 0.
  - `byte_bound`: an output bound in bytes, or `null`.
  - `budget_seconds`: the time budget of every invocation of the command, a whole number of seconds.

  Two declared commands with the same words reject the file. So does an invocation, in a runbook, the secret's command list, or any probe, that has no declared command.
- **QS7. Offline settings.** The file may declare arguments and environment that make the subject run offline. Each setting lists the words of the commands it applies to.
  - A setting applies to an invocation exactly when it lists the words of the invocation's command. It applies the same way to every generated probe and every runbook step, and never to discovery (§3).
  - Its arguments are appended after the invocation's own words. When several settings apply, their arguments are appended in the order the settings appear in the file.
  - Rejecting conditions:
    - a setting that lists words no declared command has;
    - two settings that apply to one command and set the same environment name;
    - an argument equal to `--help`, `-h`, `help`, `--version`, `-v` or `version`.
  - A declared environment value is a literal string. The engine adds it to the allow-listed environment of QH12 and masks it in every record and scorecard (QE18). It enters check ids only as its digest (QE2).
  - Every scorecard shows each offline setting and the commands it applied to.
- **QS8. Files the file names.** A file the subject file names is the binary, an executable a computation names by a relative path (QA4), or a declared file (QA4). Each is named by a path relative to the directory that holds the subject file. After every link is resolved, each must be a regular file inside that directory. That directory is neither the user's home directory nor a directory above it. A path that leaves it, a directory, a link left unresolved, or a special file rejects the file.
- **QS9. The seed and the store.**
  - **The seed** is one directory, named by a path relative to the subject file's directory and inside it, or `null` for none.
    - The engine copies the seed by content into each throwaway home (QH14).
    - A seed that holds a link or a special file rejects the file before collection, as does a seed that supplies anything at or under `.pql-tmp` (QH12).
    - With no seed, every throwaway home starts empty.
  - **The declared store** is a path relative to the home naming the file the subject keeps its data in, or `null`. It must be a regular file the seed supplies, or the file is rejected.
  - **Paths relative to the home.** The store, every bookkeeping path, and the secret's path are paths relative to the home, never absolute.
- **QS10. Unique names.** Runbook names are unique in the file. Step names are unique within their runbook. A repeat rejects the file.
- **QS11. Bounds on size.** Each of these rejects the file before collection:
  - runbooks that hold more than 2,048 checks in all;
  - more than 512 declared commands;
  - a literal longer than 64 KiB in its canonical form, or nested deeper than 48.

## 2. What a run of the subject means

These rules turn one invocation of the subject into an outcome for the check it serves. `confinement.md` says how each condition below is seen. QO6 says how they apply to a computation.

**First match.**

- **QO1. Confinement not applied.** The check is not observed with `network_unverifiable` when any of these holds:
  - the engine could not apply confinement to the invocation (QH6);
  - the grade's self-test failed (QH9);
  - the system is not one that QH1 names.

  The subject is never run without confinement instead.
- **QO2. Could not start.** The launcher reported that the binary could not be started (QH7). The check is not observed with `could_not_start` (R22).
- **QO3. Killed by the confinement.** The invocation ended by `SIGKILL` that QH21 attributes to the confinement. The check is not observed with `network_unverifiable`. Its next action names the possible causes: a network request, a child process, another refused operation, or an external kill (QE12).
  - A child process the subject starts is refused with a kill, so it falls here (R16).
  - A bind or a listen on any socket, loopback included, is a network attempt and falls here.
  - This version never reports an invocation as not observed with `network_attempted`. The confinement cannot tell which refused operation ended the process, and `network_attempted` is reserved for a verifier that evidences the attempt itself.
- **QO4. Timed out.** The engine ended the invocation at its time budget.
  - The outcome is `not_held` with `over_budget` (R22, R33) when both of these hold:
    - the engine saw the process still running as it ended it;
    - the grade's known-fast control kept within its own bound on that host (QH24).
  - Otherwise the check is not observed with `harness_fault`.
- **QO5. Other ends.** Otherwise the invocation ran to its end under confinement.
  - A process ended by any other signal crashed: the outcome is `not_held` with `crashed` (R22). A process that exits with a code, a Go program's panic exit included, did not crash.
  - An exit code the check does not accept is `not_held` with `wrong_exit` (R22). The accepted codes are:
    - for an `exit_code` predicate, the check's expected code;
    - for the bare-invocation and declared-exit-code probes, as QP2 and QP4 say;
    - for every other check, the codes of a successful run of the invocation's command (QS6).
  - Every other result is judged by the check's own predicate, and a predicate that does not hold gives `predicate`.
- **QO6. A computation's own run.** A computation's run (QA4) changes only its check:
  - When the confinement was not applied to it, it was killed by its confinement, it could not start, or it ran past its budget, the check is not observed with `computation_failed`.
  - When QO1 holds for the grade, the check is not observed with `network_unverifiable` like every other check.

## 3. Discovery

- **QD1. Discovery is a required check.** Discovery is always in the required set, as one check. It reports the commands it found against the declared count, and the names it found and missed (R42).
- **QD2. How help is asked.**
  - **The request.** Discovery asks the binary for help with exactly one form, `<command words> --help`, starting from the bare binary and descending into each command it lists.
  - **Its run.** Each request runs under the subject's full confinement, in a throwaway home that starts empty whatever the seed, with no offline setting and no other argument (QS7). Each has a budget of 10 seconds.
  - **What is read.** Help output is read from standard output, and from standard error when standard output is empty. How a command list is read is as §5 of `credit-and-routes.md` says.
  - **A leaf.** A help that exits 0 and lists no command section belongs to a leaf command.
  - **What is counted.** Every listed command at every level counts, a command that has commands of its own included. The commands `help` and `completion`, and any command the help marks as hidden, are left out.
- **QD3. What discovery compares.** Discovery compares names as well as the count. It holds when every declared command was found and the number found equals `declared_count`. With every declared command found:
  - a different count is `not_held` with `predicate`;
  - each command found but not declared is named on the scorecard and has no safety class, so QS5 applies.
- **QD4. A command discovery did not find.** When a declared command was not found, discovery and every other generated check are not observed with `precondition_unmet`. None of them is red (R42). The scorecard shows both counts and the missing names.
- **QD5. Help that cannot be read.** Discovery is not observed with `precondition_unmet` in either of these cases:
  - a help request ends in any way other than exiting 0 with output;
  - the bare binary's help lists no command section while commands other than the bare binary are declared.

  A help request that is killed or times out falls under §2 first.
- **QD6. A stated limit.** A CLI that ignores a help request runs that command's action. It does so inside the subject's full confinement, in an empty home, so it can write only there and can reach nothing outside it.

## 4. The generated probes

- **QP1. Six probes, and what enters the required set.** This version generates exactly six probes: discovery (§3), bare invocation, byte bound, declared exit codes, planted secret and store unchanged. `credit-and-routes.md` §1 names the dimension each feeds, whether it credits, and when it is not observed. What enters the required set:
  - **Discovery, bare invocation and planted secret:** one check each.
  - **Byte bound, declared exit codes and store unchanged:** one check for each command the probe runs. When the probe runs no command, it still enters as one check with no command, not observed with `empty_target`.
  - **Every probe:** it enters the required set whether or not its inputs were supplied, so a missing input shows as a reason rather than as nothing.
- **QP2. Bare invocation.** The binary is run with no arguments. It holds when the process exits with a code the bare binary's declaration accepts (QS6). When the file does not declare the bare binary, the code must be 0 and the budget is 10 seconds.
- **QP3. Byte bound.** Each declared `safe` command with a `byte_bound` is run, one check each. It holds when the bytes drained from standard output and standard error together stay within the bound. The bound is enforced by draining the streams (QE10), never by trusting the subject.
- **QP4. Declared exit codes.** Each declared `safe` command with declared `exit_codes` is run, one check each. It holds when the exit code is one of them.
- **QP5. Fresh state for every probe.** Each invocation of a probe gets a fresh copy of the seed in its own throwaway home. With no seed the home is empty.

### The store-unchanged probe

- **QP6. What it compares.** The probe runs each declared `safe` command with `writes: false`. Before and after the command, the engine snapshots the throwaway home and compares the two snapshots. The comparison covers names and contents only: which paths exist, their kinds, and the bytes of each regular file. Times, modes and owners are not compared.
- **QP7. Bookkeeping paths.** The file may declare bookkeeping paths: paths relative to the home that the subject writes as a matter of course, such as a journal.
  - Changes inside a declared bookkeeping path, or inside `.pql-tmp` (QH12), are left out of the comparison. Every scorecard lists the paths that were left out.
  - A bookkeeping path at or above the declared store, or at or above any path the seed supplies, rejects the file before collection. A seed is therefore prepared without bookkeeping files.
- **QP8. Database side files.** Three side files beside the declared store are compared as follows:
  - `<store>-shm` is SQLite's shared-memory index and holds no data. It is left out of the comparison.
  - `<store>-wal` and `<store>-journal` are a change only when they exist with one byte or more after the command.
  - A store whose bytes changed is a change whatever the side files say.
- **QP9. When it credits.** The probe credits only after the engine has shown, on a separate control copy of the same seed, that its comparison detects a change. The engine appends one byte to the control copy's declared store and confirms the comparison reports it.
  - Without that control the probe is not observed with `harness_fault`.
  - When, after the exclusions, the pristine seed holds no regular file of one byte or more, the probe is not observed with `empty_target`.
  - A link or a special file the subject created anywhere in the home is a change. The engine never follows or opens one (QH25).

### The planted-secret probe

- **QP10. Where the secret lives.** The file's `secret` is `null`, a placeholder, or an object naming:
  - `path`: one file the seed supplies, relative to the home, which is where a real secret would be read;
  - `kind`: `credential` for a credential file the subject reads, or `seeded_state` for another file. A `seeded_state` secret carries a `reason` saying why reading that file proves the secret was read;
  - `token`: a string of 8 to 256 characters that appears exactly once in the seed's copy of that file;
  - `commands`: the words of one or more declared `safe` commands that read the file.

  Each of these rejects the file:
  - a `path` the seed does not supply;
  - a `path` equal to the declared store;
  - a `token` that does not appear exactly once.

  Before each invocation the engine replaces the token in the home's copy with a fresh canary and never prints the canary. The canary is `pqlcanary` followed by 32 lowercase hexadecimal digits from a cryptographic random source.
- **QP11. A proved read.** The probe runs each listed command once, each in its own home. An invocation proved a read when all of these hold:
  - it finished with a code of a successful run of its command (QS6);
  - the planted file's access time moved;
  - the file's device and inode are unchanged;
  - an otherwise identical control invocation was killed (QH27).

  The outcome:
  - a leak in any invocation is `not_held` with `predicate`;
  - otherwise the probe holds when at least one invocation proved a read;
  - otherwise it is not observed with `input_missing`, and so is a probe whose `secret` is `null`.

  A secret that only an environment variable can carry has no file to plant, so the probe is not observed with `input_missing`.
- **QP12. What a leak is.** A leak is the canary, plain or in any encoding `credit-and-routes.md` §4 lists, found in either of these places:
  - a drained stream;
  - any file of the used home other than the planted one, bookkeeping paths and `.pql-tmp` included.
- **QP13. The scan's bounds.** The leak scan reads each regular file of the used home up to 16 MiB and at most 256 MiB in all, and each drained stream up to the bound of QE10. A scan that reached any of these bounds without finding a leak is not observed with `harness_fault`, and is never held.
- **QP14. Stated limits.** The specification states that:
  - another process reading the planted file during the invocation looks like the subject's read;
  - a read that returns no bytes at the end of the file moves its access time and counts as a read;
  - a leak through the system's unified log is not seen, because the logging service the subject may reach writes outside its home;
  - on a Mac bound to a directory server, a user or group lookup can reach that server.

## 5. Authored checks and expected values

- **QA1. Observed values.** Every observed value comes from the engine running the subject's binary under the subject's confinement, from the private copy QH8 describes. Nothing is read from the subject's store in place of running it.
- **QA2. Expected values.** An expected value is either a literal in the file, or the one JSON value printed by a computation (QA4).
- **QA3. Crediting.** Whether a check credits is fixed from the subject file before collection. An authored check credits unless one of these holds:
  - its expected value or its precondition uses any of the subject's captured output, which is self-agreement;
  - its dimension is one that only a probe may credit;
  - it fails a credit condition that `credit-and-routes.md` §2 states for its dimension.

  A non-crediting check can still fail.
- **QA4. A computation.** A computation is an argument list run without a shell, with a budget, its declared files and its captures.
  - **The executable.** The first argument names it. It is either a path relative to the subject file's directory (QS8), or an absolute path under `/bin` or `/usr/bin`.
  - **Its other arguments** are literal strings, except that the engine replaces three tokens wherever they appear:
    - `{SEED}` with the absolute path of the pristine seed directory;
    - `{SUBJECT_DIR}` with the absolute path of the subject file's directory;
    - `{CAPTURE:<step>}` with the absolute path of a file in its scratch directory holding the drained standard output of the named step.
  - **Captures.** A computation may name only an earlier step of its own runbook in a capture. Each such step must be listed in `captures`, and any other name rejects the subject file.
  - **What it may read.** It may read its executable, its declared files, each file of the pristine seed by its path, and its captures. It cannot list the seed's directories, and it may write only in its scratch directory, which is also its working directory and its `HOME`.
  - **Its run.** It reads standard input from `/dev/null`. It runs under the computation's confinement (QH16 to QH20), with no network and its own `budget_seconds`, and it may start child processes.
  - **Its result** is what it prints on standard output. Standard error is kept for the scorecard and read for nothing else.
- **QA5. Someone else's file.** The documentation and every `pql` command that grades a subject file warn that grading another person's subject file runs their computations on this machine.
- **QA6. A computation that fails.** The check is not observed with `computation_failed` when the computation:
  - fails or is absent;
  - is ended as QO6 says;
  - exits with a code other than 0;
  - prints anything other than exactly one JSON value that the judge contract's §3 accepts inside a record.
- **QA7. Runbooks.** A runbook's steps run in the order written.
  - **Each step:**
    - its invocation's time budget is the step's `budget_seconds`, and QO4 decides a step that times out;
    - its checks all judge that one invocation.
  - **Shared state.** All steps share one throwaway home holding one copy of the seed.
  - **After a failure.** A step is not held when any of its checks is not held or not observed. Every check of every later step is not observed with `earlier_step_failed` (R33).
- **QA8. The destructive-refusal check.** The check that a destructive command refuses without its confirmation is authored, and runs only in a scope that includes `destructive` (R12).
- **QA9. Predicates.** An authored check uses one of these predicates on its step's invocation. Every predicate reads the invocation's drained bytes, never the text decoded for the record (QE10).

  | Predicate | Holds when | Empty target satisfies it | Asserts emptiness |
  |---|---|---|---|
  | `exit_code` | the exit code equals the expected integer | no | no |
  | `stdout_equals` | standard output's bytes equal the expected string's UTF-8 bytes | no | no |
  | `stdout_json_equals` | standard output is one JSON value, read under the judge contract's §3, equal to the expected value, numbers compared exactly | no | no |
  | `stdout_contains` | standard output contains the expected string's UTF-8 bytes | no | no |
  | `stdout_not_contains` | standard output does not contain the expected string's UTF-8 bytes | yes | no |
  | `stderr_contains` | standard error contains the expected string's UTF-8 bytes | no | no |
  | `stdout_bytes_at_most` | the number of bytes drained from standard output is at most the expected integer | yes | no |
  | `json_path_equals` | standard output is one JSON value and the value at `path` equals the expected value | no | no |
  | `row_count` | the array at `path` has the expected number of items | when the expected value comes from a computation | when the expected value is the literal 0 |
  | `stdout_empty` | standard output is empty | no | yes |

  - **`path`** is a JSON Pointer (RFC 6901), and is `null` for a predicate that takes none. A `path` that names nothing makes the predicate not hold.
  - **An empty target.** A predicate an empty target would satisfy reports the target's size: the number of bytes, or of array items, it evaluated.
  - **A precondition.** A check that asserts emptiness has a `precondition`: a computation over the pristine seed that must print `true` to show the data exists. It reaches the judge as QE7 says. A check whose predicate does not assert emptiness has none, and one given anyway rejects the file.
  - **A cut stream.** A predicate other than `stdout_bytes_at_most` that needs a stream the engine cut (QE10) is not observed with `harness_fault`.

## 6. Readings of the requirements

| Input | Read as | Rule |
|---|---|---|
| R16 asks for no network request, and macOS can only refuse one with a kill | A kill the engine cannot attribute is not observed with `network_unverifiable`, never red; `network_attempted` is not reported in this version | QO3 |
| R22 makes a crash red, and the confinement's kill is a crash to the process | A kill by the confinement is not the subject's failure | QO3 |
| R22 makes a timeout red | Only when the host kept up, shown by a known-fast control | QO4 |
| R42 asks for a count, and a CLI can ignore a help request | Help is asked under full confinement in an empty home, and the risk is a stated limit | QD2, QD6 |
| R42 compares a count, and discovery may miss a declared command | A missing declared command leaves discovery and every generated check not-measured, never red; a count that differs with every declared command found is red | QD3, QD4 |
| R33 stops a runbook after a failed step | A step that was not observed stops it too | QA7 |
| R16 asks that the subject makes no network request, and a loopback bind reaches no network | A bind or listen is an attempt | QO3 |
| A computation is the author's oracle, not the subject | A computation that its confinement ends fails the check's expected value, `computation_failed`, and never stands for the subject's network attempt | QO6 |
