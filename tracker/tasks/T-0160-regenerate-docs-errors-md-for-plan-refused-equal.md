---
id: T-0160
title: "Regenerate docs/ERRORS.md for plan.refused.equality_group"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0160 · Regenerate docs/ERRORS.md for plan.refused.equality_group

## Goal

T-0132's review fix added a row to internal/event/catalogue.yml; docs/ was outside that task's paths, so 'make docs-check' (part of 'make check' and of ci.yml) fails until 'make docs' is run and docs/ERRORS.md committed.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: make docs by the orchestrator, one row | change next time: fixed at the source, generated docs are in every task's paths now
