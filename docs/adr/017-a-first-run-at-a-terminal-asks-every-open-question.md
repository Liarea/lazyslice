# ADR-017: A first run at a terminal asks every open question it has no shown default for

Status: proposed, 2026-09-24. Narrows ADR-008 §5 "The one-question rule" and the sentence under ADR-008 §6's question table that makes Q1, Q1′ and Q2 "mutually exclusive within a run". Every other part of ADR-008 stands as written: the ladder, the question catalogue's prompts, defaults, triggers, headless answers and flags, the controlling-terminal rules in §7, and Q1′'s "no" in §6 step 5. ADR-008 is accepted and frozen, so this is a new record rather than an edit (root CLAUDE.md).

## Context

ADR-008 §5 states the one-question rule: at most one blocking question per run, the first item on the ladder **source → target → root table → row count → masking** that could not be determined with confidence, with every item below it taking its computed default and printing it as a decision. It expects that "on the happy path the single blocking question is **Q2, the root table**". The rule comes from research/SQLIT_STUDY.md §5.3 through ARCHITECTURE.md §9, and CONCEPT.md's "Zero config" paragraph carries the same words.

Dogfood session 3 (docs/DOGFOOD_LOG.md, "Session 3, 2026-09-23: the maintainer at a terminal") was the first session in which the tool could ask anything. It was a fresh directory on a machine with no local target, and the run asked one question, Q1: `no local postgres found to load into. start one? … [Y/n]`. The root question never appeared. Because Q1 had taken the run's one slot, internal/core's `rootQuestion` skipped Q2 whenever discovery reported it had asked Q1 or Q1′ (`r.askedQ1`, from `discover.Result.Asked`). The root was chosen silently and printed afterwards as `root public.users (18 inbound - 0 outbound FKs) — --root`. The log records that a stranger does not read that line as a choice they could have made.

So the rule does the opposite of what ADR-008 wanted in the case ADR-008 was written for: the first run on a new machine. That run is the one most likely to need a target container, and it is also the run where the root has never been chosen. Under the rule it could only ever show the operator the question about the container, and never the one that decides what the slice contains (on the dogfood schema, `users` versus `clients`).

The maintainer chose between three options on 2026-09-24 (tracker T-0331, option (a)). This record writes that decision down so it can be built against.

## Options considered

**(a) Ask both when both are open.** A first run that needs a target container is exactly the run whose root also needs choosing, and two questions at a terminal do not work against zero config. A headless run still defaults both and asks neither. It costs a second prompt on the one run that has two open items, and it means the one-question sentence in CONCEPT.md, ARCHITECTURE.md §9 and README has to change. Chosen.

**(b) Make Q1 non-blocking when its default is yes.** Print `starting postgres:16 as …` and carry on, so that Q2 stays the only question. This keeps the sentence true by turning a question into a decision. But Q1 is the one question whose yes *creates* something: a container, a volume and a published port on the developer's machine. ADR-008 §7 kept it a question against lazygit's `(y/N)` precedent because of exactly that, and ADR-008's own reversal condition for Q1 watches for containers nobody wanted. Creating one without asking would take away the check that condition relies on.

**(c) Keep the rule and print the root line as a confirmation the operator can interrupt.** A line that waits is a question under another name, and one that does not wait is the line session 3 already printed and nobody read as a choice. Making that line louder is documentation as the fix for a confusing first run, which root CLAUDE.md rules out: "Change the default or the question."

## Decision

(a).

1. **The rule, restated.** A run asks **no question whose default the operator has already been shown**. At a controlling terminal, every item on ADR-008's ladder that is still open and has no safe default the operator has seen is asked, in ladder order. Today that means Q1 or Q1′ for the target, then Q2 for the root, both in one run when both are open. The rest of ADR-008 §5's rule is unchanged. An item that was settled is not asked, and prints as a decision with the flag that changes it. An item with no safe default and no one to ask stops the run and names the command to run instead.

2. **What counts as settled for Q2**, and so skips it with its default printed beside `--root`: `--root`; a root recorded in the committed `lazyslice.yml`; the root a `--tui` preview pass already showed and the operator approved (`Request.Reviewed`); or no controlling terminal, or `--yes`. A root is never settled because discovery asked Q1 or Q1′. The target question never shows a root, so its answer says nothing about one.

3. **Headless is unchanged.** With `--yes` or no controlling terminal, nothing is asked. Q1 is still its hard failure: exit 4, `target.refused.none`, naming `--create-target`. Q1′ and Q2 still take their defaults. A headless run with neither the target nor the root settled still stops at Q1's exit 4, before the root is considered.

4. **Q1 and Q1′ stay mutually exclusive.** They are two branches of one open item, the target, and a run asks at most one question per ladder item. Q1′'s "no" is still ADR-008 §6 step 5's stop, exit 4 naming `--create-target`, and not a follow-up Q1. That run stops before Q2 is reached.

5. **Q2 keeps `?`.** `?` at Q2 prints the ranked top five with their score components and asks again, exactly as ADR-008 §6 and ADR-014 have it.

6. **Q4 is untouched.** It was already outside the budget (ADR-008 §6). It is a secret prompt, not a configuration decision, and at most two can fire in a run.

## Consequences

- internal/core's `rootQuestion` no longer reads whether discovery asked a question, and the `askedQ1` field is gone from internal/core's `run`. It still skips Q2 for `--root`, a yml-recorded root, a reviewed `--tui` root and a headless run. `discover.Result.Asked` is still reported and nothing in internal/core reads it now.
- The largest number of blocking questions a first run can ask is two: Q1 or Q1′, then Q2. A run whose target is settled (by flag, committed file or an eligible candidate) asks at most Q2, which is README's quickstart shape. A run whose root is settled asks at most Q1 or Q1′.
- README's zero-config paragraph and its comparison table's "Config required before first run" cell, ARCHITECTURE.md §9's one-question paragraph and question-table note, and docs/QUICKSTART_TRANSCRIPT.md's reference to "ADR-008's one blocking question" change with this record. CONCEPT.md's "First run asks at most one blocking question" is outside this ADR's implementing paths and is owed (T-0384).
- The comments in internal/discover that described Q1 and Q1′ as spending the run's one question are corrected to say the target question takes one slot of its own.
- Tests: `TestFirstRunAsksEveryOpenQuestionAtATerminalAndNoneHeadless` (internal/core: target open or settled × root open or settled, at a terminal, with no controlling terminal, and with `--yes`) and `TestHeadlessFirstRunWithNothingSettledStopsNamingCreateTarget` (internal/core: a whole headless `Run` against a local Docker endpoint with nothing on it stops at exit 4 naming `--create-target`). The test that pinned the old rule, `TestRootQuestionQ1AlreadyAsked`, is removed.

## Reversal condition

- If two dogfood sessions at a terminal show the operator taking Q2's default on a run that also asked Q1, without reading the question, and then having to rerun with `--root`, the second question is costing more than it tells. Option (b) is then reconsidered, and this ADR is superseded.
- If a question is added to ADR-008's catalogue for row count or masking, this ADR's rule would ask it on every first run where it is open. Whether three questions at a terminal is still a zero-config first run is then a new decision, taken before the question lands.
