"""Witness the unsealed runner's refusal and banner in an isolated copy."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

guard_script = Path(__file__).resolve().parent / "draft-guard.sh"
source = Path(__file__).resolve().parents[1] / "docs/specs/conformance-collection-1.0"
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
        guard = subprocess.run(["bash", str(guard_script), str(root)], cwd=root, timeout=10, check=False)
        if guard.returncode == 0:
            raise RuntimeError("check.sh guard accepted seal")

    seal = root / "KIT.sha256"
    seal.write_text("synthetic seal\n")
    refused()
    seal.unlink()
    seal.symlink_to("absent-seal")
    refused()
    seal.unlink()
    guard = subprocess.run(["bash", str(guard_script), str(root)], timeout=10, check=False)
    if guard.returncode != 0:
        raise RuntimeError("draft guard refused unsealed tree")
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
        result = subprocess.run(["python3", "-I", str(guard_script.with_name("check-case-ids.py")), str(root / "cases")], env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"), capture_output=True, text=True, timeout=20, check=False)
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
    case["invocations"] = []
    save_ids()
    cross_run(1, "invocation not in case")
    case["invocations"] = [behaviour["invocations"][0]["key"]]
    record["evidence"][0]["observed"]["stdout"]["sha256"] = "0" * 64
    save_ids()
    cross_run(1, "stream differs from recording")
