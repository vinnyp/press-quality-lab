# Credit and routes

**What this file covers.** Which check can make each dimension green, where each expected value comes from, which known routes lead to a false pass or a false fail and what controls them, and the lists other rules refer to. These tables restate, in this specification's own words, the crediting and routing rules it takes from the CDR specification 0.9.1, which was their starting point; no text or fixture is copied from it. How to read the rules, and what binds, is as `collection-rules.md` says. The tables here bind.

## 1. The generated probes

| Probe | Rule | Dimension | Checks in the required set | Credits | Not observed when |
|---|---|---|---|---|---|
| Discovery | QD1 to QD5 | `discoverability` | one | yes | a declared command was not found, or help cannot be read (`precondition_unmet`) |
| Bare invocation | QP2 | `availability` | one | yes | only by §2 of `collection-rules.md` |
| Byte bound | QP3 | `bounding` | one per declared `safe` command with a `byte_bound` | yes | no such command (`empty_target`, one check) |
| Declared exit codes | QP4 | `execution` | one per declared `safe` command with `exit_codes` | yes | no such command (`empty_target`, one check) |
| Planted secret | QP10 to QP13 | `pii-secret-hygiene` | one | yes | no secret, or no invocation proved a read (`input_missing`); a scan stopped by a bound, or a control that ran and was not killed (`harness_fault`) |
| Store unchanged | QP6 to QP9 | `side-effect-integrity` | one per declared `safe` command with `writes: false` | yes | no such command (`empty_target`, one check); nothing left to compare (`empty_target`); no control detected a change (`harness_fault`) |

Every probe can also be not observed by §2 of `collection-rules.md` and by a placeholder (QS5). Every generated check other than discovery is not observed with `precondition_unmet` when discovery did not find a declared command (QD4). `bounding` can therefore be judged from generated checks alone, as can `discoverability`, `availability` and `execution`.

A check with no command has, as its one target, the invocation it would have used, named by its own id.

## 2. Routes to credit

Every dimension of the conformance scopes, which are 17 dimensions by default and 18 with `destructive`. "Authored" means a check in a runbook of the subject file (§5 of `collection-rules.md`). "Literal or computation" means its expected value is a literal in the file, or the one JSON value a computation prints from the pristine seed (QA2), never from the subject's own output (QA3).

The "Route to credit" and "Source" columns describe how a dimension is usually made green and bind nothing. The "Credit condition" column binds: an authored check on the dimension credits only when its condition holds, and when QA3 allows it. "None" means no condition beyond QA3.

| Dimension | Route to credit | Source of the expected value | Credit condition | No route for |
|---|---|---|---|---|
| `availability` | Bare invocation (QP2); or an authored check that a declared command exits as declared | The file's declared exit code | none | none |
| `execution` | Declared exit codes (QP4); or an authored exit-code check | The file's declared exit codes | none | none |
| `graceful-failure` | An authored check that an invalid invocation exits with a declared non-zero code and does not crash | Literal | it is an `exit_code` check whose expected value is a literal other than 0, or it gives its own `exit_codes` and none of them is 0 | none |
| `robustness` | An authored check that malformed input, such as an unknown flag value or an empty argument, gives a declared result | Literal | none | none |
| `response-correctness` | An authored check comparing output with an expected value | Literal or computation | none | a subject with no output that can be checked against the seed |
| `determinism` | An authored runbook that runs the same invocation twice, each time checked against the same expected value | Literal or computation, the same for both | another step of the check's runbook has the same words as the check's step and a check on `determinism` with the same predicate, `path` and expected value as the subject file writes them | a subject whose output carries a timestamp or random value it cannot be asked to omit |
| `provenance-honesty` | An authored check that the output states its source truthfully, for example that data came from the local store under an offline setting | Literal | none | a subject whose output never states a source |
| `completeness-signaling` | An authored check that a truncated or partial result says so, for example with a limit below the seeded row count | Literal or computation | none | a subject that never truncates |
| `output-conformance` | An authored check of the output's form, such as valid JSON with declared top-level keys | Literal | the predicate is `stdout_json_equals`, `json_path_equals` or `row_count` | none |
| `flag-conformance` | An authored check that a flag has its documented effect | Literal or computation | none | a subject without a CLI, which the judge excludes |
| `bounding` | Byte bound (QP3) | The file's declared bound | none | a subject whose `bounding` is `none`, which the judge excludes |
| `discoverability` | Discovery (QD1 to QD4) | The file's declared commands and count | none | none |
| `error-message-quality` | An authored check that an error's standard error names the problem, such as the bad flag | Literal | the predicate is `stderr_contains` | none |
| `token-economy` | An authored check that a command's output stays within a declared size for a declared request | Literal | the predicate is `stdout_bytes_at_most` | none |
| `side-effect-integrity` | Store unchanged (QP6 to QP9) only | The pristine seed | never: only the probe credits | a subject that is not store-backed, which the judge excludes |
| `idempotency` | An authored runbook that runs a writing command twice and checks the store against what a computation derives from the seed and one application | Computation | the expected value comes from a computation, and at least two steps of the check's runbook, anywhere in it, resolve to the same declared command, which has `writes: true` or is `destructive` | a read-only subject, which the judge excludes |
| `destructive-guard` | An authored check, in a scope with `destructive`, that a destructive command without its confirmation refuses: by its exit code, or by a standard-error message | Literal | the step's command is `destructive` | a subject without destructive operations, which the judge excludes |
| `pii-secret-hygiene` | Planted secret (QP10 to QP13) only | The canary the engine planted | never: only the probe credits | a subject that reads no secret from a file |

