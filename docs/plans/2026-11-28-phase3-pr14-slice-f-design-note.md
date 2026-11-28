# Design note: PR14, slice f (the scorecard and the agreement check)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR14. **Unit:** U18.

This note follows the slice d note, `docs/plans/2026-11-27-phase3-pr13-slice-d-design-note.md`. Its definitions and controls hold here. This note says only what U18 changes. Its design review is logged in `docs/agent-reviews/2026-11-28-phase3-pr14-design-note-reviews.md`.

## What the slice makes true

### Where the scorecard goes (owner ruling, 2026-11-28)

Neither sealed specification says where the scorecard goes or what form it takes. The plan says only that `pql grade` "prints a scorecard". The owner ruled:
- **By default,** `pql grade` writes the scorecard, as plain text, to standard output.
- **`pql grade --json`** writes the judge's output document to standard output instead, as `pql grade` does today: one JSON line, escaped by `terminalJSON`.
- **The stored record** is the same either way.

Standard error is unchanged. It still carries QA5's warning, the rejection line (GKD8) and every other error.

The flag changes only what is printed. It changes nothing the engine decides, stores or exits with, and a test runs the same subject both ways and compares the two stored records byte for byte, and the two exit codes. `TestJudgeAuthority` and `TestStoreFailureOutput` keep their assertions under `--json`, and each gains a twin without the flag that checks the scorecard.

The README's "writes a JSON verdict on stdout" becomes a description of both modes.

### What the scorecard reads

**Every verdict, state, count, reason and exit code comes only from the stored record** (QE14, GR3). The scorecard renders from the record document as `internal/grade` serialises it for storage. If the store write fails after the judge decided, the same document is rendered, with E-4's note. Nothing in `internal/scorecard` calls the judge, re-derives a state, or maps an exit code to a word. The plain word is the judge output's `verdict.plain`, printed as the judge wrote it.

**The run's details sit beside the record** (QE14: "The scorecard and the stored record add the evidence and the run's details the engine recorded"). The stored record holds no field for some of what the scorecard must name. It has nothing for a check's command words, discovery's names, a placeholder's location, a computation's standard error, or a declared value too short to mask. Its top level has `additionalProperties: false`.

This note does not add them to the judge input's `observed`, which would change the bytes the judge reads. Instead `internal/grade` builds a `scorecard.Details` value from the plan, the parsed subject and the collected checks. It passes `Details` with the record document to the renderer. `Details` holds names and text only, never a state, an outcome or a reason. A type test enforces this: `scorecard.Details` has no field of a judge or `decide` outcome type, and none named `state`, `outcome`, `reason` or `verdict`.

This amends KTD16's "The scorecard renders from the stored record." It now reads: the scorecard renders its verdicts from the stored record and its names from the run's details. The plan note gets the amendment with this PR.

**The stored record's interim fields are filled** (KTD16, "Interim fields"):
- **`next_actions`:**
  - One entry for each check the judge output reports with a reason in QE12's table, as `{check_id, dimension: null, reason, action}`.
  - One entry for each dimension whose reason is `placeholder_unfilled` or `scope_not_requested`, as `{check_id: null, dimension, reason, action}`.
  - The action is the row's text from `record.NextActions()`, verbatim.
  - A reason outside QE12's table gets no entry: `live_not_run` and the judge's trial reasons. Every corpus case's `dimensions` carry `live_not_run`, so this is the common case. It is PR13's reading: QE12's table lists the reasons this specification, the collection, reports. The scorecard shows such a dimension's reason as the judge wrote it, with no next action.
  - Entries follow the judge output's order: checks first, as the judge lists them, then dimensions.
- **`offline_settings`:** each setting the subject file declares, in file order, as `{applies_to, args, env_names}`. `applies_to` is the declared word lists (QS7: "Each setting lists the words of the commands it applies to"). `env_names` holds the names, never the values.
- **`bookkeeping_excluded`:** the declared bookkeeping paths, in file order.

QE19 leaves these three fields out of the comparison, and no case compares them (KTD16). Each gets unit rows, with a positive fixture and an empty twin.

### What the scorecard shows

The scorecard is plain text, in this order. Every value from the subject, the subject file or a stream passes `grade.Escape` first. Masking runs before escaping, because the stream is already wrapped by `record.Masker`.

