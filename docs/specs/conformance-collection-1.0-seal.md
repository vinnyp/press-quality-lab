# Conformance collection specification 1.0: seal

The owner ratified `conformance-collection-1.0/` on 2026-11-24, as it stood at commit 79578cf, and it was sealed in the same pull request.

## What was ratified

- **The folder.** `conformance-collection-1.0/KIT.sha256` lists all 1,060 of its other files. The SHA-256 of `KIT.sha256` is `9dc27d6b65778d97b5b767f9176a7721d8096f5aed7bb7c4d064f10583a3d0e5`.
- **The text.** The SHA-256 of the `shasum -a 256` lines of `CASES.md`, the four text files and `schema/*.json`, sorted by path, is `d02d743b4a80f9818324346418714a40c0ee3ea01ab065584299dd80ae1c5f1c`. This is the text the last round of the blind derivation read.
- **The corpus.** 152 cases: 121 judge-checked and 31 derivation-only. `CASES.md` declares the corpus complete.

## The private half

The private half stays outside every repository and is not published. It holds 72 cases, one for each rule a case can express, in the public format, and a catalogue of 40 seeded collection mistakes. Its digest, the SHA-256 of the `shasum -a 256` lines of every file under its `cases/` and `catalogue/`, sorted by path, is `e0c172814e0eb1d6f7b8f5e7b1088295cb0e8d028f6128b099390d85e420732b`, taken at the seal.

A build is scored on QE19's fields. A build that disagrees with a private case makes that case a hypothesis, checked against the text before the build is marked wrong. Feedback to a build names a rule id only.

## How it was checked

- Every judge-checked case agrees with judge 1.0, cross-checks against its form's recording, and passes the outcome checker; `check.sh` runs all three.
- A reader on a different model derived every public and private case from an export of the text alone. The last round, on the ratified text, agreed on every case. The review log `docs/agent-reviews/2026-11-24-pr3-build-peer-reviews.md` records every round and review.
- Every recording was made on macOS 27.2 (26B123), as the pull request states.
