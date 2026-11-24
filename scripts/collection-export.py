"""Export collector inputs without the corpus's judge oracle."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import sys


RULE = re.compile(r"\bQ[SHODPAET][0-9]+\b")
WORDS = "held_crediting held_non_crediting not_held not_observed unobserved failed_expected network_unverifiable harness_fault could_not_start precondition_unmet placeholder_unfilled earlier_step_failed computation_failed input_missing empty_target wrong_exit over_budget crashed PASSED FAILED PROVISIONAL".split()
RESULT = re.compile("|".join(map(re.escape, WORDS)))


def load(path):
    return json.loads(path.read_bytes())


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def digest(name):
    return hashlib.sha256(name.encode("utf-8")).digest()


def cases(directories):
    return [(index, path.parent, load(path)) for index, directory in enumerate(directories)
            for path in sorted(directory.glob("*/case.json"))]


def local(root, name):
    path = root / name
    if Path(name).is_absolute() or not path.resolve().is_relative_to(root.resolve()):
        raise ValueError("input path escapes its directory")
    return path


def export(args, corpus):
    out = args.out.resolve()
    if args.out.exists() or args.out.is_symlink():
        raise ValueError("--out must not exist")
    if args.map.resolve().is_relative_to(out):
        raise ValueError("--map must not lie inside --out")
    forms = sorted({case["input"]["form"] for _, _, case in corpus}, key=digest)
    form_ids = {name: f"f{index:02d}" for index, name in enumerate(forms, 1)}
    args.out.mkdir(parents=True)
    (out / "spec").mkdir()
    for name in ("collection-rules.md", "confinement.md", "collection-contract.md", "credit-and-routes.md"):
        shutil.copyfile(args.kit / name, out / "spec" / name)
    shutil.copytree(args.kit / "schema", out / "spec/schema")
    mapping = {"forms": {identifier: name for name, identifier in form_ids.items()}, "cases": {}}
    for form, identifier in form_ids.items():
        for name in ("seed", "computations"):
            source = local(args.kit / "subjects", form) / name
            if source.exists():
                shutil.copytree(source, out / "subjects" / identifier / name)
        recording = load(local(args.kit / "behaviour", form + ".json"))
        recording["form"] = identifier
        write(out / "behaviour" / (identifier + ".json"), recording)
    for number, (index, directory, case) in enumerate(sorted(corpus, key=lambda item: digest(item[2]["id"])), 1):
        identifier = f"k{number:03d}"
        inputs = dict(case["input"], form=form_ids[case["input"]["form"]])
        target = out / "cases" / identifier
        write(target / "case.json", {"id": identifier, "kind": case["kind"], "input": inputs})
        for name in (inputs["subject_file"], inputs["baseline"]):
            if name is not None:
                destination = local(target, name)
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(local(directory, name), destination)
        mapping["cases"][identifier] = {"id": case["id"], "cases_dir": index}
    write(args.map, mapping)


def strings(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, list):
        for item in value:
            yield from strings(item)
    elif isinstance(value, dict):
        for item in value.values():
            yield from strings(item)


def check(args, corpus):
    forms = {path.name for path in (args.kit / "subjects").iterdir() if path.is_dir()}
    ids = set()
    for _, directory, _ in corpus:
        preimages = directory / "preimages.json"
        if preimages.exists():
            ids.update(load(preimages))
        record = directory / "input.json"
        if record.exists():
            value = load(record)
            ids.update(item["id"] for item in value.get("required", []))
            ids.update(item["check_id"] for item in value.get("evidence", []))
    baselines = set()
    for path in (args.check / "cases").glob("*/case.json"):
        baseline = load(path)["input"]["baseline"]
        if baseline is not None:
            baselines.add(local(path.parent, baseline))
    findings = []
    for section in ("cases", "behaviour", "subjects"):
        for path in sorted((args.check / section).rglob("*")):
            relative = path.relative_to(args.check)
            texts = [("name", path.name)]
            if path.is_file():
                content = path.read_bytes().decode("utf-8", errors="replace")
                try:
                    values = sorted(set(strings(json.loads(content))))
                except (ValueError, TypeError):
                    texts.append(("content", content))
                else:
                    texts.extend(("JSON", value) for value in values)
            for location, text in texts:
                for form in sorted(forms):
                    if (form in text if location == "content" else form == text):
                        findings.append(f"{relative}: form name in {location}: {form}")
                for match in sorted(set(RULE.findall(text))):
                    findings.append(f"{relative}: rule id in {location}: {match}")
                for match in sorted(set(RESULT.findall(text))):
                    findings.append(f"{relative}: result word in {location}: {match}")
                if location == "name" or path not in baselines:
                    for identifier in sorted(ids):
                        if identifier in text:
                            findings.append(f"{relative}: check id in {location}: {identifier}")
    for finding in findings:
        print(finding)
    return int(bool(findings))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kit", type=Path, required=True)
    parser.add_argument("--cases", type=Path, action="append", required=True)
    parser.add_argument("--check", type=Path)
    parser.add_argument("--out", type=Path)
    parser.add_argument("--map", type=Path)
    args = parser.parse_args()
    if args.check is not None:
        if args.out is not None or args.map is not None:
            parser.error("--check cannot be combined with --out or --map")
    elif args.out is None or args.map is None:
        parser.error("export requires --out and --map")
    try:
        corpus = cases(args.cases)
        if args.check is not None:
            return check(args, corpus)
        export(args, corpus)
        return 0
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"collection export: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
