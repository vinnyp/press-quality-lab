# The judge's rulebook


**Rulebook version:** `1.0`. The judge reports this string as the version it judged under. The sealed draft reported `1.0-draft`; a baseline anchored under it is compared as any other older version is (E2).

**Version policy.** The string `1.0` names this text once it is sealed, and no other text. The string changes if and only if some input can receive a different output document or exit code, which includes any change to what X12 accepts. Purely editorial changes keep the string and are listed in a dated errata note. Every change of string forces a re-anchor (E2); there is no comparison across versions. The owner ratifies each version, and a version is final when its digest manifest is sealed.

**Starting point:** the CDR spec 0.9.1, quoted in `../reference/spec-facts.md`. That file is background: it is outside the frozen package and nothing in it is needed to build the judge. Every departure is in §14. Requirement numbers (R7, R23 …) are the requirements plan's.

**How to read the rules.** Rules have stable ids. Where a list says "first match", try its rules in the order written; the first whose condition holds decides. The order in which the input lists its checks, evidence, trials, markers and entries carries no meaning. A marker given twice is one marker. "Any" over an empty set is false. "Every" over an empty set is true, and is never used alone to reach green. A check, piece of evidence, trial or entry has, carries, gives or names something in a field when the field is not `null`; an empty string is a value.

**What binds.** These opening paragraphs and §1 to §13 bind a judge, except §9.1 and §9.4. §9.1 describes what the steering family does before the judge is called. §9.4 and §14 to §18 explain and record; where one of them disagrees with a rule, the rule is right.

**Scope and the request rules.** The rules of §4 and §4a apply to every check, piece of evidence, trial and baseline entry, whether or not its check or dimension is in scope. No rule of §4 or §4a passes over a check, a trial or an entry because it is out of scope, except X8, which says so. The scope still decides which arms are used (W2, X6) and which checks X10 requires.

**Two checks with one id.** Such a request is rejected by X10, but X12, X1 to X5, W1 to W3 and X6 to X9 are tested first, in the order of §4. Until X10, evidence or a trial that names the shared id names each of the checks, and X2 to X5 apply when they apply for any of them. A trial that W1 or W2 sets aside is set aside for every check sharing the id. Each prompt check sharing the id has its own pools: for each such check, a trial not set aside by W1 or W2 is kept in that check's pool when its condition is that check's current condition for its arm, and is set aside by W3 for that check otherwise. X6 to X9 are then tested for each check with its own pools (and, for X6's index test, the W1 trials that match that check's arm and current condition), and apply when they apply for any of the checks.

## 1. Terms

- **Dimension, group.** The 21 dimensions and their six groups are fixed by this table.

  | Group | Dimensions |
  |---|---|
  | Operability | `availability`, `execution`, `graceful-failure`, `robustness` |
  | Correctness | `response-correctness`, `determinism`, `cli-mcp-parity`, `provenance-honesty`, `completeness-signaling` |
  | Conformance | `output-conformance`, `flag-conformance`, `bounding` |
  | Usability | `discoverability`, `error-message-quality`, `token-economy` |
  | Safety | `side-effect-integrity`, `idempotency`, `destructive-guard`, `pii-secret-hygiene` |
  | Steering | `steering`, `tool-description-efficacy` |

- **Engine, family.** The engine is the program that fixes the required set, collects the evidence and calls the judge. A family is the part of it that runs one kind of check and reports what it saw.
- **Routed.** A check is routed to the dimension it names.
- **Required set.** The checks the engine fixed before collection. Each has an id, a kind (`plain`, `prompt` or `comparison`), a dimension, a record where it has one, whether it is **crediting**, and whether it is **empty-satisfiable** or **asserts emptiness**. A check is never both of the last two. A check that asserts emptiness names the precondition that must be observed. A prompt check also carries one **declared count**, which holds for every arm, and two **current conditions**: `condition`, for arm `B` in a compare scope and for the pool with no arm otherwise; and `condition_a`, for arm `A` in a compare scope, which no rule reads outside one.
- **Crediting.** Whether a held result can make the check's dimension green. It arrives set; the judge does not decide it.
- **In scope.** A dimension is in scope when the requested scope's row in §3 lists it. A check or a baseline entry is in scope exactly when its own dimension is.
- **Arm, compare scope.** A scope that includes `compare` is a compare scope. A trial's arm is `A`, `B` or none (`null` in the input). A compare scope uses arms `A` and `B`; any other scope uses the arm none.
- **Record.** The authored unit a check belongs to. An expected-failure marker names one record and one dimension.
- **Pool.** The trials of one prompt check in one arm under that check's current condition. Every prompt check has one pool for each arm the scope uses: arm `A` and arm `B` when the scope includes `compare`, otherwise the one with no arm. A pool may be empty. The trials of the check that W3 (§4a) sets aside form **earlier pools**, one for each arm and condition: each is listed with its arm, its condition, its counts and the §9.3 result it would get, and takes no part in any rule.
- **Condition.** A digest covering the subject's surface and tool descriptions, the model and its sampling settings, the prompt and framing, the declared count, and the rulebook version. The judge compares conditions for equality and never looks inside one. A current condition that is `null` differs from every condition.
- **Claimed tier.** `scripted` for a test run, which is a run against scripted model replies, otherwise `live`. It governs §4a and N1, and is the stored tier of a prompt or comparison check.
- **Stored tier.** The tier a check's result is stored and compared under: a plain check's evidence tier, and the claimed tier for a prompt or comparison check.
- **Own verdict.** The run verdict computed by §8 with no baseline comparison.

