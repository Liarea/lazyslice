---
id: T-0357
title: "README and SECURITY residual lists name the secrets T-0315 no longer reads by entropy"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0357 · README and SECURITY residual lists name the secrets T-0315 no longer reads by entropy

## Goal

T-0315 narrowed the entropy signal (THREAT_MODEL.md T1's T-0315 amendment): a secret that is a hex run of exactly 32, 40 or 64 characters, a secret column of one to four rows, and a secret in a column named type, klass or component_name are now copied when no credential name rule matches the column. That widens README.md's accepted residual 4 and SECURITY.md's residual 2 (a shape no validator knows, in a column no name rule names), which list 'a credential's entropy' among the recognised shapes without saying what it skips; add one sentence to each naming the three cases, as T-0313 did for residual 3. README.md and SECURITY.md were outside T-0315's paths.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 2026-09-24 moved to E6 phase 6
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 80f16a9 after one review round (one medium, two low): README and SECURITY's residual lists name the four secret shapes T-0315 no longer reads by entropy, matching THREAT_MODEL's amendment | went badly: the first cut said three where the amendment names four; two lows filed as a follow-up (sentence placement in SECURITY residual 2; the one-case hex qualifier and the sweep effect) | change next time: a docs task that mirrors a THREAT_MODEL amendment quotes the amendment's list in the brief
