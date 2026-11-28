# Design note: PR14, slice f (the scorecard and the agreement check)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR14. **Unit:** U18.

This note follows the slice d note, `docs/plans/2026-11-27-phase3-pr13-slice-d-design-note.md`. Its definitions and controls hold here. This note says only what U18 changes. Its design review is logged in `docs/agent-reviews/2026-11-28-phase3-pr14-design-note-reviews.md`.

## What the slice makes true

### Where the scorecard goes (owner ruling, 2026-11-28)

Neither sealed specification says where the scorecard goes or what form it takes. The plan says only that `pql grade` "prints a scorecard". The owner ruled:
- **By default,** `pql grade` and `pql anchor` write the scorecard, as plain text, to standard output.
- **With `--json`,** each writes the judge's output document to standard output instead, as today: one JSON line, escaped by `terminalJSON`.
- **The stored record** and the exit code are the same in both modes.

Standard error still carries QA5's warning, the rejection line (GKD8) and every other error. Each command's `--help` describes `--json`, and says the text output is not a stable interface.

**The flag changes only what is printed.** A test runs one subject in both modes, each against its own store, because a record's file name is its own hash. It compares the two stored records byte for byte, and the two exit codes.

**Existing tests that read standard output.** Every one of them gets `--json` and keeps its assertion. A grep finds 27 call sites of `grade.Run`, or of `run` with `grade` or `anchor`. Those that read standard output include:
- `TestJudgeAuthority` and `TestStoreFailureOutput`;
- `TestFix2TerminalJSON` and `TestFix1DeclaredRun`, whose negative substring check would go vacuous without the flag;
- `TestReleasedProcessSentinels`;
- the cleanup-output tests in `lifecycle_test.go`, `cleanup_review_test.go` and `cleanup_darwin_test.go`. Their line moves, as below.

The build reports which call sites it changed and why. A scorecard twin is added wherever the old assertion has a scorecard meaning.

### Where the scorecard's names come from

**Every verdict, state, reason, count and exit code comes from the stored record** (QE14, GR3). The scorecard renders from the record document as `internal/grade` serialises it for storage. Nothing in `internal/scorecard` calls the judge, re-derives a state, or maps an exit code to a word.

Two derived numbers are read from the judge output's own arrays: the number of held checks, and the number of checks with each result. Counting entries is reading the judge's output. The judge's `counts`, `{xfail, xpass}`, are printed as the judge wrote them. Neither enters any other count (rulebook §8).

**The names go in the evidence's `observed`.** This replaces the first draft's `Details` side channel, and KTD16's "The scorecard renders from the stored record" stands unamended. The judge contract says `expected` and `observed` are "carried for the scorecard; the judge decides nothing by them" (`evidence-record.md`, Part 2). QE19 leaves them out of the comparison, and the corpus runner reads `observed` only for a stream's `cut`. Each check's `observed` keeps the keys it has today, and gains:
- **`invocation`,** for a check whose evidence names an invocation: its words, its runbook and step, or a computation's argument list. A check that is not observed and names the invocation it would have used carries it too (QT6).
- **`computation_stderr`,** for a check that is not observed with `computation_failed`: the stream object QE10 gives, under the same masking and bound as any stream (QA4: "kept for the scorecard").
- **On the discovery check:**
  - `discovery`: `found_count`, `declared_count`, `found_names`, `missing_names`, `undeclared_names`, `request_bound_reached`, `depth_bound_reached` and `names_cut`;
  - `placeholders`: every placeholder location in the file (QS5);
  - `short_env_names`: each environment name whose declared value is shorter than 8 bytes (QE18).

  Discovery is always in the required set (QD1), so these run-level lists always have a check to carry them. A placeholder `declared_count` is carried as `null`.

So `--json` users and agents get every name the scorecard prints, from the same record.

**`decide.Discovery`'s single `Bound` flag is split into two,** one for the request bound and one for the depth bound. **`Undeclared` gains its name list.** QD3 says "each command found but not declared is named", so the list has no cap of its own. Only the walk's own bounds and QE10's limits on the record bound it. If QE10 cuts the record, the scorecard says the names were cut.

The found-names preview keeps its existing caps, 512 entries and 64 KiB, and `names_cut` says when the preview was cut. QD1 asks for the names found and missed, and missed names are never capped.

**Masking, then cutting, then escaping, on every path.** Every value from the subject, the subject file or a stream passes the masker when the evidence is built, so it is masked in the stored record, the scorecard and `--json` alike (QE18: "every record, a rejected one included"). This covers:
- `observed`'s new keys;
- `offline_settings`' `args` and `applies_to` words;
- `bookkeeping_excluded`.