## 2. Inputs

- **J1.** The request: `grade` or `anchor`.
- **J2.** The requested scope.
- **J3.** The required set.
- **J4.** Evidence for plain checks: at most one piece per check. Each names its check by id and has an outcome (`held`, `not_held` or `not_observed`), a reason where the outcome is `not_observed`, its tier, a target size and whether the precondition was observed, where the check needs them, and its evaluator.
- **J5.** Trials for prompt checks. Each names its prompt check by id and has an arm (`A`, `B`, or none), a condition, a trial index, a declared count, an outcome (`held` or `not_held`; the schema also admits `not_observed`, which X9 rejects), whether it errored, its tier, its model, its sampling settings, its framing digest and its evaluator. An evaluator is a model when its kind is `model`, whatever else it holds.
- **J6.** Subject facts: `read_only`, `has_destructive_ops`, `store_backed`, `has_cli`, each true, false or unstated; and `bounding`, which is `none`, `declared` or unstated.
- **J7.** Expected-failure markers.
- **J8.** The claimed tier.
- **J9.** Optionally a baseline (§10). A list of retired check ids and a list of accepted-regression check ids; both are empty on a grade request (X11).

## 3. Scopes

A scope is a set of parts. The parts are `conformance`, `destructive`, `steering` and `compare`. `destructive` needs `conformance`; `compare` needs `steering`. That gives the eight legal scopes below and no others.

| Scope | Dimensions in scope |
|---|---|
| `conformance` (the default) | Every dimension of Operability, Correctness, Conformance, Usability and Safety, except `destructive-guard` and `cli-mcp-parity` |
| `conformance+destructive` | The same, with `destructive-guard` |
| `steering` | `steering` |
| `steering+compare` | `steering`, `tool-description-efficacy` |
| `conformance+steering`, `conformance+destructive+steering`, `conformance+steering+compare`, `conformance+destructive+steering+compare` | The union of the named parts |

`cli-mcp-parity` is in no scope in this version: nothing feeds it until the MCP surface is probed.

## 4. Requests the judge rejects

A rejected request produces no verdict and exits with code 2. First match, in this order: X12; then X1 to X5 in number order, which see every trial; then the trials are set aside by §4a; then X6 to X11 in number order.

- **X1.** The scope is not one of the eight. An empty list of parts is not one of them, and neither is a list that names a part twice.
- **X2.** Evidence or a trial names a check that is not in the required set; or evidence names a prompt or comparison check; or a trial names a check that is not a prompt check.
- **X3.** Two pieces of evidence name the same plain check; or evidence with the outcome `not_observed` gives no reason; or evidence with another outcome gives one.
- **X4.** A marker names a dimension in the Safety or Steering group, or `cli-mcp-parity`; or names a record with no crediting check, of any kind, routed to that dimension.
- **X5.** A check is both empty-satisfiable and asserts emptiness; or asserts emptiness and names no precondition; or a prompt or comparison check is not crediting; or a check that is not a prompt check carries a declared count or a condition for either arm.

- **X6.** For each pool, empty or not, take its trials together with the trials W1 set aside that name the pool's check, arm and current condition: two of them share an index, or their indices are not `0, 1, 2 …` without a gap. No trials at all passes this test. Or a trial of the pool states a declared count other than its check's, where the check has one.

  X6 to X9 test only the trials kept after §4a. The one exception is X6's index test, which counts the indices of W1 trials as stated. Among X6 to X11, nothing else about a trial that was set aside can cause a rejection, and a W1 trial whose arm the scope does not use, or whose condition is not the current one of the check whose pool is being tested, is not counted for that pool.
- **X7.** A prompt check's declared count in the required set is below 16. A check with no declared count is not caught here.
- **X8.** A prompt check in scope has no declared count, or has no `condition`, or in a compare scope has no `condition_a`.
- **X9.** A trial's outcome is neither `held` nor `not_held`, or its evaluator is a model.
- **X10.** Two checks in the required set share an id; or a prompt check is not routed to `steering`; or a comparison check is not routed to `tool-description-efficacy`; or a plain check is routed to a dimension of the Steering group; or the scope includes `compare` and the required set does not hold exactly one comparison check; or it does not include `compare` and the required set holds any comparison check.
- **X11.** A retired check is in the current required set; or a grade request names retired checks or accepted regressions; or two baseline entries share a check id; or a baseline entry whose own kind is `prompt` has no condition, no `k` or no `n`, or an `n` of zero, or a `k` above its `n`; or the `n` of every baseline entry whose own kind is `prompt`, in scope or not, sum to more than 9007199254740991.
- **X12.** The input is not an acceptable document as the contract's §3 defines it. The contract's list is the whole of X12.

## 4a. Trials set aside

After X1 to X5 and before X6 to X11, three kinds of trial are set aside. They are listed in the output and take no part in any pool, count or fraction. Each trial is tested against W1, W2 and W3 in that order and takes the first that applies. (Where two checks share an id, W3 is decided for each of them: see "Two checks with one id".)

- **W1.** A trial whose tier differs from the claimed tier (`wrong_tier`). It joins no earlier pool.
- **W2.** A trial whose arm the scope does not use (`other_arm`): a trial with an arm when the scope has no `compare`, or with none when it has. It joins no earlier pool.
- **W3.** A trial whose condition differs from its check's current condition for its arm (`earlier_pool`). These form the earlier pools of §1. An earlier pool is judged by §9.3 against the declared count its own trials state. If its trials state more than one count, or a count of zero, or two of them share an index, or their indices are not `0, 1, 2 …` without a gap, or any has an outcome other than `held` or `not_held` or an evaluator that is a model, its rule is P1 and its counts are not given.

