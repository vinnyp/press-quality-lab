"""Fixture controls for the ledger growth decision, run by the Go ledger test."""
import importlib.util
from pathlib import Path
import unittest

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


if __name__ == '__main__':
    unittest.main()
