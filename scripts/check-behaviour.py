"""Validate recorded behaviour and QT5 agreement without running subjects."""
import argparse
import json
from pathlib import Path
import sys

from jsonschema import Draft202012Validator


def agreement(run):
    """QT5 excludes stream heads/kept bytes and the comparator control result."""
    return {
        name: ({key: value[key] for key in ("bytes", "sha256", "cut")}
               if name in ("stdout", "stderr") else
               value["changes"] if name == "home_changes" else value)
        for name, value in run.items()
    }


def check(root):
    schema = json.loads((root / "schema/behaviour.schema.json").read_text())
    validator = Draft202012Validator(schema)
    count = 0
    for path in sorted((root / "behaviour").glob("*.json")):
        recording = json.loads(path.read_text())
        validator.validate(recording)
        keys = set()
        for entry in recording["invocations"]:
            key = json.dumps(entry["key"], sort_keys=True, separators=(",", ":"))
            if key in keys:
                raise ValueError(f"{path.name}: duplicate invocation key")
            keys.add(key)
            first = agreement(entry["confined"][0])
            if any(agreement(run) != first for run in entry["confined"][1:]):
                raise ValueError(f"{path.name}: confined runs disagree")
        count += 1
    print(f"{count} behaviour recordings validated")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kit", type=Path)
    args = parser.parse_args()
    try:
        check(args.kit)
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
