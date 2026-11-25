---
title: Root doc clause and path references - Plan
type: test
date: 2026-11-24
artifact_contract: ce-unified-plan/v1
product_contract_source: ce-plan-bootstrap
execution: code
---

# Root doc clause and path references - Plan

## Goal Capsule

- **Objective:** A reviewer following `AGENTS.md`, `CODING_STANDARDS.md`, `REVIEW.md` or `CONCEPTS.md` to a clause id, a requirement id or a repository path reaches text that exists, because `gate.sh` fails when any of them stops resolving, and also fails when it found nothing to check.
- **Means:** one new test file in the existing test-only package `internal/docrefs` (KTD4).
- **Authority:** the sealed specifications, then `AGENTS.md` and `CODING_STANDARDS.md`, then this plan.
- **Stop conditions:** stop and report if a sealed folder defines an id in a form neither definition pattern in KTD1 matches, or if a reference cited in the root docs today does not resolve for a reason other than the two paths U2 fixes.
- **Execution profile:** one pull request. Codex builds U1 and U2 in a dedicated worktree; the peer-review gate runs on the diff; the owner merges, or the agent merges on the owner's say-so.

---

## Product Contract

### Summary

Add a test that reads the four root docs, finds every collection clause id, judge clause id, requirement or acceptance-example id and backtick-quoted repository path they cite, and fails with the file and line of each one that does not resolve. It fails closed when either specification has more than one version folder, and it fails when the docs yield nothing to check. `AGENTS.md`'s two paths written relative to a subfolder become repository-relative so the rule holds for every path.

### Problem Frame

Since PR #15, the root review docs point review at sealed clauses by id and at solution records by path instead of restating them, because restatements drifted from the clauses (`docs/agent-reviews/2026-11-24-pr15-review-docs-peer-reviews.md`, rows 1, 2, 20, 21 and 45). That makes every id and path load-bearing, and nothing checks them: `internal/docrefs` checks R and AE citations under `docs/` only (row 39). A mistyped id, or a solution record renamed by a refresh, would send review to the wrong text or to nothing, with every check green.

### Requirements

**Clause ids**

- DR1. Every collection clause id cited in the four root docs is defined in the collection specification's folder.
- DR2. Every judge clause id cited in the four root docs is defined in the judge specification's folder (KTD1, KTD2).
- DR3. Every R, AE, A and F id cited in the four root docs is defined in `docs/requirements.md`, with ranges such as "R1 to R42" expanded (KTD2).
- DR4. When either specification has no version folder or more than one, the test fails and says that the root docs' ids name no version, rather than choosing a folder (KTD3).
- DR5. An id defined twice in one specification folder is reported.

**Paths**

- DR6. Every backtick-quoted repository path in the four root docs, outside fenced code blocks, exists as a file or directory (KTD5). A fenced block left open at the end of a doc is reported, because every path after it would go unchecked (KTD7).

**Reporting and floors**

- DR7. Each problem names the file, the line, the reference and what to fix.
- DR8. The test fails when it evaluated nothing: across the four docs it must find at least one collection id, one judge id, one R or AE id and one path (KTD6).
- DR9. The test's comment states that it proves a cited reference exists, not that it is the right clause for the rule citing it.

**Docs**

- DR10. `AGENTS.md` writes every repository path relative to the repository root, and its Checks section names what the new check covers.

### Key Decisions

