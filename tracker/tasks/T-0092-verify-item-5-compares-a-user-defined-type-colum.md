---
id: T-0092
title: "verify item 5 compares a user-defined-type column as two different Go values"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0092 · verify item 5 compares a user-defined-type column as two different Go values

## Goal

ARCHITECTURE.md section 6 item 5 compares every unmasked column of a sampled row between source and target, and internal/verify/value.go's textOf renders both sides. The two sides do not decode a user-defined type the same way. The source is read through internal/pg's source pool in pgx.QueryExecModeExec (text results) with no type registration, so a composite arrives as the Go string '(1234.50,GBP)' and an enum array as '{pending,active}'. The target is read through the target pool, which since T-0083 registers those types and uses the extended protocol (binary results), so the same values arrive as map[string]any and []any and textOf renders them as JSON. Every sampled row of testdata/nasty.sql's public.settlements and public.account_statuses will therefore report verify.sample.differs. It does not fail the run - section 6 item 5 reports and never fails, and CodeSampleDiffers says so - and the invariant suite is green, but the check is vacuous for these columns and would stay vacuous for a real leak. It predates T-0083 (before registration the target side decoded to raw binary []byte, which matched even less); T-0083 only gave the tree a fixture that has such a column. The likely fix is in internal/verify, which was outside T-0083's paths: read the sampled columns in one representation on both sides - a ::text cast in sampleSQL for a column whose type is user-defined, or the same registration on both readers - and say in internal/verify/CLAUDE.md which.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
