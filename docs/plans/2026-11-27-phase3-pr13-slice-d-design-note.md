# Design note: PR13, slice d (scope, safety classes and destructive runbooks)

**Plan group:** `docs/plans/2026-11-24-2115-feat-grade-command-plan.md` PR13. **Unit:** U14.

This note follows the slice c note, `docs/plans/2026-11-27-phase3-pr12-slice-c-design-note.md`. Its definitions and controls hold here, among them control 2's full comparison and control 3's case-derived, ordered trace. This note says only what U14 changes. Its design review is logged in `docs/agent-reviews/2026-11-27-phase3-pr13-design-note-reviews.md`.

## What the slice makes true

### Destructive runbooks run in a destructive scope

A destructive runbook runs in a scope that includes `destructive` (QS6, QA7, QA8). PR12's level-4 sentinel naming QA8 is removed. Such a runbook's checks then go through the precedence table like any other runbook's: levels 1 to 3, then level 4's `earlier_step_failed`, and so on.

**The plan's exclusion stays where U39 built it.** A destructive runbook outside a destructive scope is not in the plan, and none of its checks is in the required set (QA7, QE3).

### The guard

Today the sentinel is the only check that works out a step's class from the subject on its own. A replacement that read the plan's own results would fail together with the plan builder it backs up. So `decide.MayServe(check, key, scopes)` works its answer out again from the source data, by its own code. `scopes` is the requested tokens verbatim: the plan carries them as `Plan.Scopes`, copied from the input, never as a flag the plan computed.
1. It resolves the key's words against the subject's declared commands by longest prefix (`collection-rules.md`, the command rule). For a runbook key it also resolves every step of that runbook in the subject's `runbooks`, as the sentinel did, so a safe first step of a destructive runbook is refused outside a destructive scope too (QA7: a step makes the whole runbook destructive).
2. When that command's class is `destructive`:
   - a probe key is refused in every scope, since no probe takes a destructive command (QS6, QP3, QP4, QP6);
   - a runbook step key is refused unless the requested scope's raw tokens include the literal `destructive`.

   The plan's exclusion tests whether the scope includes the dimension `destructive-guard`, a different test. Only the `destructive` token includes that dimension (`internal/rulebook/scope.go`), and an unknown token includes nothing (E-7), so the two agree on every input, and each catches a defect in the other.
3. At each serve only, a key whose command's class or `writes` is a placeholder is refused too, so a placeholder has two brakes, level 2 and this one, as a destructive command does (QS5).
4. Two kinds of key pass without resolution:
   - **the discovery check's own keys:** its help requests, home `empty`, identified by the check (`probe` is `discovery`), never by `--help` in the words, since a runbook step can ask for help too;
   - **a computation's keys** (`words` `null`, home `scratch`), which do not run the subject.

   Discovery's requests go through `MayServe` like every other key.
5. A refusal is an engine fault, exit 70. Nothing has been served by then.

**Where it runs.** `collect` applies `MayServe`'s destructive rule to every key of every check and to every runbook step key, an E-49 step included. It does so at its start, before the discovery walk or any other key (QE3), and again at each serve. The placeholder rule applies at each serve only, because a placeholder-class probe check is planned with keys that level 2 keeps unserved. `collect` also refuses with exit 70 an authored check that no runbook step schedules, so the probe loop can never serve an authored check. The rule lives in `decide`, and `collect` only acts on the fault, as it does for `StepRuns` (KTD11).

**The tests that replace the sentinel's.** `TestInvocationServing/Destructive` and `TestOutsideStepRuns/Destructive` build a plan while a command is `safe`, then make it `destructive`. Each keeps that construction and now expects exit 70 with nothing served. A plan whose flags are set by hand would test only that a boolean is read. `TestDestructiveSentinel` becomes a `MayServe` unit row. The non-discovery subtests of `TestInvocationServing`, and `TestUnmatchedComputationKeyFault`, plan an authored check with no runbook; they now schedule it in a runbook, and one new subtest keeps the unscheduled check and expects exit 70.

### Commands that are never invoked

**A help request is allowed in every scope.** QD2 asks every counted command for help. Its only exceptions are `help`, `completion` and hidden commands, and no class is among them. QD6 and `credit-and-routes.md` §3 keep the risk inside the confinement: a help request runs in an empty home under full confinement. QS6's gate therefore applies to a command's invocation as a probe or a runbook step, never to its help. The corpus records `remove --help` in 110 of the 121 non-derivation cases, `case-001` included; the other 11 never reach `remove`.

