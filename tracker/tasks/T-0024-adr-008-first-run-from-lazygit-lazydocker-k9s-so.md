---
id: T-0024
title: "ADR-008 first run from lazygit, lazydocker, k9s source"
epic: E3
phase: 3
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-05
outcome: "done: research/FIRST_RUN_STUDY.md from lazygit, lazydocker, k9s source; docs/adr/008-first-run.md proposed with six-step Docker order, question ladder, locality predicate"
---

# T-0024 · ADR-008 first run from lazygit, lazydocker, k9s source

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-05 closed: done: research/FIRST_RUN_STUDY.md from lazygit, lazydocker, k9s source; docs/adr/008-first-run.md proposed with six-step Docker order, question ladder, locality predicate

## Post-mortem

Went well: reading source found two Docker resolution steps the docs omit. Went badly: its verify step ran make lint on a tree another agent was mid-edit in, so the task reported blocked although its own files were fine; the fixtures commit swept its files in. Change: documentation tasks skip the check run (checks: none), and parallel code tasks are verified by the orchestrator after the batch.
