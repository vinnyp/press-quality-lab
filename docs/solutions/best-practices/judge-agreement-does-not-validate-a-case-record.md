---
title: A case that agrees with the judge can still hold a wrong record
date: 2026-11-24
category: best-practices
module: docs/specs/conformance-collection-1.0
problem_type: best_practice
component: testing_framework
severity: high
applies_when:
  - Writing or changing a judge-checked case of the conformance collection corpus
  - Writing private cases or catalogue entries in the public case format
  - Reading a green `cases` step of check.sh as evidence that the corpus is right
tags: [corpus, test-oracle, judge, recordings, subject-file, case-mode]
---

# A case that agrees with the judge can still hold a wrong record

## Context

A judge-checked case of the conformance collection corpus holds two things: `input.json`, the record a collector must send, and `expect`, the judge's output on that record. `./check.sh` (step `cases`) runs the real judge on `input.json` and compares its output with `expect`. That proves only the judge's step from record to verdict. It says nothing about whether the record is the one a collector following the text would produce from the case's subject file, form recording and host.

While building the corpus for PR #12, every case agreed with the judge, and three kinds of wrong case still passed:

- **Outcomes nobody read from the recording.** The authoring script took every outcome it was not told otherwise as held. Two leak cases stated one red check, but their form writes a debug log under `.cache/widgets` on every `account` run, so the store-unchanged check on `account` was also red. The judge cannot see this: it scores whatever outcome the record states.
- **Fields the judge ignores.** QO5 says an `exit_code` check that sees another code is `not_held` with `wrong_exit`; one case said `predicate`. Every verdict is the same either way, so judge agreement cannot catch a wrong `failure_kind`, but QE19 scores it.
- **Subject files the engine would reject.** Two subject files had a runbook step with an empty `checks` list, which `docs/specs/conformance-collection-1.0/schema/subject-file.schema.json` forbids. A third gave an integer to `stderr_contains`, which QA9 rejects. A collector would exit 2 before collection, so the expected record could never exist. Case mode validated `case.json` and `input.json` but not the subject file.

## Guidance

Check each part of a case against its own authority, not only against the judge:

- **The subject file against the schema.** Case mode now validates a judge-checked case's subject file against `docs/specs/conformance-collection-1.0/schema/subject-file.schema.json` (PR #12, pending merge). QA9's type rules (a whole number for `exit_code`, `stdout_bytes_at_most` and `row_count`; a string for the four string predicates; `null` for `stdout_empty`; `path` only for `json_path_equals` and `row_count`) are not all in the schema, so check them too.
- **Every stated outcome against the recording.** Re-derive each held or not-held outcome, its `failure_kind`, and each signal end from the form's recording (`behaviour/<form>.json`, first confined run) and the case's host, with a checker that reads only the case and the recording, never the script that wrote the case. The checker that caught the errors above evaluates exit-code acceptance (QO5), each QA9 predicate on the kept bytes, the byte bound, the store comparison after QP7's and QP8's exclusions, leaks, and QO3 to QO5 for signal ends with QT4's `kill_attribution`. It is now `scripts/check-case-outcomes.py`, which `check.sh` runs on the public cases; this note records what it must cover.
- **Fields the judge does not score.** Anything QE19 compares and the judge ignores (`failure_kind`, `target_size`, `precondition_observed`, tiers, targets) needs that independent check, because a judge round trip cannot fail on it.

A green `cases` step means the expected outputs are transcribed correctly, and no more.

## When this applies

Any time cases are written by a script or by hand: adding public cases, building the private half, writing catalogue entries, or reviewing a phase 3 build's disagreement with a case. A disagreement makes the case a hypothesis until the record is re-derived from the text and the recording.
