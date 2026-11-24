# Review

@CODING_STANDARDS.md

This file holds how review works in this repository: what each severity means here and the evidence a finding needs. The coding standards above are the rules a change is checked against.

## Severity

The most serious finding is a check, verifier or test that can report a pass without having evaluated its subject. That is a false green, and it ranks above a wrong result that is visible. Examples from past reviews: a verifier whose own working was not checked at run time, a copy that followed links and defeated a claim about secrets, and corpus checks that left tier, artifact targets and target-set completeness unchecked.

## Evidence a finding needs

- A reviewer's finding is a claim to verify against the text and the code before anyone acts on it.
- Each finding says how it was established: reproduced, confirmed against a cited clause, or not verified. A finding that was not verified is checked before it is acted on, not dropped.
- A finding against a specification cites the clause it reads, and says which reading it takes where the text could be read two ways.
