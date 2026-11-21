# press-quality-lab

press-quality-lab is a measurement engine for agent-facing tools, written in Go. It starts from the
judge: a command-line program that reads JSON on stdin and writes a JSON verdict on stdout, with a
process exit code.

Status: pre-alpha: specification only, no engine code yet.

The judge specification lives in [`docs/specs/`](docs/specs/). Licensed under the MIT License (see
[`LICENSE`](LICENSE)).
