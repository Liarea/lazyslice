---
id: T-0142
title: "Implement the mapping_file contract of ADR-006"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0142 · Implement the mapping_file contract of ADR-006

## Goal

ADR-012 deferred it past v1 and made the field an explicit exit 2. Implementing it means: transform reads the CSV, a value absent from the mapping is exit 12 naming the column and the unmapped count, the replacement must hold uniqueness, the mapping applies to the whole FK equality group, and repo protects the file (THREAT_MODEL A6). internal/transform, internal/plan, internal/repo.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
