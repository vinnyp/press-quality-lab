"""Literal controls for the ratchet predictor (not an engine conformance test).

E-17 is covered by the engine tests. The former PR8 seed-name-link test
was removed because PR8 disables the legacy safe-read phase: it accepted
any link without evaluating QS9 and therefore proved no E-17 behavior.
"""
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

    def test_pr9_discovery_tokens(self):
        # QD2 bounds, QD3, QD5, QO7: all discovery capabilities are needed.
        tokens = [
            'QD2.bounds',
            'QD3',
            'QD5',
            'QO7.discovery',
        ]
        subject = {'binary': 'bin/subject', 'declared_count': 0,
                   'seed': None, 'store': None, 'secret': None, 'commands': []}
        pre = {'probe': 'discovery', 'words': [], 'earlier': [], 'expected': None}
        built = {'levels': [1, 2, 3, 5, 7], 'builds': tokens}
        self.assertEqual(predictor.check_ending(8, built, subject, pre, []),
                         {'exit_code': None, 'rules': sorted([
                              'QD2',
                              'QD3',
                              'QD5',
                              'QP1',
                              'QP2',
                              'QP3',
                              'QP4',
                              'QP6',
                              'QP7',
                              'QP8',
                              'QP9',
                              'QO7',
                              'QP10',
                              'QP11',
                              'QP12',
                              'QP13',
                              'QA9',
                          ]),
                          'phase': 'deciding'})
        for token in tokens:
            with self.subTest(token=token.replace('.', '_')):
                missing = dict(built, builds=[t for t in tokens if t != token])
                self.assertEqual(predictor.check_ending(8, missing, subject, pre, [])['phase'],
                                 'sentinel')
        for probe in ('bare', 'planted_secret', None):
            with self.subTest(probe=probe):
                other = dict(pre, probe=probe)
                other_built = dict(built, builds=tokens + ['QO7.planted_secret'])
                self.assertEqual(predictor.check_ending(8, other_built, subject, other, [])['phase'],
                                 'sentinel')

    def test_pr9_planted_secret_combination(self):
        # QO7: the planted-secret combination gates level 7 independently.
        built = predictor.load(Path(__file__).with_name('ratchet') / 'pr9.json')
        row = self.case(95, built)
        self.assertFalse(row['predicted'])
        self.assertEqual(row['ending'],
                         {'exit_code': 70, 'rules': ['QO7'], 'phase': 'sentinel'})
        built['builds'].append('QO7.planted_secret')
        self.assertTrue(self.case(95, built)['predicted'])

    def test_prediction_files_regenerate(self):
        # QS5, QO7: preserve earlier slices and PR9's independent prediction.
        directory = Path(__file__).with_name('ratchet')
        for version in ('pr6', 'pr8', 'pr9'):
            with self.subTest(version=version):
                result = predictor.predict(predictor.load(directory / (version + '.json')))
                actual = (json.dumps(result, indent=2, sort_keys=True) + '\n').encode()
                self.assertEqual(actual, (directory / (version + '-prediction.json')).read_bytes())
        pr8 = predictor.load(directory / 'pr8-prediction.json')['predicted']
        pr9 = predictor.load(directory / 'pr9-prediction.json')['predicted']
        self.assertEqual(set(pr9), set(pr8) | {'case-061', 'case-062'})
        self.assertEqual(len(pr9), 37)

    def test_pr8_literal_placeholder_data(self):
        # QS5, E-22: the prediction classifier inspects only allowed positions.
        for literal in ({'placeholder': 'x'}, {'placeholder': 'x', 'k': 1}):
            with self.subTest(literal=literal):
                subject = {'runbooks': [{'steps': [{'checks': [
                    {'expected': {'literal': literal}}]}]}]}
                self.assertFalse(predictor.contains_placeholder(subject))
                subject['binary'] = {'placeholder': 'x', 'k': 1}
                self.assertFalse(predictor.contains_placeholder(subject))
                subject['binary'] = {'placeholder': 'Supply binary.'}
                self.assertTrue(predictor.contains_placeholder(subject))

    def test_pr8_placeholder_dependencies(self):
        # QS5, KTD2.8: the dependency classifier produces the shared oracle.
        built = predictor.load(Path(__file__).with_name('ratchet') / 'pr8.json')
        expected = predictor.load(Path(__file__).with_name('ratchet') / 'pr8-prediction.json')
        actual = predictor.predict(built)['placeholder_dependencies']
        self.assertEqual(len(actual), 121)
        self.assertEqual(actual, expected['placeholder_dependencies'])

    def test_pr8_final_override_is_independent_of_level_two(self):
        # QS5: final override follows level 8; building level 2 alone is insufficient.
        built = json.loads(Path(__file__).with_name('ratchet').joinpath('pr8.json').read_text())
        row = self.case(61, built)
        self.assertFalse(row['predicted'])
        built['levels'] = list(range(1, 8))
        row = self.case(61, built)
        self.assertFalse(row['predicted'])
        self.assertTrue(any(c['ending']['phase'] == 'sentinel' and
                            c['deciding_level'] == 8 for c in row['checks']))
        built['levels'] = list(range(1, 9))
        row = self.case(61, built)
        self.assertTrue(row['predicted'])
        self.assertTrue(any(c['ending'] == {'exit_code': None, 'rules': ['QS5'],
                                          'phase': 'deciding'} for c in row['checks']
                            if c['deciding_level'] == 8))
        built['builds'] = [r for r in built['builds'] if r['rule'] != 'QS5.final_override']
        row = self.case(61, built)
        self.assertFalse(row['predicted'])
        self.assertIn('QS5.final_override', row['ending']['rules'])

    def test_pr8_qd3_trigger_is_independent_of_placeholder_override(self):
        # QS5, QD3: an undeclared discovered command has its own override trigger.
        built = predictor.load(Path(__file__).with_name('ratchet') / 'pr8.json')
        built['levels'] = list(range(1, 9))
        built['builds'].append('QO7.planted_secret')
        row = self.case(22, built)
        self.assertFalse(row['predicted'])
        self.assertEqual(row['ending']['rules'], ['QS5.qd3_trigger'])
        overrides = [c for c in row['checks'] if c['ending']['phase'] == 'observation']
        self.assertEqual(len(overrides), 18)
        self.assertTrue(all(c['ending'] == {
            'exit_code': 70, 'rules': ['QS5.qd3_trigger'], 'phase': 'observation'}
            for c in overrides))
        built['builds'].append('QS5.qd3_trigger')
        self.assertTrue(self.case(22, built)['predicted'])
        built['builds'] = [r for r in built['builds']
                           if not isinstance(r, dict) or r['rule'] != 'QS5.final_override']
        self.assertTrue(self.case(22, built)['predicted'])

    def test_pr8_placeholder_override_converse(self):
        # QS5: the QD3 trigger cannot implement the placeholder override.
        built = predictor.load(Path(__file__).with_name('ratchet') / 'pr8.json')
        built['levels'] = list(range(1, 9))
        self.assertTrue(self.case(61, built)['predicted'])
        built['builds'] = [r for r in built['builds']
                           if not isinstance(r, dict) or r['rule'] != 'QS5.final_override']
        built['builds'].append('QS5.qd3_trigger')
        row = self.case(61, built)
        self.assertFalse(row['predicted'])
        self.assertIn('QS5.final_override', row['ending']['rules'])

    def test_pr6_prediction_history(self):
        # KTD1: earlier slice predictions remain reproducible.
        expected = predictor.load(Path(__file__).with_name('ratchet') / 'pr6-prediction.json')
        self.assertEqual(predictor.predict(BUILT), expected)

    def test_pr8_rejections_and_construction(self):
        # QS1 to QS12, QP7, QP10, QA4, QA9; U39's construction precedes collection.
        built = predictor.load(Path(__file__).with_name('ratchet') / 'pr8.json')
        result = predictor.predict(built)
        derivations = [r for r in result['cases'] if r['kind'] == 'derivation-only']
        self.assertEqual(len(derivations), 31)
        self.assertTrue(all(r['predicted'] for r in derivations))
        self.assertTrue(self.case(60, built)['predicted'])
        self.assertTrue(self.case(90, built)['predicted'])
        self.assertFalse(any(r['ending']['phase'] == 'planning' and
                             any(x != 'QE4' and x != 'QE4.anchor' for x in r['ending']['rules'])
                             for r in result['cases']))

    def test_safe_read_phase(self):
        # QS8, QS9: safe snapshots stop before planning; semantic store checks wait.
        for number, rule in ((110, 'QS8'), (112, 'QS9')):
            with self.subTest(number=number):
                row = self.case(number)
                self.assertEqual(row['ending'], {
                    'exit_code': 70, 'rules': [rule], 'phase': 'safe_read'})
                self.assertFalse(row['predicted'])
                built = copy.deepcopy(BUILT)
                built['builds'] = [r for r in built['builds']
                                   if r['rule'] != rule + '.safe_read']
                self.assertNotEqual(self.case(number, built)['ending']['phase'], 'safe_read')
        self.assertNotEqual(self.case(111)['ending']['phase'], 'safe_read')
        case = predictor.load(KIT / 'cases/case-112/case.json')
        case['input']['fixtures'] = []
        self.assertIsNone(predictor.safe_read_refusal(case, KIT / 'cases/case-112', BUILT))

    def test_level_two_is_required_and_changes_prediction(self):
        built = copy.deepcopy(BUILT)
        # U39's construction capabilities are supplied independently of levels.
        built['builds'] += ['QE2.placeholders', 'QE3.placeholder_candidates', 'QT5.computation_keys']
        before = self.case(60, built)
        self.assertFalse(before['predicted'])
        self.assertEqual(before['levels'], [2])
        self.assertEqual(before['ending'], {'exit_code': 70, 'rules': ['QS5'], 'phase': 'sentinel'})
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

    def test_build_tokens_are_classified(self):
        self.assertEqual(predictor.names(BUILT['builds']) - set(predictor.BUILD_PHASE), set())
        with patch.dict(predictor.BUILD_PHASE, clear=True):
            with self.assertRaisesRegex(ValueError, 'unclassified builds token:'):
                predictor.predict(BUILT)

    def test_level_one_does_not_reach_computation_key(self):
        for number in (3, 4):
            row = self.case(number)
            self.assertTrue(row['predicted'])
            self.assertEqual(row['levels'], [1])
            self.assertEqual(row['ending'], {'exit_code': 4, 'rules': [], 'phase': 'deciding'})
            self.assertTrue(any(c['expected'] and 'computation' in c['expected']
                                for c in predictor.load(KIT / 'cases' / f'case-{number:03d}' /
                                                        'preimages.json').values()))

    def computation_ending(self, levels):
        p = {'probe': None, 'words': ['list'], 'earlier': [],
             'expected': {'computation': {'captures': []}}}
        subject = {'binary': 'bin/subject', 'seed': None, 'store': None, 'commands': []}
        return predictor.check_ending(8, {'levels': levels}, subject, p,
                                      ['QT5.computation_keys'])

    def test_reached_computation_key_precedes_level_six_sentinel(self):
        for levels in ([1, 2, 3, 4, 5], list(range(1, 9))):
            with self.subTest(levels=levels):
                self.assertEqual(self.computation_ending(levels),
                                 {'exit_code': 70, 'rules': ['QT5.computation_keys'],
                                  'phase': 'observation'})

    def test_earlier_trigger_precedes_computation_key(self):
        self.assertEqual(self.computation_ending([1, 2, 3, 4]),
                         {'exit_code': 70, 'rules': ['QH8'], 'phase': 'sentinel'})

    def test_planning_precedes_collection_and_check_order(self):
        built = copy.deepcopy(BUILT)
        built['builds'] = [r for r in built['builds'] if r['rule'] != 'QE2.slice']
        row = self.case(3, built)
        self.assertEqual(row['ending'], {'exit_code': 70,
                         'rules': ['QE2.slice'], 'phase': 'planning'})
        original = predictor.load
        def reversed_evidence(path):
            value = original(path)
            if path.name == 'input.json':
                value['evidence'].reverse()
            return value
        with patch.object(predictor, 'load', side_effect=reversed_evidence):
            self.assertEqual(self.case(3, built), row)

    def test_case_ending_collects_first_sentinels_only(self):
        self.assertEqual(self.case(59)['ending'], {'exit_code': 70,
                         'rules': ['QA7', 'QD4', 'QH8'], 'phase': 'sentinel'})
        self.assertEqual(self.case(156)['ending'], {'exit_code': 70,
                         'rules': ['QD4', 'QH8'], 'phase': 'sentinel'})

    def test_unbuilt_derivation_is_not_refused(self):
        pending = {
            'QA3.capture_crediting',
            'QA7',
            'QA9',
            'QA9.preconditions',
            'QD2',
            'QD3',
            'QD4',
            'QD5',
            'QE2',
            'QE6',
            'QE2.captures',
            'QE2.placeholders',
            'QE3.placeholder_candidates',
            'QE4',
            'QE7.asserts_emptiness',
            'QH8',
            'QO2',
            'QO3',
            'QO4',
            'QO5',
            'QO6',
            'QO7',
            'QP10',
            'QP11',
            'QP12',
            'QP13',
            'QP2',
            'QP3',
            'QP4',
            'QP6',
            'QP7',
            'QP8',
            'QP9',
            'QS5',
        }
        self.assertEqual(self.case(114)['ending'],
                         {'exit_code': 70, 'rules': sorted(pending), 'phase': 'unrefused'})

    def test_pending_uses_built_levels(self):
        built = {'levels': [], 'builds': [], 'rejects': []}
        self.assertIn('QO1', predictor.pending_rules(built))
        built['levels'] = [1]
        pending = predictor.pending_rules(built)
        self.assertIn('QO6', pending)
        self.assertNotIn('QO1', pending)

    def test_pending_excludes_builds_and_rejects(self):
        built = {'levels': [], 'builds': [], 'rejects': []}
        self.assertIn('QE8', predictor.pending_rules(built))
        built['rejects'] = ['QE8']
        pending = predictor.pending_rules(built)
        self.assertIn('QO1', pending)
        self.assertNotIn('QE8', pending)
        built['rejects'] = []
        built['builds'] = ['QE8']
        self.assertNotIn('QE8', predictor.pending_rules(built))

    def test_pending_preserves_only_partial_rule_suffixes(self):
        built = {'levels': [], 'builds': [], 'rejects': []}
        pending = predictor.pending_rules(built)
        self.assertIn('QE2', pending)
        self.assertNotIn('QE2.captures', pending)
        built['builds'] = ['QE2.slice']
        pending = predictor.pending_rules(built)
        self.assertIn('QE2.captures', pending)
        self.assertIn('QE2', pending)
        self.assertNotIn('QE2.slice', pending)

    def test_unrefused_always_allows_engine_faults(self):
        # QE2: canonicalization; QE6: engine-caused X10 pre-check rejection.
        for built in (BUILT, {'levels': list(range(1, 9)),
                              'builds': [
                                  'QE2',
                                  'QE6',
                              ], 'rejects': ['QE6']}):
            with self.subTest(built=built):
                self.assertTrue({
                    'QE2',
                    'QE6',
                } <= predictor.pending_rules(built))

    def test_pending_tokens_are_real_rules(self):
        rules = predictor.load(KIT.parents[2] / 'internal/docrefs/rule-ledger.json')['rules']
        valid = set(rules)
        valid.update((KIT.parents[2] / 'internal/docrefs/unmapped-rules.txt').read_text().splitlines())
        for built in (BUILT, {'levels': [], 'builds': [], 'rejects': []}):
            pending = predictor.pending_rules(built)
            self.assertTrue(pending)
            for rule in pending:
                self.assertRegex(rule, r'^Q[A-Z][0-9]+(?:\.[A-Za-z0-9_]+)?$')
                self.assertIn(rule.split('.')[0], valid)

    def test_qe6_requires_required_set_and_precheck(self):
        for rule in ('QE2.slice', 'QE3.scope', 'QE7.slice', 'QE5', 'QE6'):
            built = copy.deepcopy(BUILT)
            built['builds'] = [r for r in built['builds'] if r['rule'] != rule]
            self.assertEqual(self.case(136, built)['ending'],
                             {'exit_code': 70, 'rules': [rule], 'phase': 'planning'})

    def test_unknown_rules_fail_closed(self):
        for group in ('rejects', 'builds'):
            built = copy.deepcopy(BUILT)
            built[group].append('QE999.part')
            with self.assertRaisesRegex(ValueError, '^unknown rule: QE999.part$'):
                predictor.predict(built)

    def test_marker_requires_subject_and_qh6_end(self):
        d = KIT / 'cases/case-003'
        case = predictor.load(d / 'case.json')
        doc = predictor.load(d / 'input.json')
        ps = predictor.load(d / 'preimages.json')
        e = doc['evidence'][0]
        p = ps[e['check_id']]
        subject = predictor.load(d / 'subject.json')
        host = case['input']['host']
        host['confinement']['applied'] = True
        host['self_test'] = 'passed'
        for computation, end, expected in [
                (None, {'kind': 'exit', 'code': 65}, 1),
                (None, {'kind': 'signal', 'signal': 'SIGKILL', 'by': 'before_marker'}, 1),
                (None, {'kind': 'signal', 'signal': 'SIGKILL', 'by': 'confinement'}, 7),
                ('expected', {'kind': 'exit', 'code': 65}, 7)]:
            run = ({'computation': computation}, {'markers': {'confinement': False}, 'end': end})
            with patch.object(predictor, 'runs_for', return_value=[run]):
                self.assertEqual(predictor.deciding_level(e, p, subject, host, {}, doc['evidence'], ps), expected)

    def test_grade_wide_qo1_covers_checks_without_runs(self):
        # E-15 includes empty_target and a null-secret probe.
        for probe in ('byte_bound', 'planted_secret'):
            p = {'probe': probe, 'words': None, 'expected': None,
                 'parameters': {}}
            self.assertFalse(predictor.has_invocation(p))
            self.assertFalse(predictor.has_computation(p))
            for applied, self_test in ((False, 'passed'), (True, 'failed')):
                host = {'confinement': {'applied': applied}, 'self_test': self_test}
                self.assertEqual(predictor.deciding_level(
                    {'reason': 'network_unverifiable'}, p, {'secret': None}, host,
                    {'invocations': []}, [], {}), 1)

    def test_record_gaps_follow_collection(self):
        for rule in ('QE8', 'QE9', 'QE10.record', 'QE11', 'QS4', 'QT2',
                     'QE14', 'QE16', 'QE17'):
            built = copy.deepcopy(BUILT)
            built['builds'] = [r for r in built['builds'] if r['rule'] != rule]
            self.assertEqual(self.case(3, built)['ending'],
                             {'exit_code': 70, 'rules': [rule], 'phase': 'record'})
            self.assertEqual(self.case(59, built)['ending']['phase'], 'sentinel')
        self.assertEqual(set(predictor.BUILD_PHASE.values()), {None, 6, 7, 8, 'record', 'safe_read', 'override'})
        for rule in predictor.names(BUILT['builds']):
            self.assertEqual(sum(rule in {r for r, phase in predictor.BUILD_PHASE.items()
                                         if phase == candidate}
                                 for candidate in (None, 6, 7, 'record', 'safe_read')), 1)

    def test_reason_table_covers_corpus(self):
        observed = set()
        for d in sorted((KIT / 'cases').iterdir()):
            if not (d / 'input.json').exists():
                continue
            case = predictor.load(d / 'case.json')
            doc = predictor.load(d / 'input.json')
            ps = predictor.load(d / 'preimages.json')
            subject = predictor.load(d / 'subject.json')
            recording = predictor.load(KIT / 'behaviour' / (case['input']['form'] + '.json'))
            for e in doc['evidence']:
                reason = e['reason'] or e['failure_kind'] or e['outcome']
                key = 'held/predicate/input_missing/empty_target' if reason in ('held', 'predicate', 'input_missing', 'empty_target') else reason
                level = predictor.deciding_level(e, ps[e['check_id']], subject, case['input']['host'], recording, doc['evidence'], ps)
                self.assertIn(level, predictor.REASON_LEVELS[key]['levels'])
                observed.add(key)
        self.assertEqual(observed, set(predictor.REASON_LEVELS))

    def test_out_is_deterministic_json(self):
        with tempfile.TemporaryDirectory() as temporary:
            out = Path(temporary) / 'prediction.json'
            subprocess.run([sys.executable, '-I', '-B', str(Path(__file__).with_name('predict-ratchet.py')),
                            '--built', str(Path(__file__).with_name('ratchet') / 'pr6.json'),
                            '--out', str(out), '--json'], check=True, capture_output=True)
            data = out.read_bytes()
            self.assertEqual(data, (json.dumps(json.loads(data), indent=2, sort_keys=True) + '\n').encode())
            self.assertEqual(data, Path(__file__).with_name('ratchet').joinpath('pr6-prediction.json').read_bytes())

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
