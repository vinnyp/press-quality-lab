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
step rule-growth 'check rule and pending-case ratchets'
need python3
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/check-rule-growth.py
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
 PYTHONDONTWRITEBYTECODE=1 python3 -I -B - <<'PY'
from pathlib import Path
import sys
expected = {line.split(None, 1)[1].lstrip('*') for line in Path('KIT.sha256').read_text().splitlines()}
import stat
actual = set()
bad = []
for p in Path('.').rglob('*'):
    name = str(p)
    mode = p.lstat().st_mode
    if stat.S_ISDIR(mode):
        if not any(p.iterdir()):
            actual.add(name)
    else:
        if name != 'KIT.sha256':
            actual.add(name)
        if not stat.S_ISREG(mode):
            bad.append(name)
for name in sorted(bad):
    print(f'not regular: {name}', file=sys.stderr)
for name in sorted(actual - expected):
    print(f'unlisted file: {name}', file=sys.stderr)
for name in sorted(expected - actual):
    print(f'missing file: {name}', file=sys.stderr)
sys.exit(bool(actual ^ expected or bad))
PY
 shasum -a 256 -c --quiet KIT.sha256
)
step judge-cases 'run published judge cases'
need uv
(
 cd docs/specs/judge-1.0
JUDGE="${ROOT//\'/\'\\\'\'}"
PYTHONDONTWRITEBYTECODE=1 uv run runner.py --judge "'$JUDGE/bin/pql' judge" --cases cases
)
step collection-kit 'verify sealed collection kit'
(
 cd docs/specs/conformance-collection-1.0
 PYTHONDONTWRITEBYTECODE=1 python3 -I -B - <<'PY'
from pathlib import Path
import sys
expected = {line.split(None, 1)[1].lstrip('*') for line in Path('KIT.sha256').read_text().splitlines()}
import stat
actual = set()
bad = []
for p in Path('.').rglob('*'):
    name = str(p)
    mode = p.lstat().st_mode
    if stat.S_ISDIR(mode):
        if not any(p.iterdir()):
            actual.add(name)
    else:
        if name != 'KIT.sha256':
            actual.add(name)
        if not stat.S_ISREG(mode):
            bad.append(name)
for name in sorted(bad):
    print(f'not regular: {name}', file=sys.stderr)
for name in sorted(actual - expected):
    print(f'unlisted file: {name}', file=sys.stderr)
for name in sorted(expected - actual):
    print(f'missing file: {name}', file=sys.stderr)
sys.exit(bool(actual ^ expected or bad))
PY
 shasum -a 256 -c --quiet KIT.sha256
 PYTHONDONTWRITEBYTECODE=1 python3 -I -B - <<'PYCODE'
from pathlib import Path
import sys
if 'Corpus: complete' not in Path('CASES.md').read_text().splitlines():
    sys.exit('collection corpus is not complete: missing Corpus: complete')
PYCODE
)
step collection-schemas 'validate sealed collection schemas and examples'
(
 cd docs/specs/conformance-collection-1.0
 PYTHONDONTWRITEBYTECODE=1 uv run --with jsonschema==4.23.0 python -I -B - <<'PYCODE'
from pathlib import Path
import sys
sys.dont_write_bytecode = True
sys.path.insert(0, str(Path.cwd()))
from common import draft
sys.exit(draft(Path.cwd()))
PYCODE
)
step collection-controls 'verify sealed collection and scratch runner controls'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/check-draft.py
step jcs-vectors 'verify generated JCS vectors'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/test_generate_jcs_vectors.py
step ids-tests 'test canonical JSON and collection check ids'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/test_collection_ids.py
step ratchet-prediction 'test the ratchet-set prediction script'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/test_predict_ratchet.py
step behaviour 'validate behaviour schemas and agreement'
PYTHONDONTWRITEBYTECODE=1 uv run --with jsonschema==4.23.0 python -I -B scripts/check-behaviour.py docs/specs/conformance-collection-1.0
step ids-cross-check 'cross-check collection case ids and recordings'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/check-case-ids.py docs/specs/conformance-collection-1.0/cases
step outcomes 'cross-check collection case outcomes against recordings'
PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/check-case-outcomes.py docs/specs/conformance-collection-1.0 docs/specs/conformance-collection-1.0/cases
step cases 'run collection judge-checked cases'
if [[ -d docs/specs/conformance-collection-1.0/cases ]]; then
 (
  cd docs/specs/conformance-collection-1.0
  JUDGE="${ROOT//\'/\'\\\'\'}"
  PYTHONDONTWRITEBYTECODE=1 uv run --with jsonschema==4.23.0 python -I -B runner.py --cases cases --judge "'$JUDGE/bin/pql' judge"
 )
else
 echo 'no cases yet'
fi
step corpus-schema-controls 'exercise schema validation with sealed examples'
PYTHONDONTWRITEBYTECODE=1 uv run --with jsonschema==4.23.0 python -I -B scripts/test_corpus_schema.py
step corpus-records 'run scripted corpus and validate stored copies'
(
 corpus_copies=$(mktemp -d "${TMPDIR:-/tmp}/pql-corpus-records.XXXXXX")
 trap 'rm -rf "$corpus_copies"' EXIT
 PQL_CORPUS_COPIES="$corpus_copies" GOPROXY=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local go test -count=1 -tags scripted ./internal/corpusrunner
 PYTHONDONTWRITEBYTECODE=1 uv run --with jsonschema==4.23.0 python -I -B scripts/validate-corpus-records.py --kit docs/specs/conformance-collection-1.0 --pending internal/corpusrunner/pending.txt --copies "$corpus_copies"
)
step export 'export blind collection inputs and check for leaks'
(
 export_tmp=$(mktemp -d "${TMPDIR:-/tmp}/pql-export.XXXXXX")
 trap 'rm -rf "$export_tmp"' EXIT
 kit=docs/specs/conformance-collection-1.0
 PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/collection-export.py --kit "$kit" --cases "$kit/cases" --out "$export_tmp/out" --map "$export_tmp/map.json"
 PYTHONDONTWRITEBYTECODE=1 python3 -I -B scripts/collection-export.py --check "$export_tmp/out" --cases "$kit/cases" --kit "$kit"
)
echo 'CHECK PASSED'
