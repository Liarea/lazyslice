# Prompting Claude Opus 5

For prompts in an automated coding workflow. Claims trace to the [Opus 5 page][p5];
[eff] and [bp] mark the [Effort][eff] and [best practices][bp] pages. Verified 2026-09-04.

## What is different about this model

- **Finishes tasks:** "completes full tasks rather than leaving stubs or placeholders", and "performs best when given the complete task specification up front and left to run". [p5]
- **Verifies itself**, and "catches and fixes its own mistakes well without prompting". [p5]
- **Talks more:** responses "run longer than prior Opus models'", it "narrates readily during agentic work", and files it writes are "often longer". [p5]
- **Widens scope:** it "can... expand the scope of a task, adding steps that weren't requested". [p5]
- **Delegates readily** to subagents, and coordinates them well. [p5]
- 1M context, default and maximum; behaviour "stay[s] consistent throughout the window". [p5]

## Prompt skeleton

XML tagging and explicit constraints follow general advice. [bp]

```text
<role>Who the model is for this task.</role>
<context>Repo, phase, files that matter, why this task exists.</context>
<task>Complete spec, up front. Numbered steps where order matters.</task>
<constraints>
{scope paragraph + document-length line from "Phrases that work"}
{project rules: files you may write, what not to touch}
</constraints>
<definition_of_done>Checks that must pass, artefacts that must exist.</definition_of_done>
<return_format>What to return, in what shape.</return_format>
<tone_preference>Keep outputs reasonably concise.</tone_preference>
```

The tone tag is [p5]'s reminder for the end of a long system prompt.

## Phrases that work

Verbatim [p5].

- Scope: "Deliver what was asked, at the scope intended. Make routine judgment calls yourself, and check in only when different readings of the request would lead to materially different work. If the request seems mistaken or a better approach exists, say so in a sentence and continue with the task as asked rather than quietly narrowing, widening, or transforming it. Finish the whole task, and stop short of actions that are clearly beyond what was asked."
- Documents: "Match the length of written documents to what the task needs: cover the substance, but do not pad with filler sections, redundant summaries, or boilerplate."
- Conciseness: "Keep responses focused, brief, and concise... give a high-level summary unless an in-depth explanation is specifically requested."
- Narration: "Before your first tool call, say in one sentence what you're about to do. While working, give a brief update only when you find something important or change direction. When you finish, lead with the outcome."
- Subagents: "Delegate to a subagent only for large tasks that are genuinely independent and parallelizable... Do not delegate work you can finish yourself in a handful of tool calls, and do not use subagents to verify or double-check your own work... keep spawn counts low."
- Thinking disabled: "When you use a tool, you may say a brief sentence first. If no tool can express what the user asked for, say so instead of guessing. Do not include internal or system XML tags in your response."

## Anti-patterns

All [p5].

- **Verification scaffolding** ("include a final verification step", "use a subagent to verify"), harness steps included: removing it "reduces wasted tokens with no loss in quality".
- **Re-check nags** ("double-check your answer", "re-verify before responding"): they "add cost without improving results".
- **Conservative review framing** ("only report high-severity issues", "be conservative"): taken literally; "report everything and filter in a separate pass instead".
- **Using effort to shorten output:** it controls thinking, not length. "To control response length, prompt for it explicitly."
- **Rules against thinking**, or naming `<thinking>` tags: both increase tag leakage; the general form works better.
- **Negative framing:** "Positive examples... are more effective than instructions about what not to do."
- **Vision workarounds** tuned for prior models: they "may no longer be needed".

## Effort and length guidance

- Levels `low`/`medium`/`high`(default)/`xhigh`/`max`. "Start with `high`, the default"; `xhigh` for demanding coding and agentic work, `max` when the task justifies unconstrained spending; use `low` and `medium` "liberally as your primary control for token cost and response time wherever your evals show quality holds". [eff]
- "If you carried effort defaults over from a prior model, re-run an effort sweep on your own evals." [p5]
- Thinking is on by default, disableable only at `high` or below (`xhigh`/`max` return 400). [eff] Prefer low effort over disabling it: "thinking enabled at `low` effort performs better than thinking disabled at similar cost". [p5]
- At `xhigh`/`max` set a large `max_tokens`; "Starting at 64k tokens... is a reasonable default". [eff]

## For coding agents

- **Give the whole spec up front and let it run** — strongest on "multi-file features, larger refactors, and end-to-end feature work". [p5]
- **Add no verify-then-report step;** it already does it. [p5]
- **Constrain scope** with the scope paragraph — the lever for "write only the files your task names". [p5]
- **Code review:** fast pass at low effort, thorough pass later — "Accuracy holds at lower effort settings". [p5]
- **Subagents:** paste the delegation phrase and cap with `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`, `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`, or the SDK's `max_budget_usd` (Claude Code 2.1.217+); Claude Code supplies its own instruction only under the `claude_code` preset. [p5]
- **Batching** and **tests** are absent from [p5]: use the general `<use_parallel_tool_calls>` block and anti-hardcoding prompt ("Tests are there to verify correctness, not to define the solution"). [bp]
- **Vision work:** give tools to "iteratively analyze, crop, and visually verify" — "more cost-effective... than thinking alone". [p5]

[p5]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5
[eff]: https://platform.claude.com/docs/en/build-with-claude/effort#recommended-effort-levels-for-claude-opus-5
[bp]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
