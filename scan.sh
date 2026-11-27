#!/usr/bin/env bash
set -euo pipefail
PINNED_GITLEAKS=8.30.1
fail() { echo "SCAN FAILED: $1" >&2; exit 1; }
trap 'fail "command failed (exit $?)"' ERR
[[ $# -eq 0 ]] || fail 'scan.sh takes no arguments; set SCAN_COMMITS=base..head to scan a commit range'
for tool in git gitleaks python3 mktemp tar; do
 command -v "$tool" >/dev/null || fail "$tool not found on PATH"
done
git rev-parse --is-inside-work-tree >/dev/null || fail 'not inside a git work tree'
cd "$(git rev-parse --show-toplevel)"
[[ "$(gitleaks version)" == "$PINNED_GITLEAKS" ]] || fail "gitleaks must be $PINNED_GITLEAKS"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/pql-scan.XXXXXX")"
[[ -n "$tmp" ]] || fail 'empty temporary directory'
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/files"
git ls-files -z > "$tmp/tracked"
[[ -s "$tmp/tracked" ]] || fail 'git ls-files listed no files'
if [[ -n "${SCAN_COMMITS:-}" ]]; then
 [[ "$SCAN_COMMITS" == *..* && "$SCAN_COMMITS" != *...* ]] || fail 'SCAN_COMMITS must be base..head'
 git rev-list "$SCAN_COMMITS" -- > "$tmp/commits" || fail 'invalid commit range'
 [[ -s "$tmp/commits" ]] || fail 'SCAN_COMMITS matched no commits'
 git log --format=%B "$SCAN_COMMITS" -- > "$tmp/messages" || fail 'invalid commit range'
fi
python3 - "$tmp" <<'PY' || fail 'home path or email address found (see lines above)'
import pathlib, re, sys
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
def existing_paths(paths):
    return [path for path in paths if path.exists()]

def problems(paths):
    found = []
    for path in paths:
        label = 'commit-messages' if path == scratch / 'messages' else str(path)
        for number, line in enumerate(path.read_bytes().decode('utf-8', errors='replace').splitlines(), 1):
            if home.search(line):
                found.append(f'{label}:{number}: home path')
            if any(not allowed.fullmatch(m.group()) for m in email.finditer(line)):
                found.append(f'{label}:{number}: email')
    return found

# Public-by-default controls: skip a deleted index entry without hiding a
# home path in an existing file. Both controls use the production functions.
control = scratch / 'existing'
control.write_text('/ho' + 'me/synthetic/file')
missing = scratch / 'deleted'
if existing_paths([missing, control]) != [control]:
    sys.exit('self-test failed: deleted tracked path filtering')
if problems(existing_paths([missing, control])) != [f'{control}:1: home path']:
    sys.exit('self-test failed: existing tracked home path')

paths = existing_paths([pathlib.Path(p.decode()) for p in (scratch / 'tracked').read_bytes().split(b'\0') if p])
# Python, tar and gitleaks all consume this same filtered list.
(scratch / 'tracked').write_bytes(b''.join(bytes(path) + b'\0' for path in paths))
found = problems(paths + ([scratch / 'messages'] if (scratch / 'messages').exists() else []))
for problem in found:
    print(problem)
sys.exit(bool(found))
PY
tar --null -T "$tmp/tracked" -cf "$tmp/tracked.tar"
tar -xf "$tmp/tracked.tar" -C "$tmp/files"
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
