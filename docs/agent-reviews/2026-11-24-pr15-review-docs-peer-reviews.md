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
| 9 | Case-record rule repeats `check.sh`; stale record | test (Minor) | Confirmed: `check.sh` runs both scripts; 4c3182f moved the checker in | Fixed: rule narrowed to extending the checkers; record corrected. |
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
