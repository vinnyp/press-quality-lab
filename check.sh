#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
ROOT="$PWD"
STEP=preflight
# Never call fail() inside a subshell.
fail() { trap - EXIT; echo "CHECK FAILED at step: $STEP ($1)" >&2; exit 1; }
trap 'status=$?; if (( status != 0 )); then echo "CHECK FAILED at step: $STEP (exit $status)" >&2; fi' EXIT
step() { STEP="$1"; echo "==> [$STEP] $2"; }
need() { command -v "$1" >/dev/null || fail "$1 not found on PATH"; }
step gate './gate.sh'
./gate.sh
step build 'build pql'
need go
GOPROXY=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local go build -o bin/pql ./cmd/pql
step kit 'verify sealed kit'
need shasum
need python3
(
 cd docs/specs/judge-1.0
 shasum -a 256 -c --quiet KIT.sha256
 PYTHONDONTWRITEBYTECODE=1 python3 - <<'PY'
from pathlib import Path
import sys
expected = {line.split(None, 1)[1].lstrip('*') for line in Path('KIT.sha256').read_text().splitlines()}
actual = {str(p) for p in Path('.').rglob('*') if (p.is_file() or p.is_symlink()) and str(p) != 'KIT.sha256'}
for name in sorted(actual - expected):
    print(f'unlisted file: {name}', file=sys.stderr)
for name in sorted(expected - actual):
    print(f'missing file: {name}', file=sys.stderr)
sys.exit(bool(actual ^ expected))
PY
)
step cases 'run published cases'
need uv
(
 cd docs/specs/judge-1.0
JUDGE="${ROOT//\'/\'\\\'\'}"
PYTHONDONTWRITEBYTECODE=1 uv run runner.py --judge "'$JUDGE/bin/pql' judge" --cases cases
)
step draft "verify unsealed collection kit"
(
 cd docs/specs/conformance-collection-1.0
 "$ROOT/scripts/draft-guard.sh" .
 PYTHONDONTWRITEBYTECODE=1 uv run runner.py --draft
)
step draft-controls 'verify draft seal refusal and banner'
PYTHONDONTWRITEBYTECODE=1 python3 -I scripts/check-draft.py
echo 'CHECK PASSED'
