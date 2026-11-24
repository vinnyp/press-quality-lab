# Peer-review gate — pr15-review-docs (2026-11-24)

**Mode:** build. **Reviewers:** peer-code-reviewer, peer-test-reviewer, peer-security-reviewer. **persona-version:** cache/1.4.0.

**Design round:** none. The change adds review documents and changes no gate, threshold or verdict, so the measurement lens did not fire.

**Range:** `bb088de..7e1ef44` (PR #15, first commit). Each reviewer ran in its own clone of that commit; repository-state snapshots before and after each run were identical.

**Tier rationale:** Tier 1 (code, test) always runs in build mode; the test lens was retargeted to whether the testing rules are correct and enforceable as review criteria, since the change has no code. Tier 2 security fired because the repository is public, to check the public-by-default rule and whether moving rules out of `AGENTS.md` weakened a guardrail. No Tier 3 lens applied: no interface, architecture or specification text changes. No different-model pass: `AGENTS.md` requires one for specifications and plan phases, and this is neither.

**Per-lens runtime:**

- peer-code-reviewer: runtime=claude
- peer-test-reviewer: runtime=claude
- peer-security-reviewer: runtime=claude

## Findings

### peer-code-reviewer

Verdict: needs rework before merge. 3 Major, 4 Minor, 2 Nit. The confinement rule on declared-path copies contradicted QS8 (declared files resolve links inside the subject directory; only a seed holding a link is rejected, QS9). The private-copy rule used the spike's unit, "once per scenario", where QH8 says once per grade and never remade. Moving four rules out of `AGENTS.md` left them unloaded by every authoring session. Minors: the ordering rule named map iteration, but the round-1 fix sorted lists that inherited input order; the severity section ranked a false green above confinement escapes and leaks the logs rated Blocker; the file's entry rule excluded the owner-stated rules it held and its exit rule would have removed the harness section; "the coding standards above" assumed the include was expanded.

### peer-test-reviewer

Verdict: trustworthy after the Majors. 4 Major, 6 Minor, 2 Nit. The oracle rule forbade comparing through the encoder, which `assertBytes` does at 48 call sites and the cited record allows for decisions. The clean-control rule omitted the record's clause that a control never discards an error, so it admitted the record's own failing example. Same QH8 and `AGENTS.md` findings as the code lens. Minors: breaking a rule should leave reviewable evidence; the case-record rule partly repeated `check.sh`'s `outcomes` and `ids-cross-check` steps, and its record said the checker lived outside the repository; the severity section defined only its top level; the two-readings line pulled against `AGENTS.md`; the goroutine rule did not say why it holds; the entry rule excluded rules carried over from `AGENTS.md`.

### peer-security-reviewer

Verdict: safe to ship. 3 Low, 2 Info. The two integrity invariants left the always-loaded context. The severity scale had no place for a public-disclosure breach, which `scan.sh` cannot catch for a private subject's name. The link rule was weaker than QS9 (a silent skip satisfied it). The commit message and the three files pass the public-by-default rule, and `scan.sh` over the range passed.

## Verify-the-reviewer dispositions

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 1 | Link rule contradicts QS8 and is weaker than QS9 | code (Major), security (Low) | Confirmed from QS8 and QS9 in `collection-rules.md` | Fixed: files the subject file names and the seed resolve inside its directory (QS8, QS9); a seed holding a link or special file is rejected, never followed or skipped (QS9). |
| 2 | Private copy "once per scenario" contradicts QH8 | code (Major), test (Major) | Confirmed from QH8 in `confinement.md` | Fixed: once per grade, checked before each use, never remade (QH8). |
| 3 | Moved rules no longer loaded by authoring agents | code (Major), test (Major), security (Low) | Confirmed: `CLAUDE.md` includes only `AGENTS.md`; Codex reads `AGENTS.md` | Fixed by the owner's choice: the four rules and the claim-to-verify sentence are back in `AGENTS.md` and also in the new files, which say both copies change together. |
| 4 | Oracle rule forbids the encoder, which 48 call sites use and the record allows | test (Major) | Confirmed: `assertBytes` compares against `write(want)`; record line 35 allows it for decisions | Fixed: decisions may compare through the encoder; spelling is asserted against literal bytes. |
| 5 | Clean-control rule omits "never discards an error" | test (Major) | Confirmed from the record's Prevention list | Fixed: clause added. |
| 6 | Ordering rule names map iteration, not input order | code (Minor) | Confirmed: 1a64e93 added `sort.Slice` over lists | Fixed. |
| 7 | Severity ranks a false green above leaks and escapes; no public-disclosure place; only top level defined | code, test, security | Confirmed against log rows rated Blocker and High | Fixed: three classes rank as Blocker or Critical together, and the labels and their gates are stated. |
| 8 | Entry and exit rules contradict the file's contents | code, test (Minor) | Confirmed | Fixed: owner-stated rules admitted; a rule stays while a planned phase will bring its code back. |
| 9 | Case-record rule repeats `check.sh`; stale record | test (Minor) | Confirmed: `check.sh` runs both scripts; the checker reached `main` in 32c5cb7 (PR #12) | Fixed: rule narrowed to extending the checkers; record corrected. |
| 10 | No reviewable evidence of breaking a rule | test (Minor) | Confirmed: the judge rounds kept this evidence; no file asks for it | Fixed: the mutation and the failing test are recorded. |
| 11 | Goroutine rule gives no reason | test (Minor) | Confirmed from `Safe` in `internal/judge/judge.go` | Fixed: reason added. |
| 12 | Two-readings line pulls against `AGENTS.md` | test (Minor) | Confirmed | Fixed: both readings named and reported to the owner. |
| 13 | "Above" assumes the include expanded | code (Minor) | Confirmed | Fixed. |
| 14 | Absence-assertion rule uses judge-request wording | code (Nit) | Confirmed | Fixed: "the output under test". |
| 15 | Later-rule-decides stated as unconditional | test (Nit) | Confirmed from the record | Fixed: measure again on a new macOS release. |
| 16 | `AGENTS.md` scope line no longer true | code (Nit) | Moot after #3 | No change. |
| 17 | "Never reports a pass" also governs production checks | test (Nit) | Agreed | Deferred: kept under Tests, where `AGENTS.md` has it. |
| 18 | Import guard scans only `internal/` | test (outside the diff) | Not verified in this round | Deferred: outside this change. |
| 19 | Whether Claude Code Review expands `@` includes in `REVIEW.md` | security (Info) | Not verified | Open: check on the first automated review. |

## Round 2 (delta verification)

**Range:** `7e1ef44..f467784`. **Reviewer:** peer-code-reviewer, runtime=claude, in a fresh clone; repository-state snapshots unchanged.

Verdict: ready after two Majors. Rows 3, 4, 5, 7, 11 and 15 confirmed closed. Both Majors came from restating sealed clauses: the confinement section now names each hazard and its clause, and says review checks the code against the clause itself.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 20 | Ordering rule (row 6 fix) forbids declaration order the collection spec requires (QS7, QP1, QH11) | code (Major) | Confirmed from `collection-rules.md` and the judge contract's input-order rule | Fixed: the order the specification names, a sort key or declaration order, never map order; sort by key where input order carries no meaning. |
| 21 | QS8/QS9 restatement (row 1 fix) drops conditions and over-applies to absolute `/bin` executables | code (Major) | Confirmed from QS8, QS9 and QA4 | Fixed: the rule requires every condition of QS8 and QS9 and names its two hazards (corrected in round 3: only the seed-link copy was reproduced; the missing containment rule was found in review). |
| 22 | "Once per grade" has no meaning in a test harness | code (Minor) | Confirmed from the solution record | Fixed: once per unit of work, a grade in the engine, a scenario in a test harness. |
| 23 | `REVIEW.md` repeats an `AGENTS.md` sentence without a sync note | code (Minor) | Confirmed | Fixed: note added. |
| 24 | Record still says PR #12 is pending merge | code (Minor) | Confirmed: merged as 32c5cb7 | Fixed. |
| 25 | Row 9 cites a commit on no branch | code (Nit) | Confirmed | Fixed: cites 32c5cb7. |
| 26 | Oracle rule opens with an absolute its next sentence breaks | code (Nit) | Agreed | Fixed: stated as one exception. |
| 27 | Pointer to rules by position | code (Nit) | Agreed | Fixed: rules named. |
| 28 | QH11 restatement drops "canonical and absolute" | code (Nit) | Agreed | Fixed by the change in #21's approach: the rule names the hazard and QH11 governs. |
| 29 | Majors are sometimes accepted, not resolved; `scan.sh` misses names too | code (Nit) | Confirmed from the 2026-11-22 log and `scan.sh` | Fixed. |

## Round 3

**Range:** `f467784..b1dfed7` (three commits: the round-2 fixes, the sealed-clause maintenance condition with `CONCEPTS.md`, and the `AGENTS.md` pointer to it). **Reviewers:** peer-code-reviewer, peer-test-reviewer, peer-security-reviewer, runtime=claude, each in its own clone; repository-state snapshots unchanged.

Verdicts: code ship-ready (5 Minor, 4 Nit); test trustworthy after one Major (1 Major, 4 Minor, 2 Nit); security safe to ship (2 Low, 3 Info). Rows 20 to 29 were confirmed closed. The shared finding was that the new maintenance condition was broken on landing by rules in the same file.

| # | finding | raised-by | verify | disposition |
|---|---|---|---|---|
| 30 | The one-reading-path rule restates X12 without naming it and drops some of its conditions; the judge section has no clause-authority line | test (Major) | Confirmed from judge contract §3 | Fixed: the rule cites X12 (§3), and the section says review checks code against the clause. |
| 31 | The QH8, QH11 and later-rule-decides rules still restate their clauses | code, test (Minor), security (Low) | Confirmed from QH8 and QH11 in `confinement.md` | Fixed: each names the hazard and the clause; the maintenance condition allows only as much as names the hazard. |
| 32 | The test-harness copy rule lost the consequence of a changed copy | test (Minor) | Confirmed from the solution record | Fixed: a changed copy fails the runs that would use it and is never remade. |
| 33 | "Reproduced" overstates the declared-path hazard | code, test (Minor), security (Info) | Confirmed from the 2026-11-22 log rows 11, 24, 38, 46, 54, 58 | Fixed in the rule and in row 21. |
| 34 | The `CONCEPTS.md` judge entry paraphrases the rule more weakly, with no sync note | code (Minor) | Confirmed | Fixed: the rule is quoted in full and listed among the copies. |
| 35 | The ordering rule's list of orders reads as complete | code (Minor), test (Nit) | Confirmed from QD2 and judge contract §5 | Fixed: a stated procedure's order is included, and the judge case follows the Order clause. |
| 36 | The second maintenance sentence is hard to apply | code (Minor) | Agreed | Fixed: a rule covering more than one specification is checked against each, and defers to the clause. |
| 37 | An owner's acceptance of a Major is not recorded | security (Low) | Agreed | Fixed: the acceptance and its reason go in the dispositions. |
| 38 | Clause ids name no version | test (Minor) | Agreed | Fixed: each section names the specification and its version. |
| 39 | Nothing checks that cited clause ids and paths resolve | test (Minor) | Confirmed: `internal/docrefs` checks R and AE ids under `docs/` only | Deferred: a code change for a later PR. |
| 40 | The `scan.sh` sentence misstates what it catches | code (Nit) | Confirmed from `scan.sh` | Fixed. |
| 41 | The sync note points one way only | code (Nit) | Confirmed | Fixed: the `AGENTS.md` pointer to `REVIEW.md` names the repeated rule. |
| 42 | "Before anything is read or copied" adds a timing QS8 and QS9 do not state | code (Nit) | Confirmed | Fixed by the rewrite. |
| 43 | The 223bd2f commit message calls the ordering rule a restatement | code (Nit) | Agreed | No change: commit messages are not rewritten. |
| 44 | The `CONCEPTS.md` "stable ids" wording is stated by no specification | security (Info) | Agreed | Fixed. |
