---
title: In a macOS sandbox profile the later matching rule decides, so diagnose a kill by adding one allowance to the unchanged profile
date: 2026-11-23
category: best-practices
module: conformance collection spike
problem_type: best_practice
component: testing_framework
severity: medium
applies_when:
  - Finding which allowance a program killed under a sandbox-exec profile needs
  - Writing or reordering a profile that pairs a broad allowance with a narrower kill
tags: [confinement, sandbox, macos, sbpl, sandbox-exec, diagnosis, spike]
---

# In a macOS sandbox profile the later matching rule decides

## Context

A real printed Go CLI was killed at startup under the confinement spike's subject profile (`spike/confinement/`, record `docs/spikes/2026-11-28-confinement.md`). Finding the missing allowance took two attempts.

**The first attempt:** a "complement" method. It allowed everything known and then took away one subtree at a time, reading each kill as a need. It named needs that were false, such as the throwaway home and `/dev`, which the profile already grants.

**The second attempt:** adding one class of allowance at a time to the unchanged profile. It found the real needs:
- a CoreFoundation read of the text-encoding preference in the user's real home, now prevented by `__CF_USER_TEXT_ENCODING` (QH12 of the draft collection specification);
- reads of `/private/etc/services` and `/private/etc/protocols`.

**A wrong explanation.** At the time, the false needs were put down to "a deny placed before an allow still wins". That is wrong. The draft specification's QH11 briefly said the same thing and has been corrected. Why the complement method produced the false needs was not established.

## Guidance

**When two rules of an SBPL profile match one operation, the later rule decides.** This was measured on macOS 27.2, each time with `/bin/cat /private/etc/hosts` and a profile starting `(allow default)`:

| Rules, in order | Result |
|---|---|
| `(deny file-read-data (literal …hosts) (with send-signal SIGKILL))`, then `(allow file-read-data (literal …hosts))` | runs, exit 0 |
| the same two rules in the other order | killed, 137 |
| a kill-deny on `(subpath "/private/etc")`, then an allow of the literal file | runs |
| a kill-deny on the literal file, then an allow of `(subpath "/private/etc")` | runs |

Specificity does not matter: a later, broader allow overrides an earlier, narrower kill.

**Two consequences:**
- **Write each refusal after the allowance it narrows.** The template does this, for example `(deny file-write* (literal (param "PLANTED")) …)` after `(allow file-write* (subpath (param "HOME")))`. Reversed, the planted file would be writable, which is the forgery the spike's planted-file witnesses test for.
- **Never diagnose by appending broad allowances to a profile.** An appended allow silently overrides every earlier kill on the same operation, the profile's own refusals included, so the result says nothing about what the original profile refused. Add exactly one narrow allowance at a time to the unchanged profile. Count a need only when that one addition turns a kill into the program's known output, over several runs.

**A related trap from the same work.** An extra profile fragment must be applied after the harness's self-test, and to the scenarios only. Applied before the self-test, it made the self-test fail on every attempt, and every bisection then read 0 of 3.

## Applicability

This holds for any profile run by `sandbox-exec` on the macOS versions the spike supports. A later macOS could change how rules combine; rerun the four profiles in the table before relying on the order there.
