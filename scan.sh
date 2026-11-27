#!/usr/bin/env bash
set -euo pipefail
PINNED_GITLEAKS=8.30.1
fail() { echo "SCAN FAILED: $1" >&2; exit 1; }
trap 'fail "command failed (exit $?)"' ERR
[[ $# -eq 0 ]] || fail 'scan.sh takes no arguments; set SCAN_COMMITS=base..head to scan a commit range'
for tool in git gitleaks python3 mktemp; do
 command -v "$tool" >/dev/null || fail "$tool not found on PATH"
done
git rev-parse --is-inside-work-tree >/dev/null || fail 'not inside a git work tree'
cd "$(git rev-parse --show-toplevel)"
[[ "$(gitleaks version)" == "$PINNED_GITLEAKS" ]] || fail "gitleaks must be $PINNED_GITLEAKS"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pql-scan.XXXXXX")"
[[ -n "$tmp" ]] || fail 'empty temporary directory'
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/files" "$tmp/index"
git checkout-index -a --prefix="$tmp/index/"
git ls-files -z > "$tmp/tracked"
[[ -s "$tmp/tracked" ]] || fail 'git ls-files listed no files'
if [[ -n "${SCAN_COMMITS:-}" ]]; then
 [[ "$SCAN_COMMITS" == *..* && "$SCAN_COMMITS" != *...* ]] || fail 'SCAN_COMMITS must be base..head'
 git rev-list "$SCAN_COMMITS" -- > "$tmp/commits" || fail 'invalid commit range'
 [[ -s "$tmp/commits" ]] || fail 'SCAN_COMMITS matched no commits'
 git log --format=%B "$SCAN_COMMITS" -- > "$tmp/messages" || fail 'invalid commit range'
fi
python3 - "$tmp" <<'PY' || fail 'home/email scan or its self-test failed (see above)'
import os, pathlib, re, sys
scratch = pathlib.Path(sys.argv[1])
home = re.compile(r'/Use[r]s/[^/\s]+|/ho[m]e/[^/\s]+|/roo[t](?:/|\b)|(?i:[A-Za-z]:(?:\\+|/)Use[r]s(?:\\+|/)[^\\/\s"]+)')
email = re.compile(r'[A-Za-z0-9.!#$%&\x27*+/=?^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?\.[A-Za-z]{2,}')
allowed = re.compile(r'(?:noreply@anthropic[.]com|[^@]+@users[.]noreply[.]github[.]com)\Z')
# Split fixtures so the scanner does not flag its own synthetic examples.
for case, expected in [
    ('/Use' + 'rs/alice/x', True), ('/ho' + 'me/alice', True),
    ('/roo' + 't/.ssh', True), ('C:' + '\\' + 'Users' + '\\' + 'alice', True),
    ('C:' + '\\\\' + 'Users' + '\\\\' + 'alice', True),
    ('c:/' + 'users/alice', True),
    ('https://api.example.com/users/123', False), ('/homepage', False), ('/rooted', False),
]:
    if bool(home.search(case)) != expected:
        sys.exit(f'self-test failed: {case}')
for case, expected in [
    ('noreply' + '@' + 'anthropic.com', False), ('x' + '@' + 'users.noreply.github.com', False),
    ('a' + '@' + 'b.io', True), ('noreply' + '@' + 'anthropic.com.evil.io', True),
]:
    if any(not allowed.fullmatch(m.group()) for m in email.finditer(case)) != expected:
        sys.exit(f'self-test failed: {case}')
def scan_paths(index, working, names):
    paths = [index / name for name in names]
    paths.extend(working / name for name in names
                 if (working / name).exists() or (working / name).is_symlink())
    return paths

def blob(path):
    return os.fsencode(os.readlink(path)) if path.is_symlink() else path.read_bytes()

def problems(paths):
    found = []
    for path in paths:
        label = 'commit-messages' if path == scratch / 'messages' else str(path)
        for number, line in enumerate(blob(path).decode('utf-8', errors='replace').splitlines(), 1):
            if home.search(line):
                found.append(f'{label}:{number}: home path')
            if any(not allowed.fullmatch(m.group()) for m in email.finditer(line)):
                found.append(f'{label}:{number}: email')
    return found

# Public-by-default: each control compares the whole problem list, and starts
# with clean index content that succeeds before changing only the named input.
control = scratch / 'controls'
for name in ('MissingHome', 'MissingEmail', 'StagedHome', 'WorkingHome',
             'MissingClean', 'SymlinkHome'):
    root = control / name
    index, working = root / 'index', root / 'working'
    index.mkdir(parents=True)
    working.mkdir()
    staged, disk = index / 'file', working / 'file'
    staged.write_text('clean')
    disk.write_text('clean')
    def check(expected):
        actual = problems(scan_paths(index, working, ['file']))
        if actual != expected:
            sys.exit(f'self-test failed: {name}: expected {expected}, got {actual}')
    check([])
    home_text = '/ho' + 'me/synthetic/file'
    if name in ('MissingHome', 'MissingEmail', 'StagedHome'):
        staged.write_text(('a' + '@' + 'example.invalid') if name == 'MissingEmail' else home_text)
    if name in ('MissingHome', 'MissingEmail', 'MissingClean'):
        disk.unlink()
    if name == 'WorkingHome':
        disk.write_text(home_text)
    if name == 'SymlinkHome':
        staged.unlink()
        staged.symlink_to(home_text)
        disk.unlink()
        disk.symlink_to(home_text)
    if name == 'MissingClean':
        check([])
    elif name == 'WorkingHome':
        check([f'{disk}:1: home path'])
    elif name == 'SymlinkHome':
        check([f'{staged}:1: home path', f'{disk}:1: home path'])
    else:
        kind = 'email' if name == 'MissingEmail' else 'home path'
        check([f'{staged}:1: {kind}'])

names = [os.fsdecode(p) for p in (scratch / 'tracked').read_bytes().split(b'\0') if p]
paths = scan_paths(scratch / 'index', pathlib.Path.cwd(), names)
# Copy blob bytes, including link text, for gitleaks; never follow a symlink.
for path in paths:
    source = scratch / 'index' if path.is_relative_to(scratch / 'index') else pathlib.Path.cwd()
    tree = 'index' if source == scratch / 'index' else 'working'
    target = scratch / 'files' / tree / path.relative_to(source)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(blob(path))
found = problems(paths + ([scratch / 'messages'] if (scratch / 'messages').exists() else []))
for problem in found:
    print(problem)
sys.exit(bool(found))
PY
leaks() {
 local rc=0
 rm -f "$tmp/leaks.json"
 gitleaks "$@" --report-format json --report-path "$tmp/leaks.json" || rc=$?
 if [[ "$rc" != 0 ]]; then
  if [[ -f "$tmp/leaks.json" ]]; then
   python3 - "$tmp/leaks.json" "$tmp/files/" <<'PYREPORT'
import json, os, sys
for hit in json.load(open(sys.argv[1])):
    path = hit['File']
    normalized = os.path.abspath(path)
    base = os.path.abspath(sys.argv[2])
    if os.path.commonpath([normalized, base]) == base:
        path = os.path.relpath(normalized, base)
    print(f"{path}:{hit['StartLine']}: secret")
PYREPORT
  fi
  fail "gitleaks scan failed (exit $rc)"
 fi
}
leaks dir --no-banner --redact --exit-code 1 "$tmp/files"
if [[ -n "${SCAN_COMMITS:-}" ]]; then
 leaks git --no-banner --redact --exit-code 1 --log-opts="$SCAN_COMMITS" .
fi
echo 'SCAN PASSED'
