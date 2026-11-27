# Conformance collection specification 1.0: errata and notes

The files in `conformance-collection-1.0/` are sealed and are not edited. The version policy in `collection-contract.md` (QE1) lets an erratum correct only text whose intended reading the corpus or a worked example already fixes; any other change to a binding rule is a new version. This is that note.

Nothing here changes what a collector records for any case of the corpus or which exit code the engine ends with. Where an entry and a binding rule appear to differ, the rule is right.

The specification was sealed on 2026-11-24.

## 2026-11-24

### E-1. The store as a target of a store-unchanged check that runs no command

QE8 says: "When the declared store is a path, the store-unchanged probe adds it as a target of kind `artifact`", and gives an artifact "the tier of the invocation that used it". It does not say what happens when the probe runs no command (QP1: no declared `safe` command has `writes: false`), so the probe enters the required set as one check with no command, not observed with `empty_target`, while the store is still a path.

The owner settled the reading on 2026-11-24: **such a check has no artifact target.** The store is a target only of a store-unchanged check whose invocation used it, so a check that runs no command has only the one invocation target QE8 gives a check that would use no invocation.

No case of the corpus reaches this input, so this entry records the owner's reading rather than one the corpus fixes. A later version of the specification states it in the text. The corpus's checks (`scripts/check-case-ids.py`) already take this reading.

### E-2. Who assembles a case

QT6 says that before applying §1 of `collection-rules.md`, "a collector assembles one temporary directory" holding the subject file, `seed/`, `bin/subject`, `computations/` and the case's fixtures. QT1 gives `grade` no case or fixture flag, so the text can be read two ways: the corpus harness assembles the directory and runs `grade` on its subject file, or `grade` itself takes a case directory.

The owner settled the reading on 2026-11-24: **the harness assembles the case.** `grade` takes only what QT1 and the rest of the specification name, and no case concept reaches the released binary. The assembled directory is the one QT6 describes, so the record a collector produces for each case is unchanged.

### E-3. Acceptance examples the corpus does not reach

The engine plan cites these acceptance examples as proof of the store and baseline slice, and no case of the corpus reaches them:

- AE16: no case is both regressed and missing entries.
- AE19: anchoring with a known defect and anchoring with an accepted regression. The judge refuses any anchor of a scripted run (QT2, judge rule N1), and the only real anchor case, `case-143`, anchors the known-good form.
- AE31: a baseline recorded under an older rulebook version. `case-134` reaches `not_compared` through a tier mismatch instead.
- AE33: anchoring with a retired list.

This is a finding about coverage, not a correction: every rule these examples exercise is stated in the text. The owner decides, before the baseline slice is built, whether a collection specification 1.1 adds cases for them.

## 2026-11-25

The owner settled these readings on 2026-11-25, for the engine's phase 3 build. No case of the corpus reaches any of these inputs, so each entry records the owner's reading rather than one the corpus fixes. A later version of the specification states each one in its text.

### E-3, decided

The owner decided that **no collection specification 1.1 is written for phase 3.** The baseline slice proves what version 1.0's cases reach. Engine tests cover the rest of the rules AE16, AE19's anchoring parts, AE31 and AE33 exercise, and the slice's pull request names what the corpus leaves unproved.

### E-4. A store write that fails after the judge has decided

QE14 takes the exit code from the judge's output, and QE16 makes an engine fault exit 70. **When the record cannot be stored after the judge has decided, the run exits 70.** The run did not complete, because a verdict that was never stored cannot be compared or anchored later. The scorecard still prints, with a note that the record was not stored.

### E-5. A rejected record and the record schema

QE15 keeps a record the judge rejects, and `record.schema.json` nests the judge input's schema. A record the judge rejected for breaking that schema therefore cannot validate as a stored record. **QE15 wins:** the record is stored with `judge_rejected: true` and need not validate against `record.schema.json`.

### E-6. A run ended by a signal

QE16's table names no code for a run that a signal ends. **Such a run is an engine fault and exits 70,** after the engine has ended every live process group and cleaned up.

