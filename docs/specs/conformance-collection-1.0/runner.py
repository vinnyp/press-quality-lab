# /// script
# dependencies = ["jsonschema==4.23.0"]
# ///
"""Run a command twice against each case and compare selected fields."""

import math
import argparse
import sys
from pathlib import Path
sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parent))
from common import Error, collection_cases, manifest


def main():
    """Parse command arguments and return the command exit status."""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--draft", action="store_true")
    parser.add_argument("--unsealed", action="store_true")
    parser.add_argument("--schema-root", type=Path, default=Path(__file__).resolve().parent)
    parser.add_argument("--judge")
    parser.add_argument("--cases", type=Path)
    parser.add_argument("--timeout", type=float, default=10)
    args = parser.parse_args()
    if args.draft:
        print("UNSEALED", flush=True)
        root = Path(__file__).resolve().parent
        if (root / "KIT.sha256").exists() or (root / "KIT.sha256").is_symlink():
            print("refused: draft folder contains KIT.sha256", file=sys.stderr)
            return 2
        try:
            from common import draft
            return draft(root)
        except Exception as exc:
            print("draft failed: " + str(exc), file=sys.stderr)
            return 1
    if not args.judge or not args.cases:
        parser.error("--judge and --cases are required without --draft")
    try:
        root = args.schema_root.resolve()
        if args.unsealed:
            print("UNSEALED", flush=True)
            if (root / "KIT.sha256").exists() or (root / "KIT.sha256").is_symlink():
                raise Error("--unsealed beside KIT.sha256")
        else:
            manifest(root, "KIT.sha256")
        if not math.isfinite(args.timeout) or args.timeout <= 0:
            raise Error("timeout must be positive and finite")
        return collection_cases(root, args.cases, args.judge, args.timeout)

    except (Error, OSError, ValueError, KeyError, TypeError) as exc:
        print("refused: " + str(exc), file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
