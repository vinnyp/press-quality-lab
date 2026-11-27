# Concepts

> Shared domain vocabulary for this project — entities, named processes, and status concepts with project-specific meaning. Seeded with core domain vocabulary, then accretes as ce-compound captures and refreshes learnings; direct edits are fine. Glossary only, not a spec or catch-all.

## Specifications

### Sealed specification

A versioned specification whose folder is frozen: a digest file lists every file in it, and the folder is never edited again.

A rule that needs changing gets a new version in a new folder, ratified by the owner. Its clauses carry ids that other documents cite, and they are the authority for the code and for review; a summary of a clause elsewhere never replaces the clause itself.

### Errata note

The dated note kept beside a sealed specification's folder, holding editorial corrections and the owner's readings of that version without editing the folder.

### Judge

The part of the engine that decides every verdict, by the rules of its own sealed specification.

The project's rule, quoted in full: "The judge decides every verdict. Code that collects evidence reports what it saw for one check or one trial and nothing above that: no requiredness, no rollups, no comparison, no verdict." `AGENTS.md` and `CODING_STANDARDS.md` hold the same rule; change every copy together.

## Checks

### Witness

A small program the confinement evidence runs to observe the sandbox's decision on one operation. It prints an attempt line, then performs exactly the operation a clause names, and its output and end show whether the sandbox let it return or killed it. A refused-operation witness pairs with a control that shows the same call returns when nothing refuses it.

### Coverage floor

The least a check must have evaluated before its result counts: below the floor the check fails, so a check that found nothing to look at never reports a pass.
