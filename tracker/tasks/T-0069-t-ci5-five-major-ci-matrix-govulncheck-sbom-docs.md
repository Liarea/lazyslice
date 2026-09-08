---
id: T-0069
title: "T-CI5: five-major CI matrix, govulncheck, SBOM, docs drift, unsafe-flag grep"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-07
started: 2026-09-08
closed: 2026-09-08
outcome: "done: 91eab92; Postgres 14 to 18 matrix, blocking govulncheck, SBOM and signing in the release, tools/docgen generating FLAGS.md, KEYBINDINGS.md, ERRORS.md with a drift job, unsafe-flag rail"
---

# T-0069 · T-CI5: five-major CI matrix, govulncheck, SBOM, docs drift, unsafe-flag grep

## Goal



## Acceptance



## Log

- 2026-09-07 created

- 2026-09-08 started

- 2026-09-08 closed: done: 91eab92; Postgres 14 to 18 matrix, blocking govulncheck, SBOM and signing in the release, tools/docgen generating FLAGS.md, KEYBINDINGS.md, ERRORS.md with a drift job, unsafe-flag rail

## Post-mortem

Went well: the govulncheck job found two vulnerable indirect dependencies on its first run and the orchestrator bumped them as their own commit. Went badly: a 17 MB stray binary from a bare go build sat untracked at the repo root and would have been committed by the task's own git add -A; the unsafe-flag rail matches source text and has spellings that bypass it. Change: /docgen is ignored; the rail moves onto the registered flag set in a follow-up.
