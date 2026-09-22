# ADR-014: `?` at the root-table question prints the ranked candidates; `--tui` stays the only way into the two screens

Status: accepted, 2026-09-22 (frozen at the phase 5 gate; proposed 2026-09-17). Narrows one clause of ADR-002 ("entered only by `--tui`, or by pressing `?` at a prompt") and one of ADR-008 (section 7's "`?` at Q2 prints the ranked candidates through the line printer and `$PAGER` until the Bubble Tea screens land"). It supersedes neither: ADR-002's two screens, its rule that the TUI owns no logic and that every TUI action exists as a flag first, and ADR-008's question table all stand as written. Both are accepted and frozen, which is why this is a new record rather than an edit (root CLAUDE.md).

## Context

ADR-002 gives Bubble Tea exactly two screens, the classification table with reasons and the plan table, and two ways in: `--tui`, or `?` at a prompt. ADR-008 then fixed what the prompts are. A run asks at most one blocking question, and the three it can ask are Q1 (start a target container?), Q1' (start this stopped one?) and Q2 (`root table? [customers]`). Its question table says `?` at Q2 "prints the ranked top five with score components", and section 7 adds that this goes through the line printer "until the Bubble Tea screens land".

The screens landed in phase 5 (T-TUI) behind `--tui`. Q2 itself was found unbuilt by the gate-5 audit on 2026-09-17 and landed the same day (T-0271), and building it met the gap between the two records: every prompt ADR-008 allows is asked *before a plan exists*. Q1 and Q1' are asked during discovery, before the schema is read; Q2 is asked after introspect and before plan, because its answer is the planner's input. Both of ADR-002's screens render a plan. There is no moment at which a run is stopped at a prompt and has anything either screen could show, so "`?` at a prompt enters Bubble Tea" describes a transition with no destination.

## Options considered

**A third screen, the root candidates, entered by `?` at Q2.** It would make ADR-002's sentence true. It is also a screen whose whole content is five lines, which fails ADR-002's own test for when Bubble Tea is worth its cost ("the two screens that need paging"), and it would be the first TUI surface with no `--flag` twin other than the question it interrupts.

**Answer Q2 with the default, plan, and open the plan screen.** `?` would then mean "show me what you would do", which is a reasonable thing to want, but it is what `--tui` already does from the command line and it changes what a keypress at a question means: every other answer at Q2 chooses a root, and this one would choose the default root as a side effect of asking for help.

**`?` prints, permanently, and `--tui` is the only way in.** `?` at Q2 prints the ranked top five with their score components through the line printer and asks again, exactly as ADR-008's question table says. The two screens are reached by `--tui`, which runs the preview pass that gives them something to show (T-PIN ties that preview to the run that follows).

## Decision

The third. `?` at Q2 prints the ranked top five with their score components and re-asks; it never enters Bubble Tea. Q1 and Q1' take no `?` at all, as ADR-008's table already has it. `--tui` is the only way into the classification and plan screens. ADR-008 section 7's "until the Bubble Tea screens land" is read as having no later state: the line-printer answer is the answer.

ADR-002's reversal condition about `?` ("if dogfood sessions show users cannot find `?`, make `--tui` the TTY default") is untouched and becomes easier to act on, not harder: it never depended on `?` opening a screen.

## Consequences

- internal/tui/CLAUDE.md says what `?` does and that `--tui` is the only entry (done in T-0271).
- A first run's one question stays five lines of help away from its answer, with nothing to learn to leave: no alternate screen is entered from a prompt, so a terminal that cannot host Bubble Tea behaves identically at Q2.
- The hint for the screens has to come from somewhere other than `?`. The plan's decision lines already print the flag that changes each decision; whether the plan summary should also name `--tui` when the reasons list is long is a first-run wording question for dogfood (T-0064), not decided here.

## Reversal condition

A prompt is added that is asked *after* a plan exists (a confirmation before the first drop, say). At that prompt `?` has a destination, ADR-002's sentence becomes implementable as written, and this record should be superseded by one that says which screen it opens.
