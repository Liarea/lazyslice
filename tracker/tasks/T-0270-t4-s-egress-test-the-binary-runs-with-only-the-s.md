---
id: T-0270
title: "T4's egress test: the binary runs with only the source and the target reachable, and a packet counter proves it tried nothing else"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: ""
closed: 2026-09-17
outcome: done
---

# T-0270 · T4's egress test: the binary runs with only the source and the target reachable, and a packet counter proves it tried nothing else

## Goal

THREAT_MODEL.md T4 lists as a control that 'a test runs the binary under a network namespace that allows only those endpoints' and says 'The network-namespace test is phase 5'; ARCHITECTURE.md section 14 lists it under v1 after Gate 4. The gate-5 audit on 2026-09-17 found no such test anywhere (no netns, unshare, iptables or --network none in the tree, the Makefile or CI). Build it as make egress plus a blocking CI job: a linux build of lazyslice runs inside a container that holds NET_ADMIN, on a Docker network with a source and a target Postgres, under an OUTPUT policy that accepts loopback, established traffic and the two database addresses and counts-then-rejects everything else; the run must exit 0 with a masked, loaded target, and the reject rule's packet counter must read zero (no DNS lookup, no update check, no telemetry). A negative control in the same script proves the counter counts. THREAT_MODEL.md T4's sentence moves from the future tense to naming the target and the job.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 closed: done

## Post-mortem

went well: THREAT_MODEL T4's promised control exists at last: make egress and a blocking CI job run the binary in a NET_ADMIN container whose OUTPUT policy accepts only 127.0.0.1, established traffic and the two databases, and the catch-all's packet counter reads zero after a real masked load; a negative control proves the counter counts. Review caught that accepting the whole lo interface exempted Docker's embedded resolver at 127.0.0.11, so a DNS beacon would have passed uncounted | went badly: the control was written into the threat model on 2026-09-05 as phase-5 work with no tracker task, so nothing owned it and it surfaced only when the gate item was audited against the code on 2026-09-17 | change next time: a control THREAT_MODEL names in the future tense gets a tracker task the day the sentence is written (commits 88cf9b1, 4fce950: interrupt exits non-zero, IPv6 refusal, key in an env file)
