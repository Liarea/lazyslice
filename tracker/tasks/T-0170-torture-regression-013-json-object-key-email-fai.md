---
id: T-0170
title: "torture regression 013 (json-object-key-email) fails on main"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0170 · torture regression 013 (json-object-key-email) fails on main

## Goal

TestTortureRegressions/013-json-object-key-that-parses-as-an-email.sql fails with exit 9 verify.refused.second_net on a re-run of the same fixture directory (pre-existing on main before T-0139, unrelated to the fingerprint-ordering fix); confirmed by running make torture on a stash of main. internal/invariants/torture_test.go and testdata/regressions/013-*.sql

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
