"""RFC 8785 canonical JSON and QE2 digests; never constructs a required set."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import stat
import sys


def load(path):
    """Read UTF-8 JSON, refusing duplicate keys and nonfinite numbers."""
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate key: " + key)
            result[key] = value
        return result

    def constant(value):
        raise ValueError("nonfinite number: " + value)

    return json.loads(Path(path).read_text(encoding="utf-8"), object_pairs_hook=unique, parse_constant=constant)


def number(value):
    """Serialize a binary64 value using ECMAScript's decimal thresholds."""
    value = float(value)
    if not math.isfinite(value):
        raise ValueError("nonfinite number")
    if value == 0:
        return "0"
    sign = "-" if value < 0 else ""
    raw = repr(abs(value)).lower()
    mantissa, _, exponent = raw.partition("e")
    digits = mantissa.replace(".", "")
    point = (mantissa.index(".") if "." in mantissa else len(mantissa)) + int(exponent or 0)
    while digits.startswith("0"):
        digits = digits[1:]
        point -= 1
    digits = digits.rstrip("0")
    if 0 < point <= 21:
        body = digits[:point] + ("." + digits[point:] if point < len(digits) else "0" * (point - len(digits)))
    elif -6 < point <= 0:
        body = "0." + "0" * -point + digits
    else:
        power = point - 1
        body = digits[0] + ("." + digits[1:] if len(digits) > 1 else "") + "e" + ("+" if power >= 0 else "-") + str(abs(power))
    return sign + body


def canon(value):
    """Return RFC 8785 text, with UTF-16 key order and minimal escaping."""
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return number(value)
    if isinstance(value, str):
        try:
            value.encode("utf-8")
        except UnicodeError as exc:
            raise ValueError("unpaired surrogate") from exc
        return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    if isinstance(value, list):
        return "[" + ",".join(map(canon, value)) + "]"
    if isinstance(value, dict):
        return "{" + ",".join(canon(key) + ":" + canon(value[key]) for key in sorted(value, key=lambda key: key.encode("utf-16-be"))) + "}"
    raise ValueError("not a JSON value")


def check_id(value):
    return hashlib.sha256(canon(value).encode("utf-8")).hexdigest()


def file_digest(path):
    with Path(path).open("rb") as stream:
        digest = hashlib.sha256()
        for block in iter(lambda: stream.read(65536), b""):
            digest.update(block)
        return digest.hexdigest()


def seed(directory):
    """Hash directory entries without following links or opening special files."""
    directory = Path(directory)
    if not stat.S_ISDIR(directory.lstat().st_mode):
        raise ValueError("seed is not a directory")
    entries = []

    def walk(parent):
        for path in parent.iterdir():
            mode = path.lstat().st_mode
            relative = path.relative_to(directory).as_posix()
            if stat.S_ISDIR(mode):
                entries.append([relative, "directory", None])
                walk(path)
            elif stat.S_ISREG(mode):
                entries.append([relative, "file", file_digest(path)])
            else:
                raise ValueError("seed contains a link or special file: " + relative)

    walk(directory)
    entries.sort(key=lambda entry: entry[0].encode("utf-8"))
    return check_id(entries)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("canon", "id", "seed", "file"))
    parser.add_argument("path", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "canon":
            print(canon(load(args.path)))
        elif args.command == "id":
            print(check_id(load(args.path)))
        elif args.command == "seed":
            print(seed(args.path))
        else:
            print(file_digest(args.path))
        return 0
    except (ValueError, OSError, OverflowError) as exc:
        print("refused: " + str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
