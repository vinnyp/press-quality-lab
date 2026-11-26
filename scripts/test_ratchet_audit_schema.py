"""Cross-check audit parts against the sealed subject-file schema."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest
import jsonschema

spec = importlib.util.spec_from_file_location('predictor', Path(__file__).with_name('predict-ratchet.py'))
predictor = importlib.util.module_from_spec(spec)
spec.loader.exec_module(predictor)
KIT = predictor.KIT


def problems(audit):
    schema = json.loads((KIT / 'schema/subject-file.schema.json').read_text())
    validator = jsonschema.Draft202012Validator(schema)
    result = []
    for case, entry in sorted(audit.items()):
        parts = [v.split('.', 1)[1] for v in entry['violations'] if '.' in v]
        duplicates = []
        def unique(pairs):
            result = {}
            for name, value in pairs:
                if name in result:
                    duplicates.append(name)
                result[name] = value
            return result
        subject = json.loads((KIT / 'cases' / case / 'subject.json').read_text(),
                             object_pairs_hook=unique)
        rejected = bool(duplicates) or not validator.is_valid(subject)
        if rejected != ('QS2' in entry['violations']):
            result.append((case, 'QS2', rejected))
        for part in parts:
            if rejected != (part == 'schema'):
                result.append((case, part, rejected))
    return result


class AuditSchemaTests(unittest.TestCase):
    def test_audit_labels(self):
        self.assertEqual(problems(predictor.DERIVATION_AUDIT), [])

    def test_unparted_labels_are_checked(self):
        for case in ('case-113', 'case-117', 'case-118'):
            audit = copy.deepcopy(predictor.DERIVATION_AUDIT)
            audit[case]['violations'].append('QS2')
            self.assertEqual(problems(audit), [(case, 'QS2', False)])
        audit = copy.deepcopy(predictor.DERIVATION_AUDIT)
        audit['case-129']['violations'].remove('QS2')
        self.assertEqual(problems(audit), [('case-129', 'QS2', True)])

    def test_wrong_labels_are_detected_in_both_directions(self):
        audit = copy.deepcopy(predictor.DERIVATION_AUDIT)
        audit['case-102']['violations'] = ['QS3.semantic', 'QS2']
        audit['case-104']['violations'] = ['QS6.schema']
        self.assertEqual(problems(audit), [('case-102', 'semantic', True),
                                          ('case-104', 'schema', False)])


if __name__ == '__main__':
    unittest.main()