A cut, such as `computation_stderr`'s head, comes after masking (KTD16). The renderer escapes last, with `grade.Escape`. The stream wrapper's masker stays as a backstop.

A fixture declares an environment value with a C0 character, and another with a Cf character. It puts each into every place the scorecard prints a subject value, and asserts the value is masked in the stored record and in both outputs. Its twin uses a value shorter than 8 bytes, which is not masked and is named in `short_env_names`.

**The stored record's interim fields are filled** (KTD16, "Interim fields"):
- **`next_actions`:**
  - One entry for each check the judge output reports with a reason in QE12's table, as `{check_id, dimension: null, reason, action}`.
  - One entry for each dimension whose reason is `placeholder_unfilled` or `scope_not_requested`, as `{check_id: null, dimension, reason, action}`.
  - The action is the row's full text from `record.NextActions()`. The sealed example `examples/record-01.json` shortens some rows, and QE19 does not compare this field, so either is allowed. This note stores the full row.
  - A reason outside QE12's table gets no entry: `live_not_run` and the judge's trial reasons. Every corpus case's `dimensions` carry `live_not_run`, and record-01 omits it too. QE12's table lists the reasons this specification, the collection, reports. The scorecard shows such a reason as the judge wrote it, with no next action.
  - Entries follow the judge output's order: checks first, as the judge lists them, then dimensions.
- **`offline_settings`:** each declared setting, in file order, as `{applies_to, args, env_names}`. `applies_to` is the declared word lists, which is what QS7 lists and the schema stores. `env_names` is sorted by byte value, since the subject's `env` is an object.
- **`bookkeeping_excluded`:** the declared bookkeeping paths, in file order.
  - record-01 also stores `.pql-tmp`. This note does not, because both the subject schema and the record schema cap the list at 256 entries, and adding `.pql-tmp` to 256 declared paths would break the record schema.
  - The scorecard lists `.pql-tmp` beside them, and `<store>-shm` (QP8), and says that directories on the way to a bookkeeping path are also left out (QP7).

**Determinism.** Every list built from a JSON object is sorted by byte value before it is emitted (`CODING_STANDARDS.md`: "never expose map-iteration order"). That covers `env_names`, `short_env_names` and the `/facts/<name>` placeholder locations. Placeholder locations follow the file's order: `/binary`, `/declared_count`, `/seed`, `/store`, `/secret`, then `/facts/<name>` sorted, then `/commands/<i>/class` and `/commands/<i>/writes` by index.

### What the scorecard shows

The scorecard is plain text, in this order. The renderer is total: a field it cannot find is shown as missing, never a panic. Every string it prints goes through one helper that escapes, and an AST test fails if `internal/scorecard` writes any string to its writer by any other route.

1. **The run line:** `pql <request>: <word> (exit <exit_code>)`.
   - The word is `verdict.plain` for a graded run, and `anchor.result` for an anchor request. The judge output sets `verdict` to null for an anchor and for a refusal, so neither of those can print a verdict's word.
   - `verdict.sub_label` follows in parentheses when it is non-null.
   - The exit code is the stored record's `exit_code`.
   - An anchor's own verdict is shown on its own labelled line, never on the run line. So an anchored exit 0 never shows `green` (QE21, judge errata E-4 item 1).
   - Fixtures: an anchored exit 0 whose own verdict is PASSED, its twin whose own verdict is PROVISIONAL known-defects, and a graded exit 0 that shows `green`.
2. **The process exit,** when it differs from the record's. Some runs exit 70 after the judge decided: a failed store (E-4), a failed cleanup (E-11) and a signal (E-6). The scorecard is then rendered after cleanup, in the exit funnel. Its last line is `exits 70: <reason>`, with E-4's note that the record was not stored, or E-11's directory left behind. Each of the three has a fixture.
3. **The scope.**
4. **Each dimension:** its name, its state and its reason, as the judge wrote them, then its next action when `next_actions` holds one.
   - Wherever `tool-description-efficacy` shows as green, each of its comparison checks' words shows beside it, `improved` or `no_material_change` (judge errata E-4 item 2). The conformance corpus never reaches this dimension, so a fixture proves it.
   - OUT-OF-SCOPE shows that plain word and its reason, and enters no count (rulebook §8).
5. **Each check whose result is not `held_crediting` or `held_non_crediting`,** under its dimension. It shows:
   - its result and reason, as the judge wrote them;
   - its next action;
   - its `observed.invocation`, or `discovery`, or the probe's name;
   - its full id, so that the defect's check is named (AE8).

   Held checks are counted, not listed.
