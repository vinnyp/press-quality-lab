"""Fixture controls for pending growth and the independent export count."""
import base64
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
case_ids = load('case_ids_control', 'check-case-ids.py')
outcomes = load('outcomes_control', 'check-case-outcomes.py')


class CorpusControls(unittest.TestCase):
    def testReadCalibration(self):
        # E-38, QO7: a failed calibration prevents the read control from running.
        pre = dict(probe='planted_secret', words=None, earlier=[],
                   parameters=dict(commands=[['read']]))
        subject = dict(binary='subject', seed=None, store=None,
                       secret=dict(commands=[['read']]), commands=[])
        run = dict(leak_scan=dict(leaks=False, bound_reached=False),
                   end=dict(kind='exit', code=0),
                   markers=dict(confinement=True, could_not_start=False),
                   planted=dict(atime_moved=True, same_file=True))
        for name, calibration, control, expected in [
                ('MovedKilled', 'moved', 'killed', ('held', None, None)),
                ('MovedNotKilled', 'moved', 'not_killed',
                 ('not_observed', 'harness_fault', None)),
                ('NotMovedNotKilled', 'not_moved', 'not_killed',
                 ('not_observed', 'input_missing', None))]:
            with self.subTest(name=name), patch.object(outcomes, 'key', return_value=dict(words=['read'])), \
                    patch.object(outcomes, 'find', return_value=run):
                self.assertEqual(outcomes.derive_all(
                    pre, subject, {}, dict(read_calibration=calibration,
                                          read_control=control),
                    {}, (None, False), None), (expected, None))

    def testFixedHelpLimit(self):
        # E-34, QD5: Control 3 independently enforces the same fixed request-word limit.
        for label, ancestor, child, allowed in [
            ('Exact', '', 'a' * 131064, True),
            ('PlusOne', '', 'a' * 131065, False),
            ('Ancestor', 'a' * 131060, 'tiny', False),
            ('UTF8', '', 'é' * 65532, True),
            ('UTF8PlusOne', '', 'é' * 65532 + 'a', False),
            ('NUL', '', 'bad\0name', False),
            ('InvalidUTF8', '', 'bad\udcffname', False),
        ]:
            with self.subTest(label):
                def entry(words, text):
                    stream = dict(bytes=len(text.encode('utf-8', errors='surrogateescape')), kept=base64.b64encode(text.encode('utf-8', errors='surrogateescape')).decode(), head='', cut=False)
                    return dict(key=dict(words=words + ['--help'], offline_args=[], env=[], runbook=None, step=None, computation=None, home='empty'),
                                confined=[dict(end=dict(kind='exit', code=0), markers=dict(confinement=True, could_not_start=False), stdout=stream, stderr=dict(bytes=0))])
                entries = [entry([], 'Commands:\n  ' + (ancestor or child) + '  desc')]
                if ancestor:
                    entries.append(entry([ancestor], 'Commands:\n  ' + child + '  desc'))
                if allowed:
                    entries.append(entry(([ancestor] if ancestor else []) + [child], 'leaf'))
                status = {}
                keys = case_ids.discovery_keys(dict(invocations=entries), status=status)
                self.assertEqual(len(keys), 1 + bool(ancestor) + allowed)
                self.assertEqual(status.get('unreadable', False), not allowed)
                result, missing, extra = outcomes.discovery(dict(commands=[], declared_count=len(keys)-1), dict(invocations=entries), {})
                self.assertEqual(result, ('held', None, None) if allowed else ('not_observed', 'precondition_unmet', None))
                self.assertFalse(missing)

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
            (copies / 'extra.txt').write_text('extra')
            with self.assertRaises(ValueError) as caught:
                validation.validate(root, cases, pending, copies)
            self.assertEqual(str(caught.exception),
                             "export file set differs: missing=[] extra=['extra.txt']")
            (copies / 'extra.txt').unlink()
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
