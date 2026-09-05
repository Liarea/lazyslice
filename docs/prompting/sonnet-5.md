# Prompting Claude Sonnet 5

For prompts in an automated coding workflow. Every claim traces to the
[Sonnet 5 prompting page][p5]. Verified 2026-09-04.

## What is different about this model

- "Particular strengths in coding and agentic tasks"; it "performs well out of the box on existing Claude Sonnet 4.6 prompts" — tune only the below. [p5]
- **Length is calibrated, not fixed:** it "calibrates response length to the complexity of the task rather than defaulting to a fixed verbosity". [p5]
- **Adaptive thinking is on by default.** Requests with no `thinking` field now think; on 4.6 they did not. Disable with `thinking: {type: "disabled"}`. [p5]
- **Manual extended thinking is removed** — `budget_tokens` returns a 400. [p5]
- **`temperature`, `top_p`, `top_k` at non-default values return a 400.** New for Sonnet-class models; steer tone and variety from the system prompt. [p5]
- **New tokenizer emits ~30% more tokens for the same text**, so `max_tokens` tuned for 4.6 may truncate equivalent output. [p5]
- **More agentic:** it "will reach for tools and run self-verification loops more readily" than 4.6. [p5]
- **More literal:** it "does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make". [p5]
- **Better unprompted progress updates** across long agentic traces. [p5]
- **A default house style on open-ended frontend briefs**, which can read wrong for dev tools and enterprise apps. [p5]

## Prompt skeleton

The page's structure advice: "specify the task, intent, and relevant constraints
upfront in the first human turn", because underspecified prompts spread over
multiple turns "reduce token efficiency and sometimes performance". [p5] The tags
are a container; the page prescribes no markup.

```text
<role>Who the model is for this task.</role>
<context>Repo, phase, files that matter, why this task exists.</context>
<task>The complete spec: task, intent, constraints in the first turn,
so no follow-up is needed.</task>
<constraints>
Scope stated per-item, never implied: "Apply this formatting to every
section, not just the first one."
Files you may write; what not to touch.
</constraints>
<definition_of_done>A concrete bar, not a qualitative word: checks
that must pass, artefacts that must exist.</definition_of_done>
<return_format>Exactly what to return, and in what shape.</return_format>
```

Both rules come from the page: state scope explicitly, since the model
generalizes nothing on its own, and set the bar concretely rather than "using
qualitative terms like 'important'". [p5]

## Phrases that work

Verbatim from [p5].

- Cut verbosity: `Provide concise, focused responses. Skip non-essential context, and keep examples minimal.`
- Reasoning, when pinned at low effort: `This task involves multistep reasoning. Think carefully through the problem before responding.`
- Suppress over-eager thinking: `Thinking adds latency and should only be used when it will meaningfully improve answer quality, typically for problems that require multistep reasoning. When in doubt, respond directly.`
- Finding stages: `Report every issue you find, including ones you are uncertain about or consider low-severity. Do not filter for importance or confidence at this stage… include your confidence level and an estimated severity so a downstream filter can rank them.`
- A concrete self-filter bar: `report any bugs that could cause incorrect behavior, a test failure, or a misleading result; only omit nits like pure style or naming preferences.`

## Anti-patterns

- **Negative style instructions.** "Positive examples… tend to be more effective than negative examples or instructions that tell the model what not to do." [p5]
- **Prompting around shallow reasoning.** "Raise effort to `high` or `xhigh` rather than prompting around it." [p5]
- **Interim-status scaffolding** ("After every 3 tool calls, summarize progress") — "try removing it." [p5]
- **"Be conservative" / "only report high-severity issues" / "don't nitpick"** in review harnesses: obeyed more faithfully, so recall falls while precision rises — a harness effect, not a capability regression. [p5]
- **Generic design steers** ("make it clean and minimal") "shift the model to a different fixed palette rather than producing variety"; specify a concrete alternative, or have it propose options first. [p5]

## Effort and length guidance

Default is `high`. `max` = maximum capability, no token constraint; `xhigh` =
"recommended… for the hardest coding and agentic use cases"; `high` = balanced;
`medium` = cost-sensitive, trades intelligence; `low` = "short, scoped tasks and
latency-sensitive workloads". [p5]

- It "respects effort levels strictly, especially at the low end": at `low`/`medium` it "scopes its work to what was asked rather than going above and beyond", risking under-thinking at `low`. [p5]
- Mapping: 5 `medium` ≈ 4.6 `high`; 5 `high` ≈ 4.6 `max`. Benchmark by observed thinking length, not effort name. [p5]
- At `high`/`xhigh`/`max`, "leave headroom in `max_tokens`" — thinking counts against it; a tight budget yields near-all-thinking output truncated with `stop_reason: "max_tokens"`. [p5]
- If you ran 4.6 with thinking off, "try thinking on with lower effort levels". [p5]

## For coding agents

- **Effort:** `xhigh` or `high` for coding products. [p5]
- **Autonomy:** "add autonomous features like an auto mode, and reduce the number of human interactions required", with the whole spec in turn one. [p5]
- **Tool triggering:** `high`/`xhigh` "show substantially more tool usage in agentic search and coding". With thinking disabled it "is less likely to reach for tools or consider searching" — nudge explicitly, and describe when and how each tool should be used. [p5]
- **Scope:** it infers no unrequested work; anything wanted broadly must be said broadly. [p5]
- **Review stages:** split finding from filtering, and say its job when finding "is coverage rather than filtering". [p5]

The page says nothing about subagents, test policy, edit format, or batching
parallel tool calls — unverified here; invent no rules for those.

[p5]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5
