# Peer-review gate — phase 3 PR7, unconfined safety (2026-11-26)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer;
- a different-model review on agy, which produced no output. Its headless run auto-denied the shell commands it needed, and the dispatcher marks an empty review as failed. AGENTS.md asks for a different model on specifications and plan phases, not on builds.

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Security:** this unit acts unconfined on directories a subject has written.
- **Testing:** its proofs are race and signal tests.
- **Code:** it always runs.

**The design ruling the build was given.** The sweep signals a recorded group only when the group's leader exists and its start time matches the recording. A subject's group holds only its leader (QO3). A computation's children can survive their group's leader, and that is a stated limit.

## Round 1 (head 03a5414)

**Verdicts:**
- **code:** ready after one Major;
- **security:** ship after three Mediums;
- **test:** not proven, with one Blocker and seven Majors.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The darwin walk's one real time-of-check race (`Fstatat`, then `Openat` with `O_NOFOLLOW`) had no test that fails when `O_NOFOLLOW` is removed: 20,000 racing walks stayed green | test (Blocker), security (Medium) | Reproduced; with a hook in the window, an outside file's mode changed | Fixed: a deterministic hook in that window (`TestDirectorySwapBeforeOpen`). The thousand-walk test is now a smoke test |
| 2 | The sweep stopped at the first workspace it failed on, so later dead runs' groups were never ended and every new run exited 70 | code (Major), security (Medium) | Confirmed. The sealed text left a failed start-sweep retry open | Put to the owner, who ruled (errata E-19): try every stale directory, name one that cannot be removed, and let the run go on. Fixed |
| 3 | Recording a group was opt-in (`Workspace != nil`) | security (Medium), code (Minor) | Confirmed | Fixed: the darwin real path refuses an invocation without a workspace |
| 4 | `EndGroup`'s wait for an empty group, the funnel's wait for the collector, its own group end, keeping the home on a failed end, and killing an unrecorded spawn were all untested | test (Major ×5) | Reproduced by mutation | Fixed, each with a test that fails without it |
| 5 | `Begin`'s real sweep was mocked away, and the tests swept the real temp parent | test (Major ×2), code (Minor) | Reproduced | Fixed: `Begin` has an injectable parent, and a `TestMain` guard in each package that calls `Run` |
| 6 | Cleanup opened a FIFO at `process.json` | test (Major), security (Info) | Reproduced | Fixed: `fstatat` without following first, `O_NOFOLLOW` and `O_NONBLOCK`, and the descriptor checked |
| 7 | The `groups` grammar had no negative rows. A pid of 0 would reach `kill(0)` and end the engine's own group | test (Major) | Reproduced | Fixed: rows against a positive control |
| 8 | Ignored signals became active (`nohup`), a second signal did nothing, and the post-signal wait had no bound | code (Minor), security (Low) | Confirmed | Fixed |
| 9 | `.process-pending` could wedge later records; a restored `process.json` was not atomic; E-11's directory was not named on a failed group end | code (Minor, Nit) | Confirmed | Fixed |
| 10 | `prepareAt` changes a hard-linked file's metadata | security (Info) | Confirmed: the profile kills `file-link` today | Fixed as defence in depth: a file with more than one link is only unlinked |
| 11 | The window between `ForkExec` and `RecordGroup` | security (Low), code (Minor) | Confirmed | A stated limit, with a comment at that site |
| 12 | Smaller test fixes: an `EINTR` retry, a mislabelled darwin control, and an assertion that accepted both outcomes | test (Minor) | Confirmed | Fixed |

The first macOS evidence run after this round failed 15 tests. Fourteen confinement-core evidence tests had no workspace, which the new mandatory recording refused. One race test could not `chmod` its own outside directory, which carries `uchg`. Both were fixed. The run also exposed about 280 temporary directories leaked by killed signal-test helpers: those now work in directories their parent test owns, and the leak is checked.

## Round 2 (delta, test lens; 03a5414 to 4d9db96)

Every round-1 Blocker and Major is now killed by a test. One Major remained, with three Minors and three Nits:

| # | finding | verify | disposition |
|---|---|---|---|
| 13 | One grade test still used the real temp parent | Confirmed by the parent's modification time | Fixed: its own workspace, plus `TestMain` guards |
| 14 | A FIFO renamed over `process.json` during the open is opened, though without blocking | Reproduced | Fixed: a rename-swap row; removing `O_NONBLOCK` turns it red. The window is named in a comment |
| 15 | The hard-link skip makes a multiply linked `uchg` leaf a cleanup failure that never clears | Reproduced unconfined | Kept, because a subject cannot make such a leaf. A test pins the fail-closed outcome |
| 16 | The trace closer is skipped when groups were not ended | Confirmed | That is the intent: a stuck collector may still hold the source. Stated and asserted |
| 17 | Nits: a duplicate row, a 5 s wait, the smoke test's label | Confirmed | Fixed |

## The darwin mutations the caller ran

These ran on macOS 27.2, outside the tool sandbox, in scratch copies.

| Mutation | Tests that failed |
|---|---|
| Keep the user flags | the `uchg`, `uappnd` and combined tests |
| Skip the ACL clear | the deny-ACL and combined tests |
| Skip the mode reset | `TestDarwinModeZero`, `TestDarwinCombinedDirectory` |
| Remove `FSOPT_NOFOLLOW` | `TestDarwinLeafLinkMetadata`, `TestDirectorySwapBeforeOpen`, `TestHomeSwapDuringCleanup`, `TestLockedDirectorySwapBeforeMode`, `TestThousandRacingWalks` |

## Stated limits

- A computation's child that survives its group's leader after `pql` is killed is never signalled. The sweep reports the group it cannot verify and removes the home; the child is confined to it.
- A subject started in the milliseconds between `ForkExec` and `RecordGroup` runs unrecorded if `pql` is killed with `SIGKILL` then.
