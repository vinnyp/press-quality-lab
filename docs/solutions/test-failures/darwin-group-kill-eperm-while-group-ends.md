---
title: On macOS, killing a process group returns EPERM while the group is still ending
date: 2026-11-25
category: test-failures
module: internal/confine
problem_type: runtime_error
component: testing_framework
symptoms:
  - "Every confined run's Wait fails with `operation not permitted`, although the program ran and exited"
  - "A second Kill on the same process fails intermittently (30 of 50 runs on macOS 27.2)"
  - "`go test -race` flakes on a late-kill test with `operation not permitted`"
root_cause: wrong_api
resolution_type: code_fix
severity: high
framework_version: macOS 27.2 (26B123), Go 1.27.4
tags: [confinement, macos, process-group, kill, eperm, kqueue, qh23, flaky]
---

# On macOS, killing a process group returns EPERM while the group is still ending

## Problem

QH23 says that when an invocation ends, the engine ends its whole process group. The engine does this with `kill(-pgid, SIGKILL)` after kqueue reports the leader's exit and before it reaps the leader, so the group id cannot be reused. On macOS 27.2 that call returns `EPERM`, not `ESRCH`, in two states where the group is in fact ending:

1. **After the leader exits and before it is reaped,** when the group holds only the zombie leader.
2. **While a member is mid-exit:** it is no longer runnable, but its `P_stat` is not yet `SZOMB`.

## What went wrong, twice

- **The first build** treated every error other than `ESRCH` from the group kill as a failure. Every `Wait` then returned `operation not permitted`, and all five core `macos_evidence` tests failed, though each program had run correctly.
- **The first fix** accepted `EPERM` only when a `kern.proc.pgrp` lookup showed a zombie-only group. That covered state 1 but not state 2. A second `Kill` on `sleep 60` still failed in 30 of 50 runs, and `TestLateKillAttribution` flaked under `-race`.

## Solution

`killGroup` and the repeat loop in `endGroupWith`, in `internal/confine/process_darwin.go`, handle `EPERM` this way:

- On `EPERM`, look up the group's members with `unix.SysctlKinfoProcSlice("kern.proc.pgrp", pgid)`. If the group is empty or holds only zombies, it has ended and there is no error.
- If live members remain, do not fail. Treat `EPERM` as "not yet ended" and repeat the kill until the group is empty or zombie-only, within a bounded deadline. Only an `EPERM` that outlasts the deadline is an error, because it can also mean a member the engine may not signal.
- Keep the decision itself (`groupEnded` over a list of members) untagged and pure, so Linux tests can cover it with fakes.

The tests that hold this are `TestGroupEndEPERM` (the seam), `TestConsecutiveKill` and `TestConcurrentKillWait` (darwin, without `sandbox-exec`), and `TestGroupEnd`. Run them with `go test -race -count=20 ./internal/confine/`, because a single run can pass by luck.

## Two related darwin facts found in the same work

- **The exit detail is in the event's data.** `NOTE_EXIT_DETAIL_MASK` lies inside `NOTE_PDATAMASK`, so the detail (`NOTE_EXIT_MEMORY`, `NOTE_EXIT_CSERROR`) is in the kevent's `data` field, not its `fflags`. Read from `fflags`, it is always 0, and every memory or code-signing kill would look like the confinement's (QH21). `exitDetail` and `decodeEvent` in `internal/confine/process.go` hold this, with `TestExitDetail` and `TestDecodeEvent`.
- **Use `Setsid`, not `Setpgid`.** `Setpgid` leaves the program in the engine's session with its controlling terminal, which breaks QH13 and QH16's premise that `/dev/tty` reaches nothing. `Setsid` also makes the group id equal the pid, so the group kill needs no change.

## Prevention

When code signals a process group on darwin, treat `EPERM` as a state to examine, not an error, and test it with repeated kills under `-race`.