### E-7. A requested scope the engine does not know

The command line takes `--scope conformance[,destructive]`, and QE6 names X1, the requested scope, as a rejection the user caused. **The command line passes any comma-separated scope tokens to the judge.** The pre-check rejects an unknown one under X1, and the run exits 2. Exit 64 stays for a command line that cannot be parsed: an unknown flag, a missing value, a stray argument.

### E-8. The user's home directory in QS8

QS8 rejects a subject directory that is "the user's home directory" or above it. **That home is the account's home from the system's user record, not `$HOME`.** Tests and users can set `$HOME` anywhere, and the account's home is the one QS8 protects.

### E-9. A subject file that breaks two named rules

QT6 names the deciding rule of a derivation-only case, and no case breaks two at once. **The engine checks the named rules in this order, QS1, then QS3 to QS12, then QP7, QP10, QA4 and QA9, and names the first that fails.** QS2 is named only when none of these is broken, since QT6 names the rule whose condition the file breaks even when the file's schema also rejects it.

### E-10. An entry the engine cannot read

A file in a used home can stay unreadable after the engine has safely reset its mode. **The leak scan or store-unchanged check that needed it is not observed with `harness_fault`,** by analogy with QP13's bound. It never ends as held or unchanged.

### E-11. A cleanup that cannot finish

QE20 removes the per-run directories on every exit. **When an entry cannot be removed, the run exits 70.** The scorecard names the directory left behind, and the next run's sweep retries it.

### E-12. QA6's depth limit

QA6 bounds the depth of a computation's printed value "inside a record". **The depth counts the record's nesting around the value,** not only the computation's own output.

### E-13. Stated limit: writes inside a home

Nothing in version 1.0 bounds how much a subject or a computation writes inside its home, so a subject can fill the volume within its time budget. **This is a stated limit of version 1.0,** named in the engine's documentation, and a candidate rule for a later version.

### E-14. Stated limit: base64 alignment in the leak scan

`credit-and-routes.md` §4 lists base64 forms of the canary alone. A canary inside a larger base64 value, at an offset not divisible by three, matches none of them; one example is a Basic authorisation header written to a log. This can give a false green on the planted-secret check, and can leave a declared value unmasked. **This is a stated limit of version 1.0,** named in the planted-secret check's documentation. Matching all three alignments is a candidate rule for a later version.

### E-15. QO1 and a check that runs nothing

QO1's grade-wide conditions, a failed self-test (QH9), confinement that could not be applied, and a system QH1 does not name, can hold while a required check runs neither the subject nor a computation: a store-unchanged or byte-bound probe with no command (`empty_target`), or a planted-secret probe whose `secret` is `null`. §2 tries QO1 "for any check", and QO6 says a check is not observed "like every other check" when QO1 holds for the grade. QH1 and QH9 speak only of "every check that runs the subject", and QH6 of "every check that needed it". **When a grade-wide condition of QO1 holds, every required check is not observed with `network_unverifiable`, including a check that runs nothing.** The owner ruled on 2026-11-25 that when QO1 holds for the grade, every required check is not observed. No case of the corpus has such a check under a grade-wide condition, so no record changes.

### E-16. A version that is not a string

QS1 says the file states its version in `subject_file_version`, "which is `1.0` for this specification", and the specification calls its version "this string". **A `subject_file_version` that is not the string `1.0`, the number `1.0` included, is another version and is rejected under QS1,** which E-9 checks before QS2. The owner ruled on this reading on 2026-11-25. No case of the corpus has such a file, so no record changes.

### E-17. A link in the path that names the seed

QS9 names the seed by a path "inside" the subject file's directory, and a path is inside a directory "after every link in the path is resolved" (§1's definitions). QS9 also rejects a seed that "holds a link". **A link in the path that names the seed, the seed's own name included, is resolved, as are the links in the subject file's own directory path, before "inside" is tested. The resolved seed must be a directory inside the subject file's directory. Of links on the seed's path or within the seed, only a link the seed directory contains rejects the file under QS9. QS9's other rejections are unchanged.** The owner ruled on 2026-11-25. No case of the corpus has a link in a seed's name, so no record changes.

