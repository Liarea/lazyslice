---
id: T-0058
title: "T-CORE-FIX: fail closed on low-cardinality columns (verify minValues, introspect full-read below the TABLESAMPLE floor), I6 pagila counted root, I2 derived_text exemption, event.go embeds the catalogue, drop the ci allowlist; then make integration must be fully green"
epic: E4
phase: 4
status: open
owner: opus
created: 2026-09-06
started: ""
closed: ""
outcome: ""
---

# T-0058 · T-CORE-FIX: fail closed on low-cardinality columns (verify minValues, introspect full-read below the TABLESAMPLE floor), I6 pagila counted root, I2 derived_text exemption, event.go embeds the catalogue, drop the ci allowlist; then make integration must be fully green

## Goal

Closes the two failing invariants and the T1 leak the suite found; paths internal/verify, internal/classify, internal/introspect, internal/invariants, internal/event, internal/render, .github/workflows/ci.yml

## Acceptance

make integration exits 0 with no allowlist; a 3-row table with an email column is masked; I6 counts a root no parent edge reaches

## Log

- 2026-09-06 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
