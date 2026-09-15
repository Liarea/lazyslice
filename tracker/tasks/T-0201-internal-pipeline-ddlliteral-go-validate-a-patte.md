---
id: T-0201
title: "internal/pipeline/ddlliteral.go: validate a pattern operand both raw and with metacharacters stripped, and strip _ only for LIKE-family operators"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0201 · internal/pipeline/ddlliteral.go: validate a pattern operand both raw and with metacharacters stripped, and strip _ only for LIKE-family operators

## Goal

T-0189's reviewer (low): patternMetaChars strips _ unconditionally though it is a wildcard only for LIKE, ILIKE and SIMILAR TO, and only the reduced text is validated, so the reduction can only cause misses; '^support_team@corp\.example$' still hits today but nothing pins it. Validate both lit.Text and the stripped form and take the first hit; key the metacharacter set on the operator family. Pin with a unit test in internal/pipeline.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