**An unclassified command is never invoked** (AE24). That covers a command whose class is a placeholder (QS5) and a command discovery found that the file does not declare (QD3). Both are already true. U14 adds the named tests the plan asks for, each on the ordered list of served keys (KTD2.6) and each with a mutation:
- **`case-022`** serves only `export --help`. Mutation: let a probe take each discovered command.
- **`case-063`** serves only `account --help`, while the `account` probe checks end `placeholder_unfilled` and its `seed` and `planted` invocations are not served. Mutation: skip level 2's class-placeholder block.

### QE13 and the closed set of reasons

The engine's reasons become a closed set in `internal/decide`, the nine reasons a collector sends:
- `network_unverifiable`;
- `could_not_start`;
- `input_missing`;
- `precondition_unmet`;
- `empty_target`;
- `placeholder_unfilled`;
- `computation_failed`;
- `earlier_step_failed`;
- `harness_fault`.

The set is QE12's table less two rows:
- `scope_not_requested`, a dimension's reason;
- `no_evidence`, which only the judge gives (C1) and the evidence schema does not allow.

It is also the judge's not-observed list (`docs/specs/judge-1.0/rulebook.md`) less `network_attempted`, `live_not_run` and `judgement_unverified`.

**`record.Build` checks each result.** A `not_observed` result must carry a reason in the set, and a `held` or `not_held` result must carry none. Anything else is an engine fault, exit 70. `network_attempted` is not in the set, so it is never reported (QE13, QO3). The corpus's QO3 cases, `case-080` to `case-087`, give `network_unverifiable`, which is the positive half.

### QE12's table, as data

The sealed table of next actions is written into `internal/record` as data, one row per reason. The data is written out literally in Go, backticks kept as the table has them, and is never loaded or embedded from the contract. Its test parses the table from `collection-contract.md` and requires exact equality of the reasons and their text with the Go data. A second assertion pins the closed set from both sealed sources above, so dropping or renaming a row, or a typo in an action, turns it red.

The stored record's `next_actions` stays empty until U18 fills it and the scorecard shows it (KTD16's interim rule), so QE12 stays listed.

## Readings the build settled from the text

- **`has_destructive_ops` against class.** QS6, QA7 and QA8 gate a destructive command on its class and the scope only. `has_destructive_ops` appears in the sealed prose only at QS4, and QE3 sets no condition on it. So in a destructive scope the engine runs a destructive runbook even when the fact is false or a placeholder. The judge then decides the dimension from the fact, and the engine reports the check's evidence as it saw it. The final override already exempts `destructive-guard`. No case reaches this.
- **"Every reason this specification reports"** (QE12) means the reasons for a check that is not observed and for a dimension's own reason, which are QE12's second and third sentences. A failure kind such as `wrong_exit` is not a reason (QE8 keeps `reason` apart from `failure_kind`). The judge's other reasons are outside this specification's table:
  - the steering reasons, `live_not_run` among them (`credit-and-routes.md` §2);
  - the P and K reasons;
  - the W1 to W3 set-aside reasons;
  - the anchor's refusals.
- **Where a next action's reason comes from.** It comes from the judge's output (QE14), for checks and dimensions alike. The judge can decide a check differently from the evidence: C1 gives `no_evidence`, C3 gives `empty_target` (`case-011`) and C4 gives `precondition_unmet` (`case-093`). So U18 reads both kinds of reason from the judge's output, and that is outside this slice.
- **A step whose class is a placeholder** is ruled by E-54, which the build review raised. In a scope without `destructive`, its whole runbook depends on the placeholder: no step of it runs, and each required check is `placeholder_unfilled` at level 2. In a destructive scope it runs as any runbook does, and level 2 stops the placeholder step's own checks. Level 2, `StepRuns`, `MayServe`, the trace reader and the prediction script's `depends()` all apply it. `StepRuns`' own test is a second brake behind level 2, which it calls with the same check, so dropping either one alone turns no test red; dropping both does.

## Predicted ratchet set

PR13's built set is `scripts/ratchet/pr12.json` with `QA8.destructive_runbook` added to `builds`. The script needs no other change, because the token is already modelled. `scripts/predict-ratchet.py --built scripts/ratchet/pr13.json` computes `scripts/ratchet/pr13-prediction.json`, and `check.sh` and the Go readers switch to it. The script's history loops gain PR12, so every earlier file reproduces in its tests.

**Predicted:** 142 cases, PR12's 140 plus `case-002` (exit 0) and `case-059` (exit 4). Both decide at level 8. The design review reproduced this. The plan's "QE13 cases the design note computes" adds none: `case-080` to `case-087` are already among PR12's 140.

- **`case-002`:** its destructive runbook is one step, `remove gear` without `--confirm`. The recording refuses it with exit 3 and a standard-error message naming `--confirm`, so the `destructive-guard` check is held, as is every required check.
- **`case-059`:** it adds a `destructive-guard` check on a step whose command is not destructive. That check holds but does not credit (`credit-and-routes.md` §2: "the step's command is `destructive`"), which proves the credit condition.

