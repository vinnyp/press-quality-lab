# /// script
# dependencies = ["jsonschema==4.23.0"]
# ///
"""Run a command twice against each case and compare selected fields."""

import math
import argparse
import sys
from pathlib import Path
sys.dont_write_bytecode = True
from common import Error, evaluate, load, manifest, read, validate_expect, INPUT_LIMIT


def main():
    """Parse command arguments and return the command exit status."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--judge", required=True)
    parser.add_argument("--cases", type=Path, required=True)
    parser.add_argument("--timeout", type=float, default=10)
    args = parser.parse_args()
    try:
        if not math.isfinite(args.timeout) or args.timeout <= 0:
            raise Error("timeout must be positive and finite")
        root = args.cases.resolve().parent
        manifest(root, "KIT.sha256")
        schema = load(read(root / "schema/judge-output.schema.json"))
        cases = []
        for directory in sorted(args.cases.iterdir()):
            if not directory.is_dir():
                raise Error("unexpected case file")
            case = load(read(directory / "case.json"))
            if not isinstance(case, dict) or not isinstance(case.get("id"), str):
                raise Error("invalid case")
            validate_expect(case["expect"])
            cases.append((case, directory / "input.json"))
        ids = [case["id"] for case, _ in cases]
        if len(ids) != len(set(ids)):
            raise Error("duplicate case id")
        failures = 0
        for case, data in cases:
            if data.stat().st_size > INPUT_LIMIT:
                raise Error("input size limit")
            with data.open("rb") as stream:
                result = evaluate(args.judge, stream, case["expect"], schema, args.timeout)
            if not result["pass"]:
                failures += 1
                print(case["id"] + ": " + "; ".join(result["reasons"]))
        print(
            f"{len(cases)} cases ran; {len(cases) - failures} passed; {failures} failed"
        )
        return int(failures > 0 or not cases)
    except (Error, OSError, ValueError, KeyError, TypeError) as exc:
        print("refused: " + str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