### E-18. A named file the engine cannot read

QS8 rejects a named file that leaves the subject file's directory, a directory, a link left unresolved or a special file, and QE16 lists "an unreadable subject file" under exit 2. Neither names a regular file inside the directory that cannot be read. **A binary, a computation's executable named by a relative path, or a declared file (QS8's three kinds) that is a regular file inside the directory but cannot be read rejects the subject file under QS8, with exit 2, before collection. A file or directory inside the seed that cannot be read rejects it under QS9, with exit 2.** The owner ruled on 2026-11-25. No case of the corpus has such a file, so no record changes.

### E-19. A stale run directory the start sweep cannot remove

QE20 removes, at start, every per-run directory a killed run left. E-11 makes a run exit 70 when its own cleanup cannot remove an entry, and says the next run's sweep retries it. Neither says what the new run does when that retry fails again. **The sweep tries every stale directory. One it cannot remove is named on standard error and left for a later sweep, and the new run goes on. The new run ends with exit 70 only when the directory that holds the run directories is unsafe, or when it cannot tell whether a recorded process still exists.** The owner ruled on 2026-11-26. No case of the corpus reaches a start sweep, so no record changes.

### E-20. A planted-secret token that overlaps itself

QP10 requires the planted secret's `token` to appear exactly once in the seed's copy of its file, and does not say how an occurrence that overlaps another is counted. **Each position at which the token starts is one occurrence, so a token that overlaps itself, such as `aaaaaaaa` in nine `a`s, appears more than once and rejects the file under QP10.** The owner ruled on 2026-11-26. No case of the corpus has such a token, so no record changes.

### E-21. The rule a reserved environment prefix names

QH12 rejects a subject file whose offline setting sets an environment variable whose name begins `DYLD_` or `__CF_`. E-9's order of named rules does not list QH12, and QS7, the rule for offline settings, cites QH12 for its reserved names. **Such a refusal names QS7, with exit 2, in QS7's place in E-9's order.** The owner ruled on 2026-11-26. No case of the corpus sets such a variable, so no record changes.

### E-22. A placeholder-shaped value inside a literal

QS5 defines a placeholder by its shape and rejects one outside its allowed positions. A `literal` is the output the author expects the subject to produce, and may be any JSON value. **An object inside a `literal` is data, never a placeholder, whatever its shape: it does not reject the file, and no rule treats it as a value the author has not supplied. Placeholder detection looks only at the positions QS5 allows.** The owner ruled on 2026-11-26. No case of the corpus has such a literal, so no record changes.

### E-23. An absolute executable the engine cannot read

QS12 compares the content of "an executable a computation names" with the binary's, and QA4 lets that executable be an absolute path under `/bin` or `/usr/bin`. E-18 covers a file the engine cannot read only for QS8's kinds, which are relative. **An absolute executable that exists but that the engine cannot read as a regular file, such as an execute-only file, rejects the subject file under QS12, with exit 2, because the engine cannot show that its content differs from the binary's. One that does not exist is not compared, and its computation fails at run time under QA6.** The owner ruled on 2026-11-26. No case of the corpus names such an executable, so no record changes.

### E-24. A subject file that is a link

QS8 names files relative to "the directory that holds the subject file", E-17 resolves the links in that directory's own path, and QE4 keys baselines by "the subject file's canonical path". None says what happens when the path given is itself a link. **Every link in the subject file's path, the file's own name included, is resolved once. The directory of that resolved path is the directory that holds the subject file, for QS8, QS9, QA4's `{SUBJECT_DIR}` and every other rule, and the resolved path is the canonical path QE4 uses.** The owner ruled on 2026-11-26. No case of the corpus passes a link as its subject file, so no record changes.

### E-25. Text that cannot be a capture token

QA4 replaces `{CAPTURE:<step>}`, and a capture that names a step other than a listed earlier one rejects the subject file. The schema's step names exclude `{`, `}` and `:`. **A capture token is `{CAPTURE:`, then text that matches the schema's step-name pattern, then `}`. Any other text, an unterminated `{CAPTURE:` included, is literal and left as written. A token whose name is not a listed earlier step of its runbook rejects the file under QA4.** The owner ruled on 2026-11-26. No case of the corpus has such text, so no record changes.

### E-26. A help with no recognised heading below the bare binary

`credit-and-routes.md` §5 says "A help with no section heading belongs to a leaf command (QD2). Every other form, a man-page style help included, is not read", and QD5 makes discovery not observed when a help cannot be read. A man-page style help without a recognised heading also has no section heading. At the bare binary both readings give QD5. Below it they differ. **Below the bare binary, a help request that exits 0 with output and has no section heading belongs to a leaf command. A declared command below it is therefore not found, and QD4 decides. At the bare binary, QD5's second case decides as it says. §5's heading test is applied to the output the engine kept, and it classifies every such help: one with a section heading is read, and one without belongs to a leaf. No such help is left unread for want of a heading.** The owner ruled on 2026-11-26. No case of the corpus has a headingless help below the bare binary with a declared command beneath it, so no record changes.

### E-27. Where a help line's description starts

§5 makes a command line hidden when its "description begins `(hidden)`, `[hidden]` or `HIDDEN`", and takes the command from the line's first word, with a trailing comma removed. It does not say where the description starts. Some generators print aliases after the name, as in `  account, a   Show the account`. **A command line's description is the text after the first run of blanks that follows its first word and is either two or more blank characters long or holds a tab. A blank is a space (U+0020) or a tab (U+0009). A line with no such run has no description, and is not hidden.** The owner ruled on 2026-11-26. No case of the corpus has an aliased hidden command, so no record changes.

### E-28. A signal before the launcher's marker

QH6 says confinement was not applied when no marker was written and either `sandbox-exec` itself exited or the launcher was ended by `SIGKILL` before writing its marker. QO5 decides an invocation that "ran to its end under confinement". Neither says what an invocation ended by another signal before the marker is. **An invocation of the subject that ends by any signal before the launcher's marker was written is QH6's case: the confinement was not applied, and QO1 decides, with `network_unverifiable`. QO5 decides only invocations whose marker was written. A computation's run is QO6's, which this entry does not change.** The owner ruled on 2026-11-26. No case of the corpus has such a run, so no record changes.

### E-29. QO1 against QO7's order for discovery

The precedence table puts QO1 first, for any check an invocation of which the engine could not confine. QO7 says that for each of discovery's help requests "QO1 to QO4 decide as for any invocation", and the first such request in QD2's order decides discovery. **When the engine could not confine one or more of discovery's help requests, QO1 decides discovery at level 1, ahead of a placeholder (level 2) and ahead of an earlier request that QO2 to QO4 decide. QO7's "first such request" then orders only the requests QO2 to QO4 decide.** The owner ruled on 2026-11-26. No case of the corpus has an unconfined help request, so no record changes.

### E-30. QD4's second sentence when a bound or QD5 holds

QD4's second sentence lets the other generated checks stand on their own invocations "when QD5 or a bound of QD2 decides discovery", even when the walk also missed a declared command. Discovery's own check can be decided earlier, at level 2 when `declared_count` is a placeholder. **QD4's second sentence applies whenever a bound of QD2 stopped the walk or either case of QD5 holds for any help request, whatever level decides discovery's own check. A help that QD5 makes unreadable still lists the commands it names, and the walk still descends below it (QD2). QD4's first sentence applies only when neither holds.** The owner ruled on 2026-11-26. The owner confirmed this wording on 2026-11-26, after a design review showed that an unreadable help does not stop the walk. No case of the corpus has a placeholder `declared_count` with a bound or a case of QD5, so no record changes.

### E-31. A recorded run whose marker and attribution disagree

QT5 records, for each run, whether the launcher wrote its marker, and for a `SIGKILL` what ended it, one value being `before_marker`. **A recorded run whose attribution is `before_marker` while its marker is recorded as written, or `confinement` while no marker is recorded, contradicts itself. This holds for every run of the recording, the first confined run that a scripted run reads and the others alike. The recording is invalid, and the scripted run exits 2 as QE16 says for a recording that is invalid.** The owner ruled on 2026-11-26. No run of the corpus has such a pair, so no record changes.

### E-32. A help stream past the engine's bound

QE10 keeps the first 1 MiB of each stream and records that a longer one was cut, and QT5 records a cut stream's first 64 KiB as `head`, with `kept` null. QD2 reads help from standard output, or from standard error when standard output is empty. **When the stream discovery would read from a help request was cut, that help cannot be read: QD5 decides as for any help that cannot be read, on the released and the scripted path alike, and nothing is read from the kept bytes or from `head`. A stream is empty for QD2's choice, and a help has output for QD5, by the bytes the program wrote, never by the bytes kept.** The owner ruled on 2026-11-26. No case of the corpus has a cut help stream, so no record changes.

### E-33. A command its help lists twice

QD2 says "Every listed command at every level counts", and QD3 compares "the number found" with `declared_count`. **A command is found once, however many times its parent's help lists it, by its words. The count and the walk take each command once, in the order of its first listing, so its help is requested once.** The owner ruled on 2026-11-26. No case of the corpus lists a command twice, so no record changes.

### E-34. A listed name the engine cannot pass as an argument

A command's name is the first word of its help line (§5), so a help can list a name holding a NUL byte or ill-formed UTF-8, or one whose help request's arguments pass the system's limit. Such a request cannot be made. **A help that lists such a name cannot be read: QD5 decides as for any help that cannot be read. The name is never requested. The help's other listed commands are walked, as QD2 walks below any help that ran. The limit is fixed, the same on every host and on the released and scripted paths: a help request's words, the name and the closing `--help` included, may total at most 131,072 bytes (128 KiB), counting each word's UTF-8 bytes and one byte per word. A request within it fits the argument limit of every supported macOS, so a scripted recording replays alike on any host.** The owner ruled on 2026-11-26, and on the fixed limit the same day. No case of the corpus has such a name, so no record changes.

### E-35. The planted-secret probe and an engine kill outside the budget

QO7 combines the planted-secret probe's invocations, first match, and says that an exit code other than a successful one, or a crash, only means that invocation proved no read. QO5 makes an engine kill outside the time budget `harness_fault` for one invocation. QO7's list does not name it. **When the engine killed one of the probe's invocations outside its time budget, that invocation was not fully observed. After step 1 (a leak found in any invocation) and before step 2, the check is not observed with `harness_fault`.** The owner ruled on 2026-11-26. No case of the corpus has such a kill, so no record changes.

### E-36. QO1 against a leak in the planted-secret probe

The precedence table puts QO1 first, and E-29 lifts an unconfined help request to level 1 for discovery. QO7's combination for the planted-secret probe lists a leak found in any invocation first. **When the engine could not confine one or more of the planted-secret probe's invocations, QO1 decides the check at level 1, with `network_unverifiable`, ahead of the combination and ahead of a leak found in another invocation. QO7's step 2 then orders only the invocations QO2 to QO4 decide.** The owner ruled on 2026-11-26. No case of the corpus has an unconfined planted-secret invocation, so no record changes.

### E-37. How the read proof's control invocation ends

QO7 says that when the read proof's control invocation "ends any way other than a kill after the marker", the planted-secret check is not observed with `harness_fault`. QP11 and QH27 prove a read only when the control "was killed". QO1 and E-36 speak of the probe's listed invocations, and QE8 says a control is never a target. **The control proves the read only when QH21 attributes its end to the confinement. Every other end of the control is QO7's "any other way", so the check is not observed with `harness_fault`. That includes a control that wrote no marker, a `SIGKILL` with a memory or code-signing exit detail, and a kill the engine sent. QO1 and E-36 apply to the probe's listed invocations, never to the control.** The owner ruled on 2026-11-26. No case of the corpus has such a control end, so no record changes.

### E-38. A failed read calibration and the read proof's control

QH26 says that when the volume's read calibration fails, no invocation of a planted-secret probe proves a read, and QO7's earlier steps still decide a leak, an invocation QO1 to QO4 decide, or a scan bound, "and otherwise the probe is not observed with `input_missing`". It does not mention the read proof's control. **When the read calibration failed, the read proof's control is not run, and its result cannot decide the check: QO7's leak, QO1 to QO4 and bound steps decide, E-35 and E-36 included, and otherwise the check is not observed with `input_missing`.** The owner ruled on 2026-11-26. No case of the corpus combines a failed calibration with a control that was not killed, so no record changes.

### E-39. The width of a masked declared value

QE18 masks each declared environment value of 8 bytes or more: "a declared value becomes as many `*` as its encoded form has characters". **The count is of Unicode code points in the encoded form. The 8-byte threshold stays a count of bytes.** The owner ruled on 2026-11-26. No subject of the corpus declares a value of 8 bytes or more, so no record changes.

### E-40. Stated limit: reading another process's scheduling priority

QH9 lists "its scheduling priority" among the routes to another process that the self-test shows refused with a kill. On macOS 27.2, under both profiles, changing another process's priority is refused with a kill, but reading it is permitted. **QH9's priority route is the change of another process's scheduling priority, which the self-test shows refused. The specification states the limit that a confined program can read another process's scheduling priority, and so learn its nice value and whether a process with that id exists. The profile is unchanged.** The owner ruled on 2026-11-26. No record changes.

### E-41. A side file the seed supplies

QP8 says `<store>-wal` and `<store>-journal` "are a change only when they exist with one byte or more after the command", and QP6 compares every regular file's bytes before and after. A seed copied from a database in write-ahead mode can supply a non-empty `-wal`. **QP8 adds a condition to QP6's comparison and does not replace it. A `-wal` or `-journal` is a change only when QP6 finds it changed and it holds one byte or more after the command. A side file the seed supplies and the command leaves unchanged is not a change, and neither is one the command empties.** The owner ruled on 2026-11-26. No seed of the corpus supplies a side file, so no record changes.

### E-42. A command listed twice in `secret.commands`

QP11 runs "each listed command once", QE8 numbers the planted secret's targets by listing position, and QT5 forbids two invocations with equal keys. The schema does not forbid listing a command twice. **A `secret.commands` that lists the same words twice rejects the subject file under QP10, with exit 2, in QP10's place in E-9's order.** The owner ruled on 2026-11-26. No case of the corpus lists a command twice, so no record changes.

### E-43. Stated limit: a canary in a name

QP12 finds the canary in a drained stream or in the contents of a file of the used home. **The specification states the limit that a canary written into a file's or a directory's name, or into a link's target, is not a leak QP12 finds: only stream bytes and regular files' contents are scanned. Masking (QE18) still covers names wherever a record or scorecard shows them.** The owner ruled on 2026-11-26. No record changes.

### E-44. Overlapping masked values

QE18 masks the canary and each declared value of 8 bytes or more, in every §4 encoding. It does not say what replaces two matches that overlap. **Masking first finds every match of every masked value's forms in the text, then replaces every byte any match covers. A byte that a canary form covers becomes the byte of the zero canary in the same form and position. A byte that a declared value's form covers becomes `*`, one for each code point, as E-39 says. Nothing of any masked value survives an overlap. A byte that both a canary form and a declared value's form cover becomes the zero canary's byte.** The owner ruled on 2026-11-27, and on a byte both cover the same day. No case of the corpus has overlapping masked values, so no record changes.

### E-45. Stated limit: extended attributes and resource forks

QP12 finds the canary in a drained stream or in the contents of a regular file of the used home, and E-43 states the limit for names and link targets. **The specification also states the limit that a canary written into a file's extended attribute or its resource fork is not a leak QP12 finds.** The owner ruled on 2026-11-27. No record changes.

### E-46. An inspection that cannot finish

QH25 inspects a used home without following a link, opening a special file or reading past a bound, and E-10 makes a file it cannot read `harness_fault`. **A directory of the used home the inspection cannot read, and an inspection or a leak scan that reaches its entry or time bound, make the check that needed it, the store-unchanged or the planted-secret check, not observed with `harness_fault`. That check never ends unchanged or held, and any leak already found still decides first (QO7, step 1). The other checks of the grade go on.** The owner ruled on 2026-11-27. No case of the corpus has such a home, so no record changes.

### E-47. A change found beside a gap in the inspection

E-10 and E-46 make the store-unchanged check `harness_fault`, never unchanged, when an entry cannot be read or an inspection cannot finish. They do not say what a change found elsewhere in the same home decides. **A change the comparison actually finds decides the check as not held with `predicate`, whatever else could not be read. Only an entry both snapshots inspected can show a change: when either snapshot's walk did not finish, an entry one of them never reached is neither a change nor unchanged. With no change found, an unreadable entry or an unfinished inspection gives `harness_fault`.** The owner ruled on 2026-11-27. No case of the corpus has such a home, so no record changes.

### E-48. A computed expected value of the wrong type

QA9's "Matching types" makes a literal of the wrong type reject the file, and its "Whole numbers" bullet makes a computation that supplies anything other than a whole number for `exit_code`, `stdout_bytes_at_most` or `row_count` `computation_failed`. Nothing says what a computation that supplies a non-string for a string-typed predicate decides. **A computation that supplies the expected value of `stdout_equals`, `stdout_contains`, `stdout_not_contains` or `stderr_contains` and prints anything other than a JSON string is `computation_failed`, as QA6 says.** The owner ruled on 2026-11-27. No case of the corpus has such a computation, so no record changes.

### E-49. A step with no check in the required set

QA7 runs a runbook's steps in the order written, in one shared home, and QA9 says a check left out of the required set is not evaluated and never stops a runbook. They do not say whether a step all of whose checks are outside the required set runs. **Such a step still runs in its place, so the shared home and any capture of it are what the author wrote. It is never held or not held, so it never stops the runbook. Each of its checks stays outside the required set and is not evaluated.** QS5 still applies: a step whose command depends on a placeholder does not run. The owner ruled on 2026-11-27. No case of the corpus has such a step, so no record changes.

### E-50. An empty expected string

QA9's table fixes, for each predicate, whether an empty target satisfies it. The schema lets a literal expected string be empty, and for an empty string the table's "Holds when" column contradicts its "Empty target satisfies it" column:
- `stdout_equals` and `stdout_contains` would hold on an empty output;
- `stdout_not_contains` would never hold;
- `stderr_contains` would hold on an empty standard error.

**For `stdout_equals`, `stdout_contains`, `stdout_not_contains` and `stderr_contains`, a literal expected value that is the empty string rejects the subject file, as QA9's "Matching types" says for a value of the wrong type. A computation that supplies the empty string for one of them is `computation_failed`, as E-48 says.** An empty output is asserted only by `stdout_empty`, with its precondition. The owner ruled on 2026-11-27. No case of the corpus has an empty expected string, so no record changes.

### E-51. A capture of a step that depends on a placeholder

QS5 says nothing that depends on a placeholder runs, and its list names no captures. QA4 lets a computation capture an earlier step's standard output. **A check whose computation captures a step whose command depends on a placeholder depends on that placeholder, so it is not observed with `placeholder_unfilled` (QS5, level 2). This holds whether the captured step has a required check of its own, which then stops the runbook, or has none and does not run (E-49).** The owner ruled on 2026-11-27. No case of the corpus has such a capture, so no record changes.
