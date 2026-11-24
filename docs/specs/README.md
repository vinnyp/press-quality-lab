# Specifications

This folder holds the specifications for press-quality-lab.

`judge-1.0/` is version 1.0 of the judge specification, sealed 2026-11-19. It contains the rulebook,
the contract, schemas, the evidence-record description, examples, 383 cases, and a runner. Its files
must not be edited: a changed rule means a new version in a new folder. `judge-1.0/KIT.sha256` lists
a SHA-256 digest for every other file in the folder.

`judge-1.0-errata.md` is the dated errata note for version 1.0. The sealed folder is never edited;
corrections and notes go in the errata note.

`conformance-collection-1.0/` is version 1.0 of the conformance collection specification: how the
engine turns a subject file and a requested scope into the record the judge decides, and how it
confines the subject on macOS.
It was ratified by the owner and sealed on 2026-11-24. It holds the four text files, schemas,
examples, a synthetic subject's 54 forms with their recordings, 152 cases and a runner, and
`conformance-collection-1.0/KIT.sha256` lists a SHA-256 digest for every other file in it. Its files
must not be edited. `conformance-collection-1.0-seal.md` records what was ratified, and
`conformance-collection-1.0-errata.md` is its errata note.

From inside `conformance-collection-1.0/`, run its cases against a judge with:

    uv run --with jsonschema==4.23.0 python -I runner.py --judge "<command>" --cases cases

## Running the cases against a judge

From inside `judge-1.0/`:

    uv run runner.py --judge "<command>" --cases cases

Each case directory contains `input.json` and `case.json`. A case runs twice, and the runner refuses
to start if a stray file is present. Set `PYTHONDONTWRITEBYTECODE=1` so no bytecode cache is written
into the folder.