6. **Discovery** (QD1 to QD4), from the discovery check's `observed.discovery`:
   - both counts;
   - the names found, from the preview;
   - the declared names missed;
   - each command found but not declared (QD3, AE24);
   - which bound was reached, if any (QD2).

   Undeclared names show whenever discovery found any, including when QD4 decides. That shows more than QD3 names, and no rule forbids it.
7. **Placeholders** (QS5, AE20): each location in `observed.placeholders`, as an RFC 6901 JSON Pointer. A placeholder may stand only in the eight places QS5 lists, and anywhere else it rejects the file (QS5), so the walk visits only those. The placeholder's sentence is not printed.
8. **Offline settings** (QS7), from `offline_settings`: each setting, the commands it applies to, its arguments, and its environment names. Values are never printed.
9. **Bookkeeping exclusions** (QP7), from `bookkeeping_excluded`, with the fixed lines above.
10. **QE18's note,** from `observed.short_env_names`: each name, with "not masked: shorter than 8 bytes". The value is never printed.
11. **The comparison.** With no baseline, the line "no baseline for this scope" (rulebook B1). 113 judge-checked corpus cases end on B1. With a comparison, its entries as the judge wrote them, and R2's `next_action` when it is `re_anchor`. The baseline cases are on the pending list until PR17, so only fixtures prove the comparison display here.
12. **A computation's standard error** (QA4, AE23): the masked head of `observed.computation_stderr`, under the check it served. The scorecard says when it was cut.

**A rejected record** (QE15) shows no verdict. Standard output is empty in both modes, and the rejection line goes to standard error.

**A derivation-only rejection** (exit 2, GKD8) prints no scorecard, and standard output is empty.

**E-11's line goes to standard error** whenever no scorecard prints, and always under `--json`. Today it goes to standard output (`grade.go:213`), after the JSON line. This is a change, and the cleanup-output tests move with it.

### The agreement check (AE21, GR3)

`internal/corpusrunner` today discards standard output. Now, for every case off the pending list, it runs `pql grade` once without the flag and once with `--json`.

**For a judge-checked case,** these are all equal:
- the process exit;
- `expect.exit_code`;
- the stored record's `exit_code`;
- the stored `judge_output.exit_code`.

**The scorecard is parsed** and must state what the stored `judge_output` states:
- the run line's word and exit code;
- every dimension line's state and reason;
- every listed check's result and reason;
- the held-check count;
- `counts.xfail` and `counts.xpass`.

**A literal table** in the test maps the pair (request, exit code) to the word, and checks that the run line's word is the one that pair maps to. The test writes it from the rulebook's §8 table and the contract's §4 wire spellings (`known_defect`, `not_measured`, `anchored`, `refused`), never from engine code.

**The `--json` run's standard output** must parse to a document equal to the stored `judge_output`.

**For a derivation-only case:**
- the process exit equals `expect.exit_code`;
- the rejection line's rule equals `expect.rule`;
- standard output is empty in both modes;
- no record is stored.

**Negative rows.** Each breaks one leg and asserts which leg failed, by name (`CODING_STANDARDS.md`, negative rows):
- a doctored run line;
- a run line, record and exit that agree with each other but not with the literal table;
- a stored `exit_code` changed by one;
- a dimension line with its state changed;
- a held-check count changed by one;
- a `--json` document with one field changed;
- a derivation-only case that prints a line on standard output.

**The QE14 proof** is a twin of `TestJudgeAuthority` with a well-formed doctored judge document. Its `verdict.plain`, one dimension's state and its `counts` each contradict what recomputing from the input would give. The scorecard must show each as the doctored document wrote it.

### Escaping

KTD16's one escaping function stays `grade.Escape`, with `terminalUnsafe` as its predicate. `--json` keeps `terminalJSON`, which escapes the same class set in JSON's own spelling, so the line stays one valid JSON document. `terminalJSON` passes code points below U+0020 through only because `strictjson.Write` has already escaped them inside strings.

A test goes through every code point from U+0000 to U+10FFFF and checks two things:
- `Escape` leaves no raw code point the predicate names;
- a JSON string holding the code point, written by `strictjson.Write` and then passed through `terminalJSON`, holds no such raw code point either, and parses back to the same string.

`Escape` is not injective: a literal `\u{001b}` in a value reads the same as an escaped ESC. That is a stated limit.

The scorecard has one fixture for each class KTD16 lists, in each place it prints a subject or stream value:
- a command word;
- a runbook and step name;
- a computation's argument list;
- a discovery name;
- an offline argument;
- a bookkeeping path;
- a computation's standard error;
- the cleanup path.

