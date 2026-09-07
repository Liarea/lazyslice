---
id: T-0059
title: "Fixture inet values out of RFC 5737 so masked IPs cannot equal source values; remove the flake paragraphs; mask test pins the output space"
epic: E4
phase: 4
status: done
owner: sonnet
created: 2026-09-07
started: 2026-09-07
closed: 2026-09-07
outcome: "done: 65ab0a3; fixture inet values are RFC 1918, flake paragraphs replaced by a pointer to §5, mask range test added"
---

# T-0059 · Fixture inet values out of RFC 5737 so masked IPs cannot equal source values; remove the flake paragraphs; mask test pins the output space

## Goal

Closes the measured 2.7 percent I2/nasty/values flake (T-0058 post-mortem)

## Acceptance



## Log

- 2026-09-07 created

- 2026-09-07 started

- 2026-09-07 closed: done: 65ab0a3; fixture inet values are RFC 1918, flake paragraphs replaced by a pointer to §5, mask range test added

## Post-mortem

Went well: one round; documentation constraint now sits beside the fixture rows. Went badly: the -count=2 stability run went one-for-two on a testcontainers port race unrelated to the change, so the stability claim is not verified. Change: testutil retries PortEndpoint (hardening).
