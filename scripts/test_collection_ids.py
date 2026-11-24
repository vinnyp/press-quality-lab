"""Independent examples for QE2 and RFC 8785."""
import importlib.util
import json
from pathlib import Path
import re
import struct
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("collection_ids", Path(__file__).with_name("collection_ids.py"))
ids = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ids)
KIT = Path(__file__).resolve().parents[1] / "docs/specs/conformance-collection-1.0"


class CollectionIDs(unittest.TestCase):
    def test_literals_and_lossless_numbers(self):
        self.assertEqual(ids.canon([True, False, None]), "[true,false,null]")
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "number.json"
            for text in ["NaN", "9007199254740993", "0.123456789012345678901"]:
                path.write_text(text)
                with self.subTest(text=text), self.assertRaises(ValueError):
                    ids.canon(ids.load(path))
        with self.assertRaises(ValueError):
            ids.canon(9007199254740993)

    def test_sorting(self):
        value = {"\u20ac": "Euro Sign", "\r": "Carriage Return", "\ufb33": "Hebrew Letter Dalet With Dagesh", "1": "One", "\U0001f600": "Emoji: Grinning Face", "\u0080": "Control", "\u00f6": "Latin Small Letter O With Diaeresis"}
        self.assertEqual(list(json.loads(ids.canon(value))), ["\r", "1", "\u0080", "\u00f6", "\u20ac", "\U0001f600", "\ufb33"])

    def test_numbers(self):
        for number, expected in [(1e30, "1e+30"), (4.50, "4.5"), (2e-3, "0.002"), (1e-27, "1e-27"), (-0.0, "0"), (333333333.33333329, "333333333.3333333"), (1e-6, "0.000001"), (1e20, "100000000000000000000"), (1e21, "1e+21"), (5e-324, "5e-324")]:
            with self.subTest(number=number):
                self.assertEqual(ids.canon(number), expected)
        for value in [float("nan"), float("inf"), -float("inf")]:
            with self.assertRaises(ValueError):
                ids.canon(value)

    def test_rfc_binary64_vectors(self):
        # RFC 8785 Appendix B; hexadecimal values identify exact binary64 inputs.
        vectors = {
            "0000000000000000": "0", "8000000000000000": "0",
            "0000000000000001": "5e-324", "8000000000000001": "-5e-324",
            "7fefffffffffffff": "1.7976931348623157e+308",
            "ffefffffffffffff": "-1.7976931348623157e+308",
            "4340000000000000": "9007199254740992", "c340000000000000": "-9007199254740992",
            "4430000000000000": "295147905179352830000",
            "44b52d02c7e14af5": "9.999999999999997e+22", "44b52d02c7e14af6": "1e+23",
            "44b52d02c7e14af7": "1.0000000000000001e+23",
            "444b1ae4d6e2ef4e": "999999999999999700000",
            "444b1ae4d6e2ef4f": "999999999999999900000", "444b1ae4d6e2ef50": "1e+21",
            "3eb0c6f7a0b5ed8c": "9.999999999999997e-7", "3eb0c6f7a0b5ed8d": "0.000001",
            "41b3de4355555553": "333333333.3333332", "41b3de4355555554": "333333333.33333325",
            "41b3de4355555555": "333333333.3333333", "41b3de4355555556": "333333333.3333334",
            "41b3de4355555557": "333333333.33333343", "becbf647612f3696": "-0.0000033333333333333333",
            "43143ff3c1cb0959": "1424953923781206.2",
        }
        for encoded, expected in vectors.items():
            with self.subTest(binary64=encoded):
                self.assertEqual(ids.canon(struct.unpack(">d", bytes.fromhex(encoded))[0]), expected)

    def test_strings(self):
        self.assertEqual(ids.canon("\x00\b\t\n\f\r\"\\/é"), '"\\u0000\\b\\t\\n\\f\\r\\"\\\\/é"')
        with self.assertRaises(ValueError):
            ids.canon("\ud800")
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary) / "duplicate.json"
            path.write_text('{"a":1,"a":2}')
            with self.assertRaises(ValueError):
                ids.load(path)

    def test_worked_ids(self):
        text = (KIT / "collection-contract.md").read_text()
        preimages = re.findall(r'^\s*(\{"bookkeeping"[^\n]+)\s*$', text, re.M)
        expected = ["46c4884c4195f646969ad14aca889bb614643f7c5e7e88bdec87d1032c0344d7", "c7823c4ab628d3eac5e2b51de1d41e12293d3e017f60df678eaf92532e7832f7", "40b08dbef70438992c53f14fee9f5e1297277bc94bb35cb5e2b435758df22993", "181431d07ae5e61a4a5fde89d6a606708476067a3dcdfad263b473f33adb98ba"]
        self.assertEqual(len(preimages), 4)
        with tempfile.TemporaryDirectory() as temporary:
            seed = Path(temporary)
            (seed / "data.json").write_bytes(b'{"widgets":[]}\n')
            first = ids.seed(seed)
            self.assertEqual(first, "d450f3278f1a38baa7d5306dc2a5fd3de089508bdd1f4b22e1d8d5355b5fc2ee")
            (seed / "data.json").write_bytes(b'{"widgets":[{"name":"gear"}]}\n')
            (seed / ".config/widgets").mkdir(parents=True)
            (seed / ".config/widgets/token").write_bytes(b"PLANTED-TOKEN-0001\n")
            second = ids.seed(seed)
            self.assertEqual(second, "c7456dbd3a1fad65c51397e7f49302cba427ae628dd816e9f3b289439c01ff4a")
            for index, (preimage, digest) in enumerate(zip(preimages, expected)):
                value = json.loads(preimage)
                self.assertEqual(value["seed"], first if index == 0 else second)
                self.assertEqual(ids.check_id(value), digest)

    def test_seed_refuses_links(self):
        with tempfile.TemporaryDirectory() as temporary:
            seed = Path(temporary)
            (seed / "link").symlink_to("missing")
            with self.assertRaises(ValueError):
                ids.seed(seed)

    def test_examples(self):
        for path in sorted((KIT / "examples").glob("baseline-*.json")):
            with self.subTest(example=path.name):
                value = ids.load(path)
                self.assertEqual(set(value["preimages"]), {entry["check_id"] for entry in value["baseline"]["entries"]})
                for digest, preimage in value["preimages"].items():
                    self.assertEqual(ids.check_id(preimage), digest)


if __name__ == "__main__":
    unittest.main()
