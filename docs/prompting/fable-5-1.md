# Prompting Claude Fable 5.1 — operational cheat sheet

Everything traces to [Prompting Claude Fable 5.1][page] (fetched 2026-09-05); it also covers Claude Mythos 5.1. Quoted blocks are verbatim.

## What is different about this model

- Fable 5 prompts "should perform well on Claude Fable 5.1 without changes" — what follows is deltas, not a rewrite. [src][page]
- Writes **fewer user-facing updates** in long tool-calling turns; worse at higher effort and in longer chains. [src][updates]
- In coding/computer-use loops it may issue **one tool call per turn** when the next calls are implied, not named. [src][batch]
- **Thinking blocks are conversation-bound** (accounts created on/after 2026-08-31): replaying one after the prefix changed returns a 400. [src][append]
- Prose is **denser**; chat formatting is **sparser** (less bold, fewer headers/lists) — old anti-formatting rules now backfire. [src][density], [src][format]
- More likely to reproduce source passages **unmarked as quotations**. [src][quote]
- More likely to **rewrite whole files** instead of editing surgically. [src][edits]
- Delivers what's asked "and sometimes more": nearby fixes, unrequested extensions, extra test files. [src][scope]
- At `low` effort it searches less and answers from memory more. [src][search]
- Runs safety classifiers and can return `stop_reason: "refusal"`, with fewer false positives than Fable 5 at launch; finding vulnerabilities in source code is permitted. [src][safeguard]
- Better vision, best with a crop/zoom tool. [src][vision]

## Prompt skeleton

Assembled from the page's blocks; each slot names its section.

```text
ROLE/AUTONOMY   "You are operating autonomously..."      → Finish the whole task
CONTEXT         repo facts; say if your UI hides tool output → Progress updates
TASK            the concrete ask
CONSTRAINTS     "# Delivering work" block                → Finish the whole task
                no-extras block                          → Keep changes and tests
                surgical-edit line                       → Targeted edits
                verify-names line                        → Search at low effort
DONE            "Before ending your turn, check your last paragraph..." → Finish
RETURN FORMAT   when lists/headers help                  → Formatting in chat
TAIL NUDGE      batching sentence, re-sent each turn     → Batch tool calls
```

## Phrases that work (verbatim)

Autonomy opener — "Keep it as written." [src][finish]

> You are operating autonomously. The user is not watching in real time and cannot answer questions mid-task, so asking 'Want me to…?' or 'Shall I…?' will block the work.

Definition of done [src][finish]:

> Before ending your turn, check your last paragraph. If it is a plan, an analysis, a question, a list of next steps, or a promise about work you have not done ('I'll…', 'let me know when…'), do that work now with tool calls.

Batching [src][batch]:

> First privately list what you need next; then request every item that doesn't depend on another's result in this one response.

Targeted edits [src][edits]:

> The number of tokens used to edit files is best minimized, all else being equal. Therefore, when it will not affect the end result, try to surgically edit a file rather than rewrite the entire thing.

Density [src][density]: "Please remove all mannered prose."

Also lift whole: the `# Delivering work` block [src][finish], the no-extras/tests block [src][scope], the name-verification block [src][search], the client-compaction summary [src][compact], the quoting `<example>` [src][quote].

## Anti-patterns

- Lines like "hold all findings for the final response" — audit them out first. [src][updates]
- Anti-formatting rules from older models: replace with a when-to-format rule. [src][format]
- Editing earlier turns: injecting/removing per-turn reminders, summarising older turns in place, changing the system prompt mid-session. Use turn-scoped system messages, left byte-for-byte. [src][append]
- "Does this program compile without errors?" — ask "Are there any bugs in this program?" instead. Avoid base64 in tool output. [src][safeguard]
- Forcing the lead agent to block on each subagent. [src][subagents]

## Effort and length guidance

Start at the default `high`, then sweep `low`/`medium`/`xhigh`/`max` against your evals; re-run it even if you swept Fable 5, since level names don't map to the same amount of thinking across models. At `medium` results roughly match Fable 5 at lower cost; `low` is often competitive with Opus/Sonnet on cost per task while scoring higher. [src][effort]

At `xhigh`/`max` it may draft a long deliverable in thinking and write it again. Run those at `high`; if not, set `max_tokens` to cover thinking *and* reply, and append the page's "counts toward a single limit of about [max_tokens] tokens" note to the user message. For `low`-effort search gaps, raising effort for those turns can beat prompting. [src][long], [src][search]

## For coding agents

- **Finishing:** apply both *Finish the whole task* blocks; the first alone keeps most of the effect. [src][finish]
- **Scope and tests:** the no-extras block drops unrequested additions and committed test code "with no measurable change in task success" — commit tests only where the task asks or the repo already does, ~one focused test per stated behaviour. [src][scope]
- **Edits:** surgical over rewrite. [src][edits]
- **Batching:** append the nudge as a turn-scoped system message (`clear_at: "next_user_message"`, beta `mid-conversation-system-clear-at-2026-08-21`) after each tool-result turn; without the beta, a text block after the `tool_result` blocks. [src][batch]
- **Subagents:** start-tool returns immediately, results arrive in a later `user` message, and the lead gets a separate wait tool. Lowers average time to completion at similar quality and cost. [src][subagents]
- **Progress:** updates arrive as `thinking` blocks — empty under the default `thinking.display: "omitted"`; set `"updates"` (beta `thinking-display-updates-2026-08-18`) or `"summarized"`. [src][updates]
- **Compaction:** client-side, replace the history with one summary plus the new user turn, replaying no thinking blocks; cheaper cache reads mean early compaction may no longer pay. [src][append]

[page]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1
[effort]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#consider-all-effort-levels
[updates]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#ask-for-user-facing-progress-updates
[batch]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#batch-independent-tool-calls-in-agent-loops
[append]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-the-conversation-history-append-only
[density]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#writing-density
[format]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#formatting-in-chat
[quote]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#quoting-retrieved-sources
[finish]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#finish-the-whole-task
[compact]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#tell-the-model-what-to-preserve-in-compaction-summaries
[scope]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-changes-and-tests-to-what-the-task-asks-for
[search]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#search-triggering-at-low-effort
[safeguard]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#reduce-safeguard-false-positives
[edits]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#prefer-targeted-edits-over-whole-file-rewrites
[long]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#leave-room-for-long-outputs-at-xhigh-and-max-effort
[subagents]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#let-the-lead-agent-keep-working-while-subagents-run
[vision]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#give-vision-work-tools-to-crop-and-zoom
