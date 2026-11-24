"""Witness sealed checks and runner controls using isolated copies."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

scripts = Path(__file__).resolve().parent
source = Path(__file__).resolve().parents[1] / "docs/specs/conformance-collection-1.0"
check_script = (scripts.parent / "check.sh").read_text()
sealed_checks = check_script.split("step collection-kit ", 1)[1].split("step collection-schemas ", 1)[0]
sealed_checks = sealed_checks[sealed_checks.index("(\n") + 2:sealed_checks.rindex("\n)")]
sealed_checks = sealed_checks.replace(" cd docs/specs/conformance-collection-1.0\n", "", 1)

for mutation, message in (
    ("unchanged", ""),
    ("changed byte", "FAILED"),
    ("extra file", "unlisted file: synthetic-extra"),
    ("missing file", "CASES.md"),
    ("incomplete corpus", "missing Corpus: complete"),
):
    with tempfile.TemporaryDirectory(prefix="pql-sealed-controls-") as temporary:
        root = Path(temporary) / "kit"
        shutil.copytree(source, root)
        corpus = root / "CASES.md"
        if mutation == "changed byte":
            data = corpus.read_bytes()
            corpus.write_bytes(bytes([data[0] ^ 1]) + data[1:])
        elif mutation == "extra file":
            (root / "synthetic-extra").write_text("synthetic witness\n")
        elif mutation == "missing file":
            corpus.unlink()
        elif mutation == "incomplete corpus":
            corpus.write_text("\n".join(line for line in corpus.read_text().splitlines()
                                         if line != "Corpus: complete") + "\n")
            # Refresh only this scratch digest to reach the corpus check.
            import hashlib
            seal = root / "KIT.sha256"
            lines = seal.read_text().splitlines()
            seal.write_text("\n".join(
                hashlib.sha256(corpus.read_bytes()).hexdigest() + "  CASES.md"
                if line.split(None, 1)[1].lstrip("*") == "CASES.md" else line
                for line in lines) + "\n")
        result = subprocess.run(
            ["bash", "-eu", "-o", "pipefail", "-c", sealed_checks], cwd=root,
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"),
            capture_output=True, text=True, timeout=60, check=False,
        )
        if (result.returncode == 0) != (mutation == "unchanged") or message not in result.stdout + result.stderr:
            raise RuntimeError("sealed control " + mutation + ": " + result.stdout + result.stderr)
        print("sealed control: " + mutation + (" passes" if mutation == "unchanged" else " refused: " + message))

with tempfile.TemporaryDirectory(prefix="pql-draft-controls-") as temporary:
    root = Path(temporary) / "kit"
    shutil.copytree(source, root)

    # The earlier draft step fills the uv cache used by --offline here.
    def run():
        return subprocess.run(
            ["uv", "run", "--offline", "--with", "jsonschema==4.23.0", "python", "-I", "runner.py", "--draft"], cwd=root,
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"),
            capture_output=True, text=True, timeout=60, check=False,
        )

    def refused():
        result = run()
        if result.returncode != 2 or "refused: draft folder contains KIT.sha256" not in result.stderr:
            raise RuntimeError("wrong draft refusal: " + str(result.returncode) + " " + result.stderr)

    seal = root / "KIT.sha256"
    seal.write_text("synthetic seal\n")
    refused()
    seal.unlink()
    seal.symlink_to("absent-seal")
    refused()
    seal.unlink()
    normal = run()
    if normal.returncode != 0:
        raise RuntimeError(normal.stderr)
    if not normal.stdout.startswith("UNSEALED\n"):
        raise RuntimeError("missing first-line banner")
    invalid = root / "examples/invalid/subject-file-QP10-01.json"
    repaired = json.loads(invalid.read_text())
    repaired["secret"]["reason"] = "Reading the seeded token proves the file was read."
    invalid.write_text(json.dumps(repaired) + "\n")
    accepted = run()
    if accepted.returncode != 1 or "accepted invalid example: subject-file-QP10-01.json" not in accepted.stderr:
        raise RuntimeError("draft accepted a valid negative fixture: " + str(accepted.returncode) + " " + accepted.stderr)
print("draft controls: repaired invalid example refused")
print("draft controls: regular seal refused; dangling seal refused; UNSEALED first")

# Case controls use the informative examples, not a second judge oracle.
with tempfile.TemporaryDirectory(prefix="pql-case-controls-") as temporary:
    root = Path(temporary) / "kit"
    shutil.copytree(source, root)
    (root / "KIT.sha256").unlink()
    cases = Path(temporary) / "cases"
    directory = cases / "case-001"
    directory.mkdir(parents=True)
    case = json.loads((root / "examples/case-01.json").read_text())
    record = json.loads((root / "examples/record-01.json").read_text())
    baseline = json.loads((root / "examples/baseline-01.json").read_text())
    (directory / "subject.json").write_bytes((root / "examples/subject-file.json").read_bytes())
    (directory / "input.json").write_text(json.dumps(record["judge_input"]))
    (directory / "preimages.json").write_text(json.dumps(baseline["preimages"]))

    def save_case():
        (directory / "case.json").write_text(json.dumps(case))

    def case_run(expected, message=""):
        result = subprocess.run(
            ["uv", "run", "--offline", "--with", "jsonschema==4.23.0", "python", "-I", str(root / "runner.py"), "--unsealed", "--schema-root", str(root), "--cases", str(cases), "--judge", str(source.parents[2] / "bin/pql") + " judge"],
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"), capture_output=True, text=True, timeout=60, check=False,
        )
        if result.returncode != expected or message not in result.stdout + result.stderr:
            raise RuntimeError("case control: " + result.stdout + result.stderr)
        print("case control: " + (message or "unchanged case passes"))

    save_case()
    case_run(0)
    subject_path = directory / case["input"]["subject_file"]
    valid_subject = subject_path.read_bytes()
    invalid_subject = json.loads(valid_subject)
    runbook = json.loads((root / "examples/subject-file-02.json").read_text())["runbooks"][0]
    invalid_subject["runbooks"] = [runbook]
    invalid_subject["runbooks"][0]["steps"][0]["checks"] = []
    subject_path.write_text(json.dumps(invalid_subject))
    case_run(2, "case-001: subject-file.schema.json at /runbooks/0/steps/0/checks")
    subject_path.write_text('{"runbooks": [], "runbooks": []}')
    case_run(2, "duplicate object key: runbooks")
    subject_path.write_bytes(valid_subject)
    case_run(0, "1 judge-checked: 1 passed")
    subject_path.write_text(json.dumps(invalid_subject))
    case["kind"] = "derivation-only"
    case["expect"] = {"exit_code": 2, "rule": "QS1"}
    save_case()
    input_bytes = (directory / "input.json").read_bytes()
    preimage_bytes = (directory / "preimages.json").read_bytes()
    (directory / "input.json").unlink()
    (directory / "preimages.json").unlink()
    case_run(0, "1 derivation-only unchecked")
    subject_path.write_bytes(valid_subject)
    (directory / "input.json").write_bytes(input_bytes)
    (directory / "preimages.json").write_bytes(preimage_bytes)
    case = json.loads((root / "examples/case-01.json").read_text())
    case["input"]["fixtures"] = [
        {"kind": "link", "path": "seed/missing-link", "detail": "missing-target"},
        {"kind": "special_file", "path": "seed/pipe", "detail": None},
    ]
    save_case()
    case_run(0)
    # Assembly creates these; the case directory must not contain them.
    link = directory / "missing-link"
    link.symlink_to("missing-target")
    case_run(2, "unexpected case file")
    link.unlink()
    pipe = directory / "pipe"
    os.mkfifo(pipe)
    case_run(2, "unexpected case file")
    pipe.unlink()
    case["input"]["fixtures"] = []
    save_case()
    case["expect"]["judge_output"]["checks"][0]["rule"] = "C2"
    save_case()
    case_run(1, "/checks/0/rule")
    case["expect"] = json.loads((root / "examples/case-01.json").read_text())["expect"]
    case["expect"]["exit_code"] = 1
    save_case()
    case_run(1, "/exit_code")
    case["expect"]["exit_code"] = 4
    save_case()
    (root / "KIT.sha256").write_text("synthetic seal\n")
    case_run(2, "refused: --unsealed beside KIT.sha256")
    (root / "KIT.sha256").unlink()
    (root / "KIT.sha256").symlink_to("absent-seal")
    case_run(2, "refused: --unsealed beside KIT.sha256")
    (root / "KIT.sha256").unlink()
    (directory / "stray").write_text("synthetic stray\n")
    case_run(2, "unexpected case file")

with tempfile.TemporaryDirectory(prefix="pql-id-controls-") as temporary:
    root = Path(temporary)
    directory = root / "cases/case-001"
    directory.mkdir(parents=True)
    form = root / "subjects/byte-bound/seed"
    form.mkdir(parents=True)
    (form / "data.json").write_bytes(b'{"widgets":[]}\n')
    (root / "behaviour").mkdir()
    case = json.loads((source / "examples/case-01.json").read_text())
    record = json.loads((source / "examples/record-01.json").read_text())["judge_input"]
    subject = json.loads((source / "examples/subject-file.json").read_text())
    (directory / case["input"]["subject_file"]).write_text(json.dumps(subject))
    spec = importlib.util.spec_from_file_location("collection_ids", scripts.joinpath("collection_ids.py"))
    ids = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(ids)
    digest = "46c4884c4195f646969ad14aca889bb614643f7c5e7e88bdec87d1032c0344d7"
    preimage = json.loads((source / "examples/baseline-01.json").read_text())["preimages"][digest]
    record["required"] = [c for c in record["required"] if c["id"] == digest]
    record["evidence"] = [e for e in record["evidence"] if e["check_id"] == digest]
    case["invocations"] = case["invocations"][:1]
    case["invocations"][0]["env"] = []
    behaviour = json.loads((source / "examples/behaviour-01.json").read_text())
    behaviour["invocations"][0]["key"] = case["invocations"][0]
    for stream in ("stdout", "stderr"):
        behaviour["invocations"][0]["confined"][0][stream].update(record["evidence"][0]["observed"][stream])
    (root / "behaviour/byte-bound.json").write_text(json.dumps(behaviour))

    def save_ids():
        (directory / "case.json").write_text(json.dumps(case))
        (directory / "input.json").write_text(json.dumps(record))
        (directory / "preimages.json").write_text(json.dumps({digest: preimage}))

    def cross_run(expected, message):
        result = subprocess.run(["python3", "-I", str(scripts.joinpath("check-case-ids.py")), str(root / "cases")], env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"), capture_output=True, text=True, timeout=20, check=False)
        if result.returncode != expected or message not in result.stdout + result.stderr:
            raise RuntimeError("cross-check control: " + result.stdout + result.stderr)
        print("cross-check control: " + message)

    save_ids()
    cross_run(0, "1 judge-checked")
    for kind, detail in (("link", "missing-target"), ("special_file", None)):
        case["input"]["fixtures"] = [{"kind": kind, "path": "seed/fixture", "detail": detail}]
        save_ids()
        cross_run(1, "seed contains a link or special file")
    case["input"]["fixtures"] = []
    save_ids()
    preimage["parameters"]["bound"] += 1
    save_ids()
    cross_run(1, "preimage id mismatch")
    preimage["parameters"]["bound"] -= 1
    save_ids()
    (form / "data.json").write_bytes(b"changed\n")
    cross_run(1, "seed digest mismatch")
    (form / "data.json").write_bytes(b'{"widgets":[]}\n')
    canonical_seed = preimage["seed"]
    canonical_digest = digest
    for seed in ({"placeholder": "Supply the seed so checks can read its data."}, None):
        subject["seed"] = seed
        (directory / case["input"]["subject_file"]).write_text(json.dumps(subject))
        cross_run(1, "seed digest mismatch")
        preimage["seed"] = seed
        digest = ids.check_id(preimage)
        record["required"][0]["id"] = digest
        record["evidence"][0]["check_id"] = digest
        for target in record["evidence"][0]["targets"]:
            if target["kind"] == "invocation":
                target["id"] = digest
        save_ids()
        cross_run(0, "1 judge-checked")
        preimage["seed"] = canonical_seed
        digest = canonical_digest
        record["required"][0]["id"] = digest
        record["evidence"][0]["check_id"] = digest
        for target in record["evidence"][0]["targets"]:
            if target["kind"] == "invocation":
                target["id"] = digest
        save_ids()
    subject["seed"] = "seed"
    (directory / case["input"]["subject_file"]).write_text(json.dumps(subject))
    case["invocations"] = []
    save_ids()
    cross_run(1, "invocation not in case")
    case["invocations"] = [behaviour["invocations"][0]["key"]]
    record["evidence"][0]["observed"]["stdout"]["sha256"] = "0" * 64
    save_ids()
    cross_run(1, "stream differs from recording")

    # No-command probes retain one invocation target but have no recorded run.
    preimage["words"] = None
    preimage["budget_seconds"] = None
    digest = ids.check_id(preimage)
    record["required"][0]["id"] = digest
    evidence = record["evidence"][0]
    evidence["check_id"] = digest
    evidence["observed"] = None
    evidence["targets"] = [{"kind": "invocation", "id": digest, "tier": "live"}]
    case["invocations"] = []
    save_ids()
    cross_run(0, "1 judge-checked")
    evidence["targets"][0]["id"] = digest + "#1"
    save_ids()
    cross_run(1, "no-invocation check requires one invocation target with the check id")
    evidence["targets"] = [{"kind": "invocation", "id": digest, "tier": "live"}] * 2
    save_ids()
    cross_run(1, "no-invocation check requires one invocation target with the check id")
    evidence["targets"] = evidence["targets"][:1]
    evidence["observed"] = {"nested": [{"stdout": {field: behaviour["invocations"][0]["confined"][0]["stdout"][field] for field in ("bytes", "sha256", "cut", "head")}}]}
    save_ids()
    cross_run(1, "no-invocation check has an observed stream")
    evidence["observed"] = None
    preimage["probe"] = "planted_secret"
    preimage["parameters"] = {}
    digest = ids.check_id(preimage)
    record["required"][0]["id"] = digest
    evidence["check_id"] = digest
    evidence["targets"][0]["id"] = digest
    save_ids()
    cross_run(0, "1 judge-checked")


    # A blocked discovery makes only the first help request, despite recorded help.
    preimage["probe"] = "discovery"
    digest = ids.check_id(preimage)
    record["required"][0]["id"] = digest
    evidence["check_id"] = digest
    case["input"]["host"] = {"confinement": {"applied": False}}
    help_keys = [dict(words=words + ["--help"], offline_args=[], env=[], runbook=None,
                      step=None, computation=None, home="empty")
                 for words in ([], ["one"], ["two"], ["three"], ["four"], ["five"])]
    case["invocations"] = help_keys
    template = behaviour["invocations"][0]
    behaviour["invocations"] = []
    for index, help_key in enumerate(help_keys):
        entry = json.loads(json.dumps(template))
        entry["key"] = help_key
        text = "COMMANDS:\n  one\n  two\n  three\n  four\n  five\n" if index == 0 else ""
        entry["confined"][0]["stdout"]["head"] = text
        entry["confined"][0]["stdout"]["bytes"] = len(text.encode("utf-8"))
        entry["confined"][0]["stdout"]["cut"] = False
        behaviour["invocations"].append(entry)
    (root / "behaviour/byte-bound.json").write_text(json.dumps(behaviour))
    evidence["targets"] = [{"kind": "invocation", "id": digest + "#" + str(index), "tier": "live"}
                           for index in range(1, 7)]
    save_ids()
    cross_run(1, "invalid invocation target numbering")
    evidence["targets"] = [{"kind": "invocation", "id": digest, "tier": "live"}]
    save_ids()
    cross_run(0, "1 judge-checked")

# Blind exports preserve inputs but must reveal injected oracle material.
with tempfile.TemporaryDirectory(prefix="pql-export-controls-") as temporary:
    root = Path(temporary)
    exporter = scripts.joinpath("collection-export.py")
    scratch_cases = root / "cases"
    directory = scratch_cases / "case-001"
    shutil.copytree(source / "cases/case-001", directory)

    def export_run(*arguments, expected=0, message=""):
        result = subprocess.run(
            ["python3", "-I", str(exporter), "--kit", str(source),
             "--cases", str(scratch_cases), *map(str, arguments)],
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"),
            capture_output=True, text=True, timeout=60, check=False,
        )
        if result.returncode != expected or message not in result.stdout + result.stderr:
            raise RuntimeError("export control: " + result.stdout + result.stderr)
        return result

    first = root / "first"
    second = root / "second"
    export_run("--out", first, "--map", root / "first-map.json")
    export_run("--out", second, "--map", root / "second-map.json")

    def snapshot(path):
        return {str(p.relative_to(path)): p.read_bytes() if p.is_file() else None
                for p in path.rglob("*")}

    if snapshot(first) != snapshot(second) or (root / "first-map.json").read_bytes() != (root / "second-map.json").read_bytes():
        raise RuntimeError("export control: repeated exports differ")
    print("export control: repeated exports are byte-identical")
    export_run("--check", first)
    export_run("--out", first, "--map", root / "unused-map.json", expected=2, message="--out must not exist")
    print("export control: existing output refused")
    export_run("--out", root / "inside", "--map", root / "inside/map.json", expected=2, message="--map must not lie inside --out")
    if (root / "inside").exists():
        raise RuntimeError("export control: refused map created output")
    print("export control: map inside output refused")

    subject = directory / "subject.json"
    original = subject.read_bytes()
    record = json.loads((directory / "input.json").read_text())
    check_id = record["required"][0]["id"]
    for label, token, finding in (("rule", "QS12345", "rule id"), ("check", check_id, "check id")):
        subject.write_bytes(original + token.encode() + b"\n")
        output = root / label
        export_run("--out", output, "--map", root / (label + "-map.json"))
        export_run("--check", output, expected=1, message=finding)
        subject.write_bytes(original)
        print("export control: injected " + finding + " found in copied subject")

    recording_path = next((first / "behaviour").glob("*.json"))
    recording = json.loads(recording_path.read_text())
    recording["export_control"] = "could_not_start"
    recording_path.write_text(json.dumps(recording))
    export_run("--check", first, expected=1, message="result word in JSON: could_not_start")
    print("export control: result word in recording string value found")
    del recording["export_control"]
    recording["form"] = json.loads((directory / "case.json").read_text())["input"]["form"]
    recording_path.write_text(json.dumps(recording))
    export_run("--check", first, expected=1, message="form name in JSON")
    print("export control: original form in recording found")

# Each mutation starts with a fresh copy; production recordings stay untouched.
with tempfile.TemporaryDirectory(prefix="pql-outcome-controls-") as temporary:
    for label in ("unchanged", "store", "failure-kind", "signal"):
        root = Path(temporary) / label
        shutil.copytree(source, root)
        expected = 0
        message = "0 outcome disagreements"
        if label == "store":
            path = root / "behaviour/writes-store.json"
            recording = json.loads(path.read_text())
            changed = 0
            for entry in recording["invocations"]:
                if entry["key"]["home"] == "seed" and entry["key"]["words"] and entry["key"]["words"][0] == "list":
                    for run in entry["confined"]:
                        changes = run["home_changes"]["changes"]
                        kept_changes = [change for change in changes if change["path"] != "data.json"]
                        changed += len(changes) - len(kept_changes)
                        run["home_changes"]["changes"] = kept_changes
            if not changed:
                raise RuntimeError("outcome control: store mutation changed nothing")
            path.write_text(json.dumps(recording))
            expected, message = 1, "case-012 check "
        elif label == "failure-kind":
            path = root / "cases/case-057/input.json"
            record = json.loads(path.read_text())
            evidence = next(e for e in record["evidence"] if e["failure_kind"] == "wrong_exit")
            evidence["failure_kind"] = "predicate"
            path.write_text(json.dumps(record))
            expected, message = 1, "case-057 check "
        elif label == "signal":
            path = root / "behaviour/crashes.json"
            recording = json.loads(path.read_text())
            changed = 0
            for entry in recording["invocations"]:
                end = entry["confined"][0]["end"]
                if end.get("signal") == "SIGABRT":
                    end.update(signal="SIGKILL", by="confinement")
                    changed += 1
            if not changed:
                raise RuntimeError("outcome control: signal mutation changed nothing")
            path.write_text(json.dumps(recording))
            expected, message = 1, "case-076 check "
        result = subprocess.run(
            ["python3", "-I", str(scripts.joinpath("check-case-outcomes.py")), str(root), str(root / "cases")],
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"),
            capture_output=True, text=True, timeout=60, check=False,
        )
        if result.returncode != expected or message not in result.stdout + result.stderr:
            raise RuntimeError("outcome control: " + label + ": " + result.stdout + result.stderr)
        print("outcome control: " + label + ": " + message)
