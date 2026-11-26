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
