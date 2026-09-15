---
id: T-0167
title: "ARCHITECTURE.md: document dsn.Ref.Params and lazyslice.yml's new params: key (T-0135)"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0167 · ARCHITECTURE.md: document dsn.Ref.Params and lazyslice.yml's new params: key (T-0135)

## Goal

T-0135 added an allowlisted Params map to dsn.Ref (docs/reviews/2026-09-09/REVIEW.md finding 6) and internal/emit now writes it as source:/target:'s params: key in lazyslice.yml. ARCHITECTURE.md is outside T-0135's paths so its own text was not updated: line 88's Candidate.Ref comment still reads 'host, port, database, user' (internal/pipeline/discover.go's matching comment was updated, in-paths); section 10's example lazyslice.yml does not show a params: key under source:/target: even though section 10 is described (internal/emit/CLAUDE.md's Contract line) as 'the full file spec'. Fix: update both, and cross-check section 2's dsn.Ref reference and section 9 for the same staleness.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: section 10 example and amendment written by the orchestrator with the landing | change next time: nothing
