---
id: T-0270
title: "T4's egress test: the binary runs with only the source and the target reachable, and a packet counter proves it tried nothing else"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0270 · T4's egress test: the binary runs with only the source and the target reachable, and a packet counter proves it tried nothing else

## Goal

THREAT_MODEL.md T4 lists as a control that 'a test runs the binary under a network namespace that allows only those endpoints' and says 'The network-namespace test is phase 5'; ARCHITECTURE.md section 14 lists it under v1 after Gate 4. The gate-5 audit on 2026-09-17 found no such test anywhere (no netns, unshare, iptables or --network none in the tree, the Makefile or CI). Build it as make egress plus a blocking CI job: a linux build of lazyslice runs inside a container that holds NET_ADMIN, on a Docker network with a source and a target Postgres, under an OUTPUT policy that accepts loopback, established traffic and the two database addresses and counts-then-rejects everything else; the run must exit 0 with a masked, loaded target, and the reject rule's packet counter must read zero (no DNS lookup, no update check, no telemetry). A negative control in the same script proves the counter counts. THREAT_MODEL.md T4's sentence moves from the future tense to naming the target and the job.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
