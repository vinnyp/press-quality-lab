# Coding standards

The rules code in this repository follows. Code review checks every change against them. Anything a lint, a test or a CI check already enforces is not repeated here; `gate.sh`, `check.sh` and `scan.sh` own those.

## Specifications

- Build from the specification's text, for every input its schemas allow. The cases are examples of the rules, not the rules.
- The judge decides every verdict. Code that collects evidence reports what it saw for one check or one trial and nothing above that: no requiredness, no rollups, no comparison, no verdict.

## Tests

- Test the rules, not only the given cases. A test must fail when the rule it guards is broken; check that by breaking the rule.
- A test's expected value is never built with the code under test: not its encoder, its table, or the input shape it uses. A defect then appears on both sides and cancels. Write the expected value by hand or derive it from the specification. See `docs/solutions/best-practices/test-oracle-must-not-come-from-code-under-test.md`.
- A test that a value or spelling must not appear first shows that the request writes the field it inspects. Otherwise it passes whatever the code does.
- A check that could not evaluate its subject never reports a pass. A control or probe that shows a restriction leaves clean runs alone prints what it computed, and the harness compares that output with a known value; an exit status alone is not evidence. See `docs/solutions/best-practices/clean-control-must-assert-on-output.md`.
- A cross-check or coverage check has a control for each branch: a planted defect it must fail on. It pairs its evidence one-to-one with the set it claims to cover, and each claim is tied to the data it names. A branch that skips an item it did not observe counts as a branch that never looks.
- A case's expected record is derived from its own authority (the subject-file schema, the recording, the rule text), not accepted because the judge agrees with it. The judge does not read every field the collection specification scores. See `docs/solutions/best-practices/judge-agreement-does-not-validate-a-case-record.md`.

## Judge and evidence code

- Raw bytes reach a decision through one reading path that does the size, encoding, parse and end-of-document checks. A second reader can drift from the first.
- The order of anything written out never depends on map iteration order. Sort explicitly, by the key the specification names.
- Judge code starts no goroutines, and a defect can never leave the process with the exit status that means "rejected". A Go panic that is not recovered exits with status 2.

## Confinement harness

These apply to the verifier and any code that confines a subject's run.

- No harness creates a fresh executable file for each launch. Create it once per scenario and check that it is unchanged before each reuse. See `docs/solutions/test-failures/fresh-executable-per-run-stalls-parallel-sandbox-suite.md`.
- In a sandbox profile the later rule decides. Write each refusal after the allowance it narrows, and diagnose a failure by adding one narrow allowance to the unchanged profile. See `docs/solutions/best-practices/sandbox-profile-later-rule-decides.md`.
- Values reach a sandbox profile as parameters, never spliced into its text.
- A copy of a declared path stays inside its destination and never follows a link.

## Maintaining this file

A rule enters this file only when its defect class appeared in a review finding the author fixed, a fix commit, a cleanup that removed it across the codebase, or a solution record, and no check covers it. State the class as a condition that stays true without naming volatile numbers or symbols, and keep rules grouped by area. When a check starts to enforce a rule, or the code it guarded is gone, remove the rule.
