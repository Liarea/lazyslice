---
id: T-0078
title: "internal/load/ddl: drop the dangling back-reference to target.refused.start_timeout's deleted comment"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0078 · internal/load/ddl: drop the dangling back-reference to target.refused.start_timeout's deleted comment

## Goal

internal/load/ddl/recreatable.go:53 ends 'exactly as target.refused.start_timeout records its own missing key'. T-0071 gave that code its own ArgContainer and deleted the comment the sentence points at, so the reference now sends a reader to a comment that is not there. T-PIN fixed the two equivalent references in internal/event/catalogue.yml; this file was outside T-PIN's paths. Fix: delete the trailing clause, keeping the rest of the sentence about ArgKey having no key for an index or a constraint.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
