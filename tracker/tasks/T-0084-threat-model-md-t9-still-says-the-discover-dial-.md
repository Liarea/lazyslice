---
id: T-0084
title: "THREAT_MODEL.md T9 still says the discover dial sends its reads outside a transaction"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: THREAT_MODEL T9 gained the dial bullet"
---

# T-0084 · THREAT_MODEL.md T9 still says the discover dial sends its reads outside a transaction

## Goal

T-0081 and T-0082 landed: internal/discover/probe.go now wraps its three catalog reads in BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY ... ROLLBACK (literals taken from pg.SourceShapes), and internal/pg's Tracer refuses any statement that reaches a source connection with no transaction open (ErrOutsideTransaction). THREAT_MODEL.md L138 still reads 'That guarantee is internal/pg's, not yet the binary's', names the dial as the uncovered path, and says 'T-0081 decides it ... until it closes, read the rail as per statement in internal/pg and per shape in the dial' - all three claims are now false. Reword to: read-only is per transaction everywhere, and the tracer's transaction check is what makes it a control rather than a convention. ARCHITECTURE.md section 9's 'three statements' inside the dial should also be read as three reads plus the transaction pair. Neither file was in T-0081's paths.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: THREAT_MODEL T9 gained the dial bullet

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
