# press-quality-lab

press-quality-lab is a measurement engine for agent-facing tools, written in Go. It starts from the
judge: a command-line program that reads JSON on stdin and writes a JSON verdict on stdout, with a
process exit code.

Status: pre-alpha. The judge and the grade command are implemented.

The judge specification lives in [`docs/specs/`](docs/specs/). Licensed under the MIT License (see
[`LICENSE`](LICENSE)).

Warning: grading another person's subject file runs their computations on this machine.

By default, pql grade and pql anchor print a text scorecard on standard output; --json prints the judge output as one JSON line instead.
The text output is not a stable interface; scripts and agents use --json.
An anchor's exit 0 is not a green run.

The minimum is 16 trials and the default is 40; for a prompt with a true success rate of 95%, the chance of green is 44% at 16 trials and 86% at 40.

A subject or computation can fill the volume by writing inside its home within its time budget; version 1.0 does not bound those writes.
The planted-secret check and masking can miss a canary inside a larger base64 value when its offset is not divisible by three, giving a false green or leaving a declared value unmasked.
A command that ignores a help request runs its action inside full confinement in an empty home.
Undeclared commands beyond 4,096 names or 1 MiB of names are counted but not all named; the count remains exact.
