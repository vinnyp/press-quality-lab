"""Check that the committed vectors match their independent generator."""
from pathlib import Path
import os
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[1]


class GeneratedVectors(unittest.TestCase):
    def test_checked_in_vectors(self):
        result = subprocess.run(
            [sys.executable, "-I", str(ROOT / "scripts/generate_jcs_vectors.py"), "--stdout"],
            check=True, capture_output=True,
            env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
        )
        self.assertEqual(
            (ROOT / "internal/jcs/testdata/vectors.json").read_bytes(),
            result.stdout,
        )


if __name__ == "__main__":
    unittest.main()
