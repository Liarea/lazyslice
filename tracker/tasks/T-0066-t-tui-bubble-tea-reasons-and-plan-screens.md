---
id: T-0066
title: "T-TUI: Bubble Tea reasons and plan screens"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-07
started: 2026-09-07
closed: 2026-09-08
outcome: "done: reasons and plan screens from the event stream, every binding carries its flag, footer strikes unavailable keys, leaving echoes flags into scrollback; --tui only on a TTY"
---

# T-0066 · T-TUI: Bubble Tea reasons and plan screens

## Goal



## Acceptance



## Log

- 2026-09-07 created

- 2026-09-07 started

- 2026-09-08 closed: done: reasons and plan screens from the event stream, every binding carries its flag, footer strikes unavailable keys, leaving echoes flags into scrollback; --tui only on a TTY

## Post-mortem

Went well: driving Update with recorded key sequences caught three real bugs a screenshot test would not. Went badly: bubbles textinput pulled a clipboard module not in go.mod, so a 100-line input was written instead; the preview and the run are not pinned to one snapshot. Change: T-PIN in the backlog; developers build a one-line import before relying on a library package.
