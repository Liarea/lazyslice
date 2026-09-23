---
id: T-0064
title: "Dogfood: two sessions against a real project of the maintainer's choosing, logged in docs/DOGFOOD_LOG.md"
epic: E5
phase: 5
status: done
owner: human
created: 2026-09-15
started: ""
closed: 2026-09-23
outcome: done
---

# T-0064 · Dogfood: two sessions against a real project of the maintainer's choosing, logged in docs/DOGFOOD_LOG.md

## Goal

Gate 4 item carried; only a human can pick a real database and judge the first run

## Acceptance

—

## Log

- 2026-09-23 closed: done

## Post-mortem

went well: two sessions on 2026-09-23 against a production Rails application the maintainer supplied as dumps (143 tables, 1,845 columns), run with the brew v0.2.0 binary from a directory outside the repo, nothing personal reached either target, the second net caught three copied columns, and a green run copied 29 tables / 76,750 rows in 19 s (session 1) and 34 tables / 97,719 rows in 22 s (session 2); docs/DOGFOOD_LOG.md records both by shape, never by name | went badly: session 1 needed nine runs, two --key and fifteen --unmask flags to reach green (refusals one per run), and the copy would not boot an application (state and enum columns swept into free text, framework tables empty, configuration JSON destroyed); session 2 failed the gate's "needed nothing looked up" on two counts (the yml's recorded container target could not be reached without reading its password out of Docker, and one new high-entropy value needed one new flag); the terminal question path (Q1, Q2, ?) was not exercised because both sessions were headless | change next time: seventeen tasks filed (T-0311 to T-0327) and two decision logs (T-0143, T-0272); a third session at a terminal with the maintainer present is owed for ADR-002's and ADR-008's reversal conditions
