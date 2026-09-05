# Prompting Claude Opus 5

For prompting Opus 5 in an automated coding workflow. Every claim traces to the [guide][g] or [Effort][e]; verified 2026-09-05.

## What is different about this model

- Built for agentic coding, strongest on long-horizon tasks. Opus 4.8 prompts work as-is; below is what needs tuning.[ref][g]
- **It finishes.** On multi-file features and refactors it "completes full tasks rather than leaving stubs or placeholders, and it performs best when given the complete task specification up front and left to run."[ref][cap]
- **It verifies and self-corrects unprompted.**[ref][scope]
- **It talks more**: responses, narration, corrections, written files.[ref][len]
- **It expands scope** and **delegates to subagents more readily**.[ref][sub]
- Review has high precision *and* recall, even at lower effort.[ref][cap]
- 1M context, default and maximum; instruction following, tool calling and reasoning hold across it.[ref][cap]
- Thinking is on by default, disableable only at `high` effort or below.[ref][think]

## Prompt skeleton

The guide gives no template; this orders its snippets. Give the whole spec up front, then leave it to run.[ref][cap]

```text
ROLE      Working in <repo>; what it may touch.
CONTEXT   <Files, decisions, links. Long context is fine.>
TASK      <The whole task. No staged reveals.>
CONSTRAINTS  <Scope paragraph.> <Subagent paragraph.> <Repo rules.>
DONE WHEN <Checks that must pass; files that must exist.>
RETURN    <Document-length line.> Lead with the outcome.
<tone_preference>Keep outputs reasonably concise.</tone_preference>
```

## Phrases that work

Paste, do not paraphrase.

**Scope**[ref][scope]:
> Deliver what was asked, at the scope intended. Make routine judgment calls yourself, and check in only when different readings of the request would lead to materially different work. If the request seems mistaken or a better approach exists, say so in a sentence and continue with the task as asked rather than quietly narrowing, widening, or transforming it. Finish the whole task, and stop short of actions that are clearly beyond what was asked.

**Subagents**[ref][sub]:
> Delegate to a subagent only for large tasks that are genuinely independent and parallelizable, such as a wide multi-file investigation. Do not delegate work you can finish yourself in a handful of tool calls, and do not use subagents to verify or double-check your own work. If one subagent can complete the task, use one rather than several, and keep spawn counts low.

**Deliverable length**[ref][doclen]:
> Match the length of written documents to what the task needs: cover the substance, but do not pad with filler sections, redundant summaries, or boilerplate.

**Narration cadence**[ref][narr]:
> Before your first tool call, say in one sentence what you're about to do. While working, give a brief update only when you find something important or change direction. When you finish, lead with the outcome: your first sentence should answer "what happened" or "what did you find," with supporting detail after it for readers who want it.

**Correction noise**[ref][self]:
> Only correct an earlier statement when the error would change the user's code, conclusions, or decisions. State corrections plainly and briefly, then continue the task. For slips that change nothing for the user, make the fix and move on without noting it.

Near the end of a long system prompt: `<tone_preference>Keep outputs reasonably concise.</tone_preference>`[ref][len]

## Anti-patterns

- "Include a final verification step", "use a subagent to verify" — remove; these and legacy verification scaffolding over-verify.[ref][scope]
- "Double-check your answer", "re-verify before responding" — compounds with existing behaviour.[ref][self]
- In reviews, "only report high-severity issues" or "be conservative" — taken literally, it reports less. Ask for everything; filter separately.[ref][cap]
- Any rule telling it not to think or reason — increases XML tag leakage. Naming `<thinking>` tags is less effective than a general no-internal-tags rule.[ref][think]
- Style rules phrased as prohibitions; positive examples work better.[ref][narr]
- Reusing an earlier model's effort defaults or vision workarounds unswept.[ref][cap]

## Effort and length guidance

- Effort controls thinking volume, not visible length; lowering it does not reliably shorten responses.[ref][len]
- Start at the default `high`. Use `low`/`medium` liberally as the primary cost and latency control wherever evals show quality holds; `xhigh` for demanding agentic work, `max` when the task justifies unconstrained spend.[ref][e5]
- At `xhigh`/`max` set a large `max_tokens` (64k is a reasonable start); thinking cannot be disabled there — those requests 400.[ref][e5]
- Thinking on at `low` beats thinking off at similar cost, for most tasks.[ref][think]
- Per-message effort changes preserve the prompt cache; top-level ones do not.[ref][e5]

## For coding agents

- **Finishing:** complete spec up front, then let it run.[ref][cap] On narrow tasks, paste the scope paragraph verbatim.[ref][scope]
- **Reviews:** fast pass now, thorough pass later — accuracy holds at low effort.[ref][cap]
- **Subagents:** good writer-verifier coordination, few overwrite collisions, but delegation multiplies cost on small work. Cap it: `CLAUDE_CODE_MAX_SUBAGENT_SPAWN_DEPTH`, `CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS`, SDK `max_budget_usd` (Claude Code 2.1.217+). Claude Code adds a delegation instruction only under its `claude_code` preset; otherwise add one yourself.[ref][sub]
- **Written output:** files it writes run long; add the length line to every file-producing task.[ref][doclen]
- **Vision work:** give it tools to analyze, crop and visually verify; cheaper than thinking alone.[ref][cap]
- **Thinking off:** expect occasional tool calls emitted as text (worst on search-heavy loops; leaked text persists in history, tainting later turns) and internal XML tags. One mitigation: "When you use a tool, you may say a brief sentence first. If no tool can express what the user asked for, say so instead of guessing. Do not include internal or system XML tags in your response."[ref][think]

[g]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5
[cap]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#capability-improvements
[len]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#response-length-and-verbosity
[narr]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#user-facing-progress-updates
[doclen]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#written-deliverable-length
[scope]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#task-scope-and-over-verification
[sub]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#controlling-subagent-spawning
[self]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#self-correction
[think]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-opus-5#running-with-thinking-disabled
[e]: https://platform.claude.com/docs/en/build-with-claude/effort
[e5]: https://platform.claude.com/docs/en/build-with-claude/effort#recommended-effort-levels-for-claude-opus-5
