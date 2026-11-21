# The judge contract


The rulebook says what the judge decides. This file says how it is called and what, exactly, goes in and comes out, so that one scoring harness can run every build and compare outputs as JSON values.

## 1. Calling the judge

- The judge is one program. It takes no arguments.
- It reads one JSON document from standard input, encoded as UTF-8: `schema/judge-input.schema.json`.
- It writes one JSON document to standard output: `schema/judge-output.schema.json`. It writes nothing else there.
- Its exit code is the one in the rulebook's §8 table, and equals the output's `exit_code`.
- Anything on standard error is ignored.
- It reads no file, no environment variable and no clock, and makes no network call. The same input always gives the same output.
- **Running time (informative).** Conformance sets no time limit. Acceptance does: see "How a build is scored" in `corpus/README.md`. A judge whose time grows with the square of the number of checks, trials, markers or baseline entries will fail the large cases.

## 2. Input

The schema is the definition of the document's shape. §3 adds limits on the input text that a schema cannot express, and one the schema states less strictly (how an integer is spelled). An input is acceptable only if it passes both. `-0` in an integer field is the integer zero. In rulebook terms: `request` is J1, `scope` J2, `required` J3, `evidence` J4, `trials` J5, `subject_facts` J6, `markers` J7, `claimed_tier` J8, and `baseline`, `retired` and `accepted_regressions` J9.

- `scope` is the list of parts requested, in any order. Whether the set is legal, including a part named twice, is rule X1.
- A subject fact that is unstated is `null`. `bounding` is `"none"`, `"declared"` or `null`.
- A required check carries `declared_count`, `condition` and `condition_a` only when it is a prompt check; otherwise they are `null` (X5). `condition` is the current condition for arm `B`, or for the pool with no arm. `condition_a` is the current condition for arm `A`; on a prompt check outside a compare scope no rule reads it, whatever string or `null` it holds.
- A field has a value when it is not `null`. An empty string is a value.
- A trial names its prompt check in `check_id`. There are no trials for a comparison check; the judge computes that check from the prompt checks' trials.

**Fields the judge carries no rule for.** They must still match the input schema and §3. Once the input has passed X12, no rule reads them and they must not change any result: `expected`, `observed`, `failure_kind`, `targets`, `tier` and `evaluator` on a piece of evidence, except that `tier` is the stored tier of a plain check; `precondition` on a required check, beyond X5's test that it is present; and `grade` on a trial. On a prompt or comparison check, `record` is read only by X4, and `empty_satisfiable`, `asserts_emptiness` and `precondition` only by X5. On a baseline entry whose kind is not `prompt`, `condition`, `k` and `n` are not read by any rule; they are kept as given when the entry is carried into `anchor.baseline` or `anchor.replaced`.

## 3. Rejected requests

- The request is rejected by **X12**, which is tested first, when the input is not an acceptable document. An acceptable document is valid JSON that matches the input schema and stays inside every limit below. X12 rejects nothing else, and no other rule is applied by way of the schema.
  - **Not valid JSON:** the whole input must match the `JSON-text` grammar of RFC 8259 and nothing more lenient: no `NaN` or `Infinity`, no comments, no trailing commas, no leading `+` or leading zeros on a number, no unescaped character below U+0020 inside a string. Whitespace is only space, tab, line feed and carriage return. The input must be well-formed UTF-8 as RFC 3629 defines it: no encoded surrogate (bytes `ED A0 80` to `ED BF BF`), no overlong form, nothing above U+10FFFF. Also not valid: empty or whitespace-only input; anything but whitespace after the document; a leading byte-order mark.
  - **Size:** the input is larger than 16 MiB (16,777,216 bytes), counting every byte read from standard input to end of file, whitespace included.
  - **Depth:** an array or object is nested more than 64 levels deep, counting the outermost value as level 1. Only arrays and objects are levels: a number or string inside a level-64 array is accepted, and an empty array at level 65 is rejected. The judge must reject such input, not fail on it, whatever its size.
  - **Repeated keys:** any object, at any depth, has the same key twice. Keys are compared after their escapes are decoded, as sequences of code points, with no Unicode normalisation.
  - **Ill-formed strings:** any string or key holds a `\u` escape for a surrogate code point that is not one half of a well-formed pair. A well-formed pair is a `\u` escape in D800 to DBFF immediately followed by a `\u` escape in DC00 to DFFF.
  - **Integers:** where the schema asks for an integer, the number is written with a fraction part or an exponent, such as `16.0` or `1e1`; or it is below 0 or above 9007199254740991. `-0` is the integer zero and is accepted.
  - **Number form:** any number, anywhere in the document, is written with more than 32 characters, counting every character of the number, its sign, point, `e` and exponent sign included (`-` and 31 digits is accepted; `-` and 32 digits is not); or has an exponent of more than four digits (`1e9999` is accepted, `1e10000` and `1e-10000` are not; only digits are counted, so a sign is not one of the four and `1e-9999` is accepted; leading zeros are digits, so `1e00001` is rejected). Outside integer fields, a number inside these limits is accepted whatever its magnitude: `1e9999` is far beyond a double and is accepted (rulebook K2 says how sampling numbers are compared).
