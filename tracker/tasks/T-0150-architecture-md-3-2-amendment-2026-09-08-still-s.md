---
id: T-0150
title: "ARCHITECTURE.md §3.2 amendment (2026-09-08) still says a _type value is truncated and printed at 64 bytes"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0150 · ARCHITECTURE.md §3.2 amendment (2026-09-08) still says a _type value is truncated and printed at 64 bytes

## Goal

T-0131 replaced showValue's quote-and-truncate with a value-free HMAC digest (internal/plan/polymorphic.go, internal/plan/CLAUDE.md) because the 2026-09-09 review (finding 2) reproduced the truncated value reaching stdout/--json/any log. ARCHITECTURE.md line 838 ('a value longer than 64 bytes is truncated in messages (a T4 bound)') and any other place that repeats it need the same correction: no _type value is ever printed now, only digest:XXXXXXXX.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
