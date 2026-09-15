---
id: T-0172
title: "testdata/regressions/013-json-object-key-that-parses-as-an-email.sql fails make torture on main"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0172 · testdata/regressions/013-json-object-key-that-parses-as-an-email.sql fails make torture on main

## Goal

Pre-existing failure on main (confirmed by stashing T-0119's changes and re-running), unrelated to T-0119: internal/invariants exits 9 verify.refused.second_net on public.reg013_items.payload instead of the exit-0 masked outcome the fixture's own comment expects. Blocks make torture from reporting a clean run and needs a look in internal/verify or internal/classify's JSON handling, outside T-0119's paths (internal/classify/, testdata/, docs/TORTURE.md, internal/invariants/ -- but this needs a fix in classify's JSON masking of jsonb keys or the fixture's own expectation, not just a doc update).

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