- The causes of an X12 rejection are not ordered and not reported. A judge may stop at the first it finds and need not read the rest of the input.
- When the input is acceptable, X1 to X5 are tested in number order; then the trials are set aside by W1 to W3; then X6 to X11 are tested in number order. The first rule that applies is reported. Two checks that share an id are therefore reported under X10 only when no earlier rule applies; the rulebook says how the earlier rules read such a request.
- For every input, acceptable or not, the judge writes one output document and exits with a code from the rulebook's §8 table. It never ends without a document.
- A rejection's output has `outcome` `"rejected"`, `rejected_by` set, `exit_code` `2`, `rulebook_version` set, `scope` `null`, every other array empty and every other field `null`. `verdict`, `own_verdict`, `counts`, `comparison` and `anchor` are each `null` as a whole, not an object of nulls.

## 4. Output

One document for every request. For a rejected request the last bullet of §3 overrides the table below: every list is empty, including `set_aside` and `earlier_pools`.

| Field | Content |
|---|---|
| `rulebook_version` | The judge's rulebook version, `1.0`. |
| `outcome` | `verdict` for a grade; `anchored` or `refused` for an anchor request; `rejected`. |
| `rejected_by` | The X rule for a rejection; otherwise `null`. |
| `exit_code` | The rulebook's §8 table: for a grade, the verdict's code; `0` for `anchored`; `5` for `refused`; `2` for `rejected`. |
| `scope` | The requested parts, in the order `conformance`, `destructive`, `steering`, `compare`. |
| `verdict` | For a grade: the run verdict of §8, with the comparison where one applies. `null` for an anchor request. |
| `own_verdict` | For an anchor request: the verdict with no comparison. `null` for a grade. |
| `counts`, `stale_markers` | The XFAIL and XPASS dimension counts, and each stale marker as its record and dimension. For an anchor request these come from the own verdict. |
| `checks` | One entry for every check in the required set. |
| `dimensions` | One entry for each of the 21 dimensions: its state, the deciding rule, and a reason where the rule gives one (D0 and D3). |
| `groups` | One entry for each of the six groups. |
| `comparison` | For a grade, always present. `null` for an anchor request. |
| `set_aside` | One entry for every trial removed by W1, W2 or W3, with the reason. Two trials that give equal entries give two entries. |
| `earlier_pools` | Each earlier pool with its counts and the P rule it gets. `k` and `n` are `null` whenever §4a gives the pool rule P1 for a malformed pool: more than one declared count, indices not `0, 1, 2 …`, an impossible outcome, or a model evaluator, a stated count of zero, or two trials at one index. |
| `anchor` | For an anchor request, always present. `null` for a grade. |

The names of the fields inside each of these are the schema's, and the schema's enums are the closed lists of reasons, sub-labels and states.

**A check entry.**

- An out-of-scope check has `result` `out_of_scope` and every other field `null` except `id`. If it is a prompt check, its trials are set aside and listed like any other check's, and its earlier pools are listed.
- A plain check has its result, the C rule, and a reason when Unobserved. Every other field is `null`.
- A prompt check has its result, the P rule, a reason when Unobserved, and `k` and `n`, which are integers under every P rule, P0 included. `detail`, `d_lo` and `d_hi` are `null`. `lower` and `upper` are given when `n` is above zero, whatever the rule. `to_run` is given only under P1 and is the declared count less `n`.
- A comparison check has its result, the K rule and a reason when Unobserved. `d_lo`, `d_hi` and `detail` are given only under K4; `detail` is `regressed`, `improved` or `no_material_change`, and is `null` when K4's result is Unobserved. `k`, `n`, `lower`, `upper` and `to_run` are `null`.

**The comparison block.**

- Under B1, `result` is `none`, `rule` is `B1`, `next_action` is `null`, `new` is empty, and `entries` lists every baseline entry as `set_aside` with rule `B2`; it is empty when there is no baseline.
- Otherwise `result` and `rule` come from R1 to R3; `next_action` is `re_anchor` under R2 and `null` under R1 and R3; `entries` lists every baseline entry with its class and rule; `new` lists the in-scope checks with no entry.
- For a prompt entry classed through E11, the rule is `E11`.

