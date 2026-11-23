"""Witness the unsealed runner's refusal and banner in an isolated copy."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

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
        guard = subprocess.run(["bash", "-c", "[[ ! -e KIT.sha256 && ! -L KIT.sha256 ]]"], cwd=root, timeout=10, check=False)
        if guard.returncode == 0:
            raise RuntimeError("check.sh guard accepted seal")

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
print("draft controls: regular seal refused; dangling seal refused; UNSEALED first")