Each fixture has a twin with a plain character in the same place.

### The README

The README's stale line, "The judge is not yet implemented", goes. The README keeps "writes a JSON verdict on stdout", which describes `pql judge` and is still true. It gains:
- `pql grade`'s two output modes;
- QE21's sentence: an anchor's exit 0 is not a green run (judge errata E-4 item 1);
- the 16-trial floor, the default of 40, and the chance of a green at each (judge errata E-4 item 3, E-2 item 3);
- E-13's and E-14's stated limits;
- QD6's limit: a command that ignores a help request runs its action inside full confinement in an empty home;
- QA5's warning, unchanged.

A test reads the README and asserts each of these by its exact sentence.

## Readings the build settled from the text

None of these is a two-way reading of the sealed text. Each is a gap the text leaves open. Both reviews grepped both sealed folders, both errata notes and `docs/requirements.md` for: `scorecard`, `show`, `display`, `print`, `standard output`, `location`, `applied`, `left out`, `both counts`, `says so` and `next_action`.
- **QS5's "location":** a JSON Pointer, from the schema's eight places.
- **QS7's "the commands it applied to":** the declared `applies_to`.
- **QP7's "the paths that were left out":**
  - the declared paths, `.pql-tmp`, `<store>-shm`, and the directories on the way;
  - listed whether or not the probe ran ("Every scorecard");
  - the stored field leaves out `.pql-tmp`, for the 256 cap.
- **QD3 against QD4:** undeclared names are shown whenever found.
- **QE18's "the scorecard says so":** the name, never the value.
- **QE12's coverage:** no entry for a reason outside the table, and the full row text.
- **Counts:** counting the judge output's own entries is reading it.
- **Where the scorecard goes:** settled by the owner, above.

## Predicted ratchet set

Unchanged at 142. U18 adds no case to the passing set. The agreement check is new on the same 142, and the pending list stays at 10. `observed` changes, but QE19 leaves it out of every comparison, so the prediction file regenerated from `scripts/ratchet/pr14.json`, which equals `pr13.json`, must be byte-identical to PR13's.

## Rules this slice moves off KTD22's list

Each move cites a proof test whose first body comment line names the rule, and records its mutation: the change that breaks the rule, and the test that fails.

| Rule | Proof | Mutation, and the test that fails |
|---|---|---|
| QD1 | discovery `observed` rows and the scorecard fixture | drop the found names; drop the missed names |
| QD2 | one fixture per bound, and a twin with none | swap the two bound flags; never print the bound |
| QD3 | the undeclared-names fixture, more than 512 found | cap the undeclared list; drop it |
| QD4 | the fixture from `case-020`'s shape | show only one count |
| QD6 | the README test (`kind: test`) | delete the README sentence |
| QE12 | `TestNextActionsSealed`, the `next_actions` build rows, and the scorecard rows | attach an action to `live_not_run`; drop a check entry; drop the dimension entry; omit the next action from the scorecard |
| QE14 | the doctored-document twin, and the agreement check | take the run word from the exit code; recompute a dimension state |
| QE21 | the anchored fixtures and their twin, and the README test | print `own_verdict.plain` on the run line |
| QP7 | the `bookkeeping_excluded` rows and the scorecard fixture | drop `.pql-tmp`'s line; drop a declared path |
| QS5 | one placeholder-location fixture per place | drop `/commands/i/class` |
| QS7 | the `offline_settings` rows and the scorecard fixture | store a value instead of a name; drop a setting |

**Stay listed:** QE18. It needs its `macos_evidence` entry and the darwin path (PR15, GR15). U18 builds its masking-order fixtures and its scorecard note, with the mutations "escape before masking" and "print the short value".

## Controls

- **Controls 2 and 3 hold unchanged.** Control 2 is the full QE19 comparison, and control 3 is the case-derived ordered trace. U18 changes no judge input byte that QE19 compares, and serves nothing new.
- **The agreement check is the new control,** with its negative rows.
- **The two-mode test** is a control that `--json` changes only what is printed.

## Stated limits of this slice

- **The scorecard's text is not a stable interface.** Scripts and agents use `--json`, which carries every name the scorecard shows. The README and `--help` say so.
- **`Escape` is not injective,** as above.
- **The first private scoring round,** on 2026-12-23, is listed in U18's verification. The owner ruled on 2026-11-28 that PR14 merges before it. The round scores the stored record and the exit code. Its counts are reported to the owner as counts only, when it runs.
- **Only fixtures prove some displays:** judge errata E-4 item 2 (`tool-description-efficacy`), the comparison display and QD2's bounds. No conformance case reaches them.
