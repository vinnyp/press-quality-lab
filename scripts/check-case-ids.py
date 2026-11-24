"""Cross-check case preimages, seeds and observed streams against recordings."""
import argparse
import base64
import importlib.util
from pathlib import Path
import sys

_spec = importlib.util.spec_from_file_location("collection_ids", Path(__file__).with_name("collection_ids.py"))
ids = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(ids)


def key(preimage, invocation=None):
    invocation = preimage if invocation is None else invocation
    return dict(words=invocation["words"], offline_args=invocation["offline_args"], env=sorted(invocation["env"], key=lambda name: name.encode("utf-8")), runbook=preimage["runbook"], step=preimage["step"], computation=None, home="runbook" if preimage["runbook"] is not None else "planted" if preimage["probe"] == "planted_secret" else "seed")


def discovery_keys(recording, first_only=False, status=None):
    """Walk the recorded help tree in QD2 order, without running a subject."""
    found = []

    stopped = False

    def walk(words):
        nonlocal stopped
        if stopped:
            return
        if len(words) > 64 or len(found) >= 4096:
            stopped = True
            if status is not None:
                status["bounded"] = True
            return
        invocation_key = dict(words=words + ["--help"], offline_args=[], env=[], runbook=None, step=None, computation=None, home="empty")
        matches = [entry for entry in recording["invocations"] if entry["key"] == invocation_key]
        if len(matches) != 1:
            raise ValueError("cannot decide discovery invocation: " + ids.canon(invocation_key))
        found.append(invocation_key)
        if first_only:
            return
        run = matches[0]["confined"][0]
        if not run["markers"]["confinement"] or run["markers"]["could_not_start"]:
            return
        stream = run["stdout"] if run["stdout"]["bytes"] else run["stderr"]
        if stream["cut"]:
            raise ValueError("cannot decide discovery order from truncated help")
        text = base64.b64decode(stream["kept"], validate=True).decode("utf-8", errors="replace") if stream.get("kept") is not None else stream["head"]
        if stream["bytes"] > len(text.encode("utf-8")):
            raise ValueError("cannot decide discovery order from truncated help")
        section = False
        for line in text.splitlines():
            if not line or not line.startswith(" "):
                section = line in ("COMMANDS:", "Commands:", "SUBCOMMANDS:") or line.endswith(" Commands:")
            elif section:
                pieces = line.split(None, 1)
                if not pieces:
                    continue
                command = pieces[0].removesuffix(",")
                description = pieces[1] if len(pieces) == 2 else ""
                if command not in ("help", "completion") and not description.startswith(("(hidden)", "[hidden]", "HIDDEN")):
                    walk(words + [command])
                    if stopped:
                        return

    walk([])
    return found


def target_keys(preimage, evidence, recording, host, binary):
    probe = preimage["probe"]
    if probe == "discovery":
        first_only = (isinstance(binary, dict) or
                      host.get("confinement", {}).get("applied") is False or
                      host.get("self_test") == "failed" or
                      host.get("private_copy") == "changed")
        invocations = discovery_keys(recording, first_only)
    elif probe == "planted_secret":
        invocations = [key(preimage, command) for command in preimage["parameters"].get("commands", [])]
    elif preimage["words"] is not None:
        invocations = [key(preimage)]
    else:
        invocations = []
    result = []
    check_id = evidence["check_id"]
    if not invocations:
        invocation_targets = [target for target in evidence["targets"] if target["kind"] == "invocation"]
        if len(invocation_targets) != 1 or invocation_targets[0]["id"] != check_id:
            raise ValueError("no-invocation check requires one invocation target with the check id: " + check_id)
        if any(streams(evidence["observed"])):
            raise ValueError("no-invocation check has an observed stream: " + check_id)
        return [(check_id, None)]
    names = [check_id] if len(invocations) == 1 else [check_id + "#" + str(index + 1) for index in range(len(invocations))]
    actual = [target["id"] for target in evidence["targets"] if target["kind"] == "invocation"]
    if actual != names:
        raise ValueError("invalid invocation target numbering: expected " + str(names) + "; got " + str(actual))
    result = list(zip(names, invocations))
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
        raise ValueError("no judge-checked cases checked")
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
        # These fixtures exist only after assembly. A link or named pipe in
        # the seed rejects the subject under QS9, even if the canonical seed
        # directory by itself contains only regular files.
        for fixture in case["input"]["fixtures"]:
            if fixture["kind"] in ("link", "special_file") and (fixture["path"] == "seed" or fixture["path"].startswith("seed/")):
                raise ValueError(case["id"] + ": seed contains a link or special file: " + fixture["path"])
        subject = ids.load(directory / case["input"]["subject_file"])
        subject_seed = subject["seed"]
        if subject_seed is None or isinstance(subject_seed, dict) and set(subject_seed) == {"placeholder"}:
            expected_seed = subject_seed
        else:
            expected_seed = ids.seed(seed_path) if seed_path.exists() or seed_path.is_symlink() else None
        recording = ids.load(root / "behaviour" / (case["input"]["form"] + ".json"))
        for digest, preimage in preimages.items():
            if ids.check_id(preimage) != digest:
                raise ValueError(case["id"] + ": preimage id mismatch: " + digest)
            if preimage["seed"] != expected_seed:
                raise ValueError(case["id"] + ": seed digest mismatch: " + digest)
        for evidence in record["evidence"]:
            targets = target_keys(preimages[evidence["check_id"]], evidence, recording,
                                  case["input"]["host"] or {}, subject["binary"])
            pre = preimages[evidence["check_id"]]
            artifacts = []
            if pre["probe"] == "store_unchanged" and pre["words"] is not None and isinstance(subject["store"], str):
                artifacts.append(subject["store"])
            if pre["probe"] == "planted_secret" and isinstance(subject["secret"], dict) and "placeholder" not in subject["secret"]:
                artifacts.append(subject["secret"]["path"])
            if [t["id"] for t in evidence["targets"] if t["kind"] == "artifact"] != artifacts:
                raise ValueError(case["id"] + ": artifact targets differ from subject")
            scripted = case["input"]["verifier"] == "scripted"
            tiers = []
            for name, invocation in targets:
                offline = False
                if invocation is not None and pre["probe"] != "discovery":
                    commands = [c for c in subject["commands"] if invocation["words"][:len(c["words"])] == c["words"]]
                    command = max(commands, key=lambda c: len(c["words"]), default={"words": []})
                    offline = any(command["words"] in setting["applies_to"] for setting in subject["offline"])
                tiers.append("scripted" if scripted else "offline" if offline else "live")
            artifact_tier = "scripted" if scripted else "offline" if all(t == "offline" for t in tiers) else "live"
            expected_targets = [("invocation", name, tier) for (name, _), tier in zip(targets, tiers)]
            expected_targets += [("artifact", name, artifact_tier) for name in artifacts]
            actual_targets = [(t["kind"], t["id"], t["tier"]) for t in evidence["targets"]]
            if sorted(actual_targets) != sorted(expected_targets):
                raise ValueError(case["id"] + ": target kinds, ids or tiers differ")
            tier = "scripted" if scripted else "offline" if all(t[2] == "offline" for t in expected_targets) else "live"
            if evidence["tier"] != tier:
                raise ValueError(case["id"] + ": evidence tier differs")
            runs = []
            for name, invocation_key in targets:
                if invocation_key is None:
                    continue
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
    if not checked:
        raise ValueError("no judge-checked cases checked")
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
