# Prompting Claude Opus 5

For prompting Opus 5 in an automated coding workflow. Claims trace to the [guide] or [effort doc]; verified 2026-09-05.

## What is different about this model

- "Performs well out of the box on existing Claude Opus 4.8 prompts": tune, don't rewrite. [guide]
- **Finishes tasks:** "completes full tasks rather than leaving stubs or placeholders, and it performs best when given the complete task specification up front and left to run". [guide]
- **Verifies itself:** verification instructions "cause over-verification"; removing them "reduces wasted tokens with no loss in quality". [guide]
- **Talks more:** responses, narration, and written files run longer than prior Opus. [guide]
- **Expands scope:** "adding steps that weren't requested". [guide]
- **Delegates readily;** it "multiplies cost and time when applied to small tasks". [guide]
- 1M context, default and maximum; behaviour "consistent throughout the window". [guide]

## Prompt skeleton

Constraints and return-format wording verbatim from the [guide].

```text
ROLE     You are working in <repo>, on <component>.
CONTEXT  <Files, decisions, commands. Front-load everything.>
TASK     <The complete spec, once, in full. Not a first step.>
CONSTRAINTS
  Deliver what was asked, at the scope intended. Make routine judgment calls
  yourself, and check in only when different readings of the request would lead
  to materially different work. If the request seems mistaken or a better
  approach exists, say so in a sentence and continue with the task as asked
  rather than quietly narrowing, widening, or transforming it. Finish the whole
  task, and stop short of actions that are clearly beyond what was asked.
  Write only the files named above.
  Delegate to a subagent only for large tasks that are genuinely independent and
  parallelizable. Do not delegate work you can finish yourself in a handful of
  tool calls, and do not use subagents to verify or double-check your own work.
DEFINITION OF DONE
  <Named checks that must pass, output pasted.> No stubs or TODOs.
RETURN FORMAT
  Lead with the outcome: your first sentence should answer "what happened" or
  "what did you find," with supporting detail after it.
  Match the length of written documents to what the task needs: cover the
  substance, but do not pad with filler sections, redundant summaries, or
  boilerplate.
```

## Phrases that work

From the [guide], verbatim.

- Concision: "Keep responses focused, brief, and concise. Keep disclaimers and caveats short, and spend most of the response on the main answer."
- Tail reminder in a long system prompt: `<tone_preference>Keep outputs reasonably concise.</tone_preference>`
- Narration: "Before your first tool call, say in one sentence what you're about to do. While working, give a brief update only when you find something important or change direction."
- Corrections: "Only correct an earlier statement when the error would change the user's code, conclusions, or decisions."
- Thinking disabled, against leaked tool calls and XML tags: "When you use a tool, you may say a brief sentence first. If no tool can express what the user asked for, say so instead of guessing. Do not include internal or system XML tags in your response."

## Anti-patterns

All from the [guide].

- "include a final verification step…" / "use a subagent to verify" — remove, with scaffolding that does the same.
- "double-check your answer" / "re-verify before responding" — these "add cost without improving results".
- In review prompts, "only report high-severity issues" or "be conservative" — it "may follow that instruction literally and report less".
- Lowering effort to shorten output: it "controls how much the model thinks rather than how much it says".
- Telling it not to think or reason — that "increases tag leakage"; naming `<thinking>` tags is "less effective than the general form".
- Only stating what not to do: "Positive examples… tend to be more effective"; reusing a prior model's effort defaults or vision workarounds unvalidated.

## Effort and length guidance

- Levels `low`/`medium`/`high`/`xhigh`/`max`; API default `high`. [effort doc]
- Start at `high`; `xhigh` for demanding coding and agentic work, `max` for unconstrained spend; "use `low` and `medium` liberally as your primary control for token cost and response time wherever your evals show quality holds". Sweep afresh. [effort doc]
- At `xhigh`/`max` set a large `max_tokens`; 64k is a fair default. [effort doc]
- Thinking is on by default and cannot be disabled at `xhigh`/`max` (400 error). Prefer low effort with thinking on: it "performs better than thinking disabled at similar cost". [guide] [effort doc]
- Length is a prompt problem: ask explicitly, plus a reminder near the end of a long prompt. [guide]

## For coding agents

From the [guide] unless noted.

- **Finishing:** hand over the whole spec and let it run; strongest on "multi-file features, larger refactors, and end-to-end feature work".
- **Scope:** paste the constraints paragraph above — the guide's remedy.
- **Tests:** keep real gates (lint/test) as definition-of-done; drop "verify your work".
- **Review:** accuracy "holds at lower effort settings" — fast pass now, thorough later.
- **Subagents:** cap them — `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`, `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`, `max_budget_usd`, needing Claude Code 2.1.217+. Claude Code adds a delegation instruction only under its `claude_code` system prompt preset; otherwise add one yourself.
- **Multi-agent:** writer-verifier patterns work well.
- **Vision and UI:** give it tools to "iteratively analyze, crop, and visually verify" — cheaper than thinking alone.
- **Batching:** the guide gives none; lower effort does "combine multiple operations into fewer tool calls" [effort doc]. More: unverified.

[guide]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5
[effort doc]: https://platform.claude.com/docs/en/build-with-claude/effort
