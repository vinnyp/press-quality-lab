---
title: A fresh executable for every run stalls a parallel macOS sandbox suite
date: 2026-11-23
category: test-failures
module: conformance collection spike
problem_type: test_failure
component: testing_framework
symptoms:
  - A sandbox test suite passes serially but turns flaky once its scenarios run in parallel
  - Failed runs time out before the trusted launcher writes its marker, with no harness kill
  - Each sandboxed launch takes several times longer after a change that copies the program per run
  - Even `ps` hangs on the machine while the suite runs
root_cause: missing_tooling
resolution_type: test_fix
severity: high
tags: [testing, confinement, sandbox, macos, syspolicyd, parallel, flaky, spike]
---

# A fresh executable for every run stalls a parallel macOS sandbox suite

## Problem

The confinement spike's suite, in `spike/confinement/` until it was removed after the conformance collection specification was sealed, ran about 1,900 scenarios 25 times each. Run in parallel, it went flaky and took five times longer per run than before. The cause was the harness writing a fresh copy of the launcher and the program for every run.

## Symptoms

- Under parallel load, runs failed with `timedOut` and no marker. `sandbox-exec` had not started the program within the 5-second budget.
- A group of tests passed in 16 seconds at parallelism 4 and collapsed at parallelism 8: every run failed, and the group took nearly 10 minutes.
- `ps` hung for minutes during a run.
- The slowdown began with the decision to run programs from a private copy (CR112), which the harness carried out by copying on every run.

## What Didn't Work

- **Blaming general load.** A concurrency test of a trivial `(allow default)` profile scaled cleanly to 64 parallel launches.
- **Blaming the code-signing check on freshly compiled binaries.** Forty concurrent first launches of distinct new binaries took 60 ms in a clean repeat, so first launch alone was not the cost.
- **Lowering parallelism.** It hid the problem in one run and failed differently in the next.

## Solution

Make each scenario's private copy once and reuse it across that scenario's runs. Check before every reuse that the copy is exactly as made:
- the directory entries;
- the inode, mode, size, modification time and change time;
- the SHA-256 of the bytes.

On any difference the run fails and names the change. The set is never silently remade, because a run that changed its own image is a finding. `prepareImages` and its cache were in `spike/confinement/confinement_darwin_test.go`, which is in the history at commit 490d650, the last commit on `main` that holds the spike.

After the fix the full suite ran in about 4 to 5 minutes on 18 cores and under 8 minutes with `GOMAXPROCS=3`, with every scenario 25 of 25.

## Why This Works

macOS's `syspolicyd` assesses each new executable file the first time it is launched. Measured serially over 200 launches:

| Launch | Cost |
|---|---|
| A reused binary | 2.2 ms |
| A fresh copy | 22.3 ms |
| A hard link to the same file | 2.8 ms |

Copying per run created over 100,000 new executables. `syspolicyd` fell behind:
- it used about 4.4 CPU-hours and wrote 34 GB in roughly 3.5 hours;
- it crashed six times in an hour;
- launches waited on it past their budget.

Reusing a verified copy keeps one assessment per scenario. CR112 needs a byte-identical copy in a private directory, not a new copy per run.

## Prevention

- In any harness that launches programs many times on macOS, never create a new executable file per launch. Create it once per unit of work, such as a scenario or a grade, and verify it is unchanged before each reuse.
- When a sandboxed launch slows down or stalls under load, look at the system's diagnostic reports for `syspolicyd` (disk writes, CPU, crashes) before tuning budgets or parallelism.
- The engine copies the subject's program for CR112. Make that copy once per grade, as the spike record says (`docs/spikes/2026-11-28-confinement.md`).

## Related Issues

- `docs/spikes/2026-11-28-confinement.md`, "Findings for the specification".
- `docs/solutions/best-practices/clean-control-must-assert-on-output.md`: the spike's other testing lesson.