1. **The run line:** `pql grade: <verdict.plain> (exit <exit_code>)`. With `verdict.sub_label` non-null, the label follows in parentheses. An anchor shows the plain word the judge wrote, never `green` (QE21, judge errata E-4 item 1). A fixture whose judge output is an anchored exit 0 shows `anchored`, and its twin, a non-anchor exit 0, shows `green`.
2. **The scope.**
3. **Each dimension:** its name, its state and its reason, as the judge wrote them. Then its next action when `next_actions` holds one. Then QE18's note, QS7's settings and QP7's paths where they belong, as items 7 to 9 say.
   - Wherever the scorecard shows `tool-description-efficacy` as green, it shows the comparison's word beside it, `improved` or `no_material_change` (judge errata E-4 item 2). The conformance corpus never reaches this dimension, so a fixture proves it.
   - OUT-OF-SCOPE shows that plain word and its reason, and enters no count (rulebook §8).
4. **Each check that is not held,** grouped under its dimension, and only those.
   - Its result and reason, as the judge wrote them. Then its next action.
   - Its name from `Details`:
     - the command words, or the runbook and step, for a subject invocation;
     - the argument list, for a computation;
     - `discovery`, or the probe's name.
   - Its full id, so that the defect's check is named (AE8).
   - A held check is counted, not listed. The counts are the judge output's `counts`, printed as the judge wrote them.
5. **Discovery** (QD1 to QD4):
   - the found count against the declared count;
   - the names found and the declared names missed (QD1, QD4);
   - each command found but not declared (QD3, AE24: "naming the command to classify");
   - the bound QD2 reached, if one was.

   The names come from `decide.Discovery`. `collect.Check` gains a `Discovery` field, which only the discovery check fills. The undeclared names are shown whenever discovery found any, including when QD4 decides. The scorecard then shows a superset of what QD3 names, and no rule forbids that.

   The 512-name and 64 KiB preview caps stay, and the scorecard says when it cut the list.
6. **Placeholders** (QS5, AE20): every placeholder in the file, each by its location as an RFC 6901 JSON Pointer into the subject file, such as `/binary`, `/facts/store_backed` or `/commands/1/class`. QS5 names eight places a placeholder may stand, and E-22 rejects one anywhere else, so the walk visits only those eight.
7. **Offline settings** (QS7): each declared setting and the commands it applies to, its arguments, and its environment names. Values are never printed. A declared value is masked anyway (QE18).
8. **Bookkeeping exclusions** (QP7): the declared bookkeeping paths, then `.pql-tmp`, which QP7 also leaves out of the comparison.
9. **QE18's note:** each environment name whose declared value is shorter than 8 bytes, with the words "not masked: shorter than 8 bytes". The value is never printed.
10. **A computation's standard error** (QA4, AE23), under the check it served, for a check that is not observed with `computation_failed`. It is cut to 4 KiB, and the scorecard says when it cut it.
11. **Run notes:**
    - E-4's "the record was not stored" when the store write failed after the judge decided;
    - E-11's directory left behind when cleanup failed. It moves from today's line at `grade.go:213` into the scorecard. Under `--json` it stays on standard error, so standard output stays one JSON document.

**A rejected record** (QE15): no verdict is shown. Standard output stays empty in both modes, as it is today, and the rejection line goes to standard error.

**A derivation-only rejection** (exit 2, GKD8): no scorecard. Standard output is empty.

### The agreement check (AE21, GR3)

`internal/corpusrunner` reads standard output, which it discards today. For every case off the pending list it runs `pql grade` once without the flag, and checks the following.

For a judge-checked case, these are all equal:
- the process exit;
- `expect.exit_code`;
- the stored record's `exit_code`;
- the stored `judge_output.exit_code`.

The scorecard's run line is then parsed and must state the stored `verdict.plain` and the same exit code. The test's own literal table, taken from the judge rulebook's §8 plain-word table, checks that the plain word is the one the exit code maps to. That table is written in the test from the rulebook text, never read from engine code.

For a derivation-only case, the process exit equals `expect.exit_code`, the rejection line's rule equals `expect.rule`, standard output is empty, and no record is stored.

A second pass runs each judge-checked case with `--json`. Its standard output must parse to a document equal to the stored `judge_output`.

The check's negative rows each break one leg of the agreement and assert which leg failed, by name (`CODING_STANDARDS.md`, negative rows):
- a doctored run line;
- a stored `exit_code` changed by one;
- a `--json` document with one field changed.

### Escaping

