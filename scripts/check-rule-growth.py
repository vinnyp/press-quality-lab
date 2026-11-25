#!/usr/bin/env python3
"""Ratchet the rule ledger; compare() also accepts fixture lists without git."""
import os
from pathlib import Path
import subprocess
import sys

LIST = 'internal/docrefs/unmapped-rules.txt'
LEDGER = 'internal/docrefs/rule-ledger.json'
PENDING = 'internal/corpusrunner/pending.txt'
RUNNER = 'internal/corpusrunner/runner_test.go'


def entries(text):
    return {line.strip() for line in text.splitlines()
            if line.strip() and not line.startswith('#')}


def compare(base, current, base_has_ledger):
    if current is None:
        return ['unmapped rule list removed']
    if base is None:
        return ['base ledger has no unmapped rule list'] if base_has_ledger else []
    return ['unmapped rule added: ' + rule for rule in sorted(entries(current) - entries(base))]


def compare_pending(base, current, base_has_runner):
    if current is None:
        return ['pending case list removed']
    if base is None:
        return ['base runner has no pending case list'] if base_has_runner else []
    return ['pending case added: ' + case for case in sorted(entries(current) - entries(base))]


def resolve_base():
    event = os.environ.get('RULE_EVENT', '')
    if os.environ.get('CI'):
        if event not in ('pull_request', 'push'):
            raise ValueError('CI needs RULE_EVENT pull_request or push')
        base = os.environ.get('RULE_PR_BASE' if event == 'pull_request' else 'RULE_PUSH_BEFORE', '')
        if not base or set(base) == {'0'}:
            raise ValueError('CI base commit missing')
    else:
        base = git('merge-base', 'HEAD', 'origin/main')
        print('rule growth: local base defaults to merge base with origin/main: ' + base)
    git('cat-file', '-e', base + '^{commit}')
    return base


def git(*args):
    return subprocess.check_output(['git', *args], text=True).strip()


def at_base(base, path):
    # ls-tree distinguishes an absent path from a broken/missing base commit.
    if not git('ls-tree', base, '--', path):
        return None
    return git('show', base + ':' + path)


def main():
    base = resolve_base()
    current = Path(LIST).read_text() if Path(LIST).exists() else None
    problems = compare(at_base(base, LIST), current, at_base(base, LEDGER) is not None)
    pending = Path(PENDING).read_text() if Path(PENDING).exists() else None
    problems += compare_pending(at_base(base, PENDING), pending, at_base(base, RUNNER) is not None)
    if problems:
        raise ValueError('\n'.join(problems))
    print('rule growth: no unmapped rules or pending cases added')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