## 5. The result of a plain check

For an in-scope plain check. First match.

- **C1. Unobserved**, `no_evidence`: no evidence names it.
- **C2. Unobserved**, reason as given: the outcome is `not_observed`.
- **C3. Unobserved**, `empty_target`: the outcome is `held`, the check is empty-satisfiable, and its target size is zero or absent.
- **C4. Unobserved**, `precondition_unmet`: the outcome is `held`, the check asserts emptiness, and its precondition is not observed to be true.
- **C5. Failed, expected**: the outcome is `not_held` and a marker names this check's record and dimension.
- **C6. Failed**: the outcome is `not_held`.
- **C7. Held, crediting**: the check is crediting.
- **C8. Held, non-crediting.**

In-scope prompt checks are decided by §9.3 and comparison checks by §9.5. Their results use the same names and enter §6 the same way. An out-of-scope check of any kind has the result Out of scope and no rule.

## 6. The state of a dimension

Over the in-scope checks routed to it. First match.

- **D0. OUT-OF-SCOPE:** the dimension is not in scope. The reason is `live_not_run` for a Steering-group dimension and `scope_not_requested` otherwise. It is counted nowhere.
- **D1. RED:** any check is Failed.
- **D2. N/A:** the dimension is structurally excluded (below) and no check is Failed, expected.
- **D3. HARNESS**, reason `placeholder_unfilled`: the fact that decides the dimension's exclusion is unstated.
- **D4. HARNESS:** any check is Unobserved.
- **D5. XFAIL:** any check is Failed, expected.
- **D6. XPASS:** any check is Held, crediting, and a marker names this dimension. (By X4, a marker on a dimension that reaches D6 always has such a check there.)
- **D7. GREEN:** any check is Held, crediting.
- **D8. UNCOVERED:** otherwise, including a dimension with no check.

**Structural exclusion.**

| Dimension | Excluded when | Deciding fact |
|---|---|---|
| `bounding` | `bounding` is `none` | `bounding` |
| `idempotency` | `read_only` is true | `read_only` |
| `destructive-guard` | `has_destructive_ops` is false | `has_destructive_ops` |
| `side-effect-integrity` | `store_backed` is false | `store_backed` |
| `flag-conformance` | `has_cli` is false | `has_cli` |

No other dimension has a deciding fact. (The engine never places a `flag-conformance` check for an MCP surface in the required set; the judge does not inspect surfaces.)

An excluded dimension is N/A whatever was or was not observed on it, unless a check on it is Failed (D1, RED) or Failed, expected, which keeps D2 from applying and gives XFAIL by D5 when no check on the dimension is Unobserved. When D2 applies, an Unobserved check there is listed with its result and does not make the dimension HARNESS: the fact that excludes the dimension is the subject's own declaration, and a check on a dimension that does not apply has nothing to observe. The one exception follows from the list: when a Failed, expected check keeps D2 from applying, the list goes on, and an Unobserved check then makes the dimension HARNESS by D4 before D5 is reached.

A marker is **used** when a check of its record and dimension is Failed, expected. When a dimension is XPASS, every marker that names it is **stale** and is listed with the run. Every other unused marker is dormant and is not listed, including an unused marker on a dimension that another record's marker made XFAIL.

## 7. The state of a group

Over the group's in-scope dimensions. First match.

- **G1. OUT-OF-SCOPE:** the group has no dimension in scope.
- **G2. RED:** any dimension is RED.
- **G3. GAPPED:** any dimension is HARNESS or UNCOVERED.
- **G4. KNOWN-DEFECT:** any dimension is XFAIL.
- **G5. GREEN:** any dimension is GREEN or XPASS.
- **G6. N/A:** otherwise; every in-scope dimension is N/A. No legal scope reaches this in the current version, since every group has an in-scope dimension with no deciding fact; the rule is kept so the list is complete.

## 8. The run verdict

Over every in-scope dimension, and the comparison (§11) where a baseline applies. First match.

- **V1. FAILED:** any dimension is RED, or the comparison is Regressed.
- **V2. PROVISIONAL, `gaps`:** any dimension is HARNESS or UNCOVERED; or no dimension is GREEN, XPASS or XFAIL; or the comparison is Not-measured.
- **V3. PROVISIONAL, `known-defects`:** any dimension is XFAIL.
- **V4. PASSED.**

Every run that is not rejected reports its verdict (the own verdict, for an anchor request), the scope, the rulebook version, the counts of XFAIL and XPASS dimensions, the stale markers, and, whichever rule decided the dimension or the run, every Unobserved check, every HARNESS, UNCOVERED and XFAIL dimension, and for a grade every Missing or Not compared baseline entry. The output holds all of these in its lists of checks, dimensions and entries.

| Outcome | Plain word | Exit code |
|---|---|---|
| grade: PASSED | green | 0 |
| anchor: Anchored (§12), whether the own verdict is PASSED or PROVISIONAL, `known-defects` | anchored | 0 |
| grade: FAILED | red | 1 |
| rejected request (§4) | usage | 2 |
| grade: PROVISIONAL, `known-defects` | known defect | 3 |
| grade: PROVISIONAL, `gaps` | not-measured | 4 |
| anchor: Refused (§12) | refused | 5 |

