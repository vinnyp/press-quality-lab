"""Fixture controls for pending growth and the independent export count."""
import importlib.util
import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


growth = load('growth', 'check-rule-growth.py')
validation = load('validation', 'validate-corpus-records.py')


class CorpusControls(unittest.TestCase):
    def test_growth(self):
        for base, current, runner, expected in [
            ('case-001\ncase-002\n', 'case-002\ncase-001\n', True, []),
            ('case-001\ncase-002\n', 'case-001\n', True, []),
            ('case-001\n', 'case-001\ncase-002\n', True, ['pending case added: case-002']),
            ('case-001\n', 'case-002\n', True, ['pending case added: case-002']),
            ('case-001\n', None, True, ['pending case list removed']),
            (None, 'case-001\n', True, ['base runner has no pending case list']),
            (None, 'case-001\n', False, []),
            (None, None, False, ['pending case list removed']),
        ]:
            with self.subTest(base=base, current=current, runner=runner):
                self.assertEqual(growth.compare_pending(base, current, runner), expected)

    def test_missing_ci_base(self):
        for event, key in [('pull_request', 'RULE_PR_BASE'), ('push', 'RULE_PUSH_BEFORE')]:
            for value in ['', '000000']:
                with patch.dict(os.environ, {'CI': 'true', 'RULE_EVENT': event, key: value}, clear=True):
                    with self.assertRaisesRegex(ValueError, '^CI base commit missing$'):
                        growth.resolve_base()
        with patch.dict(os.environ, {'CI': 'true', 'RULE_EVENT': 'pull_request', 'RULE_PR_BASE': 'missing'}, clear=True):
            with patch.object(growth, 'git', side_effect=ValueError('missing commit')):
                with self.assertRaisesRegex(ValueError, '^missing commit$'):
                    growth.resolve_base()

    def test_independent_count(self):
        with tempfile.TemporaryDirectory(prefix='pql-count-') as temporary:
            root = Path(temporary)
            cases = root / 'cases'
            cases.mkdir()
            for name, kind, verifier in [('case-001', 'judge-checked', 'scripted'),
                                         ('case-002', 'derivation-only', 'scripted'),
                                         ('case-003', 'judge-checked', 'real')]:
                directory = cases / name
                directory.mkdir()
                (directory / 'case.json').write_text(json.dumps(
                    {'id': name, 'kind': kind, 'input': {'verifier': verifier}}))
            pending = root / 'pending.txt'
            pending.write_text('')
            self.assertEqual(validation.expected_cases(cases, pending), {'case-001'})
            copies = root / 'copies'
            copies.mkdir()
            # Counting must fail before schema files or validators are consulted.
            with patch.dict('sys.modules', {'jsonschema': object()}):
                with self.assertRaisesRegex(ValueError, "^export file set differs: missing=\\['case-001.record.json'\\] extra=\\[\\]$"):
                    validation.validate(root, cases, pending, copies)
            pending.write_text('  # comment\n\n  case-001  \n')
            self.assertEqual(validation.expected_cases(cases, pending), set())
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                validation.validate(root, cases, pending, copies)
            self.assertEqual(output.getvalue(), 'nothing to validate: 1 pending\n')
            pending.write_text('case-999\n')
            with self.assertRaisesRegex(ValueError, '^unknown pending cases$'):
                validation.expected_cases(cases, pending)


if __name__ == '__main__':
    unittest.main()
