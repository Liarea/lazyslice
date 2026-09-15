---
id: T-0210
title: "Why a row is included, boundary crossings and cap omissions visible before copy, and a decision on which job the default slice serves"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0210 · Why a row is included, boundary crossings and cap omissions visible before copy, and a decision on which job the default slice serves

## Goal

docs/reviews/2026-09-09/REVIEW.md, 'A deterministic first-N slice may be the wrong slice': default key order favours early rows, and parent completeness is not tenant isolation. Show why-included, crossings and omissions in the plan output before anything is written, and decide whether the default serves seed-my-environment, reproduce-this-incident or CI data, since their best defaults differ. Files: internal/plan's explain output, internal/render, the TUI plan view; a phase 6 product decision, not gate 5.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