At lower levels: GREEN and XPASS are green; RED is red; XFAIL and KNOWN-DEFECT are known defect; HARNESS, UNCOVERED and GAPPED are not-measured; N/A is not applicable; OUT-OF-SCOPE carries its own state and its reason (D0) and enters no scorecard count. The judge's output itself counts only XFAIL and XPASS dimensions.

## 9. Steering

### 9.1 One trial

Decided by the steering family and reported; the judge does not recompute it.

- **S1.** A trial sends one authored prompt to the agent under test. The **first relevant call** is its first call to any tool of the subject; calls to other tools are ignored.
- **S2.** The trial is `held` when the first relevant call is to a tool in the prompt's acceptable set and no tool in its forbidden set is called at any point. Otherwise it is `not_held`, including when there is no relevant call and when the trial ends in a timeout or a turn limit.
- **S3.** A trial is **errored** only when the harness failed and the failure is evidenced on the harness side.

### 9.2 Which trials count

For one pool.

- **T1.** A trial is **uncounted** when it errored.
- **T2.** The **scored trials** are the counted trials in index order, up to the declared count. Later trials are kept in the record and ignored.
- **T3.** The **errored trials counted** are those up to and including the last scored trial, or all of the pool's when the declared count is not reached. The **error fraction** is that number divided by the number of trials over the same span, when the span has at least one trial; no rule reads it in this version, and it is defined for the scorecard.
- **T4.** An in-scope prompt check carries its declared count in the required set (X8), at least 16 (X7). Before calling the judge, the engine sets it from the subject file, or to 40 when the file states none; the judge supplies no default. Sixteen is the smallest count at which P3 can be met, so the floor and the derived minimum are the same number.

### 9.3 The result of a prompt check

A prompt check is judged on one pool: its arm `B` pool when the scope includes `compare`, otherwise its pool with no arm. With `n` scored trials of which `k` held, and, when `n` is above zero, Wilson bounds `L` and `U` (§9.6). First match.

- **P0. Unobserved**, `error_cap`: the errored trials counted by T3 number more than a quarter of the declared count. This is the same as an error fraction above 0.20 once the declared count is reached, and before then no further trial can bring the fraction back under it.
- **P1. Unobserved**, `trials_short`: `n` is below the declared count, which includes a prompt with no trials. The number still to run, the declared count less `n`, is reported.
- **P2.** Not used. Draft 4 had a second error-cap rule here; P0 decides every case it could.
- **P3. Held, crediting:** `L > 0.80`.
- **P4. Failed:** `U < 0.80`.
- **P5. Unobserved**, `interval_undecided`.

Each prompt is its own check routed to `steering`, so §6 decides the dimension: any Failed prompt makes it RED, otherwise any Unobserved prompt makes it HARNESS, otherwise it is GREEN, and with no prompt at all it is UNCOVERED.

### 9.4 What these rules promise

Computed over every declared count from 16 to 100.

- A prompt whose true rate is exactly 80% is judged green at most **2.9%** of the time (the worst case is 2.87%, at 47 trials).
- The same prompt is judged red at most **5.2%** of the time (the worst case is 5.13%, at 18 trials).
- `steering` is green only when every prompt is, so a wrong green for the dimension is no likelier than for its weakest prompt.
- A wrong red for the dimension is likelier with more prompts. For ten prompts all sitting exactly at the bar it is 36% at 40 trials and at most 41%. A subject comfortably above the bar is far below this: at a true rate of 95% and 40 trials a prompt is red less than once in a million runs.

| Declared count | Green needs | Red at or below | Green if truly 95% | Green if truly 90% | Green if truly 80% | Red if truly 80% | Red if truly 60% |
|---|---|---|---|---|---|---|---|
| 16 | 16 | 9 | 44% | 19% | 2.8% | 2.7% | 47% |
| 25 | 24 | 16 | 64% | 27% | 2.7% | 4.7% | 73% |
| 40 | 37 | 27 | 86% | 42% | 2.8% | 4.3% | 87% |
| 60 | 55 | 41 | 92% | 44% | 1.2% | 2.2% | 93% |
| 100 | 88 | 72 | 99.9% | 80% | 2.5% | 3.4% | 99.5% |

### 9.5 The result of a comparison check

Only in a scope that includes `compare`. One comparison check, routed to `tool-description-efficacy`. Arm `A` is the baseline arm and arm `B` the candidate. The judge computes this result itself from the trials; no family reports it. First match.

- **K1. Unobserved**, `trials_short`: no prompt check is in scope, or any prompt's pool in either arm is P1 and none is P0.
- **K2. Unobserved**, `arms_not_comparable`: taken across both arms and every prompt, the scored trials do not all share one model, one sampling value and one framing digest; or any of them has no sampling value. Two sampling values are the same when they are equal as JSON values, whatever the order of their keys. Numbers are compared by their exact decimal value, not as floating-point numbers: `0`, `0.0`, `0e5` and `-0` are the same, `0.1` and `0.10` are the same, and `0.1` and `0.1000000000000000055` differ. A judge can do this by stripping leading and trailing zeros and then comparing the sign, the remaining digits as a digit string (up to 32 digits, which do not fit a 64-bit integer), and the adjusted decimal exponent, which is small because an exponent has at most four digits (contract §3). Every spelling of zero is equal, whatever its sign or exponent; `1e2`, `100` and `100.0` are the same. The comparison is recursive: objects are equal when they have the same keys with equal values, arrays when they have the same length and equal elements in order, and a number never equals a string, boolean or null. Strings are equal when they are the same sequence of code points; no Unicode normalisation is applied. Only `null` is no sampling value; an empty object is a value.
- **K3. Unobserved**, `error_cap`: any prompt's pool in either arm is P0.
- **K4.** Otherwise let each **arm total** be `k` held out of `n` scored trials summed over its prompts, `d = k_B/n_B − k_A/n_A`, and `(d_lo, d_hi)` its Newcombe interval (§9.6). First match:
  - **Failed** (`regressed`): `d_hi < −0.10`.
  - **Held, crediting** (`improved`): `d_lo > 0.10`.
  - **Held, crediting** (`no_material_change`): `d_lo > −0.10` and `d_hi < 0.10`.
  - **Unobserved**, `interval_undecided`.

