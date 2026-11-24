# The corpus

Each directory under `cases/` is one case in the format QT6 of `collection-contract.md` defines. A judge-checked case holds the judge input a collector must send, its preimages, and the judge's whole output with the engine's exit code. A derivation-only case ends before any record exists and names the rule that decides its exit code.

Run the judge-checked cases with `uv run runner.py --judge "<command>" --cases cases`. Run Python with `PYTHONDONTWRITEBYTECODE=1`: a stray file makes the runner refuse. The runner checks each record against its verdict; it does not check that a collector would produce that record from the recording, which is what a collector's own tests check.

Two cases, `case-069` and `case-070`, use the real verifier. A collector meets them only in a local run on a macOS version QH1 names.

## Not expressible as a case

A case fixes a subject file, a form and its recording, a request and scope, a verifier, a scripted host, a baseline and fixtures, and expects the judge's output and the engine's exit code. These rules govern something none of those carries.

- QA5: the warning is text in the documentation and on the terminal, and a case expects only the judge's output and the exit code.
- QE12: next actions appear on the scorecard, which a case does not expect.
- QE15: the judge rejects only a record that breaks the judge contract, and a collector that follows this specification sends none, so no case input leads to one.
- QE17: where records are stored is neither a case input nor part of what a case expects.
- QE20: removing homes, scratch directories and private copies is neither a case input nor part of what a case expects.
- QH1: the system's version is not a case input. A scripted run takes whether confinement was applied from the host file (QT4), which `case-003` and `case-004` force.
- QH2: whether the system accepts the profile is not a case input. Its effect, confinement not applied, is QT4's `confinement` field.
- QH3: the user the engine runs as is not a case input.
- QH11: the profile is a fixed text, and a scripted run applies no profile (QT3).
- QH13: a scripted run starts no process (QT3), so it has no descriptors or terminal to set up.
- QH18: a computation's own calls are not recorded, and a scripted run starts no process (QT3).
- QH19: a child that leaves its process group is beyond what the engine observes, so no recording can show it.
- QH23: a scripted run starts no process (QT3), so it has no process group to end.
- QP14: each limit it states is something the engine cannot observe, so no recording can show it.

Corpus: complete
