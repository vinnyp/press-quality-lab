"""Cross-check case preimages, seeds and observed streams against recordings."""
import argparse
import importlib.util
from pathlib import Path
import sys

_spec = importlib.util.spec_from_file_location("collection_ids", Path(__file__).with_name("collection_ids.py"))
ids = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(ids)


def key(preimage, invocation=None):
    invocation = preimage if invocation is None else invocation
    return dict(words=invocation["words"], offline_args=invocation["offline_args"], env=sorted(invocation["env"], key=lambda name: name.encode("utf-8")), runbook=preimage["runbook"], step=preimage["step"], computation=None)


def discovery_keys(recording):
    """Walk the recorded help tree in QD2 order, without running a subject."""
    found = []

    def walk(words):
        invocation_key = dict(words=words + ["--help"], offline_args=[], env=[], runbook=None, step=None, computation=None)
        matches = [entry for entry in recording["invocations"] if entry["key"] == invocation_key]
        if len(matches) != 1:
            raise ValueError("cannot decide discovery invocation: " + ids.canon(invocation_key))
        found.append(invocation_key)
        run = matches[0]["confined"][0]
        stream = run["stdout"] if run["stdout"]["bytes"] else run["stderr"]
        if stream["cut"] or stream["bytes"] > len(stream["head"].encode("utf-8")):
            raise ValueError("cannot decide discovery order from truncated help")
        section = False
        for line in stream["head"].splitlines():
            if not line or not line.startswith(" "):
                section = line in ("COMMANDS:", "Commands:", "SUBCOMMANDS:") or line.endswith(" Commands:")
            elif section:
                pieces = line.split(None, 1)
                if not pieces:
                    continue
                command = pieces[0].removesuffix(",")
                description = pieces[1] if len(pieces) == 2 else ""
                if command not in ("help", "completion") and not description.startswith(("(hidden)", "[hidden]", "HIDDEN")):
                    if len(words) >= 64 or len(found) >= 4096:
                        return
                    walk(words + [command])

    walk([])
    return found


def target_keys(preimage, evidence, recording):
    probe = preimage["probe"]
    if probe == "discovery":
        invocations = discovery_keys(recording)
    elif probe == "planted_secret":
        invocations = [key(preimage, command) for command in preimage["parameters"].get("commands", [])]
    elif preimage["words"] is not None:
        invocations = [key(preimage)]
    else:
        invocations = []
    result = []
    check_id = evidence["check_id"]
    for target in evidence["targets"]:
        if target["kind"] != "invocation":
            continue
        if not invocations:
            raise ValueError("cannot decide invocation for no-command target " + target["id"])
        names = [check_id] if len(invocations) == 1 else [check_id + "#" + str(index + 1) for index in range(len(invocations))]
        if target["id"] not in names:
            raise ValueError("invalid invocation target numbering: " + target["id"])
        result.append((target["id"], invocations[names.index(target["id"])]))
    return result


def streams(value, path=()):
    if isinstance(value, dict):
        if set(value) == {"bytes", "sha256", "cut", "head"}:
            yield path, value
        else:
            for name, child in value.items():
                yield from streams(child, path + (name,))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from streams(child, path + (index,))


def check(cases):
    if not cases.exists() and not cases.is_symlink():
        print("no cases yet")
        return
    checked = 0
    for directory in sorted(cases.iterdir()):
        case = ids.load(directory / "case.json")
        if case["kind"] != "judge-checked":
            continue
        checked += 1
        preimages = ids.load(directory / "preimages.json")
        record = ids.load(directory / "input.json")
        if set(preimages) != {entry["id"] for entry in record["required"]}:
            raise ValueError(case["id"] + ": preimage keys differ from required")
        root = cases.parent
        seed_path = root / "subjects" / case["input"]["form"] / "seed"
        expected_seed = ids.seed(seed_path) if seed_path.exists() or seed_path.is_symlink() else None
        recording = ids.load(root / "behaviour" / (case["input"]["form"] + ".json"))
        for digest, preimage in preimages.items():
            if ids.check_id(preimage) != digest:
                raise ValueError(case["id"] + ": preimage id mismatch: " + digest)
            if preimage["seed"] != expected_seed:
                raise ValueError(case["id"] + ": seed digest mismatch: " + digest)
        for evidence in record["evidence"]:
            targets = target_keys(preimages[evidence["check_id"]], evidence, recording)
            runs = []
            for name, invocation_key in targets:
                if invocation_key not in case["invocations"]:
                    raise ValueError(case["id"] + ": invocation not in case: " + ids.canon(invocation_key))
                matches = [entry for entry in recording["invocations"] if entry["key"] == invocation_key]
                if len(matches) != 1:
                    raise ValueError(case["id"] + ": cannot decide recorded invocation: " + name)
                runs.append((name, matches[0]["confined"][0]))
            for path, stream in streams(evidence["observed"]):
                names = [name for name in path if name in ("stdout", "stderr")]
                selected = [run for name, run in runs if name in path] if len(runs) != 1 else [runs[0][1]]
                if len(names) != 1 or len(selected) != 1:
                    raise ValueError(case["id"] + ": cannot decide stream invocation at " + str(path))
                recorded = selected[0][names[0]]
                if any(stream[field] != recorded[field] for field in ("bytes", "sha256")):
                    raise ValueError(case["id"] + ": stream differs from recording at " + str(path))
    print(f"{checked} judge-checked cases cross-checked")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cases", type=Path)
    args = parser.parse_args()
    try:
        check(args.cases)
        return 0
    except (ValueError, OSError, KeyError, TypeError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
