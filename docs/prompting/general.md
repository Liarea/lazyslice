# Prompting: general cheat sheet

Everything below traces to [Prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices).

## What is different about this model

Covers Fable/Mythos 5.1 and 5, Opus 5 and 4.6–4.8, Sonnet 5 and 4.6, Haiku 4.5; [read the per-model page first](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#model-specific-guidance).

- **Verbosity.** Latest models are "[More direct and grounded](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#communication-style-and-verbosity)" and may skip summaries after tool calls. **Opus 5** is the exception: longer by default, and effort does not reliably change visible length — ask for conciseness. **Fable 5.1** writes *fewer* progress updates during agentic work — ask for them, and drop any "keep it brief" instruction.
- **Thinking defaults.** Off when `thinking` is omitted on Opus 4.6–4.8 and Sonnet 4.6; on by default on Opus 5 and Sonnet 5; always on and adaptive-only on Fable/Mythos 5 and 5.1. `budget_tokens` [400s on Claude 4.7 and later](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#leverage-thinking--interleaved-thinking-capabilities).
- **Prefill is gone.** Last-turn assistant prefills [400 on Claude 4.6 and later](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#migrating-away-from-prefilled-responses); use structured outputs, tool enums or XML tags.
- **Self-verification.** "Verify your answer against [test criteria]" works reliably — **except on Opus 5**, which verifies well unaided; remove such instructions when migrating to it.

**Haiku 4.5 in an automated coding workflow:** the page says almost nothing — no per-model guide row, no coding guidance. It appears twice: in the covered-models list, and among the models with [context awareness](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#context-awareness-and-multiwindow-workflows), tracking their remaining token budget, which means it may wrap up early (pair with the "do not stop early" prompt below). Anything more is **unverified**.

## Prompt skeleton

[XML tags](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#structure-prompts-with-xml-tags) throughout; long inputs at the top, query at the end ("[up to 30 percent](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#long-context-prompting)" better).

```xml
<!-- system: You are a [role]. One sentence is enough. -->
<documents><document index="1"><source>…</source><document_content>…</document_content></document></documents>
<context>Why this matters — Claude generalizes from the explanation.</context>
<task>1. … 2. … 3. …</task>   <!-- numbered when order or completeness matters -->
<constraints>What to do, not what not to do.</constraints>
<definition_of_done>Verify your answer against […] before finishing.</definition_of_done>
<return_format>Put X in <x> tags.</return_format>
<examples><example>…</example></examples>  <!-- 3–5, relevant and diverse -->
```

## Phrases that work (verbatim from the page)

- "Include as many relevant features and interactions as possible. Go beyond the basics to create a fully-featured implementation."
- Not "Can you suggest some changes…" but "Change this function to improve its performance." or "Make these edits to the authentication flow."
- "Your response should be composed of smoothly flowing prose paragraphs." (not "Do not use markdown")
- "After receiving tool results, carefully reflect on their quality and determine optimal next steps."
- "choose an approach and commit to it… If you're weighing two approaches, pick one and see it through."
- "Never speculate about code you have not opened… you MUST read the file before answering."
- "If you create any temporary new files, scripts, or helper files for iteration, clean up these files… at the end of the task."

## Anti-patterns

- Vague asks ("Create an analytics dashboard"). [Golden rule](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#be-clear-and-direct): if a colleague with minimal context would be confused, so will Claude.
- Bare prohibitions with no reason ("NEVER use ellipses") instead of the motivation.
- "CRITICAL: You MUST use this tool when…" — Opus 4.5/4.6 are more system-prompt-responsive; say "Use this tool when…". Likewise "If in doubt, use [tool]" now [overtriggers](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#overthinking-and-excessive-thoroughness).
- A heavy anti-markdown block on Fable 5.1, which already formats sparsely.
- Prescriptive reasoning plans: "[Prefer general instructions over prescriptive steps](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#leverage-thinking--interleaved-thinking-capabilities)."
- Removing tests, hardcoding to test inputs, guessed tool parameters.
- "think" when thinking is disabled on Opus 4.5 — use "consider" or "evaluate".

## Effort and length guidance

Control depth with `effort` plus `max_tokens` under `thinking: {type: "adaptive"}`, not `budget_tokens`; lower effort when the model over-explores. Length is a separate lever — ask for conciseness in words. To suppress needless thinking: "Thinking adds latency and should only be used when it will meaningfully improve answer quality… When in doubt, respond directly."

## For coding agents

- **Finishing.** "do not stop tasks early due to token budget concerns… Never artificially stop any task early", and "It's encouraged to spend your entire output context working on the task - just make sure you don't run out of context with significant uncommitted work."
- **Scope.** Against [overeagerness](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#overeagerness): "Only make changes that are directly requested or clearly necessary… A bug fix doesn't need surrounding code cleaned up." No docstrings on untouched code, no guards for impossible states.
- **Tests.** Write them first, track in `tests.json`, and say "It is unacceptable to remove or edit tests". "Tests are there to verify correctness, not to define the solution"; report infeasible tasks or wrong tests "rather than working around them".
- **Edits.** Say "change"/"make these edits", not "suggest". Set the default with `<default_to_action>` or `<do_not_act_before_instructions>`.
- **Subagents.** Claude delegates natively; Opus 4.6 and 5 overuse it. Damp: "Use subagents when tasks can run in parallel, require isolated context, or involve independent workstreams… For simple tasks… work directly rather than delegating."
- **Batching.** "make all of the independent tool calls in parallel… Never use placeholders or guess missing parameters." On Fable 5.1, resend this as a turn-scoped system message after each round of tool results.
- **State across windows.** Git as the log, JSON for structured state, freeform progress notes; prefer a fresh window over compaction, and be prescriptive on restart ("Review progress.txt, tests.json, and the git logs").
- **Risky actions.** "for actions that are hard to reverse, affect shared systems, or could be destructive, ask the user before proceeding… don't bypass safety checks (e.g. --no-verify)".

