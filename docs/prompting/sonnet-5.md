# Prompting Claude Sonnet 5

Cheat sheet for prompting Sonnet 5 in an automated coding workflow. All of it traces to [the guide][g]. Verified 2026-09-05.

## What is different about this model

- Strong at coding and agentic tasks; it "performs well out of the box on existing Claude Sonnet 4.6 prompts" ([guide][g]).
- **Length tracks task complexity**, not a fixed verbosity: shorter on lookups, longer on open-ended analysis ([length][len]).
- **Adaptive thinking on by default** — a request with no `thinking` field now thinks (a change from 4.6). Manual `budget_tokens` thinking is removed; it returns 400 ([effort][eff]).
- **More agentic by default**: reaches for tools and runs self-verification loops more readily ([tools][tool]).
- **More literal**: it "does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make", most so at low effort ([literal][lit]).
- **Better unprompted progress updates** across long agentic traces ([updates][upd]).
- **`temperature`/`top_p`/`top_k` return 400** at non-default values, new for Sonnet-class; steer tone from the prompt ([tone][tone], [design][des]).
- **New tokenizer**, ~30% more tokens for the same text, so 4.6-era `max_tokens` may truncate ([effort][eff]).

## Prompt skeleton

No template is published. This orders the guide's own advice: everything in turn one, scope explicit, a concrete bar.

```text
ROLE      You are working in <repo>. <What the harness may touch.>

CONTEXT   <Files, prior decisions, links.>

TASK      <The complete task, intent, and relevant constraints, up front.>
          # Well-specified upfront descriptions "maximize autonomy and
          # intelligence while minimizing extra token usage after user turns."

CONSTRAINTS
          <Scope, per item — it will not generalize an instruction on its own.>
          Provide concise, focused responses. Skip non-essential context, and
          keep examples minimal.

DEFINITION OF DONE
          <Observable, not "high quality": tests green, no behaviour change
          outside <file>.>

RETURN FORMAT
          <Exact shape. Findings: confidence and estimated severity per item.>
```

## Phrases that work

Verbatim from the guide.

| Symptom | Phrase |
| --- | --- |
| Too verbose | `Provide concise, focused responses. Skip non-essential context, and keep examples minimal.` |
| Under-thinking you must keep at `low` | `This task involves multistep reasoning. Think carefully through the problem before responding.` |
| Thinking blocks too frequent | `Thinking adds latency and should only be used when it will meaningfully improve answer quality, typically for problems that require multistep reasoning. When in doubt, respond directly.` |
| Scope read too narrowly | `Apply this formatting to every section, not just the first one.` |
| Review recall dropped | `Report every issue you find, including ones you are uncertain about or consider low-severity. Do not filter for importance or confidence at this stage - a separate verification step will do that. Your goal here is coverage: it is better to surface a finding that later gets filtered out than to silently drop a real bug. For each finding, include your confidence level and an estimated severity so a downstream filter can rank them.` |
| Single-pass self-filter | `report any bugs that could cause incorrect behavior, a test failure, or a misleading result; only omit nits like pure style or naming preferences` |

## Anti-patterns

- **Prompting around shallow reasoning.** "Raise effort to `high` or `xhigh` rather than prompting around it" ([effort][eff]).
- **Negative verbosity instructions.** Positive examples of appropriate concision beat "don't do X" ([length][len]).
- **Interim-status scaffolding** ("After every 3 tool calls, summarize progress"): remove it ([updates][upd]).
- **"Be conservative" / "only high-severity" / "don't nitpick"** in review harnesses: obeyed more faithfully than by earlier models, so precision rises and measured recall falls ([review][rev]).
- **Qualitative bars** like "important" — say where the bar is ([review][rev]).
- **`temperature` for variety**, `budget_tokens` for thinking: both 400 errors.
- **Underspecified prompts drip-fed over turns** — worse token efficiency, sometimes worse performance ([interactive][int]).

## Effort and length guidance

Default `high`. `xhigh` is "the recommended setting for the hardest coding and agentic use cases"; `max` is unconstrained spend; `medium` trades intelligence for cost; `low` suits "short, scoped tasks and latency-sensitive workloads", with under-thinking risk on moderately complex work ([effort][eff]).

Migration mapping: Sonnet 5 `medium` ≈ 4.6 `high`; Sonnet 5 `high` ≈ 4.6 `max`. Benchmark by observed thinking length, not effort name.

At `high`/`xhigh`/`max`, leave `max_tokens` headroom: thinking counts against it, and a tight budget yields near-all-thinking plus a truncated answer with `stop_reason: "max_tokens"`. Raise it or drop to `medium`, and add the ~30% tokenizer inflation.

## For coding agents

- Use `xhigh` or `high` effort, add autonomous features like an auto mode, cut required human interactions ([interactive][int]).
- Task, intent and constraints go in the **first** turn — the guide's route to autonomy and token efficiency.
- State scope per item; an instruction given for one file is not applied to the rest.
- Effort drives tool use: `high`/`xhigh` show "substantially more tool usage in agentic search and coding". With thinking disabled it reaches for tools less — nudge explicitly if you depend on them ([tools][tool]).
- If a tool is skipped, "clearly describe why and how it should" use it.
- In review, split coverage from filtering: finding is coverage; a separate verification/ranking stage uses the emitted confidence and severity ([review][rev]).
- Validate prompt changes against a subset of your evals or test cases (recall or F1).

**Not on this page:** nothing on task-finishing behaviour, edit strategy, test-writing, subagents, or tool-call batching. Treat such rules as unverified here; see [best practices][bp].

[g]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5
[len]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#response-length-and-verbosity
[eff]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#calibrating-effort-and-thinking-depth
[tool]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#tool-use-triggering
[lit]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#more-literal-instruction-following
[upd]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#user-facing-progress-updates
[tone]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#tone-and-writing-style
[rev]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#code-review-harnesses
[int]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#interactive-coding-products
[des]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5#design-and-frontend-defaults
[bp]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
