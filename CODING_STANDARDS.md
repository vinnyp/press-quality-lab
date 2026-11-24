# Coding standards

The rules code in this repository follows. Code review checks every change against them. Anything a lint, a test or a CI check already enforces is not repeated here; `gate.sh`, `check.sh` and `scan.sh` own those. Some rules are also in `AGENTS.md`, so every agent loads them: both rules under Specifications, and the opening sentences of "Test the rules" and "A check that could not evaluate its subject" under Tests. The judge rule is also quoted in `CONCEPTS.md`. Change every copy together.

## Specifications

- Build from the specification's text, for every input its schemas allow. The cases are examples of the rules, not the rules.
- The judge decides every verdict. Code that collects evidence reports what it saw for one check or one trial and nothing above that: no requiredness, no rollups, no comparison, no verdict.

## Tests

- Test the rules, not only the given cases. A test must fail when the rule it guards is broken; check that by breaking the rule. A change that adds a test of a rule records, in the pull request or its review log, the mutation that broke the rule and the test that failed on it. A failure to build does not count.
- The expected side of a test does not come from the code under test, with one exception: a test of a decision may compare through the production encoder, because a wrong decision changes the document. A property of how the output is spelled or encoded is asserted against literal bytes the encoder did not write. A schema, table or input shape on the expected side comes from the specification or from literals, never from the implementation's copy. See `docs/solutions/best-practices/test-oracle-must-not-come-from-code-under-test.md`.
- A test that a value or spelling must not appear first shows that the output under test contains the field it inspects. Otherwise it passes whatever the code does.
- A check that could not evaluate its subject never reports a pass. A control or probe that shows a restriction leaves clean runs alone prints what it computed, and the harness compares that output with a known value; an exit status alone is not evidence. A control never discards an error from the operation it exercises: each restricted operation's result appears in its output, so a refusal changes what it prints. See `docs/solutions/best-practices/clean-control-must-assert-on-output.md`.
- A cross-check or coverage check has a control for each branch: a planted defect it must fail on. It pairs its evidence one-to-one with the set it claims to cover, and each claim is tied to the data it names. A branch that skips an item it did not observe counts as a branch that never looks.
- A change that adds a field the collection cases compare, or a new kind of case, extends the corpus checkers that `check.sh` runs to cover it, with a planted-defect control, in the same change. Agreement with the judge does not validate a case record, because the judge does not read every field the collection specification scores. See `docs/solutions/best-practices/judge-agreement-does-not-validate-a-case-record.md`.

## Judge and evidence code

Where a rule here names a clause of the judge specification 1.0, review checks the code against the clause itself, not against this summary.

- Raw bytes reach a decision through one reading path that applies every condition of X12 (judge contract §3). Two readers once existed and were merged into one; a second reader can drift from the first.
- Every list written out is in the order the specification names: a sort key, a declared order, or the order of a stated procedure. It is never in map-iteration order. Where the specification says input order carries no meaning, as the judge contract's Input order clause does, the output follows its Order clause (judge contract §5).
- Judge code starts no goroutines, and a defect can never leave the process with the exit status a rejection uses. `recover` covers only the goroutine it runs in, and a Go panic that is not recovered exits with status 2, which is that status.

## Confinement harness

These apply to the verifier and any code that confines a subject's run. Each names a hazard past work found and the clause of the collection specification 1.0 that governs it. Review checks the code against the clause itself, not against this summary.

- The engine makes and checks its private copies as QH8 requires. A test harness follows the same pattern once per scenario: a changed copy fails the runs that would use it and is never remade. Hazard: a fresh executable per launch stalled a parallel suite. See `docs/solutions/test-failures/fresh-executable-per-run-stalls-parallel-sandbox-suite.md`.
- A profile's lines keep the order QH11's template gives them, because the later rule decides. Diagnose a failure by adding one narrow allowance to the unchanged profile. The rule order was measured on one macOS release; measure it again on a new one. See `docs/solutions/best-practices/sandbox-profile-later-rule-decides.md`.
- Every path reaches a sandbox profile as QH11 requires. Hazard: a path spliced into profile text granted a read of a decoy (reproduced).
- The files a subject file names and its seed meet QS8 and QS9, and the seed is copied by content as QS9 requires, never through a link. Hazards: a seed copy that followed a link exposed a decoy (reproduced), and named paths had no containment rule (found in review).

## Maintaining this file

A rule enters this file when its defect class appeared in a review finding the author fixed, a fix commit, a cleanup that removed it across the codebase, or a solution record, and no check covers it; or when the owner states it. State the class as a condition that stays true without naming volatile numbers or symbols, and keep rules grouped by area. A rule that a sealed specification clause governs names the hazard and the clause, with the specification's version, and restates no more of the clause than it takes to name the hazard, because a restatement drifts from the clause and review then checks the wrong text. Before a rule is worded to cover code that more than one specification governs, check it against each of those specifications' clauses; where they differ, the rule defers to the clause. Remove a rule when a check starts to enforce it, or when the code it guards is gone and no planned phase will bring it back.