- **The check covers `AGENTS.md` as well as the three review docs.** Governs DR1, DR2, DR3, DR6, DR10. (session-settled: user-directed — chosen over covering only `CODING_STANDARDS.md`, `REVIEW.md` and `CONCEPTS.md`: `AGENTS.md` is the file every agent loads.)
- **Build the check now, then run the peer-review gate on it.** (session-settled: user-directed — chosen over leaving round 3's row 39 deferred.)

### Scope Boundaries

- Section references such as "judge contract §3" or "rulebook §8" are not checked.
- `README.md` and `CLAUDE.md` are not checked. `CLAUDE.md` holds only includes, and `README.md` cites one path.
- The check proves existence, not fit: a valid id cited for the wrong rule still passes (row 45 is that case).
- Indented code blocks are not parsed; none of the four docs has a backtick inside one today.
- Considered and not built: reporting a judge-shaped token whose letter is not a judge family (for example `Y12`). Prose and product names (`M1`, `U2`) would fail falsely; build it if a mistyped judge letter is ever found in review.
- Considered and not built: opening the repository through `os.OpenRoot` to refuse symlinks that escape it. The check only tests that a path exists, never reads through it, and an escape needs a committed symlink, which review sees; build it if the check ever reads file contents through a cited path.
- The existence check is case-insensitive on macOS and case-sensitive on Linux; CI on Linux is authoritative.

### Deferred to Follow-Up Work

- Extending the import guard to scan beyond `internal/` (raised in PR #15 round 1, outside this change).

---

## Planning Contract

### Key Technical Decisions

- KTD1. **Definitions are matched as line-start list items.** A collection definition is a line matching `^- \*\*(Q[A-Z]+[0-9]+[a-z]?)\. ` in a top-level `*.md` file of the collection folder; a judge definition is a line matching `^- \*\*([A-Z][0-9]+[a-z]?)\.` in a top-level `*.md` file of the judge folder. Measured on the current tree: 103 collection definitions and 97 judge definitions, none repeated. The line-start anchor keeps inline bold in worked examples (for example P3 and R2 in the rulebook's cases) from counting as definitions, and the optional lowercase letter covers E9a and E9b. A judge pattern requiring `.**` straight after the id would match only 40 of the 97, because most definitions carry a title inside the bold span.
- KTD2. **`R` is never a judge family in the root docs; R and AE resolve as requirements.** The judge defines its own R1 to R3, but `AGENTS.md` cites requirements as "R1 to R42", and the project reads a bare R id as a requirement everywhere else. The judge families are derived from the definitions in KTD1, minus R. R and AE citations reuse the existing `findCitations`, `parseDefinitions` and `unresolved` from `internal/docrefs/docrefs_test.go`, so ranges such as "R1 to R42" expand as they do under `docs/`. A and F citations resolve against the actor and flow definitions `actorFlowPattern` already matches, with the same range expansion. Citation tokens on both clause families accept the same optional lowercase letter their definitions do, so a mistyped `QH8a` is reported, not skipped.
- KTD3. **A specification's folder is found by listing directories.** The folder is the one directory under `docs/specs/` whose name starts `conformance-collection-` or `judge-`. Files beside the folders (`*-errata.md`, `*-seal.md`) are not directories and never count; a name glob would match them and fail today. Zero or two matches fail with the DR4 message.
- KTD4. **One new test file in `internal/docrefs`, reusing that package's helpers.** The package is test-only and already owns citation checks, `repositoryRoot`, `lineAt` and the `problem` type, and the gate runs it on Linux from a full checkout. The expected definitions come from the sealed files themselves, not from any engine code.
- KTD5. **Path rule.** A path is an inline backtick span, outside a fenced block, with no whitespace, that does not start with `$`, `~` or `/` and contains none of `<`, `>`, `*`, `{`, `}`, `@`, `:`, and that either contains `/` or ends in a repository file extension (`.md`, `.sh`, `.go`, `.mod`, `.sum`, `.json`, `.yaml`, `.yml`, `.py`). A span is a host, and skipped, only when its first segment is dotted and that segment's last dot-separated label is all letters and at least two long (`github.com/spf13/cobra`, `users.noreply.github.com`); `judge-1.0/x` is therefore a path. One trailing `/` is dropped before the existence check. Every line is scanned for every span on it, not only the first. A `..` segment is never resolved past the root: the filesystem rejects it and the check reports the path as unresolved.
- KTD6. **Floors are totals across the four docs.** Only `CODING_STANDARDS.md` cites clause ids today, and only `AGENTS.md` cites R and AE ids, so a per-file floor would fail on a correct tree. Each floor carries a comment naming the source it guards, as `coverageFloors` does, and its problem names the root docs rather than one file.
- KTD7. **Fences follow CommonMark's opener and closer rules, and fail closed.** A fence opens on a line of three or more backticks or tildes whose info string holds no backtick, and closes on a line of the same character at least as long. A fence still open at the end of a doc is a problem naming its opening line.

---

## Implementation Units

### U1. The root doc reference check

- **Goal:** the test that enforces DR1 to DR9.
- **Requirements:** DR1, DR2, DR3, DR4, DR5, DR6, DR7, DR8, DR9.
- **Dependencies:** U2, because the live test fails on the two subfolder-relative paths U2 fixes.
- **Files:** `internal/docrefs/rootdocs_test.go` (new), `internal/docrefs/doc.go` (package comment). This seam catches a dangling reference in the four root docs; it misses a valid reference cited for the wrong rule, and any reference in other files.
- **Approach:**
  1. A pure check over an `fs.FS` that locates both specification folders (KTD3), collects definitions (KTD1), scans each root doc for collection ids, judge ids (KTD2), R and AE ids, and paths (KTD5), and returns problems and counts.
  2. Fixture tests over `testing/fstest.MapFS` for each rule.
  3. One live test over `os.DirFS(repositoryRoot())` that asserts no problems and the DR8 floors.
- **Patterns to follow:** `TestDocsReferencesResolve`, `TestCheckDocsOnFixtures` and `TestRulesOfTheCheck` in `internal/docrefs/docrefs_test.go`; `coverageFloors` for the floor comments.
- **Test scenarios:**
  - a defined collection id and a defined judge id pass; an undefined one of each is reported with file and line;
  - `AQH8` is not a citation; `E9a` is a citation of `E9a`; `QH8x` is a citation and is reported as undefined;
  - a definition in a subfolder (`cases/case-001/README.md`) does not count;
  - an inline bold `**P3.**` mid-line, an indented `  - **P4.` and a mid-line `see - **P4.** here` do not count as definitions;
  - an id defined in two top-level files of one folder is reported;
  - `R42` and `AE36` in a root doc resolve against a fixture `docs/requirements.md`, an id above the highest defined requirement is reported, and `R2` is never treated as a judge id;
  - "R1 to R4" expands and reports the one id in that range the fixture leaves undefined;
  - `A3` and `F2` are checked as actor and flow ids, never as judge ids, and an undefined `A9` is reported;
  - no `conformance-collection-*` directory, and two such directories, are each reported; an errata and a seal file beside a single folder are not;
  - an existing file path passes; an existing directory passes with and without a trailing `/`; a missing path is reported with file and line;
  - `$HOME/x/y`, `~/x/y`, `/abs/x`, `github.com/a/b`, `docs/<name>/x` and `docs/*/x` are not checked, and `.github/workflows` is;
  - `judge-1.0/x` is checked as a path and reported when missing;
  - a root-level `gate.sh` passes and a missing `scann.sh` is reported; a bare word such as `pql` is not checked;
  - `docs/../AGENTS.md` is reported as unresolved;
  - a line holding an existing path and then a missing one reports the missing one;
  - a backtick path inside a fenced block is not checked, a path after the closing fence is, a `~~~` line does not close a backtick fence and a backtick line does not close a `~~~` fence;
  - a fence left open at the end of a doc is reported with its opening line, and a line like ```` ```x``` ```` does not open a fence;
  - each of the four root docs, when missing, is reported, and the list of checked docs is exactly the four;
  - for each floor, a fixture that removes only that category's citations fails that floor and no other.
  - every fixture asserts its exact problem list and, where it counts, its exact counts.
- **Verification:** the live test passes on the current tree, and each rule's fixture test fails when that rule is broken (see the Verification Contract).

### U2. Root-relative paths in `AGENTS.md`

- **Goal:** every path `AGENTS.md` quotes resolves from the repository root, and the Checks section names the new check.
- **Requirements:** DR10.
- **Dependencies:** none.
- **Files:** `AGENTS.md`.
- **Approach:**
  1. Change the `judge-1.0/` span in "Where things are" to `docs/specs/judge-1.0/`, and `2026-11-21-engine-plan.md` to `docs/plans/2026-11-21-engine-plan.md`.
  2. Extend the Checks sentence on the R and AE citation test to say the tests also check the clause ids, requirement, actor and flow ids, and quoted repository paths cited in the four root docs.
- **Test expectation:** none -- a documentation edit; U1's live test is its check.
- **Verification:** U1's live test reports no path problem for `AGENTS.md`, and reports one when either path is reverted.

---

## Verification Contract

| Gate | What it proves |
|---|---|
| `./gate.sh` | build, staticcheck, errcheck, exhaustive and every test, including U1's live and fixture tests, with the network off |
| `./scan.sh` | the diff and commit messages carry no home path, email or secret |
| a mutation run per rule, through `mutation-verify.sh` with `go test -v -count=1 ./internal/docrefs/`, a `-skip` knock-out of the hazard test and its expected failure text | each fixture test fails when its rule is broken: the unbounded id pattern, the undefined-id report for each clause family, the R exclusion from judge families, the subfolder exclusion, the line-start anchor, the directory-only version lookup, the host rule, the extension rule for root-level files, the scan of every span on a line, the trailing-slash trim, the fenced-block skip, the fence closer and its character match, the unclosed-fence report, the duplicate-definition report, the R, AE, A and F resolution against `docs/requirements.md`, the missing-path report, the missing-root-doc report, and each floor's own counter |
| a live-tree control | in a scratch copy, breaking one real cited path in each of the four docs that cites one turns the live test red |

`CODING_STANDARDS.md` requires a change that adds a test of a rule to record the mutation that broke it and the test that failed. Each record quotes `mutation-verify.sh`'s verdict line, with its `matched:` text and its knock-out clause, and goes in the review log.

---

## Definition of Done

- U1 and U2 are in one commit series on the pull request's branch, and `./gate.sh` and `./scan.sh` pass.
- Every rule in the Verification Contract has a recorded mutation and the test that failed on it.
- The peer-review gate has run on the diff, and every Blocker and Major is resolved or accepted by the owner with the reason recorded.
- CI is green on the pull request's head.
- No abandoned helpers, commented-out code or scratch files remain in the diff.
