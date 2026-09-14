---
id: T-0029
title: "Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history"
epic: E9
phase: later
status: open
owner: fable
created: 2026-09-05
started: ""
closed: ""
outcome: ""
---

# T-0029 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history

## Goal

Gareth deferred encryption on 2026-09-05; apply only if still wanted at go-public time. Paths: every CLAUDE.md, tracker/, research/, docs/prompting/, docs/BUILD_PLAN*, docs/OPERATING_MODEL.md, docs/RUNBOOK.md, .claude/, NAME.md.

## Acceptance



## Log

- 2026-09-05 created

- 2026-09-14 2026-09-14 orchestrator recommendation: cancel. Nothing in the AI-specific paths is a secret (scanned); encrypting them costs tokens on every read and write, blocks contributors, and the operating model is part of what the project shows. Gareth decides at T-0154.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