"No material change" says only that the two arms are alike. Two arms that are both poor are alike; `steering` reports that they are poor.

Summing over prompts treats every trial as independent. When prompts change by different amounts in different directions, the arm totals can hide it: a prompt that falls is offset by one that rises. This is a known limit.

### 9.6 The intervals

With `z = 1.959964` and `p = k/n`:

```
centre = (p + (z·z)/(2·n)) / (1 + (z·z)/n)
half   = z · sqrt( (p·(1−p))/n + (z·z)/(4·n·n) ) / (1 + (z·z)/n)
L = centre − half        U = centre + half

d_lo = d − sqrt( (p_B − L_B)² + (U_A − p_A)² )
d_hi = d + sqrt( (U_B − p_B)² + (p_A − L_A)² )
```

`k` and `n` enter the formulas as doubles. A total is summed as an integer and then converted; X11 and the size limit of X12 keep every total at or below 9007199254740991, so the sum fits 64 bits and the conversion is exact. All arithmetic is IEEE 754 binary64, round to nearest, ties to even, with no extended-precision intermediates; `sqrt` is the correctly rounded square root; `z` is the double nearest `1.959964`. `d_lo` and `d_hi` are computed from unrounded bounds, in IEEE double arithmetic. Every bound is then rounded to six decimal places, correctly rounded from the double's exact value, before it is compared or reported. A double that lies exactly half way between two six-place values rounds to the one whose last digit is even. The formulas are evaluated as grouped above, one operation at a time and left to right within a group, with no fused multiply-add; a square is its operand multiplied by itself. At `k = n = 16`, `L` is `0.806392`.

## 10. What a baseline holds

One entry for each check that was in scope when the baseline was anchored and whose result was Held or Failed: its id, kind, dimension, `held` or `failed`, the rulebook version, and its stored tier. A check that failed as expected is stored as `failed`. A prompt check's entry also holds the condition, `k` and `n` of the pool it was judged on (§9.3): its arm `B` pool in a compare scope.

## 11. The comparison

- **B1.** With no baseline, or one with no entry whose dimension is in scope, there is **no comparison**, and the scorecard says there is no baseline for this scope. No check is listed as New.
- **B2.** Entries whose dimension is out of scope are **Set aside**: they are listed as not compared for that reason and affect nothing, including R2.

**Each in-scope entry for a plain check is classed, first match:**

- **E1. Missing:** no check in the required set has the entry's id, or the current check's result is Unobserved.
- **E2. Not compared:** the entry's rulebook version differs from the judge's; or its stored tier differs from the stored tier of the current result; or the current check's kind or dimension differs from the entry's.
- **E3. Regressed:** the entry is `held` and the result is Failed or Failed, expected.
- **E4. Improved:** the entry is `failed` and the result is Held, crediting or non-crediting.
- **E5. Same.**

**Each in-scope entry for a prompt check is classed, first match:**

- **E6. Missing:** no check in the required set has the entry's id; or the current check is a prompt check and its result is P0 or P1; or the current check is not a prompt check and its result is Unobserved.
- **E7. Not compared:** as E2.
- **E8. Regressed:** the entry is `held` and the result is P4.
- **E9. Improved:** the entry is `failed` and the result is P3.
- **E9a. Same:** the entry is `failed` and the result is P4.
- **E9b. Not compared:** the entry is `failed`.
- **E10. Same:** the entry's condition equals the current one, and its `k` and `n` equal the current ones.
- **E11.** Otherwise the prompt joins the **changed set**, which therefore holds only entries that are `held`. If the changed set is not empty, sum the entries' `k` and `n` as arm `A` and the current pools' as arm `B`, and apply K4 to those totals: `regressed` makes every prompt in the set **Regressed**; `improved` makes them **Improved**; `no_material_change` makes them **Same**; undecided makes them **Not compared**.

In E10 and E11 the current condition, `k` and `n` are those of the pool the prompt is judged on (§9.3): its arm `B` pool in a compare scope.

An in-scope baseline entry for a comparison check is classed by E1 to E5.

The lists are first match, so E1 and E6 come before E2 and E7: an entry that E1 or E6 classes Missing is Missing even when the check's kind or dimension has also changed. The two lists differ on purpose for an undecided prompt: a plain entry whose check is now a prompt at P5 is Missing by E1, because P5 is Unobserved; a prompt entry whose prompt is P5 is not Missing by E6, and goes on to E7 and the rules after it. A current check that is out of scope has no result and no stored tier; its dimension differs from the entry's, so the entry is Not compared.

An entry is matched to a current check by id alone, and the list above that applies is chosen by the entry's kind. In-scope checks whose id matches no entry at all, set aside or not, are **New** and take no part.

**The comparison's result, first match:**

- **R1. Regressed:** any entry is Regressed.
- **R2. Not-measured:** any entry is Missing or Not compared. The next action reported is to re-anchor.
- **R3. Not regressed.**

