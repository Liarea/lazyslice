---
id: T-0082
title: "No mechanism detects a source statement sent outside a transaction"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: folded into T-0081; the tracer refuses any source statement outside a transaction, pinned by unit and real-server tests"
---

# T-0082 · No mechanism detects a source statement sent outside a transaction

## Goal

T-0076 removed the AfterConnect exec that set default_transaction_read_only=on on every source pool connection (it leaked through a transaction-pooling PgBouncer onto the shared server connection). What it removed was a pool-wide rail covering every statement on any source-mode pool, including statements written in packages internal/pg cannot see; what replaced it is per-call-site discipline in internal/pg/source.go plus TestSystemIDRunsInsideAReadOnlyTransaction, which pins one function. Nothing structural now detects a source statement issued outside a transaction, and the evidence that this matters landed in the same change: internal/discover/probe.go sends three catalog reads on a tracer-carrying pool with no BEGIN, and that was caught by prose review, not by a check (T-0081 is the discover half). Give the rail a mechanism: internal/pg/tracer.go already sees every statement on the source pool and already refuses by cancelling the context, but it enforces shape only. Track BEGIN/ROLLBACK/COMMIT per pgx.Conn in TraceQueryStart and refuse - or at minimum record a violation for - any non-BEGIN statement seen on a connection with no open transaction. Note this cannot land alone: it fails internal/discover's dial the moment it is armed, so it sequences after T-0081 or lands with it. Files: internal/pg/tracer.go, internal/pg/CLAUDE.md, and whatever T-0081 does to internal/discover/probe.go.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 T-0076 review round 2: reviewers ask the orchestrator to re-home this from E9 to E5 phase 5 with T-0081. This is the structural check that would have caught the internal/discover regression T-0076 introduced; leaving it in Later means the phase-5 gap outlives phase 5. Sequencing is unchanged: it fails the dial the moment it is armed, so it lands after T-0081 or with it. A developer can only file into E9, which is why it is there.

- 2026-09-08 moved to E5 phase 5

- 2026-09-08 closed: done: folded into T-0081; the tracer refuses any source statement outside a transaction, pinned by unit and real-server tests

## Post-mortem

Went well: cheap structural check. Went badly: the dial path still continues to the next candidate on a violation rather than failing the run; a design question recorded in internal/discover/CLAUDE.md. Change: none.