`cli-mcp-parity` is in no scope in this version and has no route. Steering's dimensions are outside this specification.

**Where the table states no condition.** An authored check routed to a dimension whose condition is "none" credits whenever QA3 allows it. The engine does not test that the check measures that dimension: the routing is the author's claim, and this is a stated limit.

**Restricted credit.** Only the planted-secret probe credits `pii-secret-hygiene`, and only the store-unchanged probe credits `side-effect-integrity`. Any other check routed to either dimension can be not held, but is non-crediting. A pattern scan of output can show that a secret leaked; it can never show that none did.

**Self-agreement.** A check whose expected value uses any of the subject's captured output is non-crediting (QA3). It can fail, and it cannot make a dimension green.

## 3. False passes, false fails and their controls

A green speaks only for the invocations and offline settings the subject file declares, and for the home outside its declared bookkeeping paths.

| Route | Effect | Control | Rule |
|---|---|---|---|
| A missing or empty store answers a read with success and no rows | false pass of an emptiness check, or of a check that any rows match | A check that an empty target would satisfy states its target size, and an empty target is not observed; a check that asserts emptiness first proves the data exists | QA9, QE7, QE8, the judge's C3 |
| Confinement was never applied | false pass of anything the subject would have refused | The launcher's marker, and the self-test before any check | QH5, QH6, QH9 |
| The self-test passes under a profile that grants one refused operation | false pass of that operation's refusal | The confinement spike's record, `docs/spikes/2026-11-28-confinement.md`, showed once that each self-test program is necessary, by a profile that grants exactly its operation, on the spike code that the main branch holds from commit `5aa554a`, on which the confinement rules froze; a grade does not repeat this | QH9 |
| An author routes a check to a dimension it does not measure | false pass of that dimension | The credit conditions of §2, where the table states one; otherwise none, a stated limit | QA3 |
| A kill by the engine or a timeout is read as the confinement's | false fail | A kill counts only after the marker, not at the budget, and not by the engine | QH21 |
| A slow host makes a fast command miss its budget | false fail | The known-fast control | QO4, QH24 |
| The store-unchanged comparison cannot see a change | false pass | A control copy that must show a written change | QP9 |
| A bookkeeping exclusion hides a change to the data | false pass | The store and the seed's paths cannot be excluded, and every exclusion is shown | QP7 |
| The planted secret was never read, so no leak was possible | false pass | A read is proved by two signals and an unchanged file, on a volume shown to record reads | QP11, QH26, QH27 |
| The subject forges the access time by writing, renaming or replacing the file | false pass | Every such write is refused with a kill | QH27 |
| A leak goes to the unified log | false pass | None: a stated limit | QP14 |
| A computation runs or reads the subject to produce the expected value | false pass of self-agreement | The computation's profile refuses the subject's binary and its copy | QH17 |
| A copy of the subject at another path | false pass of self-agreement | None: a stated limit | QH17 |
| A CLI that ignores a help request runs an action during discovery | false fail of a later check | Help runs only in an empty home under full confinement | QD2, QD6 |
| A declared command missing from discovery | false fail of every generated check | Discovery and the generated checks are not observed, never red | QD4 |
| A database read that creates SQLite's shared-memory index | false fail of store unchanged | The `-shm` file is left out, and the other side files count only with bytes | QP8 |
| A step's output passed the bound the engine keeps | false pass or fail of a predicate | Predicates read drained bytes; a predicate needing a cut stream is not observed | QA9, QE10 |
| A changed private copy of the binary | false result of any kind | The copy is checked unchanged before each use | QH8 |

## 4. Encodings of a leaked secret

The leak scan looks for the canary in each of these forms. The same list governs masking (QE18) and the masking of recorded behaviour (QT5).

| Name | Form |
|---|---|
| `plain` | as written |
| `base64` | base64 in the standard alphabet, with padding |
| `base64-nopad` | base64 in the standard alphabet, without padding |
| `base64url` | base64 in the URL-safe alphabet, with padding |
| `base64url-nopad` | base64 in the URL-safe alphabet, without padding |
| `hex-lower` | hexadecimal in lower case |
| `hex-upper` | hexadecimal in upper case |
| `percent-upper` | percent-encoding of every byte, with upper-case hexadecimal digits |
| `percent-lower` | percent-encoding of every byte, with lower-case hexadecimal digits |

For the canary's alphabet (QP10), `base64` and `base64url` give the same text, and each unpadded form is a prefix of its padded one. The table keeps all four so that the scan, and masking, match each spelling a subject could produce.

## 5. Help formats discovery reads

A **section heading** is a line that starts with no space and is one of these:

| Generator | Heading |
|---|---|
| Cobra | `Available Commands:`, `Additional Commands:`, or any other line ending in ` Commands:`, as Cobra prints for a command group |
| urfave/cli | `COMMANDS:` |
| clap | `Commands:` or `SUBCOMMANDS:` |

A **section** runs from its heading to the next line that is empty or starts with no space. Each line in it that starts with a space lists one command: its first word after the leading spaces, with a trailing comma removed. A command line whose description begins `(hidden)`, `[hidden]` or `HIDDEN` is hidden.

A help with no section heading belongs to a leaf command (QD2). Every other form, a man-page style help included, is not read, and discovery is then not observed with `precondition_unmet` when it needed the list (QD5).

Help formats, predicates and probes are extended only by a new specification version. In 1.0 there is no registry or extension point.
