# press-quality-lab: contributor and agent guide

A measurement engine for agent-facing command-line tools, written in Go. The README is the entry point. This file holds the rules that apply when you write code or documents here, whether you are a person or an agent.

## Where things are

- `docs/plans/`: plans. Start with `2026-11-21-engine-plan.md`: the goal, the milestone, the phases and what is not yet specified.
- `docs/specs/`: specifications. `judge-1.0/` is the judge specification, version 1.0.
- The engine's command is `pql`, under `cmd/pql`; the plan's §4 gives the intended package layout.
- `docs/solutions/`: documented solutions to past problems (bugs, best practices, workflow patterns), organized by category with YAML frontmatter (`module`, `tags`, `problem_type`).

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
- After a solved, verified problem, automatically invoke the `ce-compound` skill with `mode:non-interactive` at the completion checkpoint when the work corrected a wrong belief about how something behaves: an approach that failed, a review finding that exposed a real defect, or an assumption, document, or first diagnosis that turned out wrong. A naming, style, or preference change does not count. Work that went as first planned does not need it. Do not judge whether the lesson deserves a document: `ce-compound` decides whether it qualifies and what holds it (a test, a lint rule, a coding standard, or a learning), and writes nothing when nothing qualifies. Capture at the checkpoint so what it writes can ship in the PR that produced it, and only where the repository treats captured learnings as tracked, committed knowledge.
- Write every report, summary, or handoff to the user through the `ce-noslop` skill. This applies when you are the top-level agent writing to the user, not when you are a subagent reporting to its caller. Do not apply it to code, config, verbatim quotes, or text the user asked to post as written.

## Checks

`gate.sh` runs the Go build, lints and tests with pinned tools and the network off.
`gate.sh` uses the `go` first on PATH and requires go1.27.3 (`GOTOOLCHAIN=local`), so machines with a newer Go must put go1.27.3 first on PATH.
`check.sh` runs the gate, builds the binary, verifies the kit digest and file set, and runs the published cases.
`scan.sh` checks tracked files and an optional commit range for home paths, emails and secrets with gitleaks.
Only noreply@anthropic.com and addresses at `users.noreply.github.com` are allowed emails.
The `ci` workflow runs three jobs: `gate`, `cases` and `scan`. The `cases` job stays red until the judge lands.

Install the pinned tools once, then run `go mod download`:

```sh
GOBIN="$HOME/.gate-tools/go/bin" go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOBIN="$HOME/.gate-tools/go/bin" go install github.com/kisielk/errcheck@v1.30.0
GOBIN="$HOME/.gate-tools/go/bin" go install github.com/nishanths/exhaustive/cmd/exhaustive@v0.17.1
```

The sealed specification digest must keep verifying:

    cd docs/specs/judge-1.0 && shasum -a 256 -c KIT.sha256

Run Python there with `PYTHONDONTWRITEBYTECODE=1`; a stray file makes the case runner refuse to start.

## Tests

- Test the rules, not only the given cases. A test must fail when the rule it guards is broken; check that by breaking the rule.
- A check that could not evaluate its subject never reports a pass.
- Cobra (`github.com/spf13/cobra`) is the approved command framework, used under `cmd/` only. The import-guard test enforces this. Any other dependency beyond the standard library is a deliberate, reviewed addition. The gate runs with the network off.

## Review

A non-trivial change is reviewed before it merges by someone, or some agent, that did not write it. Specifications and each phase of the plan also get one review by a different model. Treat a reviewer's finding as a claim to verify against the text and the code before acting on it.

## Spending

Nothing calls a model or a subject's live API unless the owner has approved the planned count and expected cost for that run.
