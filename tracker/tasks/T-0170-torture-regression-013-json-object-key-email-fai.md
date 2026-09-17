---
id: T-0170
title: "torture regression 013 (json-object-key-email) fails on main"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: ""
closed: 2026-09-17
outcome: done
---

# T-0170 · torture regression 013 (json-object-key-email) fails on main

## Goal

TestTortureRegressions/013-json-object-key-that-parses-as-an-email.sql fails with exit 9 verify.refused.second_net on a re-run of the same fixture directory (pre-existing on main before T-0139, unrelated to the fingerprint-ordering fix); confirmed by running make torture on a stash of main. internal/invariants/torture_test.go and testdata/regressions/013-*.sql

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-17 closed: done

## Post-mortem

went well: regression 013 passes on main; make torture on 2026-09-17 reports --- PASS: TestTortureRegressions/013-json-object-key-that-parses-as-an-email.sql, and the torture CI job is green | went badly: the failure was fixed by the json-key work that followed (T-0137's follow-ups and T-0172's rule that a task touching the regression corpus is verified under make torture) and nobody closed the report | change next time: a task filed for a red regression is closed by the landing that turns it green, named in that commit's tracker entry
