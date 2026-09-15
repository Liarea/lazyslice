---
id: T-0173
title: "Stop.Args is never populated by core.wrap, so many transcript error lines render generic while only the final exit line is specific"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0173 · Stop.Args is never populated by core.wrap, so many transcript error lines render generic while only the final exit line is specific

## Goal

T-FAILUX audit (2026-09-14): internal/core/codes.go's wrap() helper (internal/core/core.go) never sets Stop.Args, and r.report() (internal/core/run.go) sends the Error event with Args: s.Args for every Stop core produces this way — so the mid-run transcript line for e.g. run.refused.usage (internal/event/catalogue.yml, message 'a flag names something this source does not have', args: []) never names the flag, table or column that actually failed, even though core.Stop.Message (built by wrap's fmt.Sprintf, e.g. '--unmask public.foo=bar') does carry it and reaches the user only in cmd/lazyslice's final 'lazyslice: <code>: <message>' line. About 30 call sites in internal/core/run.go use wrap() this way (CodeUsage, CodeInternal, CodeSecretRefusedKey, ...). Fix is either to have wrap() accept an event.Args map that r.report can forward, or to have cmd/lazyslice's report() and the transcript converge on one line per refusal. Root CLAUDE.md's 'each user-facing error must say what went wrong, which table or column, and the exact flag that fixes it' is met today only by the last line printed, not by the transcript event.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
