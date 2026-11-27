# Peer-review gate — phase 3 PR11, the macOS self-test track (2026-11-27)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer.

The plan had its different-model review; AGENTS.md asks for a different model on specifications and plan phases, not on builds.

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Security:** this is the confinement's own evidence: the self-test, the known-fast control and the calibration.
- **Testing:** every proof is a macOS test the gate can only compile, so a test that passes for the wrong reason ships unseen.
- **Code:** draining, the environment and the raw ends feed every invocation.

## Evidence found and fixed while building

Several tests passed for the wrong reason, or failed for one, before any review. The caller found each on macOS:

| Unit | What was wrong | Fix |
|---|---|---|
| U28 | The descriptor knock-out's overlay named the launcher under `/tmp`, while the build used `/private/tmp`, so the knock-out built the unmodified launcher | The root is resolved, and the test asserts the knock-out binary differs |
| U31 | The network probe only created a socket, which the template does not refuse | Real TCP, Unix, UDP, bind and listen attempts, each beside an unconfined control |
| U31 | A `setsockopt` before the attempt line was itself killed, so four rows died before their operation | The call was removed |
| U29 | The priority program only read the priority, which macOS allows | It attempts a change, as the spike measured. The read is the owner's stated limit, E-40 |
| U16 | The arguments and environment programs share one call, so "exactly one program" could not hold | The plan's U16 is amended: a grant names the programs its operation permits |
| U16 | Some calls cannot succeed in any confined run (`setsid`, `/dev/tty`) | The plan's U16 is amended: the sandbox's decision, a return or a kill, is measured |
| U16 | The system-info witness ran a copied platform binary, which macOS will not run | A probe measured `statfs` as the call `system-info` governs |
| U16 | The page-size witness read the numeric MIB, which resolves to the startup line's compat entry | It reads `hw.pagesize` by name and checks the value |

## Round 1 (head 2273ee4)

**Verdicts:**
- **code:** ready after one Major;
- **test:** trustworthy after three Majors;
- **security:** ship after one Medium.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | A budget timer firing just after a confinement kill recorded it as a crash, a red where QO3 says never red | code (Major) | Reproduced | Fixed, with the facts kept only when the engine's kill happened |
| 2 | QH17's subject-denial rows could not fail: the default deny already killed them | test (Major) | Confirmed against the template | Rows on otherwise allowed paths, with clean controls |
| 3 | The drain's POLLHUP guard had no test | test (Major), code (Minor) | Reproduced | Round 2 made the test deterministic (row 10) |
| 4 | U16's pure oracle tests never ran in the gate | test (Major), code (Minor) | Confirmed against `gate.sh` | Added to the gate's test loop |
| 5 | The calibration failed open on an error path | security (Medium) | Reproduced | Fails closed |
| 6 | Reading another process's priority is not refused | security, code, test | Measured | Put to the owner, who ruled E-40: a stated limit |
| 7 | Lows and Minors: the decoy outliving the engine, an extra dylib load, a listen on all interfaces, unchecked decoy files, a failed sample skipping the kill, EINTR, and test gaps | security, code, test | Confirmed | Fixed |

The round-1 fixes then broke the macOS evidence: 83 of 123 U16 rows. The profile builder refused every absent path parameter, and a pid check reached programs it should not. Both were fixed without changing a witness.

## Round 2 (delta, ac96a93 to 20baec4)

**Verdict:** one Blocker and two Majors.

| # | finding | verify | disposition |
|---|---|---|---|
| 8 | Three new pid rows sat in an untagged file and expected 64, while the Linux stub returns 70, so the Linux CI gate would fail | Simulated | They expect the stub's code off darwin/arm64 |
| 9 | The round-1 follow-up made `SUBJECT` and `SUBJECT_COPY` optional, against QH11 | Confirmed against QH11 | Required again; the necessity scenario names real files |
| 10 | The POLLHUP test passed with the guard removed | Reproduced | A deterministic test through the drain's seam |
| 11 | The decoy check followed links, a dead KTD20 exemption, an errored suite keeping its facts, and nits | Confirmed | Fixed |

## Evidence

- `gate.sh`, `check.sh` and `SCAN_COMMITS=<base>..HEAD ./scan.sh` pass at the merged head.
- macOS evidence, taken at the merged head and named in the pull request: the broad set, and U16's necessity and allowance suite, each repeated 25 times.
