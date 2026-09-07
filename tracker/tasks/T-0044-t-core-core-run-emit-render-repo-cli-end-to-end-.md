---
id: T-0044
title: "T-CORE: core run, emit, render, repo, CLI end to end; integration job blocking again"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-07
outcome: "done: core merged at 9bd9e24, blockers closed by T-0058"
---

# T-0044 · T-CORE: core run, emit, render, repo, CLI end to end; integration job blocking again

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 Gareth 2026-09-06: pause at end of phase 4; phase 5 waits for his go-ahead (weekly usage).

- 2026-09-06 blocked: Core wired end to end; four of six invariants pass on both fixtures. Blocked on: (1) v1-blocking leak, I2/nasty: public.devices.owned_by carries a cleartext email because a 3-row table yields no TABLESAMPLE sample (no classifier value signal) and verify's second net returns nil below minValues=3, so both nets fail open; (2) I6/pagila: root customer holds 101 rows because a PARENT_ONLY push widens the root key set, the invariant fixture must count a root with no incoming parent edge; (3) I2/pagila: derived_text masks film.fulltext to empty and the loaded-output guard reads it as unloaded, exempt derived_text; (4) internal/render carries a byte-identical copy of internal/event/catalogue.yml because go:embed cannot cross packages, event.go should embed and export Catalogue(); (5) ci.yml's integration wrapper pins TestI2 and TestI6 as known blockers and must drop the allowlist when they pass. Paused 2026-09-06 at Gareth's request to conserve usage.

- 2026-09-07 Gareth 2026-09-06: continue; pause after phase 5 is fully complete.

- 2026-09-07 closed: done: core merged at 9bd9e24, blockers closed by T-0058

## Post-mortem

Went well: core wired ten packages into one run on the first attempt and the invariant suite immediately earned its keep. Went badly: two invariants and a leak surfaced only at this stage; five findings sat outside the developer's paths. Change: none beyond what T-0058 recorded.
