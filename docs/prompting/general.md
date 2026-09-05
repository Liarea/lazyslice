# Prompting current Claude models

Operational cheat sheet. Every line traces to [Prompting best practices][pbp] (fetched 2026-09-04); anything else is marked unverified.

## What is different about this model

The page covers Fable 5.1/Mythos 5.1, Fable 5/Mythos 5, Opus 5, Opus 4.8/4.7/4.6, Sonnet 5/4.6 and Haiku 4.5, and says to read the [per-model page][model] first ([intro][pbp]).

- **Less verbose**: the model "may skip verbal summaries after tool calls, jumping directly to the next action" ([verbosity][verb]).
- **Opus 5 is the exception** — longer by default, and effort does not reliably change visible length. Fable 5.1 is the opposite in agentic work: fewer user-facing updates, so ask for progress text and drop any "keep it brief" instruction ([verbosity][verb]).
- **Literal instruction following**: "Can you suggest some changes" gets suggestions, not edits ([tool usage][tools]).
- **Thinking is adaptive**: `thinking: {type: "adaptive"}` on 4.6+; always on and the only mode on the 5-series ([thinking][think]).
- **Prefilled assistant turns return a 400** from 4.6 onward ([prefill][prefill]).
- **Parallel tool calls and subagents happen unprompted**, with overuse a real risk ([parallel][par], [subagents][sub]).
- **Context awareness**: Sonnet 5/4.6/4.5 and Haiku 4.5 track their remaining token budget ([context awareness][ctx]).

## Prompt skeleton

XML tags keep instructions, context, examples and inputs distinct; use consistent descriptive names ([XML][xml]). Put long inputs (20k+ tokens) **above** the query — queries at the end can improve quality by up to 30% ([long context][long]).

```xml
system: You are a <role>.                  <!-- one sentence helps: [role] -->

<documents><document index="1"><source>…</source>
<document_content>…</document_content></document></documents>
<context>Why this matters: …</context>      <!-- motivation generalizes: [context] -->
<task>1. … 2. … 3. …</task>                <!-- numbered if order matters: [clear] -->
<constraints>…</constraints>
<examples><example>…</example></examples>  <!-- 3–5, diverse: [examples] -->
<definition_of_done>…</definition_of_done> <!-- success criteria: [research] -->
<return_format>Prose in <answer> tags.</return_format>
```

## Phrases that work (verbatim from the page)

- Scope creep: "Avoid over-engineering. Only make changes that are directly requested or clearly necessary." ([overeagerness][over])
- Grounding: "Never speculate about code you have not opened." ([hallucinations][hall])
- Tests: "Tests are there to verify correctness, not to define the solution." / "It is unacceptable to remove or edit tests because this could lead to missing or buggy functionality." ([tests][testing], [multi-window][multi])
- Batching: "make all of the independent tool calls in parallel" / "Never use placeholders or guess missing parameters in tool calls." ([parallel][par])
- Committing: "choose an approach and commit to it." ([overthinking][ot])
- Cleanup: "If you create any temporary new files, scripts, or helper files… remove them at the end of the task." ([file creation][files])
- Going further: "Include as many relevant features and interactions as possible. Go beyond the basics." ([clear][clear])

## Anti-patterns

- Saying what **not** to do instead of what to do; "Your response should be composed of smoothly flowing prose paragraphs" beats "Do not use markdown" ([format][fmt]).
- Shouting: "CRITICAL: You MUST use this tool when…" now overtriggers; "Use this tool when…" is the fix ([tool usage][tools]).
- Blanket defaults ("Default to using [tool]", "If in doubt, use [tool]") overtrigger ([overthinking][ot]).
- Leftover anti-laziness prompting from older generations ([migration][mig]).
- Vague asks: a colleague with minimal context should be able to follow it ([clear][clear]).
- A heavy anti-markdown block on Fable 5.1, which already formats less ([format][fmt]).
- Editing earlier turns or rebuilding `system`/`tools` mid-run: keep history append-only, pass thinking blocks back unchanged ([migration][mig]).

## Effort and length guidance

Control depth with `effort`, not `budget_tokens` — deprecated on 4.6/Sonnet 4.6, a 400 on 4.7 and later; `max_tokens` is the hard ceiling, and lower effort is the fallback when the model explores too much ([overthinking][ot]). To damp thinking: "Thinking adds latency and should only be used when it will meaningfully improve answer quality… When in doubt, respond directly." ([thinking][think]). Length is a separate lever — ask for conciseness or for summaries explicitly ([verbosity][verb]).

## For coding agents

- **Finishing**: say compaction is automatic — "do not stop tasks early due to token budget concerns… Never artificially stop any task early" ([context awareness][ctx]) — and "It's encouraged to spend your entire output context working on the task" ([multi-window][multi]).
- **Scope**: paste the over-engineering block — no unrequested features, no docstrings on untouched code, no defensive coding for impossible cases, no one-off abstractions ([overeagerness][over]).
- **Tests**: write tests first, track them in `tests.json`-style structured state, keep freeform progress notes, use git, ask for incremental progress ([multi-window][multi], [state][state]).
- **Edits**: be imperative ("Change this function…", "Make these edits…"), or install `<default_to_action>`; `<do_not_act_before_instructions>` for the opposite ([tool usage][tools]). Require confirmation for destructive or shared-system actions; forbid `--no-verify`-style shortcuts ([autonomy][auto]).
- **Subagents**: damp with "Use subagents when tasks can run in parallel, require isolated context, or involve independent workstreams… For simple tasks, sequential operations, single-file edits… work directly rather than delegating." ([subagents][sub]).
- **Batching**: the parallel-calls block lifts success toward ~100%; on Fable 5.1 in long loops resend it as a turn-scoped system message after each round of tool results ([parallel][par]).
- **Fresh windows over compaction**: be prescriptive — "Call pwd…", "Review progress.txt, tests.json, and the git logs." ([multi-window][multi]).

## Haiku 4.5 in an automated coding workflow

The page gives Haiku 4.5 no row in the model-specific table and no coding-agent guidance of its own; it names the model twice — among the covered models, and as one with context awareness ([context awareness][ctx]). So: apply the general agentic guidance, and pair the compaction/persistence prompt above with the memory tool, which "pairs well with context awareness" ([context awareness][ctx]). Anything more is unverified from this page.

[pbp]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices
[model]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#model-specific-guidance
[clear]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#be-clear-and-direct
[context]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#add-context-to-improve-performance
[examples]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#use-examples-effectively
[xml]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#structure-prompts-with-xml-tags
[role]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#give-claude-a-role
[long]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#long-context-prompting
[verb]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#communication-style-and-verbosity
[fmt]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#control-the-format-of-responses
[prefill]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#migrating-away-from-prefilled-responses
[tools]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#tool-usage
[par]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#optimize-parallel-tool-calling
[ot]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#overthinking-and-excessive-thoroughness
[think]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#leverage-thinking--interleaved-thinking-capabilities
[ctx]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#context-awareness-and-multiwindow-workflows
[multi]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#workflows-across-multiple-context-windows
[state]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#state-management-best-practices
[auto]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#balancing-autonomy-and-safety
[research]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#research-and-information-gathering
[sub]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#subagent-orchestration
[files]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#reduce-file-creation-in-agentic-coding
[over]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#overeagerness
[testing]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#avoid-focusing-on-passing-tests-and-hardcoding
[hall]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#minimizing-hallucinations-in-agentic-coding
[mig]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices#migration-considerations
