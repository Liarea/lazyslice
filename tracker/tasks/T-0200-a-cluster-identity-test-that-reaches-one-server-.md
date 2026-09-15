---
id: T-0200
title: "A cluster identity test that reaches one server over a genuinely different socket"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0200 · A cluster identity test that reaches one server over a genuinely different socket

## Goal

T-0190's TestOneClusterReachedOverTwoEndpointsHasOneClusterIdentity (internal/pg/gate_integration_test.go) cannot fail on the transport defect: testutil.SecondEndpoint dials the same backend host:port, so inet_server_addr()/inet_server_port() answer identically over both routes, and the pre-fix SQL passes it. The fix round documented that rather than closing it. To close it, reach the container over a second server-side socket - a reader inside the container on its unix socket via docker exec, or a listener bound on a second address in the container - so the pre-fix statement demonstrably fails. Until then cluster_test.go's text checks are the only thing pinning the class.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
