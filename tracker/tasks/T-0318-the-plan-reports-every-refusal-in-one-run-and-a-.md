---
id: T-0318
title: "The plan reports every refusal in one run, and a no-identity hint names the columns"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0318 · The plan reports every refusal in one run, and a no-identity hint names the columns

## Goal

Dogfood session 1 took nine runs to reach a green verify, each refused on the next single cause: one join table with no identity (a second identical one was implied but not named), then ten masked columns under unique indexes named one per run, then the second net naming one column per run. internal/plan should collect every refusal it can find (no_identity, unique_domain, skip_parent, unwritable) and print them all before exiting 12 with the first code; the no_identity hint for a table with three columns or fewer should print --key TABLE=col1,col2 with the actual columns, since a Rails join table's composite key is the obvious answer. Pin with a fixture holding two no-identity tables and two unique-domain refusals.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 9b79da0 after one review round with three findings, all fixed; the --key hint now refuses to promise a nullable or uncomparable column and never re-offers the columns the pseudo-key probe already failed on, and a Plan()-level integration test pins all four collected refusals in one run; a regression in the developer's own first draft of the fallback was caught by an existing integration test before it shipped | went badly: four low findings left for a follow-up (plan.go's before-the-first-key comments and ARCHITECTURE section 3.9 not stating that a collecting refusal now reads more of the source first, an empty Refusals returned as a non-nil error, a stale fail-fast sentence in the package CLAUDE.md, a fixture comment) | change next time: when a refusal becomes collecting, the brief lists every comment and section that said fail-fast so the developer updates them in the same landing