**The anchor block.** `result` is `refused` under N1 to N6 and `anchored` under N7. `rule` is N1 to N7. Under a refusal, `reason` is set, `baseline` is `null` and `replaced` is empty; `checks` names the checks that caused it under N5 and N6 and is empty under N1 to N4. Under N7, `reason` is `null`, `checks` is empty, `baseline` is the new baseline in the input's baseline shape with its entries ordered by `check_id`. A new entry holds the check's id, kind and dimension; `held` or `failed` as its result, `failed` for a check that failed as expected; the judge's rulebook version; the check's stored tier; and, for a prompt check, its current condition and the `k` and `n` of the pool it was judged on, otherwise `null` for all three. The new baseline holds one new entry for each in-scope check whose result is Held or Failed (rulebook §10 and §12); an in-scope check that is Unobserved gets none. It carries over every out-of-scope entry of the replaced baseline, except any whose id a new entry carries. An entry carried over from the replaced baseline is unchanged as a JSON value. `replaced` holds, in the same order, the replaced in-scope entries and any out-of-scope entry dropped because a new entry carries its id.

**Wire spellings.**

| Rulebook | Output |
|---|---|
| Unobserved; Failed; Failed, expected; Held, crediting; Held, non-crediting; Out of scope | `unobserved`, `failed`, `failed_expected`, `held_crediting`, `held_non_crediting`, `out_of_scope` |
| Set aside, Missing, Not compared, Regressed, Improved, Same | `set_aside`, `missing`, `not_compared`, `regressed`, `improved`, `same` |
| No comparison, Regressed, Not-measured, Not regressed | `none`, `regressed`, `not_measured`, `not_regressed` |
| green, red, known defect, not-measured | `green`, `red`, `known_defect`, `not_measured` |
| Anchored, Refused | `anchored`, `refused` |
| Dimension and group states, verdict tokens, sub-labels, reasons | As written in the rulebook |

The "passed over" list of the rulebook's §8 is not a separate field: it is every check whose result is `unobserved`, every dimension whose state is HARNESS, UNCOVERED or XFAIL, and every entry classed `missing` or `not_compared`.

## 5. Exact form

- **Order.** `checks` by `id`; `comparison.entries`, `anchor.baseline.entries` and `anchor.replaced` by `check_id`; `comparison.new` and `anchor.checks` ascending. All of these are ascending Unicode code-point order; conditions and records compare the same way, and trial indices compare as numbers, ascending. `dimensions` and `groups` in the order of the rulebook's §1 table. `stale_markers` by dimension in that table's order, then by record. `set_aside` by `check_id`, then arm in the order `null`, `A`, `B`, then condition, then trial index, then reason in the order `wrong_tier`, `other_arm`, `earlier_pool`. Entries that still tie are equal. (Two trials that tie this far share an arm, so only `wrong_tier` can tie with another reason; the full order is given for completeness.) `earlier_pools` by `check_id`, then arm in the same order, then condition.
- **Bounds.** `lower`, `upper`, `d_lo` and `d_hi` are strings with exactly six digits after the point: the IEEE double's exact value, rounded to six places with a tie going to the even last digit, which is what a C `%.6f` conversion gives; a formatter that rounds ties another way does not conform. There are no leading zeros before the units digit. A value that prints as negative zero is written `"0.000000"`. The rulebook's comparisons against `0.80` and `0.10` use the rounded value.
- **Rule ids** are written as in the rulebook: `C7`, `D4`, `E9a`, `P0`. `P2` is never written.
- **Input order.** The order of the input's lists carries no meaning, and `stale_markers` lists each marker once.
- **Absent values** are `null`, never omitted. Every field in the schema is always present.
- **Integers.** Every integer in the output (`exit_code`, `counts.xfail`, `counts.xpass`, `trial_index`, `to_run`, and `k` and `n` wherever they appear: the fields the output schema types as an integer) is written as an integer literal, with no fraction part and no exponent, and never as `-0`: `2`, never `2.0`. This is part of conformance: an output that breaks it is wrong although it is equal as a JSON value.

Two conforming outputs are the same when they are equal as JSON values. Key order and whitespace do not matter.

## 6. Examples

- `examples/case-01`: the rulebook's worked case 1 as a whole run. A default grade with three checks on `execution`, one held and two unobserved through a harness fault. The run is PROVISIONAL, `gaps`, exit 4.
- `examples/case-02`: the same input with the scope `destructive` alone, which is not legal. Rejected by X1, exit 2.
