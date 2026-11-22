---
title: A clean control must assert on its output, not its exit status
date: 2026-11-22
category: best-practices
module: conformance collection spike
problem_type: best_practice
component: testing_framework
severity: high
applies_when:
  - A sandbox or verifier is tested by running a clean program and checking that it survives
  - A probe concludes that a restriction is harmless because the restricted program exited 0
  - A confinement profile is called complete because every violation on a list was stopped
tags: [testing, confinement, positive-control, false-green, sandbox, spike]
---

# A clean control must assert on its output, not its exit status

## Context

The phase 2 feasibility probes tested a macOS sandbox profile in two directions: programs that break a rule must be killed, and clean programs must be left alone. The [phase 2 plan](../../plans/2026-11-21-2309-feat-conformance-collection-spec-plan.md) first recorded the result as every violation killed and every clean run finished, including under a profile that refused lookups of system daemons.

The second half was false. Under that profile the current-user lookup failed. The test program ignored the error and exited 0, and the harness read only the exit status, so a broken run counted as a clean one.

The first half was incomplete. The violations tested were the ones the plan already named: network, child process, file read and file write. Under the same profile a program could read another process's environment, where credentials often live, and could kill another process. The design review raised the process gaps, and reproducing its findings exposed the false clean result.

## Guidance

- **A clean control prints what it computed, and the harness compares that with a known value.** A run counts as clean only when the exit status and the output are both right.
- **A control program does not discard errors.** Write it so that a refused operation changes what it prints.
- **List violations from what the platform lets a process do, not from the rules already written.** For a process that means at least network, processes (start, inspect, signal), files (contents and metadata), environment and system services.
- **Try each candidate fix against the attack and against the clean controls.** In this session's re-test, several plausible process rules changed nothing and the environment stayed readable. One broader rule stopped the attack by killing every clean program too. Only a kill on process inspection with self allowed did both jobs.
- **Run the controls under the exact profile that will be used.** Rules that work separately can fail together. A deny on file writes killed every program until device paths were allowed.

The plan carries this as requirements: CR74 for a self-test in every grade, and CR97 for judging the spike's clean programs by output. When the verifier is built in phase 3, that self-test is the check that should hold this lesson.

## Why This Matters

The engine exists so that green never means "did not look". A control that checks only survival is that same false green, inside the instrument that all offline evidence rests on. If a restriction silently breaks clean programs, their failures are later graded as the subject's defects.

## When to Apply

- Running the no-network spike on macOS, Linux or a CI runner image.
- Writing the verifier's per-grade self-test.
- Widening or narrowing the allow-list of paths or system services.

## Examples

Before, the control could not fail:

```go
user.Current() // error discarded
fmt.Println("finished")
```

```python
ok = result.returncode == 0
```

After, a refused lookup shows:

```go
if u, err := user.Current(); err == nil && u.Username != "" {
	fmt.Println("user lookup ok")
} else {
	fmt.Println("user lookup failed", err)
}
```

```python
ok = result.returncode == 0 and "user lookup ok" in result.stdout
```

With output assertions and the process and service rules added, the re-run on macOS 27.2 killed 350 of 350 violations across 14 kinds and returned correct output on 125 of 125 clean runs.

## Related

- [A test's expected value must not come from the code under test](test-oracle-must-not-come-from-code-under-test.md): the other way a test can agree with a bug.
- The plan's probe table, under Sources / Research, records both the failed and the corrected results.
