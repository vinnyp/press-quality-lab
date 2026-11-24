# Review

@CODING_STANDARDS.md

This file holds how review works in this repository: which findings rank highest and the evidence a finding needs. The rules a change is checked against are in `CODING_STANDARDS.md`, included above for Claude Code Review.

## Severity

Reviewers grade findings Blocker, Major, Minor and Nit, or Critical, High, Medium, Low and Info for security and privacy. A Blocker or Critical is resolved before work continues, and a Major or High is resolved or accepted by the owner before merge. An acceptance and its reason are recorded in the review log's dispositions.

These rank as Blocker or Critical:

- A check, verifier or test that can report a pass without having evaluated its subject. Past examples: a verifier whose own working was not checked at run time, and corpus checks that left tier, artifact targets and target-set completeness unchecked.
- A breach of the public-by-default rule in `AGENTS.md`: a secret, personal data, or a private subject's name, host or data in a tracked file or a commit message. `scan.sh` checks home paths, emails and secrets; it cannot catch a real name, a username outside a home path, or a private subject's name, host or data, so review is the only check for those, and a push makes the breach permanent.
- A subject escaping its confinement, or reading or leaking a file it was never granted. Past example: a seed copy that followed links and defeated the claim about secrets.

A wrong result that is visible is still serious; it ranks with these when it decides a verdict.

## Evidence a finding needs

- A reviewer's finding is a claim to verify against the text and the code before anyone acts on it. `AGENTS.md` carries the same rule; change both copies together.
- Each finding says how it was established: reproduced, confirmed against a cited clause, or not verified. A finding that was not verified is checked before it is acted on, not dropped.
- A finding against a specification cites the clause it reads. Where the text can honestly be read two ways, the finding names both readings and which one it assumes, and the two-way reading itself is reported to the owner, as `AGENTS.md` requires.
