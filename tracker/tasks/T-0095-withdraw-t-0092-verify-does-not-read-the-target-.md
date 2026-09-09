---
id: T-0095
title: "Withdraw T-0092: verify does not read the target through the type-registered pool"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: T-0092 cancelled"
---

# T-0095 · Withdraw T-0092: verify does not read the target through the type-registered pool

## Goal

T-0092 says verify's section 6 item 5 sample comparison will report verify.sample.differs for every user-defined-type column because the target side now decodes through a registered, binary pool. That premise is false and the task should be cancelled or rewritten. internal/verify reads the target through internal/core's own second target pool - run.go:664 opens it with pg.Connect(ctx, d, nil), no AfterConnect and no type registration, and run.go:1590's readableWriter.Query is what verify's targetReader resolves to - so internal/pg's compositeCodec and the registered OIDs are not on verify's path at all. Measured on an unregistered pool against postgres:16: a composite scans into an any as the string (1234.50,GBP) and an enum array as {pending,active}, which is exactly what the source side gives, so the two sides agree and the check is not vacuous. The same correction applies to the residual scan (section 6 item 1), which a reviewer raised for the same reason: internal/verify/residual.go reads through the same unregistered pool, so the bytes internal/transform put in the filter are reproducible. Only the orchestrator may close a task, which is why this is filed rather than closed. If it is rewritten rather than cancelled, the real remaining question is whether internal/core should read the target through the registered pool at all - it deliberately does not, and internal/pg/types.go's DecodeValue comment now says so.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: T-0092 cancelled

## Post-mortem

Went well: measurement beat the reviewer's assumption. Went badly: nothing. Change: none.
