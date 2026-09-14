---
id: T-0029
title: "Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history"
epic: E9
phase: later
status: cancelled
owner: fable
created: 2026-09-05
started: ""
closed: 2026-09-14
outcome: cancelled
---

# T-0029 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history

## Goal

Gareth deferred encryption on 2026-09-05; apply only if still wanted at go-public time. Paths: every CLAUDE.md, tracker/, research/, docs/prompting/, docs/BUILD_PLAN*, docs/OPERATING_MODEL.md, docs/RUNBOOK.md, .claude/, NAME.md.

## Acceptance



## Log

- 2026-09-05 created

- 2026-09-14 2026-09-14 orchestrator recommendation: cancel. Nothing in the AI-specific paths is a secret (scanned); encrypting them costs tokens on every read and write, blocks contributors, and the operating model is part of what the project shows. Gareth decides at T-0154.

- 2026-09-14 2026-09-14 correction: the go-public task is T-0156, not T-0154

- 2026-09-14 cancelled: Gareth 2026-09-14: keep the AI-specific files public; nothing in them is a secret and encryption would cost tokens on every read and block contributors

## Post-mortem

Cancelled. Reason: Gareth 2026-09-14: keep the AI-specific files public; nothing in them is a secret and encryption would cost tokens on every read and block contributors
