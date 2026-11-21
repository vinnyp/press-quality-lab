# Evidence record — draft 5


This is the shape every family writes and the only thing the judge reads about what happened. It has three parts: what must be checked, fixed before collection (R21); what was seen for a plain check (R1, R3); and what was seen in one steering trial.

## Part 1 — a required check (`required[]`)

| Field | Meaning |
|---|---|
| `id` | Stable identity of the check. Matches evidence and trials to it, and matches a baseline across runs (R25). |
| `kind` | `plain`, `prompt` or `comparison`. |
| `dimension` | The dimension the rulebook routes the check to. |
| `record` | The authored record a plain check belongs to, or `null`. Expected-failure markers act per record and dimension. |
| `crediting` | Whether a held result can make the dimension green. |
| `empty_satisfiable` | True when an empty target would satisfy the predicate, so a target size is needed. |
| `asserts_emptiness` | True when the check asserts that something is empty, so an observed precondition is needed. Never true together with `empty_satisfiable`. |
| `precondition` | For a check that asserts emptiness, what must be separately observed. Otherwise `null`. |
| `declared_count` | For a prompt check, the number of scored trials its pool needs: at least 16, and 40 unless the subject file says otherwise. Otherwise `null`. |
| `condition` | For a prompt check, its current condition for arm `B`, or for the pool with no arm. Otherwise `null`. |
| `condition_a` | For a prompt check in a compare scope, its current condition for arm `A`. Otherwise `null`. |

How a check came to exist (authored, generated, or by trial, R2) is not a field. The judge does not need it.

## Part 2 — evidence for a plain check (`evidence[]`)

| Field | Meaning |
|---|---|
| `check_id` | The required check this is evidence for. |
| `outcome` | `held`, `not_held` or `not_observed` (R1). |
| `reason` | With `not_observed`, a closed code for why; otherwise `null`. |
| `failure_kind` | With `not_held`, a closed code for the subject-side failure: `predicate`, `wrong_exit`, `crashed`, `over_budget` (R22). Otherwise `null`. |
| `target_size` | For an empty-satisfiable check, how large the target was. Zero or `null` on a `held` outcome is judged Unobserved (rulebook C3). |
| `precondition_observed` | For a check that asserts emptiness, whether the precondition was seen to hold. |
| `expected`, `observed` | What the check expected and saw, as any JSON value. Carried for the scorecard; the judge decides nothing by them. |
| `targets` | What was evaluated: one or more sources, each with `kind` (`invocation` or `artifact`), `id`, and its own `tier` (`offline`, `live`, `replay` or `scripted`). |
| `tier` | Derived by the engine from `targets`: the tier they share, or `mixed`. This is the tier the result is stored and compared under. |
| `evaluator` | Who decided the outcome: `kind` (`deterministic` or `model`) and, for a model, its identity in `model`. |

## Part 3 — one steering trial (`trials[]`)

A trial is not a piece of evidence with extra fields. It has its own, smaller shape.

| Field | Meaning |
|---|---|
| `check_id` | The prompt check the trial belongs to. |
| `arm` | `A`, `B`, or `null` when two versions are not being compared. |
| `condition` | Digest of the subject's surface and tool descriptions, the model and its sampling settings, the prompt and framing, the declared count, and the rulebook version. |
| `trial_index` | Position within the pool, from zero with no gaps. |
| `declared_count` | The count declared for the trial's pool. |
| `outcome` | `held` or `not_held`. A trial is never `not_observed`. |
| `errored` | Whether the harness failed on this trial, with the failure evidenced on the harness side. An errored trial is not scored. |
| `tier` | `live`, `scripted`, `replay` or `offline`. |
| `model` | Identity of the agent under test. |
| `sampling` | Its sampling settings, or `null` where the harness does not expose them. |
| `framing` | Digest of the fixed framing sent with the prompt. |
| `evaluator` | As in Part 2. A request holding a trial decided by a model is rejected in this version (rulebook X9), unless the trial was set aside first. |
| `grade` | Optional closed code (`full`, `partial`, `none`). No rule reads it in this version. |

## What the record deliberately does not carry

- **No severity.** There is no "warn". A check is in the required set or it is not.
- **No verdict of any kind.** Not a dimension state, not a pass count, not a comparison.
- **No free text the judge reads.** Reasons and failure kinds are closed codes.
- **No requiredness.** Evidence cannot say a check was optional.
