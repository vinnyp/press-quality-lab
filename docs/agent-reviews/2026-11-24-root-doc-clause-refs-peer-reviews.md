# Peer-review gate — root-doc-clause-refs (2026-11-24)

**Mode:** build. **Reviewers:** peer-code-reviewer, peer-test-reviewer, peer-security-reviewer, peer-experiment-reviewer (measurement lens). **persona-version:** cache/1.4.0.

**Design round:** none. The change adds a gate check, which is measurement-shaped, so the measurement lens ran in build mode. The plan `docs/plans/2026-11-24-1931-test-root-doc-clause-refs-plan.md` served as the design note; it had a document review (one combined coherence, feasibility and scope reviewer) but no design-mode measurement review. That gap let a Blocker into the plan's path rule (row 1), which round 1 caught.

**Tier rationale:** Tier 1 code and test always run in build mode. Tier 2 security fired because the repository is public, to check the public-by-default rule and whether a cited path could read outside the repository. The measurement lens fired because the change adds a check to the gate. No Tier 3 lens applied.

**Per-lens runtime:**

- peer-code-reviewer: runtime=claude
- peer-test-reviewer: runtime=claude
- peer-security-reviewer: runtime=claude
- peer-experiment-reviewer: runtime=claude

Each reviewer ran in its own clone; repository-state snapshots before and after each run were identical.

## Round 1

**Range:** `567fd43..a431b7e` (the plan, then the first build). The same pass (codex) wrote the check and its fixtures and recorded 13 mutants.

Verdicts: code ready after one Major; test trustworthy after one Blocker; security safe to ship (2 Low); measurement not yet sound (1 Blocker, 3 Major). Reviewers re-ran recorded mutants through `mutation-verify.sh` with knock-outs and controls; those were caught, and several new mutants survived.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The host rule takes any dotted first segment as a host, so `judge-1.0/...` is never checked; AGENTS.md citing `judge-1.0/no-such-file.md` stays green | test, measurement (Blocker) | Reproduced on a scratch copy: the live test passed with 15 paths | Fixed: a host's first segment ends in a letters-only label of two or more; `judge-1.0/x` is a path. The plan's KTD5 was the defect. |
| 2 | A fence that never closes passes the suite and silences every later path | code (Major), test (Major) | Reproduced by both through `mutation-verify.sh` | Fixed: fixtures for a path after a closed fence, and an unclosed fence is reported (KTD7). |
| 3 | Slash-less root files are never checked, so `REVIEW.md` and `CONCEPTS.md` contribute no paths, against AGENTS.md's claim | measurement (Major) | Confirmed: `scann.sh` in REVIEW.md stayed green | Fixed: spans ending in a listed file extension are checked; AGENTS.md's engine-plan path made repository-relative. |
| 4 | A malformed fence opener disables path checking for the rest of a doc | measurement (Major) | Reproduced | Fixed: an opener whose info string holds a backtick is not a fence; an unclosed fence is reported. |
| 5 | Scanning only the first span on each line survives the suite | measurement (Major) | Reproduced | Fixed: a fixture with an existing then a missing path on one line. |
| 6 | Floor fixture empties all four docs, so a floor reading the wrong counter survives | test (Major) | Reproduced | Fixed: one fixture per floor removes only that category. |
| 7 | Dropping `^` from either definition pattern survives | test (Minor) | Reproduced | Fixed: indented and mid-line definition fixtures. |
| 8 | Dropping the closing-fence character match survives | code, test (Minor) | Reproduced | Fixed. |
| 9 | Removing `CONCEPTS.md` from the doc list survives | code (Minor) | Reproduced | Fixed: the doc list is pinned. |
| 10 | Fixtures assert with `strings.Contains`, so extra problems pass | test (Minor) | Confirmed | Fixed: exact problem lists and counts. |
| 11 | `QH8a` is skipped: Q citations lack the optional letter their definitions allow | security (Low), measurement, code | Confirmed | Fixed: `QH8x` is reported as undefined. |
| 12 | Actor and flow ids (A1 to A4, F1 to F5) are not checked | measurement (Minor) | Confirmed | Fixed: resolved against `docs/requirements.md`. |
| 13 | Missing mutant records for the undefined-id reports, the family filter, the trailing slash, the missing root doc | measurement, code (Minor) | Confirmed | Fixed: 30 mutants and 6 live controls recorded (below). |
| 14 | Floor problems filed under `AGENTS.md:1`; broken DR4 message; R/AE problems say no fix | code, measurement (Nit) | Confirmed | Fixed. |
| 15 | The plan names the branch with the owner's username | security (Low) | Confirmed | Fixed: the plan names no branch. |
| 16 | A judge-shaped token whose letter is no judge family (`Y12`) is skipped | measurement (Minor) | Confirmed | Considered and not built: prose such as `M1` or `U2` would fail falsely; recorded in the plan's Scope Boundaries. |
| 17 | `os.DirFS` follows a committed symlink | security (Info) | Confirmed; `..` is rejected by `fs.ValidPath` | Considered and not built: existence only, never read through; recorded in the plan. |
| 18 | `fs.Stat` is case-insensitive on macOS | measurement (Nit) | Agreed | Recorded in the plan: Linux CI is authoritative. |

## Round 2 (delta check)

**Range:** `a431b7e..7666c6d` (plan revision `0033489`, rebuild `7666c6d`). **Reviewers:** peer-test-reviewer and peer-experiment-reviewer, who raised most of round 1.

Verdicts: test trustworthy; measurement sound for its stated decision. Every round-1 finding confirmed closed. The test lens re-ran four recorded mutants (host rule, fence character, every span on a line, judge floor counter), each CAUGHT with knock-out and control. The measurement lens added live controls for the id rules: renaming X12, QH8, the last requirement id, the last actor id and the first flow id to undefined ids each produced the exact expected problem against the real specifications.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 19 | Inside an open fence, a closer with trailing text (```` ```go ````) is not covered | test, measurement (Minor) | Reproduced: mutant not caught | Fixed: fixture added. |
| 20 | The four-space indent guard on fences is not covered | test, measurement (Minor) | Reproduced | Fixed: fixture added. |
| 21 | An unpaired backtick run ends the scan of its line | measurement (Minor) | Reproduced | Fixed: scanning resumes after the run; fixture added. |
| 22 | The collection family length check is not covered | test (Nit) | Reproduced; can only over-report | Fixed: `Q1` fixture. |
| 23 | `./gate.sh` and `#anchor` fail; `x.go:12` is skipped | test, measurement (Nit) | Confirmed | Recorded in the plan's KTD5 as deliberate. |
| 24 | The duplicate actor or flow definition report has no fixture | measurement (Minor) | Confirmed | No change: the existing pin that each actor and flow id appears once already fails first. |

## Mutation evidence

The build's 30 recorded mutants each ran through `mutation-verify.sh` with `--syntax-cmd 'go vet ./internal/docrefs/'`, a go `-skip` knock-out of the hazard subtest and `--knockout-delta-line '=== RUN '`; every verdict reads CAUGHT with its knock-out clause. They cover each rule in the plan's Verification Contract. The six live controls broke one real cited path in each of the four root docs, and reverted each of AGENTS.md's two corrected paths; each turned the live test red naming the path. The round-2 fixes add four more mutants, one per new fixture.
