---
id: T-0233
title: "internal/pg: data_directory is a weak identity field, and a savepoint failure no longer collapses the whole identity"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0233 · internal/pg: data_directory is a weak identity field, and a savepoint failure no longer collapses the whole identity

## Goal

T-0222's reviewer (two lows): clusterIDWeakField treats data_directory as strong though it is /var/lib/postgresql/data on every stock image, so two sibling containers that can read it but not the start time compare same=true; and in readClusterID a failed SAVEPOINT or RELEASE returns an empty identity, the all-or-nothing collapse the task removed, reintroduced on a rarer path. Treat data_directory as weak, skip only the guarded read on a savepoint failure, keep the start time on a release failure. Files: internal/pg/source.go, cluster_test.go, ARCHITECTURE.md section 9's third amendment.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
