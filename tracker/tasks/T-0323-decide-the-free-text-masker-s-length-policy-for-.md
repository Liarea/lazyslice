---
id: T-0323
title: "Decide the free_text masker's length policy for short enum-like values"
epic: E9
phase: ""
status: done
owner: human
created: 2026-09-23
started: ""
closed: 2026-09-23
outcome: done
---

# T-0323 · Decide the free_text masker's length policy for short enum-like values

## Goal

Dogfood session 1: a users.role column with three distinct values masked to three distinct 255-character strings of random words, because the free_text masker draws its length from the column's width; the same for ui_mode, os_type, state, log_level and timezone columns. An app that compares role to 'admin' breaks even where masking the column is right. Options: fit the output length to the input (leaks length, which THREAT_MODEL does not promise to hide), emit one short token per distinct input, or keep column width. The maintainer decides; then the masker task.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-23 closed: done

## Post-mortem

went well: decided by the maintainer 2026-09-23: the free_text masker fits its output to the input's length for short values, so a role column with three short values gets three short tokens, not three column-width paragraphs; length is not a property THREAT_MODEL.md promises to hide | went badly: nothing; the dogfood copy made the cost of column-width output plain | change next time: the masker task carries the decision
