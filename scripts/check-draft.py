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
            ["uv", "run", "--offline", "runner.py", "--draft"], cwd=root,
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