A marker never softens a comparison (E3).

Not regressed says only that nothing in the baseline got worse. A prompt that is P5 can be classed Same by E10 or E11, and the comparison check can be Held while a prompt is P5; `steering` is then HARNESS and the run is not green.

A baseline this judge anchored never holds a `failed` prompt or comparison entry, because a failed prompt makes the run red and N2 refuses to anchor it. E9, E9a and E9b therefore apply only to a baseline made some other way; they are kept so that every entry has a class.

## 12. Anchoring

For an anchor request. Compute the own verdict, then, first match:

- **N1. Refused**, `test_run`: the claimed tier is `scripted`.
- **N2. Refused**, `red`: the own verdict is FAILED.
- **N3. Refused**, `not_measured`: the own verdict is PROVISIONAL, `gaps`.
- **N4. Refused**, `stale_marker`: any dimension is XPASS.
- **N5. Refused**, `unretired_check`: an in-scope entry of the replaced baseline is absent from the required set and not named as retired.
- **N6. Refused**, `unaccepted_regression`: an in-scope entry of the replaced baseline is Regressed under §11 and not named as an accepted regression.
- **N7. Anchored.**

A refusal exits 5. Under N5 and N6 it names the checks that caused it; under N1 to N4 it names none, since the cause is not a check. The new baseline holds one entry per in-scope check as §10 defines, and every out-of-scope entry of the replaced one unchanged, except an out-of-scope entry whose id one of the new entries carries, which is dropped. The record of the anchor keeps the replaced in-scope entries and any entry dropped this way. An in-scope entry whose check now has no entry under §10, because it is out of scope or Unobserved, has no successor and is in the record only.

## 13. Closed lists

**Reasons a family may report with `not_observed`:** `harness_fault`, `could_not_start`, `input_missing`, `precondition_unmet`, `empty_target`, `placeholder_unfilled`, `computation_failed`, `earlier_step_failed`, `network_attempted`, `network_unverifiable`, `live_not_run`, `judgement_unverified`.

**Reasons the judge produces:** `no_evidence` (C1), `empty_target` (C3), `precondition_unmet` (C4), `placeholder_unfilled` (D3), `trials_short` (P1, K1), `error_cap` (P0, K3), `interval_undecided` (P5, K4), `arms_not_comparable` (K2), `scope_not_requested` and `live_not_run` (D0). Trials set aside carry `wrong_tier` (W1), `other_arm` (W2) or `earlier_pool` (W3).

**Result names.** A check: Unobserved; Failed; Failed, expected; Held, crediting; Held, non-crediting; or Out of scope when its dimension is. A comparison check that is not Unobserved also carries `regressed`, `improved` or `no_material_change`. A baseline entry: Set aside, Missing, Not compared, Regressed, Improved, Same; a check with no entry is New. A comparison: no comparison, Regressed, Not-measured, Not regressed. An anchor: Anchored, or Refused with a reason. A rejected request carries the id of the rule that rejected it, `X1` to `X12`.

**Anchor refusals:** `test_run`, `red`, `not_measured`, `stale_marker`, `unretired_check`, `unaccepted_regression`.

**How the spec's result values arrive as evidence:** `pass` is `held`. `fail` is `not_held`. `error` is `not_held` when the subject crashed, exited wrongly or ran past its budget, and `not_observed` with `harness_fault` when the engine or harness faulted. `gated` is `not_observed`, `precondition_unmet`. `not_evaluated` is `not_observed`, `input_missing`. `vacuous` is `not_observed`, `empty_target`. `harness` is `not_observed` with `harness_fault`, `could_not_start` or `live_not_run`.

## 14. Amendments to spec 0.9.1

| # | Spec | This rulebook | Requirement |
|---|---|---|---|
| A1 | §9.5 rule 6, §15.1: a crediting pass covers a dimension though another check feeding it did not run | An Unobserved check makes the dimension HARNESS (D4 before D7) | R8 |
| A2 | §9.5, §9.7, §9.8: HARNESS is outside the denominator | HARNESS counts at every level and blocks green (G3, V2) | R8 |
| A3 | §9.5: XFAIL ranks above HARNESS | HARNESS ranks above XFAIL (D3, D4 before D5) | R23 |
| A4 | §9.7, §9.8: a known defect outranks a gap | A gap outranks a known defect (G3 before G4, V2 before V3) | R23 |
| A5 | §9.6: an omitted capability counts as false and can exclude a dimension | An unstated fact makes the dimension HARNESS (D3) | R21 |
| A6 | §9.6: `steering` and `tool-description-efficacy` are N/A outside live mode or without an experiment | Both are a matter of scope | R19 |
| A7 | §15.1: `error` always grades RED | A subject fault is red; an engine or harness fault is not-measured | R7, R22 |
| A8 | §5.4 against Appendix B.3: unresolved whether an over-budget invocation is `error` or `not_evaluated` | Over budget is a subject failure and red | R22 |
| A9 | §9.2: an empty target is vacuous, and so is an assert skipped because its prerequisite observation was absent | A check that asserts emptiness names its precondition and is Unobserved without it (C4) | R3 |
| A10 | §15.1: `gated`, `not_evaluated` and `vacuous` ran without credit, leaving the dimension UNCOVERED or, beside a crediting pass, GREEN | Each is Unobserved, which makes the dimension HARNESS | R7, R8 |
| A11 | §9.6: `cli-mcp-parity` is excluded or applicable by the subject's surfaces and parity records; `flag-conformance` is N/A on an MCP surface | `cli-mcp-parity` is in no scope in this version; `flag-conformance` is excluded when the subject has no CLI | R19 |
| A12 | §5.3: a regression diff identifies an element by record, surface and position | A check has one stable id | R25 |
| A13 | §7: the steering asserts are non-crediting, so they never make `steering` green | A prompt judged P3 is crediting | R13, R14 |
| A14 | §9.6: nothing feeds `tool-description-efficacy` | The comparison check feeds it | R29 |
| A15 | §9.1: judging of prose pass criteria is left to the implementation | Prose criteria play no part; S2 decides a trial | R13 |
| A16 | No notion of a trial, arm, pool or declared count | §9 | R28, R30, R31 |
| A17 | §9.5a: a consumer must not treat a passed run with stale markers as a clean baseline | N4 refuses to anchor it | R26 |
| A18 | Regression and baselines are left to the consumer | §10 to §12 | R9, R25, R26, R41 |
| A19 | No process exit codes; no notion of scope | §8, §3 | R24, R19 |

