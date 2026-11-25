"""Literal controls for the ratchet predictor (not an engine conformance test)."""
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location('predict_ratchet', Path(__file__).with_name('predict-ratchet.py'))
predictor = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(predictor)
KIT = Path(__file__).resolve().parents[1] / 'docs/specs/conformance-collection-1.0'
BUILT = json.loads(Path(__file__).with_name('ratchet').joinpath('pr6.json').read_text())


class PredictorTests(unittest.TestCase):
    def case(self, number, built=None):
        return predictor.predict_case(KIT / 'cases' / f'case-{number:03d}', built or BUILT)

    def test_level_two_is_required_and_changes_prediction(self):
        built = copy.deepcopy(BUILT)
        # U39's construction capabilities are supplied independently of levels.
        built['builds'] += ['QE2.placeholders', 'QE3.placeholder_candidates']
        before = self.case(60, built)
        self.assertFalse(before['predicted'])
        self.assertEqual(before['levels'], [2])
        self.assertIn('deciding level 2 (QS5) unbuilt', before['blockers'][0])
        built['levels'] = [1, 2]
        after = self.case(60, built)
        self.assertEqual((after['predicted'], after['blockers']), (True, []))
        # Adding level 2 cannot complete a case whose checks reach level 8.
        self.assertFalse(self.case(1, built)['predicted'])

    def test_trigger_holds_even_when_a_later_level_is_built(self):
        subject = {'binary': {'placeholder': 'binary is missing'}}
        p = {'probe': None, 'words': ['list'], 'earlier': [], 'expected': None}
        self.assertEqual(predictor.check_block(8, [1, 3, 4, 5, 6, 7, 8], subject, p),
                         'level 2 (QS5) trigger holds; exit 70')
        subject = {'binary': 'bin/subject', 'seed': None, 'store': None, 'commands': []}
        self.assertIsNone(predictor.check_block(8, [1, 3, 4, 5, 6, 7, 8], subject, p))
        p['probe'] = 'bare'
        self.assertEqual(predictor.check_block(8, [1, 2, 4, 5, 6, 7, 8], subject, p),
                         'level 3 (QD4) trigger holds; exit 70')

    def test_rejects_and_parts(self):
        self.assertTrue(self.case(116)['predicted'])
        self.assertEqual(self.case(114)['blockers'], ['rejects lacks QS11.literal'])
        built = copy.deepcopy(BUILT)
        built['rejects'] = [r for r in built['rejects'] if r['rule'] != 'QS11.schema']
        self.assertEqual(self.case(116, built)['blockers'], ['rejects lacks QS11.schema'])
        self.assertEqual(self.case(125)['blockers'], ['rejects lacks QA9.validation'])

    def test_named_rule_precedes_schema(self):
        row = self.case(101)
        self.assertTrue(row['predicted'])
        self.assertIn('violations: QS4, QS2', row['detail'])
        built = {'levels': [1], 'rejects': ['QS2'], 'builds': []}
        self.assertEqual(self.case(101, built)['blockers'], ['rejects lacks QS4'])

    def test_earlier_named_rule_blocks_expected_rejection(self):
        audit = copy.deepcopy(predictor.DERIVATION_AUDIT)
        audit['case-116']['violations'].append('QS6.semantic')
        with patch.object(predictor, 'DERIVATION_AUDIT', audit):
            row = self.case(116)
        self.assertFalse(row['predicted'])
        self.assertEqual(row['blockers'], ['earlier violation: QS6.semantic',
                         'first refusal is QS6.semantic, expected QS11',
                         'rejects lacks QS6.semantic'])

    def test_real_verifier_never_predicted(self):
        built = {'levels': list(range(1, 9)), 'rejects': [], 'builds': []}
        for number in (69, 70, 143):
            with self.subTest(number=number):
                row = self.case(number, built)
                self.assertEqual((row['predicted'], row['blockers']),
                                 (False, ['real verifier excluded']))

    def test_construction_is_independent_of_level_one(self):
        built = copy.deepcopy(BUILT)
        built['builds'] = [r for r in built['builds'] if r['rule'] != 'QE8']
        row = self.case(4, built)
        self.assertEqual(row['levels'], [1])
        self.assertEqual(row['blockers'], ['builds lacks QE8'])

    def test_anchor_predictions_and_schema_extras(self):
        result = predictor.predict(BUILT)
        self.assertEqual(result['predicted'], ['case-003', 'case-004', 'case-100',
                         'case-101', 'case-102', 'case-103', 'case-116', 'case-129', 'case-136'])
        self.assertEqual(len(result['cases']), 152)

    def test_audit_fails_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            case = json.loads((KIT / 'cases/case-116/case.json').read_text())
            (directory / 'case.json').write_text(json.dumps(case))
            (directory / 'subject.json').write_text('{}')
            with self.assertRaisesRegex(ValueError, 'subject changed; re-audit rule order'):
                predictor.predict_case(directory, BUILT)

    def test_overloaded_reason_mappings(self):
        # The expected levels are literal readings of section 2's order.
        for number, reason, expected_level in [
                (3, 'network_unverifiable', 1), (73, 'harness_fault', 7),
                (79, 'harness_fault', 7), (7, 'harness_fault', 8),
                (5, 'harness_fault', 5), (153, 'harness_fault', 6),
                (152, 'harness_fault', 8), (51, 'computation_failed', 6),
                (20, 'precondition_unmet', 3), (44, 'precondition_unmet', 8)]:
            with self.subTest(number=number, reason=reason):
                d = KIT / 'cases' / f'case-{number:03d}'
                case = json.loads((d / 'case.json').read_text())
                document = json.loads((d / 'input.json').read_text())
                preimages = json.loads((d / 'preimages.json').read_text())
                subject = json.loads((d / 'subject.json').read_text())
                recording = json.loads((KIT / 'behaviour' / (case['input']['form'] + '.json')).read_text())
                levels = [predictor.deciding_level(e, preimages[e['check_id']], subject,
                          case['input']['host'], recording, document['evidence'], preimages)
                          for e in document['evidence'] if e['reason'] == reason]
                self.assertTrue(levels)
                self.assertEqual(set(levels), {expected_level})
        # QD4 still decides discovery when QS5 masks the planted-secret check.
        d = KIT / 'cases/case-091'
        case = json.loads((d / 'case.json').read_text())
        doc = json.loads((d / 'input.json').read_text())
        preimages = json.loads((d / 'preimages.json').read_text())
        discovery = next(e for e in doc['evidence'] if preimages[e['check_id']]['probe'] == 'discovery')
        level = predictor.deciding_level(discovery, preimages[discovery['check_id']],
                json.loads((d / 'subject.json').read_text()), case['input']['host'],
                json.loads((KIT / 'behaviour' / (case['input']['form'] + '.json')).read_text()),
                doc['evidence'], preimages)
        self.assertEqual(level, 3)

    def test_cli_json(self):
        completed = subprocess.run([sys.executable, '-I', '-B', str(Path(__file__).with_name('predict-ratchet.py')),
                                    '--built', str(Path(__file__).with_name('ratchet') / 'pr6.json'), '--json'],
                                   check=True, capture_output=True, text=True)
        result = json.loads(completed.stdout)
        self.assertEqual(result['predicted'][:2], ['case-003', 'case-004'])
        self.assertEqual(result['cases'][0]['case'], 'case-001')
        self.assertEqual(result['reason_to_level']['earlier_step_failed']['levels'], [4])
        self.assertEqual(completed.stderr, '')


if __name__ == '__main__':
    unittest.main()
