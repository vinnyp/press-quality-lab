# Judge specification 1.0: errata and notes

The files in `judge-1.0/` are sealed and are not edited. The rulebook's version policy says that purely editorial corrections keep the version string `1.0` and are listed in a dated note. This is that note.

Nothing here changes what a judge outputs for any input. No entry changes a binding rule, a schema or a case. Where an entry and a binding rule of the rulebook appear to differ, the rule is right.

## 2026-11-21

### E-1. A stale entry in the rulebook's §15

§15 says that requirement R31 "lists what makes up a condition and does not include the declared count". R31 was amended on 2026-11-18 and does include the declared trial count. The rulebook (§1, "Condition") and the requirement agree. That bullet of §15 no longer describes a departure.

### E-2. Departures from the requirements that §15 does not list

§15 lists where the rulebook departs from the requirements in `docs/requirements.md`. These belong on that list. Each is deliberate, and each is the rule as sealed.

1. **R20, the plain words.** R20 names four outcomes and says every state maps onto them. The rulebook's §8 has a fifth plain word below the level of a run: N/A is "not applicable". It applies to a dimension that is structurally excluded for the subject (D2), and to a group whose in-scope dimensions are all N/A (G6), which no legal scope reaches in this version. A dimension that is N/A enters no verdict. A run still has only the four outcomes.
2. **R24, exit code zero.** R24 says only a green run exits zero. That holds for a grade request. An anchor request that is Anchored also exits zero, whether its own verdict is PASSED or PROVISIONAL, `known-defects` (§8, §12). An exit code of zero from an anchor request means the baseline was saved. It does not mean the run is green.
3. **R28, the minimum trial count.** R28 says the minimum is derived from the threshold and stated false-pass and false-fail targets. The rulebook fixes the interval at the 95% level and takes as the floor the smallest count at which green can be reached, which is 16 (T4, X7). It then reports the error rates that follow (§9.4). The owner settled this on 2026-11-21: **the targets are the rates §9.4 reports.** A prompt whose true rate is exactly 80% is judged green at most 2.9% of the time and red at most 5.2% of the time, at every declared count from 16 to 100. Sixteen is a floor, not a recommendation. At 16 trials a prompt whose true rate is 95% is judged green 44% of the time; at the default of 40 it is 86%.
4. **R28, green without clearing a threshold.** R28 says a steering dimension is green only when the interval clears the threshold. For `tool-description-efficacy` the result `no_material_change` is Held, crediting (K4), so the dimension can be GREEN when the interval of the difference sits inside the margin. Green there means the candidate arm is not materially worse than the baseline arm. It does not mean the candidate is better. The owner confirmed this on 2026-11-21. A comparison check that is not Unobserved carries `regressed`, `improved` or `no_material_change` (§13), so one that is Held always carries `improved` or `no_material_change`, and that word is what says which.
5. **R25, what a baseline stores.** R25 says a baseline stores the observation status of every required check. Under §10 a baseline holds an entry only for an in-scope check whose result was Held or Failed. An Unobserved check has no entry. A run with an Unobserved in-scope check on an applicable dimension is PROVISIONAL, `gaps` and is refused anchoring (N3), so the only checks left without an entry are those on a dimension that is N/A.
6. **R25, an unchanged condition.** R25 says a trial-based dimension whose condition is unchanged since the baseline is not regressed by identity. Under E10 a prompt entry is Same by identity only when its condition, its `k` and its `n` all equal the current ones. E6 and E7 are tested first, as for every prompt entry. For a `held` entry whose condition is equal and whose counts are not, E8 or E11 then decides. The rule is stricter than the requirement: a different pool of trials presented under the same condition is not waved through.
7. **R3, model evaluators.** R3 says evidence names who decided the outcome, a deterministic rule or a named model. In this version a kept trial whose evaluator is a model is rejected (X9). A steering trial is decided by the tool calls it makes (S1, S2): its first relevant call, and whether any forbidden tool is called at any point. No rule provides for a model's judgement of a trial. A family that needs a model's judgement needs a new version of the rulebook, as R40 already expects.

These are not departures, and are noted so that they are not raised again:

- **R28 and D1.** R28 says a dimension is judged only once the declared count is reached. Each prompt check is judged only at its declared count (P1 comes before P3 and P4). The `steering` dimension is RED as soon as one prompt is Failed, even while another prompt is short of its count, because red outranks not-measured (R23).
- **R28, R30 and the error cap.** Before the declared count is reached, a pool with too many errored trials is Unobserved either way. The rulebook gives `error_cap` when the errored trials already exceed a quarter of the declared count, and `trials_short` otherwise, because until then further trials can still bring the fraction back under the cap. The verdict is the one R30 asks for.

### E-3. Two meanings of R1, R2 and R3

In the rulebook's §11, and wherever the specification refers to the comparison's result, R1, R2 and R3 are the ids of the rulebook's three comparison-result rules. Everywhere else in the specification an R followed by a number is a requirement in `docs/requirements.md`. The two are told apart by context: a comparison result is Regressed, Not-measured or Not regressed.

### E-4. What the engine must do because of these

These bind the engine and its specifications. They do not bind a judge.

1. The engine's documentation and its scorecard never present an exit code of zero from an anchor request as a green run (E-2, item 2).
2. Wherever the scorecard shows `tool-description-efficacy` as green, it shows the comparison's word beside it: `improved` or `no_material_change` (E-2, item 4).
3. The engine's documentation says that 16 trials is a floor and gives the default of 40, with the chance of a green at each (E-2, item 3).
