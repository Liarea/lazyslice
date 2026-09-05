# Prompting Claude Fable 5.1 — operational cheat sheet

For automated coding workflows. Every claim traces to
[Prompting Claude Fable 5.1](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1)
(the page also covers Claude Mythos 5.1); links below point into it.

## What is different about this model

- Fable 5 prompts "should perform well on Claude Fable 5.1 without changes" — these are deltas, not a rewrite.
- Fewer user-facing updates during long tool-calling turns than Fable 5, more pronounced at higher effort and in longer tool chains ([progress updates](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#ask-for-user-facing-progress-updates)).
- In coding/computer-use loops where the next calls are *implied*, it may issue one tool call per turn instead of batching ([batching](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#batch-independent-tool-calls-in-agent-loops)).
- Thinking blocks bind to the conversation that produced them (accounts created on/after 2026-08-31); replaying one after the prefix changed returns 400 ([append-only](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-the-conversation-history-append-only)).
- Prose can be denser than Fable 5's; it uses **less** bold, and fewer headers/lists than earlier models ([density](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#writing-density), [formatting](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#formatting-in-chat)).
- More likely to reproduce source passages unmarked when summarizing ([quoting](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#quoting-retrieved-sources)); more likely to rewrite a whole file for a small change ([edits](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#prefer-targeted-edits-over-whole-file-rewrites)).
- On open-ended features it "delivers what's asked for and sometimes more" ([scope](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#keep-changes-and-tests-to-what-the-task-asks-for)).
- Safety classifiers can return `stop_reason: "refusal"`; fewer false positives than Fable 5 at launch, and finding vulnerabilities in source is permitted ([safeguards](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-fable-5-1#reduce-safeguard-false-positives)).

## Prompt skeleton

```
ROLE / AUTONOMY  (system) — "You are operating autonomously. The user is not watching
  in real time and cannot answer questions mid-task…" [finish-the-whole-task block]
CONTEXT — the inputs the answer depends on; docs for any lesser-known language
  (a documented cure for refusal false positives). [safeguards]
TASK — one clear goal; the model "can execute very long tasks without much guidance
  on methodology, especially when the goal is clear". [finish-the-whole-task]
CONSTRAINTS — the "# Delivering work" block + the extras/tests block verbatim. [scope]
DEFINITION OF DONE — "Before ending your turn, check your last paragraph…" ;
  "End your turn only when the task is complete or you are blocked on input only the
  user can provide." [finish-the-whole-task]
RETURN FORMAT — "Close with a short recap that stands on its own — what you found,
  what you did, and what's next." [progress updates]
PER-TURN NUDGE — batching sentence, re-sent each turn as a turn-scoped system message. [batching]
```

## Phrases that work (verbatim from the page)

- Autonomy: `You are operating autonomously. The user is not watching in real time and cannot answer questions mid-task, so asking 'Want me to…?' or 'Shall I…?' will block the work.` Keep the opening sentence as written — it "carries much of the effect".
- Scope: `The user's request — or the plan they approved — sets the scope, and the scope is the deliverable: don't quietly narrow, widen, or swap it.`
- Batching: `First privately list what you need next; then request every item that doesn't depend on another's result in this one response.`
- Edits: `The number of tokens used to edit files is best minimized, all else being equal. Therefore, when it will not affect the end result, try to surgically edit a file rather than rewrite the entire thing.`
- Verification at low effort: `…the name itself is the thing to verify: search before answering, and include the name as the user wrote it in at least one query alongside any reformulations.`
- Prose: `Please remove all mannered prose.` (short form of a fuller definition).
- Refusals: ask `Are there any bugs in this program?`, not `Does this program compile without errors?`

## Anti-patterns

- Lines like `hold all findings for the final response` — audit them out first.
- Carrying over anti-formatting rules from earlier models; say when formatting *is* appropriate instead.
- Editing earlier turns: injecting/removing per-turn reminders, summarizing older turns in place, changing the system prompt mid-session. Use turn-scoped (`clear_at: "next_user_message"`) system messages; leave earlier copies byte-for-byte.
- Base64 in tool output — removing it is the recommended fix.
- Blocking the lead agent on each subagent.
- Assuming the user sees command output your UI collapses.
- Assuming Fable 5's effort sweep transfers: "effort level names don't correspond to the same amount of thinking across models."

## Effort and length guidance

Start at the default `high`, then sweep `low`/`medium`/`xhigh`/`max` against your own evals; effort is the primary intelligence/latency/cost control. `medium` roughly matches Fable 5 at lower cost. At `low` it calls search tools less — raise effort for those turns or add the verification nudge. At `xhigh`/`max` it may draft a long deliverable in thinking and write it again: prefer `high`, set `max_tokens` to cover thinking *and* reply, and append the page's "single limit of about [max_tokens] tokens" note. Cache reads are cheaper, so compacting early to save cost may no longer be the right tradeoff.

## For coding agents

Apply both finish-the-task blocks (the first alone keeps most of the effect). Add the extras block — it drops unrequested additions and committed test code "substantially with no measurable change in task success": report pre-existing bugs as follow-ups, implement the reading the wording most directly supports, discard scratch checks, and commit tests only where the task asks or the repo already keeps them, "roughly one focused test per stated behavior". Add the surgical-edit line, and the batching nudge after each tool-result turn. Let subagent-start tools return immediately, deliver results in a later `user` message, and give the lead a separate wait tool. For dense images, give it a crop/zoom tool or a container with PIL/OpenCV.