KTD16's one escaping function stays `grade.Escape`, with `terminalUnsafe` as its predicate. `--json` keeps `terminalJSON`, which escapes the same class set in JSON's own spelling, so the line stays one valid JSON document. Both use `terminalUnsafe`. `terminalJSON` passes code points below U+0020 through only because `strictjson.Write` has already escaped them inside strings. A test goes through every code point from U+0000 to U+10FFFF and checks two things:
- `Escape` leaves no raw code point the predicate names;
- a JSON string holding the code point, written by `strictjson.Write` and then passed through `terminalJSON`, holds no such raw code point either, and still parses back to the same string.

The scorecard has one fixture for each class KTD16 lists. Each puts a member of the class into a subject value the scorecard prints: a command word, an offline argument, a bookkeeping path or a placeholder's sentence. The output must hold the escaped spelling and no raw member, and each fixture has a twin with a plain character in the same place.

### The README

The README's stale lines are replaced:
- "The judge is not yet implemented";
- "writes a JSON verdict on stdout".

It gains:
- both output modes;
- QE21: an anchor's exit 0 is not a green run (judge errata E-4 item 1);
- the 16-trial floor and the default of 40 (judge errata E-4 item 3);
- E-13's and E-14's stated limits;
- QD6's limit: a command that ignores a help request runs its action inside full confinement in an empty home;
- QA5's warning, unchanged.

A test reads the README and fails if any of these is missing, each by a phrase the test names. It also fails if the README says "green" within the same sentence as "anchor" without "not".

## Readings the build settled from the text

None of these is a two-way reading of the sealed text. Each is a gap the text leaves open. Each was grepped across both sealed folders, both errata notes and `docs/requirements.md` for: `scorecard`, `show`, `display`, `print`, `standard output`, `location`, `applied`, `left out`, `both counts`, `says so`.
- **QS5's "location":** a JSON Pointer, as above.
- **QS7's "the commands it applied to":** the declared `applies_to`, which is what the record schema stores.
- **QP7's "the paths that were left out":** the declared bookkeeping paths, and `.pql-tmp`. The paths are listed whether or not the probe ran ("Every scorecard").
- **QD3 against QD4:** undeclared names shown whenever found.
- **QE18's "the scorecard says so":** the name, never the value.
- **QE12's coverage:** no `next_actions` entry for a reason outside QE12's table.
- **Where the scorecard goes:** settled by the owner, above.

## Predicted ratchet set

Unchanged at 142. U18 adds no case to the passing set: the agreement check is a new assertion on the same 142, and the pending list stays at 10. `scripts/ratchet/pr14.json` equals `pr13.json`, and its prediction file is regenerated and must be byte-identical to PR13's.

## Rules this slice moves off KTD22's list

Each move cites a proof test whose first body comment line names the rule. Each records its mutation, the change that breaks the rule and the test that fails:

| Rule | Proof | Mutation, and the test that fails |
|---|---|---|
| QD1, QD3, QD4 | scorecard fixtures from `case-020`, `case-022` and `case-091`'s shapes | drop the missing names; drop the undeclared names; show only one count |
| QD2 | a fixture with each bound reached, and its twin with none | never print the bound |
| QD6 | README test (`limit` kind) | delete the README line |
| QE12 | the `next_actions` rows: a check reason, a dimension reason, and a reason outside the table with no entry | attach an action to `live_not_run`; drop the dimension entry |
| QE14 | the agreement check, plus the `Details` type test | take the plain word from the exit code; give `Details` a `state` field |
| QE21 | the anchored fixture and its twin, and the README test | print `green` for exit 0 |
| QP7 | `bookkeeping_excluded` rows and the scorecard fixture | drop `.pql-tmp`; drop a declared path |
| QS5 | placeholder-location fixtures, one per place | drop `/commands/i/class` |
| QS7 | `offline_settings` rows and the scorecard fixture | store a value instead of a name; drop a setting |

**Stay listed:** QE18. It needs its `macos_evidence` entry and the darwin path (PR15, GR15). U18 builds its scorecard note, with a fixture and a mutation, "print the short value".

## Controls

Control 2 (the full QE19 comparison) and control 3 (the case-derived ordered trace) hold unchanged, because U18 changes no judge input byte. The new control is the agreement check, with its negative rows. A test also runs one case with and without `--json` and asserts the two stored records are byte-identical.

## Stated limits of this slice

- **The scorecard's text is not a stable interface.** Scripts and agents use `--json`. The README says so.
- **The first private scoring round,** on 2026-12-23, is listed in U18's verification. The owner ruled on 2026-11-28 that PR14 merges before it. It scores the stored record and the exit code, not the scorecard. Its counts are reported to the owner as counts only, when it runs.
- **Judge errata E-4 item 2** (`tool-description-efficacy`) is not reachable in conformance. Only a fixture proves it.
