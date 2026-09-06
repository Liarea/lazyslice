---
id: T-0022
title: "Invariant suite I1-I6, black box"
epic: E3
phase: 3
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-05
outcome: "done: 3b9c04f; I1-I6 as black-box tests against the binary and two containers, each failing for the right reason today"
---

# T-0022 · Invariant suite I1-I6, black box

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-05 closed: done: 3b9c04f; I1-I6 as black-box tests against the binary and two containers, each failing for the right reason today

- 2026-09-05 Low findings for the review fix task: child environment must unset PG*/LAZYSLICE_*/rung-1 URL variables by filtering os.Environ(), not by assigning empty strings (an implementation keyed on presence would run with an empty masking key); include PGPASSFILE, PGSSLMODE, PGOPTIONS, PGSERVICEFILE; I1's FK join on (schema, constraint_name) is a cartesian product when two tables share a constraint name, read edges from pg_constraint by OID instead; pg_dump receives the password in argv, pass PGPASSWORD via env; the built binary path is shared and never cleaned, use a per-run temp dir; I4 fingerprints only relkind r, its doc comment overclaims, either widen to sequences and indexes or narrow the comment; no fixture run exercises --unmask so outsideSecondNet is untested.

## Post-mortem

Went well: the reviewer caught that opted-out columns were being held to the wrong net, and the developer improved on the suggested fix. Went badly: my tracker commits swept the suite in mid-task (see operating model). Change: path-scoped staging; parallel steps commit once.
