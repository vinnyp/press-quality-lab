# Concepts

> Shared domain vocabulary for this project — entities, named processes, and status concepts with project-specific meaning. Seeded with core domain vocabulary, then accretes as ce-compound captures and refreshes learnings; direct edits are fine. Glossary only, not a spec or catch-all.

## Specifications

### Sealed specification

A versioned specification whose folder is frozen: a digest file lists every file in it, and the folder is never edited again.

A rule that needs changing gets a new version in a new folder, ratified by the owner. Its clauses carry stable ids, and they are the authority for the code and for review; a summary of a clause elsewhere never replaces the clause itself.

### Errata note

The dated note kept beside a sealed specification's folder, holding editorial corrections and the owner's readings of that version without editing the folder.

### Judge

The part of the engine that decides every verdict, by the rules of its own sealed specification. Code that collects evidence reports only what it saw for one check or one trial and leaves every verdict to the judge.