**Kept unchanged:** the dimensions and groups; which plain checks are crediting, including the restricted-credit dimensions; a failed check overriding a structural exclusion; markers acting per record and dimension; an unexpected failure dominating an expected one; a stale marker counting as green with an advisory; no marker on Safety or Steering.

## 15. Where this version departs from the requirements plan

These need the plan amended, or the rule changed back.

- **R30** said a trial that errored after a tool was chosen is scored on that choice. Here every errored trial is uncounted (T1): a trial cut off after an acceptable first call was never observed to avoid the forbidden tools. The owner chose this on 2026-11-18 and R30 in the plan is amended to match. Timeouts and stalls are still failures (S2).
- **R30** and **AE14** say scripted trials presented in a live run are not counted. They are set aside before any other rule sees them (W1).
- **R31** lists what makes up a condition and does not include the declared count. Here it does (§1), so that changing the count starts a new pool and the trials already run under the old count are kept as an earlier pool, not mixed in.
- **R19** and **AE9** say a dimension outside the scope is shown as not-measured with its reason. Here it has its own state, OUT-OF-SCOPE, shown with that plain word and reason (D0, §8); the difference is that it is named separately so that it can be kept out of every count.
- **R25** and **AE18** say baseline checks outside the scope are listed as not compared. Here they are Set aside (B2): listed as not compared, and kept out of rule R2.
- **AE19** describes accepting a steering regression that still clears the bar. At 40 trials a prompt is green only at 37 or more held, so a drop larger than the ten-point margin cannot happen while every prompt stays green. The case is reachable at about a hundred trials a prompt: ten prompts at 1000 against 880 give an interval of −0.142 to −0.101.

## 16. Worked cases

1. `execution`: one check Held, crediting; two Unobserved, `harness_fault`. **D4 HARNESS.** Under the spec, GREEN.
2. `response-correctness`: one Failed, two Unobserved. **D1 RED**; the two are listed.
3. `output-conformance`: one Failed, expected; one Unobserved. **D4 HARNESS.** Under the spec, XFAIL.
4. `bounding` unstated, its one check `held`. **D3 HARNESS**, `placeholder_unfilled`. With that check `not_held`: **D1 RED**.
5. `discoverability` with no check. **D8 UNCOVERED**; the run is V2, exit 4.
6. Scope `conformance`; every in-scope dimension GREEN or N/A. **V4 PASSED**, exit 0.
7. Scope `conformance`; one dimension XFAIL, one HARNESS, none RED. **V2**, exit 4; the XFAIL dimension is listed.
8. A run in which no in-scope dimension is GREEN, XPASS or XFAIL and none is HARNESS, UNCOVERED or RED would have every dimension N/A. No scope allows that, since most dimensions have no deciding fact; V2's middle clause guards it all the same.
9. A prompt with 40 scored trials, 37 held: `L` is `0.801358`. **P3.** With 36 held: **P5.**
10. A prompt with a declared count of 40 and 38 scored trials. **P1**, two to run.
11. Scope `steering+compare`; arm A 8 of 16 on its one prompt, arm B 16 of 16. The prompt check is judged on arm B: **P3**, `steering` GREEN. The comparison: `d` is 0.5, the interval is 0.207 to 0.720, **improved**.
12. Baseline of twelve plain checks, all `held`; the same twelve Held now. **R3.**
13. The same baseline; nine are in the required set and Held, three are absent. Three E1. **R2**; V2, exit 4.
14. As case 13, and one of the nine is Failed. **R1**; V1, exit 1; the three are listed.
15. Baseline under an older rulebook version. Every entry E2. **R2.** An anchor request with the same evidence and a PASSED own verdict: **N7**, exit 0.
16. Baseline holds conformance and steering entries; the request is a default grade. Steering entries are Set aside. Conformance all E5. **R3.**
17. Steering baseline, ten prompts at 38 of 40; one tool description changed, and each prompt is now 31 of 40. Each is P5, so none is E6, E8 or E10; all join the changed set. Totals 380 and 310 of 400: the interval is −0.222 to −0.129. **Regressed**, R1, V1.
18. The same baseline; current totals 372 of 400. The interval is −0.054 to 0.013. **Same**, R3.

## 17. Settled by the corpus

Four case writers, working from draft 4 alone, filed eighteen inputs the text could be read two ways on. Each is settled here; the rule that now decides it is named. The table has sixteen rows because two rows each settle two inputs.

