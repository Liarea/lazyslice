---
id: T-0255
title: "A standby's data directory is not evidence of a different cluster, and the standby's sender address is compared with the target"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0255 · A standby's data directory is not evidence of a different cluster, and the standby's sender address is compared with the target

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the wrong-target attacker's standby variant): with a monitoring-grade role that can read data_directory (pg_read_all_settings), a co-hosted standby and primary differ on it by construction, the field stays decisive, and a named target on the primary reads as a different cluster with no warning. When Source.Replica reports a standby, data_directory joins the start time among the fields that cannot prove difference; and when pg_stat_wal_receiver's sender host and port resolve to the target's server, report SameCluster true outright instead of throwing the address into a header string. Extend cluster_test.go's standby case with data_directory filled and differing. Files: internal/pg/source.go, internal/pg/target.go, internal/pg/cluster_test.go, ARCHITECTURE.md section 9, THREAT_MODEL.md T2's T-0241 paragraph.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
