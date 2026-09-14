---
id: T-0148
title: "Regenerate docs/ERRORS.md for the four exit-4 target-ownership codes"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0148 · Regenerate docs/ERRORS.md for the four exit-4 target-ownership codes

## Goal

T-0130 added target.refused.lease_held, load.refused.target_locked, load.refused.target_changed and load.refused.marker_changed to internal/event/catalogue.yml; docs/ERRORS.md is generated from that file by tools/docgen and docs/ was outside T-0130's paths, so 'make docs-check' (and therefore 'make check') fails until someone runs 'make docs' and commits docs/ERRORS.md. No content decision is involved: the four rows are already written.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
