---
title: A sandbox evidence test must prove the sandbox decided on the operation it names
date: 2026-11-27
category: best-practices
module: internal/confine
problem_type: best_practice
component: testing_framework
severity: high
applies_when:
  - Writing or changing a macOS confinement evidence test (the `macos_evidence` or `necessity_test` tags)
  - Adding a self-test witness, a necessity grant row or an allowance-removal row
  - Building a knock-out binary with `go build -overlay` to show a guard is load-bearing
tags: [testing, confinement, sandbox, false-green, knock-out, macos, evidence]
---

# A sandbox evidence test must prove the sandbox decided on the operation it names

## Context

PR11 built phase 3's macOS self-test track: the self-test programs (QH9), the computation profile (QH16 to QH20), and the necessity and allowance evidence (U16). These tests run only outside the tool sandbox, so the gate can only compile them. Six of them passed or failed for the wrong reason, and each was found only by running it on macOS 27.2:

1. **A knock-out that never applied.** It was built with `go build -overlay` and named its source under `/tmp`, while the build saw `/private/tmp`. The overlay did not apply, and the "knock-out" was the unmodified launcher, so it passed for the wrong reason.
2. **A network probe that only called `socket()`.** The template does not refuse socket creation, so the call returned and the row failed. The refused operations QH9 names are a TCP and a Unix connect, a UDP send, a bind and a listen.
3. **A `setsockopt` before the attempt line.** The template kills `setsockopt`, so four network rows died before their operation and never reached it.
4. **A copied platform binary.** A witness ran a private copy of `/usr/bin/sqlite3`. macOS refuses to run a copied platform binary, ending it with the code-signing exit detail (`0x40000`), so the unmodified control was killed and the row was never evaluated.
5. **A numeric sysctl id.** A page-size witness read `HW_PAGESIZE` by its numeric MIB, which resolves to `hw.pagesize_compat`, an entry the startup line already grants. Removing the allowance under test changed nothing.
6. **An assertion of `err != nil`.** A decoy-integrity test accepted any error. A stale socket from a first decoy made the second fail to bind, so the self-test failed with `EOF` before the decoy check ran. The test passed for the wrong reason until it asserted the exact error text.

## Guidance

`CODING_STANDARDS.md` holds the rule under Confinement harness. In short:

- **Attempt the named operation, and only it.** The witness prints its attempt line, then performs the exact operation the clause names. Anything the profile could refuse comes after, or not at all. A setup call that must come first, such as reading a priority before changing it, is one the profile allows.
- **Pair it with a control.** The same call, run unconfined or under the unmodified profile, must return. Otherwise a refusal shows nothing: the call may simply not work.
- **Prove a knock-out applied.** Resolve every path you overlay or compare with `EvalSymlinks`, since `/tmp` and `/var` are links on macOS. Then assert the knock-out binary's bytes differ from the real one.
- **Assert the exact result.** Compare the printed output with the value the call should give, never merely "it failed" or "it was killed". A wrong error, or an early kill, then shows.
- **When unsure what an allowance governs, measure it.** A probe that runs candidate calls with and without the allowance finds the governed call. For `system-info` on macOS 27.2 it was `statfs` and `fstatfs`, not `gethostuuid`, `getattrlist` or `confstr`.

## Measured facts (macOS 27.2, build 26B123)

- A private copy of a platform binary such as `/usr/bin/sqlite3` is refused to run, with exit detail `0x40000`. Run a system tool in place, which the computation profile allows under `/bin` and `/usr/bin`.
- The numeric MIB `{CTL_HW, HW_PAGESIZE}` resolves to `hw.pagesize_compat`. Read by name, `hw.pagesize` and `hw.memsize` are both 8-byte integers, so a check on width cannot tell them apart. Check the value against `os.Getpagesize()`.
- Under both profiles, reading another process's scheduling priority is allowed, and changing it is killed (E-40).

## Why This Matters

These tests are the only evidence that the confinement holds. A witness that dies before its operation, or a knock-out that never applied, reads as proof of a refusal the sandbox never made. That is the false green the engine exists to rule out, inside the instrument itself.

## When to Apply

- Any new or changed witness, grant row or allowance row in `internal/confine`.
- Any knock-out built from an overlay.
- Re-measuring the profile on a new macOS release.

## Related

- `docs/solutions/best-practices/clean-control-must-assert-on-output.md`: the clean-program half of the same rule.
- `docs/solutions/best-practices/sandbox-profile-later-rule-decides.md`: why a grant is appended last.
- `docs/agent-reviews/2026-11-27-phase3-pr11-self-test-track-build-reviews.md`: the table of these fixes.
