---
id: T-0190
title: "Cluster identity does not depend on the transport: sqlClusterID uses values that are the same for every session on the cluster"
epic: E5
phase: 5
status: open
owner: opus
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0190 · Cluster identity does not depend on the transport: sqlClusterID uses values that are the same for every session on the cluster

## Goal

Red team 2026-09-15 R2-06: internal/pg/source.go sqlClusterID concatenates pg_postmaster_start_time with inet_server_addr and inet_server_port, which are properties of the connection, so one production cluster reached over two transports (a second published port, a proxy, a different interface) yields two identities and ARCHITECTURE.md section 9 rule 1 is defeated for the read-only role the docs recommend (pg_control_system needs EXECUTE the role does not have). Use values that are the same for every session: system_identifier when readable, otherwise pg_postmaster_start_time plus the database oid of the maintenance database plus data_directory when readable, plus the server version, and never anything from inet_server_*. Record the rule in ARCHITECTURE.md section 9 and THREAT_MODEL.md T2 as dated amendments; an integration test that reaches the same container over two published ports and is refused.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
