# Prompting cheat sheet

Every line traces to [Prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices), read 2026-09-05. Quotes verbatim.

## What is different about this model

It covers Fable 5.1/5, Mythos 5.1/5, Opus 5 and 4.6–4.8, Sonnet 5 and 4.6, and Haiku 4.5; read the [model-specific guidance](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#model-specific-guidance) table first.

- Current models are "more direct and grounded" and "less verbose", and may skip summaries after tool calls.
- **Opus 5 inverts this**: responses run longer and `effort` "does not reliably change visible response length". Ask for concision explicitly. It self-verifies well unprompted, so "remove these instructions rather than rewriting them" when migrating.
- **Fable 5.1** writes *fewer* progress updates in agent loops; ask for them and drop any "keep it brief" line. It formats less already, so anti-markdown blocks suppress needed structure.
- Instruction following is literal: "can you suggest some changes" gets suggestions, not edits.
- Opus 4.5/4.6 respond more to system prompts, so old anti-laziness prompting now **over**triggers. Opus 4.6 over-explores and has "a strong predilection for subagents"; so does Opus 5.
- Thinking is adaptive: on by default for Opus 5 / Sonnet 5, always on for Fable/Mythos 5.x, off unless asked for on Opus 4.6–4.8 and Sonnet 4.6. `budget_tokens` is deprecated and 400s on Claude 4.7+.
- Prefilled last assistant turns 400 from Claude 4.6 on. Keep history append-only and return thinking blocks unchanged; editing earlier turns invalidates later ones.

## Prompt skeleton

XML tags with "consistent, descriptive tag names"; long inputs first, since queries at the end "can improve response quality by up to 30 percent".

```xml
You are a helpful coding assistant specializing in Python.

<context>Why this matters, who reads it.</context>
<documents><document index="1"><source>schema.sql</source><document_content>…</document_content></document></documents>
<instructions>Numbered steps when order matters. What to do, not what not to do.</instructions>
<constraints>Scope, tools, safety rules.</constraints>
<examples><example>3–5 relevant, diverse examples.</example></examples>
<definition_of_done>Before you finish, verify your answer against [test criteria].</definition_of_done>
<output_format>Write the prose sections in <smoothly_flowing_prose_paragraphs> tags.</output_format>
```

Golden rule: "Show your prompt to a colleague with minimal context… If they'd be confused, Claude will be too."

## Phrases that work

- "Include as many relevant features and interactions as possible. Go beyond the basics to create a fully-featured implementation."
- "Change this function to improve its performance." / "Make these edits to the authentication flow."
- "By default, implement changes rather than only suggesting them." (Inverse: "Do not jump into implementation or change files unless clearly instructed.")
- "If you intend to call multiple tools and there are no dependencies between the tool calls, make all of the independent tool calls in parallel… Never use placeholders."
- "Never speculate about code you have not opened… read the file before answering."
- "It is unacceptable to remove or edit tests because this could lead to missing or buggy functionality."

## Anti-patterns

- Vague asks; negative-only format rules; rules with no reason given.
- Shouty over-prompting ("CRITICAL: You MUST…"), "Default to using [tool]", "If in doubt, use [tool]". Prefer "Use [tool] when it would enhance your understanding."
- Prefill; `budget_tokens`; hand-written reasoning plans ("think thoroughly" beats a prescribed sequence); anti-markdown blocks on Fable 5.1.

## Effort and length

`effort` plus query complexity sets thinking depth; lower it when a model over-explores, and use `max_tokens` as the ceiling. It does not control Opus 5's visible length. To cut latency: "Thinking adds latency and should only be used when it will meaningfully improve answer quality… When in doubt, respond directly."

## For coding agents

- **Finishing:** "do not stop tasks early due to token budget concerns… Never artificially stop any task early"; use the whole output context, but "don't run out of context with significant uncommitted work."
- **Scope:** nothing beyond what was asked; no docstrings on code you didn't change; no handling for impossible cases ("Only validate at system boundaries"); no abstractions for one-off operations.
- **Tests:** write them first into a structured `tests.json`, never delete them. "Tests are there to verify correctness, not to define the solution. Do not hard-code values or create solutions that only work for specific test inputs." If one is wrong, "inform me rather than working around them".
- **Edits:** read before claiming; standard tools over helper scripts; clean up temp files. Confirm before destructive or shared-system actions (`rm -rf`, `git push --force`, PR comments); "don't bypass safety checks (e.g. --no-verify)".
- **Subagents:** native, no prompting needed; damp overuse — "For simple tasks, sequential operations, single-file edits… work directly rather than delegating."
- **Batching:** independent calls already run in parallel; the phrase above pushes it to ~100%. On Fable 5.1, resend it as a turn-scoped system message after each round of tool results.
- **State:** a distinct first-window prompt (write tests, `init.sh`), then git logs, `progress.txt`, `tests.json`; prefer a fresh window over compaction, restarting prescriptively ("Call pwd; you can only read and write files in this directory.").

## Haiku 4.5 in an automated coding workflow

One thing is specific to it: Haiku 4.5, with Sonnet 5/4.6/4.5, has [context awareness](https://platform.claude.com/docs/en/build-with-claude/context-windows#context-awareness) — it tracks its remaining token budget, which "enables Claude to execute tasks and manage context more effectively". So it "may sometimes naturally try to wrap up work as it approaches the context limit": if the harness compacts or saves state to files, say so, with the never-stop-early phrasing above. It has no row in the model-specific table and no prompting page of its own, so everything else here is the all-models advice; its thinking and `effort` defaults are not stated — **unverified**.
