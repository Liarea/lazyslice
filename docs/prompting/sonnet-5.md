# Prompting Claude Sonnet 5

For prompting Sonnet 5 in an automated coding workflow. Every claim traces to the [guide]; verified 2026-09-05. What the guide omits is marked unverified.

## What is different about this model

- Strong on coding and agentic tasks; "performs well out of the box on existing Claude Sonnet 4.6 prompts" — tune, don't rewrite. [guide]
- **Length calibrated to task complexity**, not a fixed verbosity: shorter on lookups, longer on open-ended analysis. [guide]
- **Adaptive thinking on by default.** A request with no `thinking` field now thinks; on 4.6 it did not. Disable: `thinking: {type: "disabled"}`. [guide]
- **Manual extended thinking removed:** `thinking: {type: "enabled", budget_tokens: N}` returns a 400. [guide]
- **`temperature`, `top_p`, `top_k` rejected** — a non-default value is a 400, new for Sonnet-class models. Steer tone from the prompt. [guide]
- **More agentic than 4.6:** reaches for tools and runs self-verification loops more readily. [guide]
- **More literal:** it "does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make." [guide]
- **Better unprompted progress updates** across long agentic traces. [guide]
- **New tokenizer**, ~30% more tokens for the same text, so `max_tokens` tuned for 4.6 may truncate. [guide]

## Prompt skeleton

The guide's autonomy rule: "specify the task, intent, and relevant constraints upfront in the first human turn". This skeleton is that rule plus the scope, bar, and format wording the guide gives elsewhere.

```text
ROLE     You are working in <repo>, on <component>.
CONTEXT  <Files, prior decisions, commands. Front-load all of it.>
TASK     <The complete spec and its intent, in the first turn.>
CONSTRAINTS
  Apply this to every <file/section/case>, not just the first one.
  <Each constraint explicit — nothing left to be inferred.>
DEFINITION OF DONE
  <A concrete bar, never a qualitative word like "important". The guide's
  worked example: "report any bugs that could cause incorrect behavior, a
  test failure, or a misleading result; only omit nits like pure style or
  naming preferences.">
RETURN FORMAT
  <Exact shape, plus a positive example of the concision you want.>
```

## Phrases that work

Verbatim from the [guide].

- Cut verbosity: `Provide concise, focused responses. Skip non-essential context, and keep examples minimal.`
- Deepen reasoning when effort must stay `low`: `This task involves multistep reasoning. Think carefully through the problem before responding.`
- Suppress over-triggered thinking: `Thinking adds latency and should only be used when it will meaningfully improve answer quality, typically for problems that require multistep reasoning. When in doubt, respond directly.`
- Broaden a scoped instruction: `Apply this formatting to every section, not just the first one`
- Recover code-review recall: `Report every issue you find, including ones you are uncertain about or consider low-severity. Do not filter for importance or confidence at this stage - a separate verification step will do that. Your goal here is coverage: it is better to surface a finding that later gets filtered out than to silently drop a real bug. For each finding, include your confidence level and an estimated severity so a downstream filter can rank them.`

## Anti-patterns

- **Prompting around shallow reasoning.** "Raise effort to `high` or `xhigh` rather than prompting around it." [guide]
- **Negative instructions for style.** "Positive examples showing how Claude can communicate with the appropriate level of concision tend to be more effective than negative examples or instructions that tell the model what not to do." [guide]
- **Interim-status scaffolding** ("After every 3 tool calls, summarize progress") — remove and retest. [guide]
- **Conservative review language** ("only report high-severity issues", "be conservative", "don't nitpick") is followed faithfully and depresses recall — a harness effect, not a capability regression. [guide]
- **Sampling parameters** left in a migrated call: a hard 400. [guide]
- **Assuming an instruction generalizes.** State scope. [guide]
- **Vague steers** ("don't use that color") shift the model to a different fixed default, not to variety. [guide]

## Effort and length guidance

Effort defaults to `high`. `max` = maximum capability, no token constraint; `xhigh` = "recommended setting for the hardest coding and agentic use cases"; `high` = default balance; `medium` = cost-sensitive, trades intelligence; `low` = "short, scoped tasks and latency-sensitive workloads that are not intelligence-sensitive." At `low`/`medium` the model scopes work to what was asked; at `low` on moderately complex tasks there is "some risk of under-thinking." Migration: Sonnet 5 medium ≈ 4.6 high, Sonnet 5 high ≈ 4.6 max — benchmark by thinking length, not effort name. At `high` and above leave `max_tokens` headroom: thinking counts against it, and a tight budget truncates the answer with `stop_reason: "max_tokens"`. [guide]

## For coding agents

- Use `xhigh` or `high` effort, add autonomous features such as an auto mode, and reduce required human interactions. [guide]
- Put task, intent, and constraints in the first turn; progressive underspecification costs tokens and performance. [guide]
- `high`/`xhigh` "show substantially more tool usage in agentic search and coding". With thinking disabled the model reaches for tools less — nudge it explicitly in the system prompt, describing when and how each tool should be used. [guide]
- Split finding from filtering: the finding stage's job is coverage; a separate verification, deduplication, or ranking stage filters. Validate against a subset of your evals for recall or F1. [guide]

Subagents, parallel tool-call batching, test-writing, and edit mechanics are not covered on the page — unverified; do not assume 4.6-era habits carry over.

[guide]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-sonnet-5
