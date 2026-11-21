# press-quality-lab: contributor and agent guide

A measurement engine for agent-facing command-line tools, written in Go. The README is the entry point. This file holds the rules that apply when you write code or documents here, whether you are a person or an agent.

## Where things are

- `docs/plans/`: plans. Start with `2026-11-21-engine-plan.md`: the goal, the milestone, the phases and what is not yet specified.
- `docs/specs/`: specifications. `judge-1.0/` is the judge specification, version 1.0.
- The engine's command is `pql`. No engine code exists yet; the plan's §4 gives the intended package layout.

## Specifications are the authority

- Build from the specification's text, for every input its schemas allow. The cases are examples of the rules, not the rules.
- A sealed specification folder is never edited. Its digest file lists every file in it. A rule that needs changing gets a new version in a new folder, ratified by the owner.
- If the rulebook, the contract and the schemas disagree on some input, or the text can honestly be read two ways, stop and report the input and both readings. Do not pick one quietly and do not work around it in the engine.
- The judge decides every verdict. Code that collects evidence reports what it saw for one check or one trial and nothing above that: no requiredness, no rollups, no comparison, no verdict.

## Nothing is ported

This engine replaces an earlier one. That engine's code is reference only and its outputs are not an oracle. Do not copy code from it, and do not look for other implementations of the judge on this machine or elsewhere.

## Public by default

This repository is public. Every tracked file and every commit message must be safe to publish:

- no real names, emails or usernames other than the repository's own GitHub URL;
- no absolute paths under a user's home directory;
- no names, hosts or data of private subjects. A real subject's file lives outside this repository;
- no secrets, tokens or credentials, and no file that holds them.

Test subjects in this repository are synthetic.

## How changes land

- Every change reaches `main` through a pull request, including the maintainer's. No direct push to `main`.
- Work that creates a branch or a commit is done in a dedicated git worktree, not in a checkout another session may be using.
- Commit subjects follow Conventional Commits (`feat:`, `fix:`, `docs:` …).
- Hooks and checks are never skipped (`--no-verify` and its equivalents are not used).
- CI must be green before a merge. A local run is not a substitute for checking the pull request's own checks.

## Checks

The plan's phase 0 adds one gate that CI runs on every pull request: the Go build and lints at pinned versions with the network off, the project's tests, the published specification cases run against the built judge, and a scan for secrets and personal paths. Until it lands, a change to anything under `docs/specs/judge-1.0/` must leave its digest file verifying:

    cd docs/specs/judge-1.0 && shasum -a 256 -c KIT.sha256

Run Python there with `PYTHONDONTWRITEBYTECODE=1`; a stray file makes the case runner refuse to start.

## Tests

- Test the rules, not only the given cases. A test must fail when the rule it guards is broken; check that by breaking the rule.
- A check that could not evaluate its subject never reports a pass.
- Dependencies beyond the Go standard library are a deliberate, reviewed addition. The gate runs with the network off.

## Review

A non-trivial change is reviewed before it merges by someone, or some agent, that did not write it. Specifications and each phase of the plan also get one review by a different model. Treat a reviewer's finding as a claim to verify against the text and the code before acting on it.

## Spending

Nothing calls a model or a subject's live API unless the owner has approved the planned count and expected cost for that run.
