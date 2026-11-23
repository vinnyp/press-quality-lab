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

    def run():
        return subprocess.run(
            ["uv", "run", "--offline", "runner.py", "--draft"], cwd=root,
            env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"),
            capture_output=True, text=True, timeout=60, check=False,
        )

    seal = root / "KIT.sha256"
    seal.write_text("synthetic seal\n")
    if run().returncode != 2:
        raise RuntimeError("regular seal was accepted")
    seal.unlink()
    seal.symlink_to("absent-seal")
    if run().returncode != 2:
        raise RuntimeError("dangling seal was accepted")
    seal.unlink()
    normal = run()
    if normal.returncode != 0:
        raise RuntimeError(normal.stderr)
    if not normal.stdout.startswith("UNSEALED\n"):
        raise RuntimeError("missing first-line banner")
print("draft controls: regular seal refused; dangling seal refused; UNSEALED first")