| Cases | Ending |
|---|---|
| `case-069`, `case-070`, `case-143` | excluded, the real verifier |
| `case-071` | exit 70, `QE4.anchor` |
| `case-130` to `case-134`, `case-137` | exit 70, QE4 |

## Rules this slice moves off KTD22's list

| Rule | Proof | Its mutations, each with what fails |
|---|---|---|
| QA8 | `case-001`, `case-002`, `case-059`, the mixed-runbook rows and the guard's rows | Drop the plan's destructive-runbook exclusion: the mixed-runbook rows and the guard's rows (in `case-001` the runbook's one check is out of scope anyway). Let the scope include `destructive-guard` without the `destructive` token: `case-001`'s required set, and `MayServe`. Keep destructive runbooks excluded in every scope: `case-002` and `case-059` |
| QE13 | The closed set and `record.Build`'s checks, plus `case-080` to `case-087` | add `network_attempted` to the set: the QE12 set assertion and the `network_attempted` row. Emit it for a QO3 kill: `record.Build`'s refusal, and `case-080` to `case-087` |

**Stay listed:**

| Rule | Still lacks |
|---|---|
| QS6 | the real path running a `safe` command under full confinement (PR15) |
| QA7 | the shared home on the real path (PR15) |
| QE12 | the stored record's `next_actions` and the scorecard (U18) |

Their mutations are fixed now:

| Rule | Its mutation |
|---|---|
| QS6 | let a probe take a `destructive` command, which `MayServe` and the required-set comparison must catch |
| QE12 | drop a row from the table, or change an action's text |

## Controls

1. **The ratchet.** `case-002` and `case-059` leave `internal/corpusrunner/pending.txt` and must pass. Every other pending case must still fail at its predicted ending.
2. **Every check, compared in full** (control 2).
3. **The trace** (control 3).
   - **The corpus.** The reader's hard-coded "no key of a destructive runbook" becomes a rule derived from the case, which matters in the destructive-scope branch. With `destructive` in the case's scope, the runbook's keys are expected by the rules every runbook follows, so `case-002` and `case-059` expect `remove gear`.
   - **A direct unit control.** Outside a destructive scope, the corpus can never exercise the rule, because such a runbook has no evidence. So a direct test gives the reader a mixed runbook that has a required check and a destructive step, and checks no key is expected under `conformance` and its keys under `conformance,destructive`.
4. **Unit rows for every branch the corpus lacks:**
   - **Runbook shape:**
     - a mixed runbook whose destructive step is not its first, in scope and out of it;
     - a destructive step after a not-held step, which is `earlier_step_failed`;
     - an E-49 destructive step with no required check, under a plan built without the exclusion, which gives exit 70.
   - **Command resolution:** a `safe` `remove preview` under a destructive `remove`, and a destructive `config reset` under a safe `config`.
   - **The `destructive-guard` check:** an exit 0, and a standard error without the message, each not held.
   - **Conditions on the run:**
     - a destructive runbook under a grade-wide QO1 condition (E-15);
     - one whose destructive command has a placeholder `writes`, which level 2 stops;
     - a mixed runbook of a safe step, a placeholder-class step and a safe step (E-54): in a conformance scope nothing is served and every check is `placeholder_unfilled`; in a destructive scope the first step runs, the placeholder step's checks are `placeholder_unfilled`, and the last is `earlier_step_failed`.
   - **The guard:**
     - a plan built without the exclusion, which gives exit 70 before any key is served;
     - a probe whose command is destructive;
     - an authored check that no step schedules;
     - discovery's `remove --help` served, a runbook step `remove --help` in a conformance scope refused, a computation key served, and a placeholder-class key refused at serve.
   - **Scope at the command line:** `--scope destructive` alone, an unknown token and a repeated token each give exit 2 with nothing served (E-7, X1).
   - **The fact:** class `destructive` with `has_destructive_ops` false, in a destructive scope.
   - **The reason checks:** `network_attempted`, `no_evidence` and `scope_not_requested` refused; an empty reason on `not_observed`, and a reason on `held`, each refused.
5. **Mutations.** Each fixed mutation above turns a row or case red. So do these:
   - `MayServe` reading the plan's results instead of the subject;
   - serving `remove gear` in `case-001`.

## Stated limits of this slice

- **What a `destructive-guard` check proves.** The engine does not test that an author's check really omits the confirmation (`credit-and-routes.md` §3, the row where an author routes a check), and it never inspects the store after the step. A step that deletes, then prints the message with the refusal's exit code, holds. Only the throwaway home is at risk, and the confinement still holds.
- The real path's runs of a destructive step wait for PR15.