| Input | Settled as | Rule |
|---|---|---|
| An empty list of scope parts | Rejected | X1 |
| An earlier pool whose indices do not start at zero | Accepted; the pool is listed with rule P1 and no counts | §4a W3 |
| A wrong-tier trial stating another declared count | Accepted and set aside | X6 note |
| A wrong-tier trial decided by a model | Accepted and set aside | X6 note |
| An earlier-pool trial with an impossible outcome | Accepted; the pool is listed with rule P1 and no counts | §4a W3 |
| An earlier pool whose trials state a count below 16 | Accepted; X7 is about the required set | X7 |
| A second condition supplied on a prompt check outside a compare scope | Ignored | no rule reads it |
| Trials listed out of index order | Accepted; input order carries no meaning | How to read the rules |
| Two checks sharing an id, each with evidence | Rejected by X3, which is tested before X10 | contract §3 |
| Errored trials after the last scored trial | Not counted by P0 | T3, P0 |
| "Still to run" | The declared count less the scored trials | P1 |
| Sampling values with keys in another order | The same | K2 |
| The checks a refusal names for a stale marker or an unstated fact | None | §12 |
| A baseline entry whose check is now routed to another dimension, or is now of another kind | Set aside by B2 when the entry's own dimension is out of scope; otherwise Missing when E1 or E6 says so and Not compared when not. In every case the check is not New | B2, E1, E2, §11 |
| A plain check routed to `steering` | Rejected | X10 |
| The same marker given twice | One marker | How to read the rules |

The writers also showed three rules cannot fire: draft 4's P2, because P0 decides every case it could; G6; and the middle clause of V2 on its own. P2 is withdrawn. G6 and the clause are kept so that every list is complete.

## 17a. Settled by review

Three reviews of draft 5 found the inputs below. None changes a result the corpus already fixed unless the last column says so.

| Input | Settled as | Rule |
|---|---|---|
| A scope that names a part twice | Rejected | X1 |
| Two checks sharing an id, where a rule tested before X10 depends on which is meant | The name means each of them; X2 to X5 apply when they apply for any; each check then has its own pools and X6 to X9 are tested for each | Two checks with one id |
| A trial of a prompt check that is out of scope | Tested and set aside like any other; its earlier pools are listed | How to read the rules |
| A pool with no kept trial, whose wrong-tier trials have a gap in their indices | Rejected | X6 |
| A trial stating a count where its check has none | Not X6; X8 rejects the request when the check is in scope | X6, X8 |
| A baseline prompt entry with no counts, no trials, or more held than run | Rejected | X11 |
| A baseline entry of kind `plain` with no counts, whose check is now a prompt check | Accepted; X11 goes by the entry's own kind | X11 |
| An earlier pool stating a count of zero, or with two trials at one index | Listed with rule P1 and no counts | §4a W3 |
| An Unobserved check on an excluded dimension | The dimension is N/A | §6 |
| An unused marker on a dimension another marker made XFAIL | Dormant; not listed | §6 |
| Sampling values `0` and `0.0` | The same | K2 |
| A prompt that is P5 and whose baseline entry is `held` | Same, Improved, Regressed or Not compared by E10 or E11; the run is not green because `steering` is HARNESS | §11 |
| A baseline entry whose check changed kind or dimension and is now Unobserved | Missing, except a prompt entry whose check is still a prompt at P5, which is Not compared | E1 before E2; E6 before E7 |
| Anchoring when an out-of-scope entry's check is now in scope | The old entry is dropped and recorded; the id appears once | §12 |
| Anchoring when an in-scope entry's check is now out of scope | The entry has no successor and is recorded | §12 |

## 17b. Settled after six trial implementations

Six independent judges were built from the sealed draft and reviewed. The reviews found inputs the sealed text left open or decided two ways. Version 1.0 settles each.

| Input | Settled as | Rule |
|---|---|---|
| A prompt baseline entry whose check is now a plain check and Unobserved | Missing | E6 |
| An object that repeats a key | Rejected | X12, contract §3 |
| A string holding an escape for an unpaired surrogate | Rejected | X12, contract §3 |
| A document nested deeper than 64 levels | Rejected | X12, contract §3 |
| An integer field above 9007199254740991 | Rejected | X12, contract §3 |
| A number written with more than 32 characters, or with an exponent of more than four digits | Rejected | X12, contract §3 |
| An input larger than 16 MiB | Rejected | X12, contract §3 |
| Sampling values that differ as decimals but round to one floating-point number | Different | K2 |
| An integer written `-0` | The integer zero | contract §3 |
| Integers in the output | Written as integer literals | contract §5 |
| Baseline prompt entries whose `n` sum past 9007199254740991 | Rejected | X11 |
| A prompt baseline entry, on another dimension, whose prompt is now P5 | Not compared | E6, then E7 |
| Two prompt checks sharing an id with different conditions | Each keeps the trials under its own condition; the other's are earlier pools for it | Two checks with one id |
| Two comparison checks in a scope without `compare` | Rejected | X10 |

## 18. Open

- **O1.** The 95% level, the error cap of 0.20 and the margin of 0.10 are choices made in this file. The margin is the one the current engine's arm comparison uses; the other two have no precedent. They are fixed for this version.
- **O2.** K4 and E11 sum over prompts (§9.5). A per-prompt comparison would not hide offsetting changes and would need far more trials.
- **O3.** A retired check can be named only when anchoring, so a check removed on purpose makes every grade not-measured until the baseline is re-anchored.
- **O4.** A prompt whose true rate is 60% is still not-measured about one time in eight at 40 trials, and about half the time at 16.
