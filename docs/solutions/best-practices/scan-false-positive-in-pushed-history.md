---
title: A scanner false positive in pushed history is allowed by fingerprint, never rewritten
date: 2026-11-26
category: best-practices
module: scan.sh
problem_type: best_practice
component: tooling
severity: medium
applies_when:
  - The scan job fails on a commit that is already pushed, and the finding is not a real secret
  - Code lists rule tokens or field names whose spelling ends in a word gitleaks treats as a credential name
tags: [gitleaks, scan, false-positive, pushed-history, gitleaksignore, ci]
---

# A scanner false positive in pushed history is allowed by fingerprint, never rewritten

## Context

`scan.sh` runs gitleaks over the tracked tree and, when `SCAN_COMMITS=base..head` is set, over every commit in that range. CI sets the range, so a commit that has been pushed stays in the scan until it reaches `main`.

In phase 3's PR6 (#22), a Python script listed two rule-token strings on one line. Each was a rule id joined by a dot to a field name ending in `_keys`. gitleaks' `generic-api-key` rule read the pair as a credential assignment. The tree had already been reformatted, but the pushed commit still held the line, and the repository forbids force-pushing, so the commit could not be rewritten.

The first fix added a `.gitleaksignore` whose comment quoted the two strings to explain the entry. That comment produced a second finding of the same kind in the commit that added it, and that commit was pushed too.

## Guidance

To recover from a false positive in a pushed commit:

1. Confirm it is not a secret. Read the flagged line at that commit, not in the tree.
2. Add the finding's exact fingerprint to `.gitleaksignore` at the repository root, one per line, in gitleaks' form `commit:file:rule:line`. A fingerprint allows that one finding and nothing else. The scan's JSON report names the commit, file, rule and line.
3. Describe the finding in the comment without repeating the flagged text. The comment is scanned like any other line. The same goes for commit messages, which `scan.sh` also checks.
4. Re-run the range scan as CI runs it, `SCAN_COMMITS=$(git merge-base HEAD origin/main)..HEAD ./scan.sh`, and check its exit status directly, not through a pipe. `scan.sh` refuses positional arguments, so a range must come through the environment variable.

To avoid it: put each such token string on its own line. In this repository the finding came from two tokens on one line with a credential-like word next to them.

## Why This Matters

The only ways to clear a finding in a pushed commit are a fingerprint or a history rewrite, and the repository allows only the first. A path or rule allowlist would also hide real secrets later. Each failed attempt adds another pushed commit, and a quoting mistake in that commit adds a second finding, as it did here.

## When to Apply

- A CI `scan` failure on a commit that is already on the pull request's branch.
- Before pushing code or prose that lists rule tokens, field names or other identifiers ending in words like `key`, `token` or `secret`.

## Related

- `.gitleaksignore`, whose two entries and comment are the worked example.
- `scan.sh`, for how the tree, commit messages and commit range are scanned.
