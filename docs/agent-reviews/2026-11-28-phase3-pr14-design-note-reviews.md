# Peer-review gate — phase 3 PR14, slice f design note (2026-11-28)

**Mode:** design.

**Reviewers:**
- peer-interface-reviewer, because the slice changes `pql grade`'s output contract;
- a different-model review on agy (gemini-4-argon-high).

**Owner rulings before the review (2026-11-28):**
- The scorecard goes to standard output by default, with `--json` printing the judge's document instead.
- PR14 merges before the first private scoring round.

## Round 1 (note at d66a284)

**Verdicts:**
- **interface:** no Blocker, eleven Majors.
- **agy:** two Blockers and five Majors, most of them overlapping the interface review.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | The `Details` side channel left `--json` users without the names, and skipped masking at build time. The judge contract already carries such names in `observed` ("carried for the scorecard"), which QE19 does not compare | interface (Major) | Confirmed against `evidence-record.md` Part 2 and the runner's QE19 comparison | Names move into `observed`; KTD16 stands unamended |
| 2 | Masking ran after escaping for values built outside the record, so a declared value with a control character printed unmasked | interface (Major), agy (Blocker) | Confirmed against `record/mask.go` and `Escape` | The order is mask, then cut, then escape, at build time. Fixtures hold C0 and Cf values |
| 3 | The newly stored `offline_settings` and `bookkeeping_excluded`, and computation standard error's cut, were not masked first | interface (Major) | Confirmed | Built through the masker; the cut comes after masking |
| 4 | An anchor has no `verdict.plain`: the run line would print the own verdict's `green` for exit 0, against QE21. The word table collided at exit 0 and used §8's spellings, not the wire's. `pql anchor` had no `--json` | interface (Major), agy (Blocker) | Confirmed against the judge output schema and contract | The run word is `anchor.result` for an anchor. The table is keyed on (request, exit) with wire spellings. `pql anchor` gets `--json` |
| 5 | After E-4, E-11 or E-6 the run line would say the judge's exit while the process exits 70. Cleanup runs after printing | interface (Major) | Confirmed against the funnel | Rendered in the funnel after cleanup, ending with an `exits 70` line; a fixture for each |
| 6 | The cleanup line goes to standard output today, not standard error, and has no home when no scorecard prints | interface (Major), agy (Major) | Confirmed against `grade.go:213` | Moved to standard error whenever no scorecard prints, and under `--json` |
| 7 | The judge's `counts` holds only `xfail` and `xpass`; no held-check count exists | interface (Major), agy (Major) | Confirmed against the schema | Held checks are counted from the judge output's own entries, a reading the note states; `counts` printed as written |
| 8 | B1's "no baseline for this scope" was missing, though 113 cases reach it | interface (Major) | Confirmed by count | Added, with the comparison display, proved by fixtures |
| 9 | The QE14 ledger proof survived its own mutation | interface (Major) | Reasoned from the code | A doctored-document twin, and the agreement check compares every dimension line and the counts |
| 10 | Discovery could not name undeclared commands past the 512-name cap, nor say which bound was reached | interface (Major), agy (Major) | Confirmed against `decide.Discovery` | The bound flag is split, and undeclared names get their own uncapped list |
| 11 | Map iteration order for `env_names`, the QE18 names and `/facts` locations | agy (Major) | Confirmed against `CODING_STANDARDS.md` | Each sorted by byte value; placeholder locations in a fixed order |
| 12 | Existing tests that read standard output were missing from the migration | agy (Major) | Confirmed: three named tests exist, 27 call sites | Every reading test gets `--json`; the build reports each site |
| 13 | Minors: the record-01 differences (`.pql-tmp`, row text), QD6's ledger kind, the README's `pql judge` line, escaping coverage per location, comparison words per check, `--help`, negative-row legs, renderer totality, determinism of the byte test | interface, agy | Confirmed | Each fixed in the note |

## Round 2 (note at 214e4ca)

**Verdicts:**
- **interface:** no Blocker. Ten of its eleven round 1 Majors are fixed, and two new Majors.
- **agy:** no Blocker, four Majors, two of them the same as the interface review's.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 14 | QE10's 4 MiB cap, in check-id order, replaces a whole evidence value with its digest. A real CLI's discovery `observed` can pass the cap by itself and lose every name | interface (Major), agy (Major) | Confirmed against `record.bound` | The scorecard renders from the masked record before QE10's bound. Its verdict lines are still the stored record's. The names in the stored record and `--json` become a stated limit. KTD16 is amended to match |
| 15 | `observed` is an array today, so "keeps its keys" was undefined | interface (Major), agy (Major) | Confirmed against `evidence.go` | Defined as an object with `streams` and an aligned `invocations` array, and discovery's lists on the discovery check |
| 16 | Discovery when the walk did not run would report every declared name as missed | interface, agy | Confirmed against `Discovery.Ran` | `ran`, with null counts and lists |
| 17 | Two runs of one case share a trace and a store | agy (Major) | Confirmed against `readTrace` and the store's hash names | A trace and a store for each mode, compared byte for byte |
| 18 | The note was said to move QE18, QA4 and QA6 | agy (Major) | Refuted: the note lists QE18 as staying, and moves neither QA4 nor QA6 | No change |
| 19 | Minors: the bound flags' attribution, one-to-one pairing and `sub_label` in the agreement check, several post-judge faults, `env_names` masking, a refused anchor's reason, checks with a null reason or out of scope, `computation_stderr` duplicating a stream, the argument list as declared, always rendering in the funnel, `-shm` conditional on a path store, `check.sh`'s ratchet file | interface, agy | Confirmed | Each fixed in the note |

## Round 3 (confirmation, note at 4b9b609)

**Verdict:** interface reviewer, no Blocker, one Major. Both round 2 Majors are fixed, and it confirmed that rendering before the bound cannot make any verdict, state, reason, count or exit code differ from the stored record's.

| # | finding | verify | disposition |
|---|---|---|---|
| 20 | Splitting the bound flag as written could leave neither flag set after a trimmed walk. QD2's `precondition_unmet` would then be skipped, and a CLI with more than 4,096 commands would go red | Confirmed against `discovery.go:106,183-191` and `discoveryLevel8` | The request bound is set whenever the walk did not stop on depth and a request was trimmed or remained. The decision reads both flags together. Two fixtures |
| 21 | The copy taken before the bound needs a defined point and type, because the bound replaces values in the shared maps | Confirmed | `record.Build` copies `expected` and `observed` between masking and the bound, in a type holding only those two fields |
| 22 | The undeclared-names cap departs from QD3's "each" | Confirmed | Put to the owner, who accepted it as a stated limit on 2026-11-28 |
| 23 | Nits: `invocations` alignment, a stale key name, two contradictory sentences, a second signal in the funnel | Confirmed | Fixed |

The design review is closed. The bound-flag rule is checked again in the build review, by its two fixtures.

## Amendment during the build (commit 1475219)

Part 2's build stopped, rightly, on a gap: the record carried neither a probe check's name nor the declared store, which the scorecard needs for its `-shm` lines. The caller amended the note: every check's `observed` carries `probe`, and the discovery check's carries `store`. The build's code review raised this as unreviewed, and checked it there. It is logged here as a build-time amendment.
