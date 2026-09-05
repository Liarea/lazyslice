# Prompting Claude Fable 5.1 — operational cheat sheet

All claims trace to [Prompting Claude Fable 5.1][page] (retrieved 2026-09-04); it covers Fable 5.1 and Mythos 5.1.

## What is different about this model

Fable 5 prompts "should perform well on Claude Fable 5.1 without changes", but:

- **Quieter during tool chains**, more so at higher effort. Updates come as progress-update `thinking` blocks, **empty** under default `thinking.display: "omitted"` ([updates]).
- **One tool call per turn where the next are implied, not named** — coding, bash-and-editor and computer-use loops ([batching]).
- **History must be append-only.** Thinking blocks are valid only in the conversation that produced them (accounts created on/after 2026-08-31); replaying one after a changed prefix returns 400 ([append-only]).
- **Denser prose**, but *less* bold, headers and lists than earlier models, so old anti-formatting rules now hurt ([density], [formatting]).
- **Whole-file rewrites** instead of targeted edits ([edits]), and **scope creep** — "and sometimes more": nearby fixes, unmentioned extensions, extra test files ([scope]).
- **Less searching at `low` effort** ([search]).
- **Safety classifiers** can return `stop_reason: "refusal"`, with fewer false positives than Fable 5 at launch; finding vulnerabilities in source code is permitted ([safeguards]).

## Prompt skeleton

```text
ROLE: You are operating autonomously. The user is not watching in real time
and cannot answer questions mid-task.

CONTEXT: <repo, files, decisions; note if your UI hides tool output>
TASK: <the deliverable, stated once and in full>

CONSTRAINTS
- The scope is the deliverable; don't narrow, widen or swap it.
- Anything else you notice is a follow-up, not a change.
- Surgically edit files rather than rewrite them.
- Commit tests only where the task asks or the repo already keeps them.

DEFINITION OF DONE
Before ending your turn, check your last paragraph. If it is a plan, an
analysis, a question, or a promise about work you have not done, do that
work now.

RETURN FORMAT: <exact shape, plus a recap that stands on its own>
```

Each line compresses a verbatim block ([finish], [scope], [edits]); paste the full ones when length allows.

## Phrases that work (verbatim)

Batching ([batching]), appended to each request:
> First privately list what you need next; then request every item that doesn't depend on another's result in this one response.

Autonomy ([finish]) — "the opening sentence… carries much of the effect. Keep it as written":
> You are operating autonomously. The user is not watching in real time and cannot answer questions mid-task, so asking 'Want me to…?' or 'Shall I…?' will block the work. For reversible actions that follow from the original request, proceed without asking.

Scope ([finish]):
> The user's request — or the plan they approved — sets the scope, and the scope is the deliverable: don't quietly narrow, widen, or swap it.

Targeted edits ([edits]):
> The number of tokens used to edit files is best minimized, all else being equal. Therefore, when it will not affect the end result, try to surgically edit a file rather than rewrite the entire thing.

Prose ([density]): "Please remove all mannered prose."

## Anti-patterns

- Lines like "hold all findings for the final response" ([updates]).
- Old anti-formatting rules; say when formatting *is* appropriate ([formatting]).
- Injecting or removing per-turn reminders, summarising turns in place, changing the system prompt mid-session — use turn-scoped system messages instead (`clear_at: "next_user_message"`, beta `mid-conversation-system-clear-at-2026-08-21`) ([append-only]).
- "Does this program compile without errors?" — ask "Are there any bugs in this program?". Base64 in tool output also trips classifiers ([safeguards]).
- Forcing the lead agent to block on each subagent ([subagents]).

## Effort and length guidance

Start at the default `high`, then sweep all levels against your evals — re-run even if you swept Fable 5, since "effort level names don't correspond to the same amount of thinking across models". `medium` roughly matches Fable 5 at lower cost; `low` is often competitive with Opus and Sonnet on cost per task while scoring higher ([effort]). Raise effort for turns where it skips searching ([search]).

Prefer `high` for long deliverables; at `xhigh`/`max` the model may draft them in thinking, then write them again. Set `max_tokens` to cover thinking *and* reply, and append the budget note ("Everything produced in one reply, including any reasoning or drafting done before the reply, counts toward a single limit of about [max_tokens] tokens"), which "makes the thinking much shorter" ([long-outputs]).

## For coding agents

- **Finishing:** apply both task-completion blocks; under length pressure use only the first, which "keeps most of the effect". Exception: when the user is thinking out loud, the deliverable is your assessment — report and stop ([finish]).
- **State changes** ([finish]): "Before running a command that changes system state (such as restarts, deletes, or config edits), check that the evidence actually supports that specific action."
- **Scope and tests:** pre-existing bugs become follow-ups; ambiguity resolves to the most directly supported reading; scratch checks stay unversioned; committed tests are sized like neighbouring files, "roughly one focused test per stated behavior". Reported: unrequested additions and test code "drop substantially with no measurable change in task success" ([scope]).
- **Batching:** send the nudge as a fresh turn-scoped system message after each `tool_result` turn; without the beta, in a text block after those blocks ([batching]).
- **Subagents:** start-subagent tool returns immediately, results arrive in a later `user` message, lead gets a separate wait tool — lower average time to completion at similar quality and cost ([subagents]).

[page]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1
[effort]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#consider-all-effort-levels
[updates]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#ask-for-user-facing-progress-updates
[batching]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#batch-independent-tool-calls-in-agent-loops
[append-only]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-the-conversation-history-append-only
[density]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#writing-density
[formatting]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#formatting-in-chat
[finish]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#finish-the-whole-task
[scope]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-changes-and-tests-to-what-the-task-asks-for
[search]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#search-triggering-at-low-effort
[safeguards]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#reduce-safeguard-false-positives
[edits]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#prefer-targeted-edits-over-whole-file-rewrites
[long-outputs]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#leave-room-for-long-outputs-at-xhigh-and-max-effort
[subagents]: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#let-the-lead-agent-keep-working-while-subagents-run
