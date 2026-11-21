#!/usr/bin/env bash
# Go quality gate. Copy this file and staticcheck.conf into the Go project root
# (next to go.mod) and run:  ./gate.sh [--timeout SECONDS]
# Exit 0 = every step passed. Any other exit = the gate failed; the failing step
# is printed as "GATE FAILED at step: <name>".
#
# Network: none while running. Needed ONCE, to install the pinned tools:
#   GOBIN="$HOME/.gate-tools/go/bin" go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
#   GOBIN="$HOME/.gate-tools/go/bin" go install github.com/kisielk/errcheck@v1.30.0
#   GOBIN="$HOME/.gate-tools/go/bin" go install github.com/nishanths/exhaustive/cmd/exhaustive@v0.17.1
# and (if the project has dependencies) `go mod download` once. The gate itself
# runs with GOPROXY=off and GOFLAGS=-mod=readonly, so a missing dependency fails
# loudly instead of reaching for the network.
set -euo pipefail

PINNED_GO="go1.27.4"
PINNED_STATICCHECK="v0.8.2"
PINNED_ERRCHECK="v1.30.0"
PINNED_EXHAUSTIVE="v0.17.1"

TOOLS_BIN="${GATE_TOOLS_BIN:-$HOME/.gate-tools/go/bin}"
LIMIT="${GATE_TIMEOUT_SECS:-600}"
if [[ "${1:-}" == "--timeout" ]]; then LIMIT="${2:?--timeout needs seconds}"; shift 2; fi
[[ "$LIMIT" =~ ^[0-9]+$ ]] || { echo "GATE ERROR: timeout must be whole seconds, got '$LIMIT'" >&2; exit 2; }

# ---- whole-script time limit: re-run ourselves under timeout(1) ----
if [[ -z "${GATE_INNER:-}" ]]; then
  TO="$(command -v timeout || command -v gtimeout || true)"
  [[ -n "$TO" ]] || { echo "GATE ERROR: need timeout(1) or gtimeout(1) on PATH" >&2; exit 2; }
  rc=0
  GATE_INNER=1 GATE_TIMEOUT_SECS="$LIMIT" "$TO" -k 10 "$LIMIT" bash "$0" || rc=$?
  if [[ $rc -eq 124 || $rc -eq 137 ]]; then
    echo "GATE FAILED at step: timeout (whole-script limit ${LIMIT}s exceeded)" >&2
  fi
  exit "$rc"
fi
trap 'pkill -P $$ 2>/dev/null || true; exit 124' TERM

cd "$(dirname "$0")"
export GOPROXY=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local
export PATH="$TOOLS_BIN:$PATH"
[[ -f go.mod ]] || { echo "GATE FAILED at step: preflight (no go.mod next to gate.sh)" >&2; exit 1; }

STEP="preflight"
fail() { echo "GATE FAILED at step: $STEP${1:+ ($1)}" >&2; exit 1; }

step() { # step <name> <cmd...>
  STEP="$1"; shift
  echo "==> [$STEP] $*"
  "$@" || fail "exit $?"
}

# ---- preflight: pinned versions ----
[[ "$(go env GOVERSION)" == "$PINNED_GO" ]] || fail "go is $(go env GOVERSION), need $PINNED_GO"
check_tool() { # check_tool <binary> <module path> <version>
  local bin path got
  bin="$(command -v "$1" || true)"
  [[ -n "$bin" ]] || fail "$1 not found on PATH or in $TOOLS_BIN; install it (see header)"
  got="$(go version -m "$bin" 2>/dev/null | awk -v m="$2" '$1=="mod" && $2==m {print $3}')"
  [[ "$got" == "$3" ]] || fail "$1 is '${got:-unknown}', need $3"
  echo "    tool ok: $1 $3"
}
check_tool staticcheck honnef.co/go/tools "$PINNED_STATICCHECK"
check_tool errcheck github.com/kisielk/errcheck "$PINNED_ERRCHECK"
check_tool exhaustive github.com/nishanths/exhaustive "$PINNED_EXHAUSTIVE"

# ---- steps ----
step build go build ./...
step vet go vet ./...
# Checks come from staticcheck.conf (all checks except package-comment style).
step staticcheck staticcheck ./...
# Switches over enum-like constant types must list every constant. A default
# case does NOT count as covering missing constants (no -d flag).
step exhaustive exhaustive -check switch ./...
# Every returned error must be handled: no bare calls, no `_ =` discards,
# no unchecked type assertions.
step errcheck errcheck -blank -asserts ./...
step test go test -count=1 ./...

echo "GATE PASSED"
