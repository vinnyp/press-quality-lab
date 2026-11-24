# Conformance collection specification 1.0: errata and notes

The files in `conformance-collection-1.0/` are sealed and are not edited. The version policy in `collection-contract.md` (QE1) lets an erratum correct only text whose intended reading the corpus or a worked example already fixes; any other change to a binding rule is a new version. This is that note.

Nothing here changes what a collector records for any case of the corpus or which exit code the engine ends with. Where an entry and a binding rule appear to differ, the rule is right.

The specification was sealed on 2026-11-24.

## 2026-11-24

### E-1. The store as a target of a store-unchanged check that runs no command

QE8 says: "When the declared store is a path, the store-unchanged probe adds it as a target of kind `artifact`", and gives an artifact "the tier of the invocation that used it". It does not say what happens when the probe runs no command (QP1: no declared `safe` command has `writes: false`), so the probe enters the required set as one check with no command, not observed with `empty_target`, while the store is still a path.

The owner settled the reading on 2026-11-24: **such a check has no artifact target.** The store is a target only of a store-unchanged check whose invocation used it, so a check that runs no command has only the one invocation target QE8 gives a check that would use no invocation.

No case of the corpus reaches this input, so this entry records the owner's reading rather than one the corpus fixes. A later version of the specification states it in the text. The corpus's checks (`scripts/check-case-ids.py`) already take this reading.
