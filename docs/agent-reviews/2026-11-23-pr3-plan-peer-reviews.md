# Peer-review gate — pr3-plan (2026-11-23)

**Mode:** design. **Reviewers:** peer-staff-software-engineer-reviewer, peer-plan-reviewer, peer-experiment-reviewer. **persona-version:** cache/1.4.0.

**Design round:** Round 1 (design).

**Subject:** the PR3 revision of `docs/plans/2026-11-21-2309-feat-conformance-collection-spec-plan.md`: units U7 to U15, KTD11 and KTD25 to KTD30, the CR49, CR56 and CR61 amendments, and the sections they touch. Round 1 read commit c735aac, round 2 read e4f5a03 and round 3 read 9bc9c1b. 10cf61f closes round 3.

**Tier rationale:**
- **The baseline.** The staff engineer is the design-mode baseline.
- **Tier 2.** The plan reviewer runs because the plan is executed unattended. The measurement lens runs because the corpus defines what each check proves and which verdict it gets.
- **Tier 3.** Standards was not run on the plan, because the specification text it adds gets its own standards review in U9. Security and privacy did not apply: no new trust boundary, and private cases are handled by rule.
- **A different model.** The staff lens also ran on agy, as `AGENTS.md` asks for each phase of the plan.
- **The document review before the gate.** A lite review found three fixes, all applied.

**Spec gate:** the load-bearing control is the macOS sandbox profile, whose substrate the spike verified and froze (`docs/spikes/2026-11-28-confinement.md`). The plan adds no new platform control.

**Per-lens runtime:**

- peer-staff-software-engineer-reviewer: runtime=claude for rounds 1 to 3; also runtime=agy for round 1, rc 0, 35,147 bytes
- peer-plan-reviewer: runtime=claude for rounds 1 to 3
- peer-experiment-reviewer: runtime=claude for rounds 1 to 3. It sits outside `peer-reviewer-agents`, so the Claude pass is its only pass.

**Isolation:** every Claude reviewer read an isolated clone, and agy read the worktree only while it was unchanged. The worktree's own changes between rounds are the author's revisions.

Findings are condensed, and local paths removed, because this repository is public.

## Round 1 (design)

### peer-staff-software-engineer-reviewer (claude and agy)

- **BLOCKER:** the behaviour recording format had no schema or rule, though it is sealed and phase 3 reads it.
- **MAJOR:**
  - the invocation list was open-ended, and computations were missing;
  - the case layout conflicted with QS8;
  - case mode could not run without a manifest;
  - acceptance-example coverage had no source;
  - nothing checked hand-written ids against subject files;
  - a profile or harness change had no stop rule;
  - CAE3's crash depended on an unverified platform fact;
  - CR52's direct run was dropped;
  - deleting the spike would remove a control the sealed text cites;
  - the private half was underspecified;
  - there was no thin slice, and U8 was too large.
- **agy (staff lens), additionally:**
  - `subject_test.go`'s Linux scope;
  - a deadlock between corpus authoring and the coverage test;
  - the export contents, the draft notices being edited after the freeze, and the `check.sh` steps that survive the seal.

### peer-plan-reviewer

- **BLOCKER:** the case layout broke QS8 and QS9.
- **MAJOR:**
  - deleting the harness in the same pull request voided its macOS evidence;
  - acceptance-example coverage had no source;
  - the expected record was undefined;
  - U8 was too large;
  - invocations depended on subject files written later;
  - the form list was not closed;
  - the behaviour format was unspecified;
  - nobody was named to run the recording, or from which sandbox;
  - there was no stop rule for a profile change.
- **Spec coverage gaps:** CR52, CR61, CR105 and CR73.

### peer-experiment-reviewer

- **BLOCKER:** a defect form that never performs its violation passed every gate, because recordings had no positive control.
- **MAJOR:**
  - the engine-side controls could not be scored;
  - coverage was counted by rule id only;
  - nine dimensions credited on the author's label;
  - the id generator was tested on one preimage shape;
  - the 20-case blind sample was too small;
  - the export leaked answers;
  - the catalogue of seeded mistakes was never validated;
  - each recording came from a single run.

