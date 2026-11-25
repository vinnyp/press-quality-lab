"""Fixture controls for the ledger growth decision, run by the Go ledger test."""
import importlib.util
from pathlib import Path
import unittest
import os
import subprocess
import shutil
import sys

spec = importlib.util.spec_from_file_location('growth', Path(__file__).with_name('check-rule-growth.py'))
growth = importlib.util.module_from_spec(spec)
spec.loader.exec_module(growth)


class GrowthTest(unittest.TestCase):
    def test_decisions(self):
        cases = [
            ('QH1\nQH2\n', 'QH2\nQH1\nQH1\n', True, []),
            ('QH1\nQH2\n', 'QH1\n', True, []),
            ('QH1\n', 'QH1\nQH2\n', True, ['unmapped rule added: QH2']),
            ('QH1\n', None, True, ['unmapped rule list removed']),
            (None, 'QH1\n', False, []),
            (None, 'QH1\n', True, ['base ledger has no unmapped rule list']),
            (None, None, False, ['unmapped rule list removed']),
        ]
        for base, current, ledger, expected in cases:
            with self.subTest(base=base, current=current, ledger=ledger):
                self.assertEqual(growth.compare(base, current, ledger), expected)

    def test_line_grammar(self):
        self.assertEqual(growth.entries("  # comment\n\n  QH1  \nQH2\n"), {"QH1", "QH2"})

    def test_main_ci(self):
        root = Path(subprocess.check_output(
            ['sh', '-c', 'set -eu; p=$(mktemp -d "${TMPDIR:-/tmp}/pql-growth.XXXXXX"); test -n "$p"; printf "%s" "$p"'],
            text=True))
        self.assertTrue(root.is_absolute())
        script = Path(__file__).with_name('check-rule-growth.py').resolve()
        def git(*args):
            return subprocess.check_output(
                ['git', '-C', str(root), '-c', 'user.name=Synthetic',
                 '-c', 'user.email=fixture@users.noreply.github.com', *args],
                text=True, stderr=subprocess.STDOUT).strip()
        try:
            git('init', '-q')
            for path, data in [(growth.LIST, 'QH1\nQH2\n'),
                               (growth.PENDING, 'case-001\ncase-002\n'),
                               (growth.LEDGER, '{}'), (growth.RUNNER, '// fixture')]:
                target = root / path
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text(data)
            git('add', '.')
            git('commit', '-qm', 'test: synthetic base')
            base = git('rev-parse', 'HEAD')
            env = dict(os.environ, CI='true', RULE_EVENT='pull_request', RULE_PR_BASE=base)
            for path, original, added, shrunk, message in [
                (growth.LIST, 'QH1\nQH2\n', 'QH3\n', 'QH1\n', 'unmapped rule added: QH3'),
                (growth.PENDING, 'case-001\ncase-002\n', 'case-003\n', 'case-001\n', 'pending case added: case-003'),
            ]:
                with self.subTest(path=path):
                    (root / path).write_text(original + added)
                    result = subprocess.run([sys.executable, '-I', '-B', str(script)],
                                            cwd=root, env=env, text=True, capture_output=True)
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(result.stderr.strip(), message)
                    (root / path).write_text(shrunk)
                    result = subprocess.run([sys.executable, '-I', '-B', str(script)],
                                            cwd=root, env=env, text=True, capture_output=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout.strip(), 'rule growth: no unmapped rules or pending cases added')
                    (root / path).write_text(original)
        finally:
            shutil.rmtree(root)


if __name__ == '__main__':
    unittest.main()
