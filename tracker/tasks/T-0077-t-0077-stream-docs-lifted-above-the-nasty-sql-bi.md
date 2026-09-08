---
id: T-0077
title: "T-0077: stream_docs lifted above the nasty.sql big gate; introspect table list updated; extract workaround removed"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: 7142c52; stream_docs declared beside stream_rows above the gate, only the fill is gated; introspect table list updated; extract workaround removed"
---

# T-0077 · T-0077: stream_docs lifted above the nasty.sql big gate; introspect table list updated; extract workaround removed

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: 7142c52; stream_docs declared beside stream_rows above the gate, only the fill is gated; introspect table list updated; extract workaround removed

## Post-mortem

Went well: one round, verified on a live database. Went badly: two stale constraint-count comments were bumped without re-deriving. Change: prefer stating the invariant over absolute counts in comments.
