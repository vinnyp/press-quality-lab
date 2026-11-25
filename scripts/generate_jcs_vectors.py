"""Generate checked-in JCS vectors once; Go tests never invoke Python."""
import json
import re
import sys
from pathlib import Path
import struct
sys.path.insert(0, str(Path(__file__).resolve().parent))
import collection_ids as ids

root = Path(__file__).resolve().parents[1]
vectors = []

def add(name, text):
    value = json.loads(text, parse_int=float)
    vectors.append(dict(name=re.sub(r"[^A-Za-z0-9_]", "_", name), input=text, canonical=ids.canon(value), digest=ids.check_id(value)))

add("utf16", json.dumps({"\ue000": 1, "\uffff": 2, "\U00010000": 3, "\U0001f600": 4, "a": 5, "aa": 6, "": 7}))
add("escapes", json.dumps("".join(chr(i) for i in range(32)) + '"\\/é<>\u2028\u2029'))
add("literals", '[true,false,null,{},[]]')
for text in ['-0', '0.10', '1.0', '1E+30', '2e-3', '1e-27', '1e-6', '1e-7', '1e20', '1e21']:
    add("number-" + text, text)
# RFC 8785 Appendix B binary64 inputs, including adjacent shortest-form boundaries.
for bits in ['0000000000000001','8000000000000001','7fefffffffffffff','ffefffffffffffff','4340000000000000','c340000000000000','4430000000000000','44b52d02c7e14af5','44b52d02c7e14af6','44b52d02c7e14af7','444b1ae4d6e2ef4e','444b1ae4d6e2ef4f','444b1ae4d6e2ef50','3eb0c6f7a0b5ed8c','3eb0c6f7a0b5ed8d','41b3de4355555553','41b3de4355555554','41b3de4355555555','41b3de4355555556','41b3de4355555557','becbf647612f3696','43143ff3c1cb0959']:
    value = struct.unpack('>d', bytes.fromhex(bits))[0]
    add("binary64-" + bits, ids.number(value))
generated = json.dumps(vectors, ensure_ascii=False, indent=2) + '\n'
target = root / 'internal/jcs/testdata/vectors.json'
if '--stdout' in sys.argv:
    sys.stdout.write(generated)
elif '--check' in sys.argv:
    if target.read_text() != generated:
        raise SystemExit('FAIL: checked-in JCS vectors differ from generator')
else:
    target.write_text(generated)
