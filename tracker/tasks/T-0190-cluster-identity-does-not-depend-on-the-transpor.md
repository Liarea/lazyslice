---
id: T-0190
title: "Cluster identity does not depend on the transport: sqlClusterID uses values that are the same for every session on the cluster"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-15
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0190 · Cluster identity does not depend on the transport: sqlClusterID uses values that are the same for every session on the cluster

## Goal

Red team 2026-09-15 R2-06: internal/pg/source.go sqlClusterID concatenates pg_postmaster_start_time with inet_server_addr and inet_server_port, which are properties of the connection, so one production cluster reached over two transports (a second published port, a proxy, a different interface) yields two identities and ARCHITECTURE.md section 9 rule 1 is defeated for the read-only role the docs recommend (pg_control_system needs EXECUTE the role does not have). Use values that are the same for every session: system_identifier when readable, otherwise pg_postmaster_start_time plus the database oid of the maintenance database plus data_directory when readable, plus the server version, and never anything from inet_server_*. Record the rule in ARCHITECTURE.md section 9 and THREAT_MODEL.md T2 as dated amendments; an integration test that reaches the same container over two published ports and is refused.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: the finding reduced to one sentence, every value must be the same for every session on the postmaster, and writing it into ARCHITECTURE 9 and THREAT_MODEL T2 first made the code follow; the Opus reviewer caught three real defects in the first cut: the start time rendered in the session's time zone, so one server read as two; every field had an equal veto so a matching system identifier could be overruled by a restart; and the regression test passed against the old SQL because two published ports cannot vary the server-side socket; all fixed with a negative control | went badly: the workflow's verify step reported Docker down while Docker was up minutes later, so the landing was by hand after a local run of make check and the pg, testutil, load and core integration suites; the unix-socket case cannot be reproduced in the container suite (T-0200); internal/load's gate test retypes the old identity SQL (T-0199); the maintenance-database oid field carries no information on 15 and newer (T-0206) | change next time: before citing a test as the evidence for a fix, revert the fix and watch that test fail, and say so in the document when it cannot; the verify agent should retry docker info once before declaring Docker down
