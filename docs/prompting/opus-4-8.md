# Prompting Claude Opus 4.8

For prompts written into an automated coding workflow. Every claim traces to the
[Opus 4.8 prompting page][p48]. Verified 2026-09-04.

## What is different about this model

- Strong at "long-horizon agentic work, knowledge work, vision, and memory tasks"; it "performs well out of the box on existing Claude Opus 4.7 prompts" — tune only the below.
- **Length is calibrated,** not fixed: shorter on lookups, "much longer" on open-ended analysis.
- **Literal.** It "interprets prompts literally and explicitly, particularly at lower effort levels", "does not silently generalize an instruction from one item to another, and it does not infer requests you didn't make".
- **Prefers reasoning to tool calls;** effort is the lever that raises tool usage.
- **Spawns fewer subagents** by default, but is steerable by prompt.
- **Thinking is off** unless you set `thinking: {type: "adaptive"}`.
- **Better at finding bugs,** yet a harness tuned for an older model can *measure* lower recall, because 4.8 obeys "don't nitpick" more faithfully.
- **More regular, higher-quality progress updates** during long traces.
- **Direct, opinionated prose:** "minimal validation-forward phrasing and sparing emoji use".
- **Persistent design default:** cream `#F4F1EA`, serif display type, terracotta accent — wrong for dev tools and dashboards.

## Prompt skeleton

Give "the task, intent, and relevant constraints upfront in the first human turn", scope stated explicitly:

```text
<role>Who the model is for this task.</role>
<context>Repo, phase, files that matter, why the task exists.</context>
<task>The complete, accurate spec, up front. Numbered where order matters.</task>
<constraints>
Scope written out per item, never implied
("Apply this to every section, not just the first one").
Files you may write; what not to touch.
</constraints>
<definition_of_done>Checks that must pass; artefacts that must exist.</definition_of_done>
<return_format>What to return, in what shape.</return_format>
```

Because 4.8 is literal, every implicit "and the rest" belongs in `<constraints>`. Because it is autonomous, front-load the whole spec: "ambiguous or underspecified prompts conveyed progressively over multiple user turns tend to relatively reduce token efficiency and sometimes performance".

## Phrases that work

Verbatim from the page.

- Cut verbosity: "Provide concise, focused responses. Skip non-essential context, and keep examples minimal."
- Depth while pinned at low effort: "This task involves multistep reasoning. Think carefully through the problem before responding."
- Over-triggered thinking: "Thinking adds latency and should only be used when it will meaningfully improve answer quality — typically for problems that require multistep reasoning. When in doubt, respond directly."
- Subagents: "Do not spawn a subagent for work you can complete directly in a single response (e.g. refactoring a function you can already see). / Spawn multiple subagents in the same turn when fanning out across items or reading multiple files."
- Review coverage: "Report every issue you find, including ones you are uncertain about or consider low-severity. Do not filter for importance or confidence at this stage - a separate verification step will do that. Your goal here is coverage: it is better to surface a finding that later gets filtered out than to silently drop a real bug. For each finding, include your confidence level and an estimated severity so a downstream filter can rank them."
- Generalisation: "Apply this formatting to every section, not just the first one."

## Anti-patterns

- **Prompting around shallow reasoning.** "Raise effort to `high` or `xhigh` rather than prompting around it"; under-thinking is an effort problem first.
- **Interim-status scaffolding** ("After every 3 tool calls, summarize progress") — try removing it.
- **Negative instructions.** "Positive examples… tend to be more effective than negative examples or instructions that tell the model what not to do."
- **Qualitative bars in review prompts** ("be conservative", "important"). Be concrete: bugs causing "incorrect behavior, a test failure, or a misleading result", omitting only style and naming nits.
- **Generic design negations** ("don't use cream") — they shift it "to a different fixed palette rather than producing variety". Give a concrete palette and type spec, or have it propose directions first.
- **Small output budgets at high effort.** At `max`/`xhigh`, "start at 64k tokens and tune from there".

## Effort and length guidance

"Effort is likely to be more important for this model than for any prior Opus."

- `max` — intelligence-demanding tasks; diminishing returns, "sometimes prone to overthinking".
- `xhigh` — "the best setting for most coding and agentic use cases"; our default.
- `high` — balances tokens and intelligence; the floor for intelligence-sensitive work.
- `medium` — cost-sensitive work, trading off intelligence.
- `low` — "short, scoped tasks and latency-sensitive workloads that are not intelligence-sensitive".

It "respects effort levels strictly": at `low`/`medium` it "scopes its work to what was asked rather than going above and beyond", risking under-thinking. Control length by prompt; do not starve effort for brevity.

## For coding agents

- **Effort:** start at `xhigh`; `xhigh` or `high` for interactive products.
- **Autonomy over turns:** it "tends to use more tokens in interactive settings… because it reasons more after user turns". For performance *and* efficiency, "add autonomous features like an auto mode, and reduce the number of human interactions required from your users".
- **Scope:** it will not widen a task by itself, so state anything you want generalised.
- **Tool use:** if a tool is under-used, raise effort first, then "clearly describe why and how it should" use it.
- **Subagents and batching:** it under-delegates; state the rule — none for work doable in one response, several in one turn "when fanning out across items or reading multiple files".
- **Review stages:** split finding from filtering; the finding stage's job "is coverage rather than filtering". Iterate prompts "against a subset of your evals or test cases".

**Unverified:** the page says nothing about stub/task-completion behaviour, tests as verification, or edit mechanics. Treat those as general practice, not 4.8 guidance.

[p48]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-4-8
