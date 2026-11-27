# Peer-review gate — phase 3 PR9, slice b1 (2026-11-26)

**Mode:** build.

**Reviewers:**
- peer-code-reviewer;
- peer-test-reviewer, with mutations in scratch copies;
- peer-security-reviewer.

The design note had its different-model review on agy (logged in `docs/agent-reviews/2026-11-26-phase3-pr9-design-note-reviews.md`). AGENTS.md asks for a different model on specifications and plan phases, not on builds.

**persona-version:** cache/1.9.5.

**Tier rationale:**
- **Security:** help output and every invocation's output come from an untrusted subject.
- **Testing:** three rules leave the unmapped list, and the per-check comparison and the trace are the only evidence for discovery's logic.
- **Code:** discovery and the invocation outcome rules decide most checks.

## Round 1 (head 95ca50c)

**Verdicts:**
- **code:** ready after two Majors, with 29 of its own mutations killed;
- **test:** trustworthy after three Majors and one High. It ran 58 mutations, and 11 survived;
- **security:** ship after two Mediums.

No reviewer found a fault that becomes a pass, a command QS5 forbids being run, or a walk past QD2's bounds.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The walk copied every listed child twice with no bound on fan-out: 1 MiB of help per level reached about 27 GB of heap in 65 requests | code (Major), test (High), security (Medium) | Reproduced by all three | Fixed: pending capped by the requests left, nodes by parent index, found commands as counts and flags. A wide-help test asserts a ceiling |
| 2 | Raw help streams stayed alive for all 4,096 requests | security (Medium) | Reproduced | Fixed: the bytes are dropped once read and the drop is marked |
| 3 | Emptiness and "with output" were judged by the bytes kept, so a cut help read as empty | code (Major) | Reproduced | The owner ruled E-32: a cut help is unreadable (QD5), and the bytes written decide. Fixed in both readers |
| 4 | A command listed twice counted twice | code (Minor) | Two-way | The owner ruled E-33: found once, requested once. Fixed |
| 5 | A listed name with a NUL, ill-formed UTF-8 or an oversize argv | security (Low), code (Nit) | Open: no clause covers it | The owner ruled E-34: the help is unreadable, and the name is never requested. Fixed |
| 6 | Nothing asserted what a bound decides; removing `d.Bound` gave a false red | test (Major) | Reproduced | Fixed: the bound rows assert `precondition_unmet` at level 8 |
| 7 | E-29's rows went through a test helper, not `Collect` | test (Major) | Reproduced | Fixed: they run through `Collect`, and level 7 now faults on an unconfined invocation instead of stamping level 1 |
| 8 | QD5's negative row declared no command | test (Major) | Reproduced | Fixed: a row declares the bare binary |
| 9 | Smaller: the override on another dimension, KTD13's guard row for `Attribution`, the longest-prefix row, control 3's own-invocation QO1 bullet, exit detail 0x10000, the escaped cleanup error, a caller's plan mutated in place, a projection order that differed from `Attribute`, and the `!Ran` guard | test, code, security (Minor, Low, Nit) | Confirmed | Fixed |

31 mutations, each turning a named test red, are recorded with round 1's fix.

## Round 2 (delta, 95ca50c to 9a3a30a)

**Verdict:** ready after two Majors. No Blocker.

| # | finding | verify | disposition |
|---|---|---|---|
| 10 | E-34's walk-side size did not match the arguments the darwin source builds; JSON escaping could push a request past the limit and fault | Confirmed by reading both sides | The owner ruled that E-34's limit is fixed on every host: 131,072 bytes of a request's words, each word's UTF-8 bytes plus one, `--help` included. The darwin check stays as a backstop |
| 11 | The declared-command lookup encoded each child's full path: 18 s for 5,000 names under an 848 KiB ancestor | Measured | Fixed: a declared-prefix index, so a lookup costs the name's length |
| 12 | Scripted results depended on the host's `kern.argmax` | Open | Covered by the ruling in row 10 |
| 13 | The crash form of the exit-racing-kill race faulted | Confirmed against QO5 | Fixed: the program's crash |
| 14 | `ProjectEnd` emitted a null `by` for some `SIGKILL` ends; dropped bytes were not marked; the E-34 rows did not pin the arithmetic; the memory test covered short names only | Confirmed | Fixed |

12 mutations, each turning a named test red, are recorded with round 2's fix.

## Round 3 (confirmation, 9a3a30a to d67021a)

No Blocker or Major remained. One Minor and one Nit were fixed after it:
- **A two-ancestor boundary row:** the existing rows had at most one ancestor;
- **The memory test's comment and ceiling:** they described one row as the worst case.

## Owner rulings during this review

E-32, E-33 and E-34, with E-34's fixed limit, in `docs/specs/conformance-collection-1.0-errata.md`.

## Evidence

- `gate.sh`, `check.sh` and `SCAN_COMMITS=<base>..HEAD ./scan.sh` pass.
- 37 predicted cases pass, all 112 pending judge-checked cases end where `scripts/ratchet/pr9-prediction.json` says, and the prediction regenerates byte-identical.
- macOS evidence at d67021a: 1,151 passed, 0 failed, no temporary directory left behind.
