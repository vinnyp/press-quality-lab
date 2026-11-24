# The corpus

Each directory under `cases/` is one case in the format QT6 of `collection-contract.md` defines. A judge-checked case holds the judge input a collector must send, its preimages, and the judge's whole output with the engine's exit code. A derivation-only case ends before any record exists and names the rule that decides its exit code.

Run the judge-checked cases with `uv run runner.py --judge "<command>" --cases cases`. Run Python with `PYTHONDONTWRITEBYTECODE=1`: a stray file makes the runner refuse. The runner checks each record against its verdict; it does not check that a collector would produce that record from the recording, which is what a collector's own tests check.

Three cases use the real verifier: `case-069` and `case-070` grade the known-good form, and `case-143` anchors it. A collector meets them only in a local run on a macOS version QH1 names.

## Stated limits

The specification states limits it does not detect. These cases show a limit with a recorded behaviour:

- `case-141` is a known false pass. A check routed to `completeness-signaling` that only reads an exit code credits, and the dimension is green, because routing is the author's claim where `credit-and-routes.md` §2 states no condition.
- `case-077` shows that a subject that kills itself with `SIGKILL` reads as a kill by the confinement (QH21), so it is not measured rather than red.
- `case-026` shows a subject that ignores a help request and runs that command's action inside the confinement, in an empty home (QD6).

The other stated limits are things the engine cannot observe, so no recording can show them: a leak through the system's unified log (QP14), another process reading the planted file (QP14, QH28), a read of no bytes at the end of the file (QP14, QH28), a directory-server lookup (QP14), a copy of the subject's binary at another path (QH17), and a child that leaves its process group (QH19).

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

These rules govern how a real run confines the subject or what only a scorecard shows. A scripted run takes each invocation's end from its recording (QT3), so its record cannot break them, and the cases do not cite them:

- QE18: masking shows in `expected`, `observed` and the scorecard, which QE19 does not compare; the recordings are masked before recording (QT5).
- QH10: the profile's kill on refusal shows in a scripted record only as a recorded kill, which QO3 decides.
- QH12: the environment the engine builds is part of a real run; a scripted run passes no environment to anything.
- QH15: what the profile allows shows in a scripted record only as recorded ends.
- QH16: a computation's profile shows in a scripted record only as a computation's recorded end, which QO6 decides.
- QH20: a computation's lack of network shows in a scripted record only as its recorded end.
- QH22: a refusal turned into a kill shows in a scripted record only as a recorded kill.

Corpus: complete
