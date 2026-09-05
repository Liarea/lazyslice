# Prompting Claude Opus 4.8

For prompts written into an automated coding workflow. Every claim traces to the
[Opus 4.8 prompting page][p48]. Verified 2026-09-05.

## What is different about this model

All from [p48].

- Strong at "long-horizon agentic work, knowledge work, vision, and memory tasks"; it "performs well out of the box on existing Claude Opus 4.7 prompts".
- **Length is calibrated, not fixed:** shorter on lookups, "much longer" on open-ended analysis.
- **Literal.** It "interprets prompts literally and explicitly, particularly at lower effort levels", "does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make".
- **Prefers reasoning over tool calls.** Effort is the lever that raises tool usage.
- **Spawns fewer subagents** by default, though the behaviour "is steerable through prompting".
- **Thinking is off** unless you set `thinking: {type: "adaptive"}`.
- **Better at finding bugs**, yet an older harness can *measure* lower recall because 4.8 obeys "don't nitpick" faithfully — "a harness effect, not a capability regression".
- **More regular, higher-quality progress updates** in long agentic traces.
- **Direct, opinionated prose;** a persistent cream/serif frontend house style that "will feel off for dashboards, dev tools".

## Prompt skeleton

Per [p48]: "specify the task, intent, and relevant constraints upfront in the
first human turn". The tags are a container; the page prescribes no markup.

```text
<role>Who the model is for this task.</role>
<context>Repo, phase, files that matter, why this task exists.</context>
<task>The complete spec, first turn, no follow-up needed.</task>
<constraints>Scope per item, never implied ("every section, not just
the first one"). Files you may write; what not to touch.</constraints>
<definition_of_done>A concrete bar, not a qualitative word: checks that
must pass, artefacts that must exist.</definition_of_done>
<return_format>Exactly what to return, and in what shape.</return_format>
```

Literalism means every implicit "and the rest" belongs in `<constraints>`, and a
vague bar gets obeyed as written — the page's model of a concrete one: "report any
bugs that could cause incorrect behavior, a test failure, or a misleading result;
only omit nits like pure style or naming preferences". [p48]

## Phrases that work

Verbatim from [p48].

- Verbosity: "Provide concise, focused responses. Skip non-essential context, and keep examples minimal."
- Depth while pinned at low effort: "This task involves multistep reasoning. Think carefully through the problem before responding."
- Over-triggered thinking: "Thinking adds latency and should only be used when it will meaningfully improve answer quality — typically for problems that require multistep reasoning. When in doubt, respond directly."
- Subagents: "Do not spawn a subagent for work you can complete directly in a single response (e.g. refactoring a function you can already see). / Spawn multiple subagents in the same turn when fanning out across items or reading multiple files."
- Review coverage: "Report every issue you find, including ones you are uncertain about or consider low-severity. Do not filter for importance or confidence at this stage - a separate verification step will do that. Your goal here is coverage: it is better to surface a finding that later gets filtered out than to silently drop a real bug. For each finding, include your confidence level and an estimated severity so a downstream filter can rank them."
- Generalisation: "Apply this formatting to every section, not just the first one."

## Anti-patterns

All from [p48].

- **Prompting around shallow reasoning.** "Raise effort to `high` or `xhigh` rather than prompting around it."
- **Interim-status scaffolding** ("After every 3 tool calls, summarize progress") — "try removing it".
- **Negative instructions.** "Positive examples … tend to be more effective than negative examples or instructions that tell the model what not to do."
- **Qualitative bars in review prompts** — "be conservative", "don't nitpick", "important". They now get followed.
- **Generic design negations** ("don't use cream"), which "shift the model to a different fixed palette rather than producing variety". Give a concrete spec instead.
- **Small output budgets at high effort.** At `max`/`xhigh`, "start at 64k tokens and tune from there".

## Effort and length guidance

"Effort is likely to be more important for this model than for any prior Opus." [p48]

- `max` — intelligence-demanding tasks; diminishing returns, "prone to overthinking".
- `xhigh` — "the best setting for most coding and agentic use cases"; our default.
- `high` — the floor for intelligence-sensitive work.
- `medium` — cost-sensitive work, trading off intelligence.
- `low` — "short, scoped tasks and latency-sensitive workloads".

It "respects effort levels strictly": at `low` and `medium` it "scopes its work to
what was asked rather than going above and beyond", risking under-thinking. [p48]
Control length by prompt; do not starve effort to get brevity.

## For coding agents

All from [p48].

- **Effort:** start at `xhigh`; `xhigh` or `high` for interactive products.
- **Autonomy over turns:** it "reasons more after user turns", so "add autonomous features like an auto mode, and reduce the number of human interactions required from your users".
- **Scope:** it will not widen a task by itself; state anything you want generalised.
- **Tool use:** if a tool is under-used, raise effort first, then "clearly describe why and how it should" use it.
- **Subagents and batching:** it under-delegates; state the rule — none for work doable in one response, several in one turn "when fanning out across items or reading multiple files".
- **Review stages:** split finding from filtering; at the finding stage the job "is coverage rather than filtering". Iterate prompts "against a subset of your evals".

**Unverified:** the page says nothing about task-completion/stub behaviour, tests
as verification, or file-edit mechanics — general practice, not 4.8 guidance.

[p48]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-4-8
