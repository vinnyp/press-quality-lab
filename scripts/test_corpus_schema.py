"""KTD15 schema controls use the sealed kit's examples, never engine output."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest

import jsonschema

spec = importlib.util.spec_from_file_location(
    'validation', Path(__file__).with_name('validate-corpus-records.py'))
validation = importlib.util.module_from_spec(spec)
spec.loader.exec_module(validation)
KIT = Path(__file__).resolve().parents[1] / 'docs/specs/conformance-collection-1.0'


class SchemaControls(unittest.TestCase):
    def test_schema_examples(self):
        for name, record, baseline, error, message in [
            ('valid_record', 'record-01.json', None, None, 'corpus exports: 1 records and 0 baselines validated'),
            ('valid_baseline', 'record-01.json', 'baseline-01.json', None, 'corpus exports: 1 records and 1 baselines validated'),
            ('rejected_record', 'record-02.json', None, ValueError, 'corpus record was judge rejected: case-001'),
            ('invalid_baseline', 'record-01.json', 'invalid/baseline-QE2-01.json', jsonschema.ValidationError, None),
        ]:
            with self.subTest(name=name), tempfile.TemporaryDirectory(prefix='pql-schema-') as temporary:
                root = Path(temporary)
                cases = root / 'cases'
                case = cases / 'case-001'
                case.mkdir(parents=True)
                (case / 'case.json').write_text(json.dumps({
                    'id': 'case-001', 'kind': 'judge-checked', 'input': {'verifier': 'scripted'}}))
                pending = root / 'pending.txt'
                pending.write_text('')
                copies = root / 'copies'
                copies.mkdir()
                (copies / 'case-001.record.json').write_bytes((KIT / 'examples' / record).read_bytes())
                if baseline:
                    (copies / 'case-001.baseline.json').write_bytes((KIT / 'examples' / baseline).read_bytes())
                self.assertEqual(validation.expected_cases(cases, pending), {'case-001'})
                if error:
                    with self.assertRaises(error) as caught:
                        validation.validate(KIT, cases, pending, copies)
                    if message:
                        self.assertEqual(str(caught.exception), message)
                else:
                    output = io.StringIO()
                    with contextlib.redirect_stdout(output):
                        validation.validate(KIT, cases, pending, copies)
                    self.assertEqual(output.getvalue(), message + '\n')


if __name__ == '__main__':
    unittest.main()
