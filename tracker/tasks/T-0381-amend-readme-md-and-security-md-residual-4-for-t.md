---
id: T-0381
title: "Amend README.md and SECURITY.md residual 4 for T-0317's network_id/phone name-veto widening"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0381 · Amend README.md and SECURITY.md residual 4 for T-0317's network_id/phone name-veto widening

## Goal

T-0317 (THREAT_MODEL.md) widens the fourth accepted residual: a minority of IP/MAC addresses under a version/build/release column name, and a guessed-region phone under a key/code/license/serial/token name beside only a likely (not certain) neighbour, are now copied unmasked. README.md:439-456 residual 4 and SECURITY.md's matching entry list 'phone' and 'IP or MAC address' as known shapes and need the same two cases named, the way T-0316's card-shape widening was added to both.

## Acceptance

—

## Log

- 2026-09-25 2026-09-24 created
- 2026-09-25 2026-09-25 moved to E6 phase 6
- 2026-09-25 2026-09-25 2026-09-24 orchestrator: re-homed to E6 phase 6 so README and SECURITY keep step before v0.4.0. Also from T-0359's review (low): THREAT_MODEL's T-0317 amendment is headlined a narrowing but gives no torture-corpus before/after measurement, unlike T-0311 and T-0316; say plainly in the amendment that no corpus-wide measurement was taken, or run the ten-schema classify --json comparison and record the count. Paths README.md, SECURITY.md, THREAT_MODEL.md.
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 7d527c1 with no review finding above low: README and SECURITY residual 4 name T-0317's two widenings the way T-0316's card widening was carried, and THREAT_MODEL's amendment now says plainly that no corpus-wide measurement was taken | went badly: the developer chose the plain statement over running the ten-schema comparison, which is the honest answer for a docs task; two lows (the certain-neighbour qualifier, a rewrap) logged on T-0388 with the other residual-wording follow-ups | change next time: nothing