## Round 2

- **Every round-1 Blocker and Major was resolved or partly resolved**, with the clause quoted per reviewer.
- **New majors:**
  - KTD11 still carried the old sample rule;
  - the coverage gate was never switched on;
  - private cases could reach the public log;
  - the export dropped `verifier`;
  - KTD29's scope needed CR49, a schema and a precedence rule;
  - the form list missed what coverage now required;
  - coverage had no tags to read;
  - computation files would make ids platform-dependent.

## Round 3

- **Every round-2 Major was resolved.**
- **New majors:**
  - `covers` tags were self-reported;
  - `input.invocations` leaked part of the answer to the blind reader.
- **Closed in 10cf61f, without a fourth round:** both majors, plus the minors on the tag grammar, the tool set, kill attribution and the crash check's fallback.

## Verify-the-reviewer dispositions

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | Recording format unspecified | staff, plan, agy | Confirmed: no schema existed, and `draft()` reads only `examples/` | Fixed: KTD25 and U11 step 1 define it, with a schema |
| 2 | Case layout breaks QS8 and QS9 | plan, staff, agy | Confirmed against `collection-rules.md` QS8 and QS9 | Fixed: KTD30 case assembly |
| 3 | No positive control for violating forms | measurement, staff, plan | Confirmed: the spike record required one, and U7 had dropped it | Fixed: KTD25, U13 and U7 |
| 4 | Harness deletion voids the macOS evidence | plan, agy | Confirmed against `AGENTS.md`'s evidence rule | Fixed: U15 in PR4 |
| 5 | Case mode needs a manifest | staff, agy, plan | Confirmed: `runner.py` calls `manifest()` first | Fixed: KTD26 `--unsealed` and `--schema-root` |
| 6 | Engine-side controls cannot be scored | measurement | Confirmed: no form can make them fail | Fixed: KTD29 scripted host, CR49 amended; owner to confirm |
| 7 | Blind sample too small | measurement | Confirmed by arithmetic | Fixed: KTD11 derives the whole set, with a conflict call-out for the owner |
| 8 | Coverage gate never switched on | measurement, staff | Confirmed: no unit made the declaration | Fixed: U8 step 6, and U10's sealed check refuses a kit without it |
| 9 | Private cases reach the public log | staff | Confirmed: the old clause had been dropped | Fixed: KTD11 and U9 step 3 |
| 10 | Export drops `verifier`, and leaks `invocations` | staff, plan | Confirmed against QT2 and KTD27 | Fixed: `verifier` kept, `invocations` moved out of `input` |
| 11 | Coverage tags are self-reported | measurement | Confirmed: the gate checked only the vocabulary | Fixed: KTD27 checks tags against case content, except `order:` tags |
| 12 | Computation digests depend on the platform | plan | Confirmed: QE2 puts the file digest in the preimage | Fixed: KTD30 POSIX `sh` with a named tool set |
| 13 | A Go crash may end by `SIGKILL` | staff, plan | Not verified: the spike never ran it | Fixed: U11 step 0 checks it first, and a `SIGKILL` goes to the owner |
| 14 | U8 too large, with no slice | staff, plan, agy | Confirmed | Fixed: U11, U12, U13, U8 and U14 |
| 15 | Deleted suite cited by sealed text | staff | Confirmed: `confinement.md` QH9 and `credit-and-routes.md` §3 | Fixed: U11 step 5 rewords both to cite the spike record, and U10 gives phase 3 the duty to repeat it |
| 16 | Nine dimensions credit on the author's label | measurement | Confirmed in `credit-and-routes.md` §2 | Fixed: U11 step 3; the owner sees the changes before ratification |
| 17 | Acceptance-example coverage has no source | plan, staff | Confirmed: no file under the folder names one | Fixed: KTD27 keeps them out of the folder, and U8's review log maps all 32 |
| 18 | Minors in all three rounds | all | Checked against the plan | Fixed, except one accepted as is: an invocation left off a case's list and implied by nothing else is caught by U9's derivation, not the gate |

**Gate:** no Blocker or Major is open. The owner confirms the CR49 amendment and KTD11's sample at the pull request.
