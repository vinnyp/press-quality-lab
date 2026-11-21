# Specifications

This folder holds the specifications for press-quality-lab.

`judge-1.0/` is version 1.0 of the judge specification, sealed 2026-11-19. It contains the rulebook,
the contract, schemas, the evidence-record description, examples, 383 cases, and a runner. Its files
must not be edited: a changed rule means a new version in a new folder. `judge-1.0/KIT.sha256` lists
a SHA-256 digest for every other file in the folder.

## Running the cases against a judge

From inside `judge-1.0/`:

    uv run runner.py --judge "<command>" --cases cases

Each case directory contains `input.json` and `case.json`. A case runs twice, and the runner refuses
to start if a stray file is present. Set `PYTHONDONTWRITEBYTECODE=1` so no bytecode cache is written
into the folder.
