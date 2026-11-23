# Credit and routes

**Draft.** This folder is a draft of the conformance collection specification, version 1.0. It is not sealed, and it holds no `KIT.sha256`.

**What this file covers.** Which check can make each dimension green, where each expected value comes from, which known routes lead to a false pass or a false fail and what controls them, and the lists other rules refer to. These tables restate, in this specification's own words, the crediting and routing rules it takes from the CDR specification 0.9.1, which was their starting point; no text or fixture is copied from it. How to read the rules, and what binds, is as `collection-rules.md` says. The tables here bind.

## 1. The generated probes

| Probe | Rule | Dimension | Credits | Not observed when |
|---|---|---|---|---|
| Discovery | QD1 to QD4 | `discoverability` | yes | help cannot be read in a format §5 lists (`precondition_unmet`); any rule of §2 of `collection-rules.md` applies |
| Bare invocation | QP2 | `availability` | yes | §2 of `collection-rules.md` |
| Byte bound | QP3 | `bounding` | yes | no command declares a bound (`empty_target`); §2 of `collection-rules.md` |
| Declared exit codes | QP4 | `execution` | yes | no command declares exit codes (`empty_target`); §2 of `collection-rules.md` |
| Planted secret | QP10 to QP13 | `pii-secret-hygiene` | yes | no proved read (`input_missing`); a scan stopped by a bound (`harness_fault`) |
| Store unchanged | QP6 to QP9 | `side-effect-integrity` | yes | no control detected a change (`harness_fault`); nothing left to compare (`empty_target`) |

Every generated check other than discovery is not observed with `precondition_unmet` when discovery did not find a declared command (QD4). `bounding` can therefore be judged from generated checks alone, as can `discoverability`, `availability` and `execution`.

## 2. Routes to credit

Every dimension of the conformance scopes, which are 17 dimensions by default and 18 with `destructive`. "Authored" means a check in a runbook of the subject file (§5 of `collection-rules.md`). "Literal or computation" means its expected value is a literal in the file, or the one JSON value a computation prints from the pristine seed (QA2), never from the subject's own output (QA3).

| Dimension | Route to credit | Source of the expected value | No route for |
|---|---|---|---|
| `availability` | Bare invocation (QP2); or an authored check that a declared command exits as declared | The file's declared exit code | none |
| `execution` | Declared exit codes (QP4); or an authored exit-code check | The file's declared exit codes | none |
| `graceful-failure` | An authored check that an invalid invocation exits with a declared non-zero code and does not crash | Literal | none |
| `robustness` | An authored check that malformed input, such as an unknown flag value or an empty argument, gives a declared result | Literal | none |
| `response-correctness` | An authored check comparing output with an expected value | Literal or computation | a subject with no output that can be checked against the seed |
| `determinism` | An authored runbook that runs the same invocation twice, each time checked against the same expected value | Literal or computation, the same for both | a subject whose output carries a timestamp or random value it cannot be asked to omit |
| `provenance-honesty` | An authored check that the output states its source truthfully, for example that data came from the local store under an offline setting | Literal | a subject whose output never states a source |
| `completeness-signaling` | An authored check that a truncated or partial result says so, for example with a limit below the seeded row count | Literal or computation | a subject that never truncates |
| `output-conformance` | An authored check of the output's form, such as valid JSON with declared top-level keys | Literal | none |
| `flag-conformance` | An authored check that a flag has its documented effect | Literal or computation | a subject without a CLI, which the judge excludes |
| `bounding` | Byte bound (QP3) | The file's declared bound | a subject whose `bounding` is `none`, which the judge excludes |
| `discoverability` | Discovery (QD1 to QD4) | The file's declared commands and count | none |
| `error-message-quality` | An authored check that an error's standard error names the problem, such as the bad flag | Literal | none |
| `token-economy` | An authored check that a command's output stays within a declared size for a declared request | Literal | none |
| `side-effect-integrity` | Store unchanged (QP6 to QP9) only | The pristine seed | a subject that is not store-backed, which the judge excludes |
| `idempotency` | An authored runbook that runs a writing command twice and checks the store against what a computation derives from the seed and one application | Computation | a read-only subject, which the judge excludes |
| `destructive-guard` | An authored check, in a scope with `destructive`, that a destructive command without its confirmation refuses: by its exit code, or by a standard-error message | Literal | a subject without destructive operations, which the judge excludes |
| `pii-secret-hygiene` | Planted secret (QP10 to QP13) only | The canary the engine planted | a subject that reads no secret from a file |

