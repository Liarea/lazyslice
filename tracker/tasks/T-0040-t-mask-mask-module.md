---
id: T-0040
title: "T-MASK: mask module"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: 2d1de18; key, HKDF, HMAC, every category generator with Domain(), small-domain reporting, unique-domain refusal, format preservation"
---

# T-0040 · T-MASK: mask module

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: 2d1de18; key, HKDF, HMAC, every category generator with Domain(), small-domain reporting, unique-domain refusal, format preservation

- 2026-09-06 Low findings for hardening: mask.Pick unique branch passes when Rows<=0 (Required returns 0), refuse bestD<=0 regardless; freeTextExact lacks MaxLen clamp; satAdd overflows at 1<<62+1<<62; checkValues treats NOT IN / NOT (= ANY) lists as allowed values; network_id MAC-shaped value in varchar(15/16) gives Domain 768 but ErrNoRoom at mask time; fixedMasker writes a literal not admissible under a CHECK list; rules.yml accepts integer for phone which now refuses at plan.

## Post-mortem

Went well: eight findings fixed in two rounds, none disputed; Domain() now takes the minimum over every reachable branch. Went badly: several low findings remain (unique branch with unknown row count, free-text exact-length clamp, satAdd overflow, NOT IN negation in check parsing); the classify rule pack can name a masker no registry test resolves. Change: hardening task E5 for the lows; classify or plan must walk rules.yml masker ids against the registry.
