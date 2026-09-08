---
id: T-0081
title: "Stale default_transaction_read_only prose outside internal/pg after T-0076"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0081 · Stale default_transaction_read_only prose outside internal/pg after T-0076

## Goal

T-0076 removed the AfterConnect exec that set default_transaction_read_only=on on every source connection (it leaked through a transaction-pooling PgBouncer onto the shared server connection). Three comments outside internal/pg still describe that setting as present and are now wrong: internal/discover/probe.go's dialShapes doc comment ("Passing a tracer to pg.Connect is also what brings default_transaction_read_only=on, set on every connection as it is opened"), internal/discover/CLAUDE.md's "The dial runs under the source allowlist" bullet ("pg.Connect, which is also what brings default_transaction_read_only = on on the connection"), and internal/load/CLAUDE.md's type-registration bullet, which points at "internal/pg's AfterConnect" as the place target type registration would go — there is no AfterConnect on the source pool any more, and the target pool never had one. Reword all three: the read-only guarantee is now the REPEATABLE READ READ ONLY transaction under every statement, per transaction and never a session default (THREAT_MODEL.md T9, internal/pg/CLAUDE.md). The discover dial in particular now has no read-only cover beyond the allowlist for its three statements, since pg.Connect sets nothing and probe does not open a transaction — decide whether that is acceptable (they are three catalog SELECTs on an allowlist) or whether the dial should open a read-only transaction too, and say which in the comment. Also check research/SQLIT_STUDY.md L566 and research/proposals/*, which describe the setting as a design decision; research documents are frozen phase-1 records, so they are probably left alone, but the ADR/architecture text is not.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 T-0076 review round 2: reviewers ask that this be priced as a rail restoration, not a prose cleanup. The substantive half is internal/discover/probe.go:99-121 — three catalog reads on a source-mode pool with no BEGIN, on the path that dials production candidates. Nothing in internal/pg blocks it: pg.SourceShapes() already exports source.begin and source.rollback carrying the same two literals Source sends, so probe can register them alongside dialShapes() and wrap the three reads in that pair (do not re-write the BEGIN text). The three stale comments (probe.go:67, internal/discover/CLAUDE.md:217, internal/load/CLAUDE.md:170) are the cheap half and can be split off. Reviewers also ask the orchestrator to re-home this from E9 to E5 phase 5, since the gap was opened in phase 5; a developer can only file into E9.

- 2026-09-08 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