`cli-mcp-parity` is in no scope in this version and has no route. Steering's dimensions are outside this specification.

**Restricted credit.** Only the planted-secret probe credits `pii-secret-hygiene`, and only the store-unchanged probe credits `side-effect-integrity`. Any other check routed to either dimension can be not held, but is non-crediting. A pattern scan of output can show that a secret leaked; it can never show that none did.

**Self-agreement.** A check whose expected value uses any of the subject's captured output is non-crediting (QA3). It can fail, and it cannot make a dimension green.

## 3. False passes, false fails and their controls

A green speaks only for the invocations and offline settings the subject file declares, and for the home outside its declared bookkeeping paths.

| Route | Effect | Control | Rule |
|---|---|---|---|
| A missing or empty store answers a read with success and no rows | false pass of an emptiness check, or of a check that any rows match | A check that an empty target would satisfy states its target size, and an empty target is not observed | QE7, QE8, the judge's C3 |
| Confinement was never applied | false pass of anything the subject would have refused | The launcher's marker, and the self-test before any check | QH5, QH6, QH9 |
| The self-test passes under a profile that grants one refused operation | false pass of that operation's refusal | Each self-test program is shown necessary by a profile that grants exactly its operation | QH9 |
| A kill by the engine or a timeout is read as the confinement's | false fail | A kill counts only after the marker, not at the budget, and not by the engine | QH21 |
| A slow host makes a fast command miss its budget | false fail | The known-fast control | QO4, QH24 |
| The store-unchanged comparison cannot see a change | false pass | A control copy that must show a written change | QP9 |
| A bookkeeping exclusion hides a change to the data | false pass | The store and the seed's paths cannot be excluded, and every exclusion is shown | QP7 |
| The planted secret was never read, so no leak was possible | false pass | A read is proved by two signals and an unchanged file, on a volume shown to record reads | QP11, QH26, QH27 |
| The subject forges the access time by writing, renaming or replacing the file | false pass | Every such write is refused with a kill | QH27 |
| A leak goes to the unified log | false pass | None: a stated limit | QP14 |
| A computation runs or reads the subject to produce the expected value | false pass of self-agreement | The computation's profile refuses the subject's binary and its copy | QH17 |
| A copy of the subject at another path | false pass of self-agreement | None: a stated limit | QH17 |
| A CLI that ignores a help request runs an action during discovery | false fail of a later check | Help runs only in an empty home under full confinement | QD2, QD5 |
| A declared command missing from discovery | false fail of every generated check | The generated checks are not observed, never red | QD4 |
| A changed private copy of the binary | false result of any kind | The copy is checked unchanged before each use | QH8 |

## 4. Encodings of a leaked secret

The leak scan looks for the canary in each of these forms: as written; base64 in the standard and the URL-safe alphabets, each with and without padding; hexadecimal in lower and upper case; and percent-encoding of every byte. The same list governs masking (QE18).

## 5. Help formats discovery reads

| Generator | Section that lists commands |
|---|---|
| Cobra | `Available Commands:` |
| urfave/cli | `COMMANDS:` |
| clap | `Commands:` or `SUBCOMMANDS:` |

In each, a command line is the section's indented first word on a line. Help in no listed format makes discovery not observed with `precondition_unmet`.
