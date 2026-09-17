---
id: T-0271
title: "Q2, the root-table question ADR-008 makes the happy path's one question, is asked at a terminal when nothing names the root"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0271 · Q2, the root-table question ADR-008 makes the happy path's one question, is asked at a terminal when nothing names the root

## Goal

ADR-008 (accepted) says the single blocking question a first run should normally see is Q2, 'root table? [customers]', defaulting to the top-scoring table, with ? printing the ranked top five with score components, taking its default with no controlling terminal or under --yes, never asked when --root or lazyslice.yml names the root, and never asked when Q1 or Q1' was already asked in the same run (the one-question rule). The gate-5 audit on 2026-09-17 found it was never built: internal/discover asks Q1 and Q1' and cannot ask Q2 because it has no schema, and internal/plan's chooseRoot takes the default silently. The question belongs where the schema and the prompter meet (internal/core, after introspect and before plan), reads the controlling terminal through the same prompter seam discover uses, and prints the chosen root as a decision beside --root either way. ADR-008's own test list names TestNoControllingTerminalBehavesAsYes for the default path.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
