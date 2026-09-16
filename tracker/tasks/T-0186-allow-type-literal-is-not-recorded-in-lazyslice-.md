---
id: T-0186
title: "--allow-type-literal is not recorded in lazyslice.yml"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-15
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0186 · --allow-type-literal is not recorded in lazyslice.yml

## Goal

--unmask TABLE.COL=REASON round-trips through lazyslice.yml (pipeline.Config.Columns[..].Unmask) so a re-run keeps the opt-out; --allow-type-literal TYPE=REASON, added for the T-REDFIX review's fourth finding, is flag-only, so every run of a schema with an opted-out enum label must pass it again or refuse at exit 13. Needs a types: block in internal/emit's reader and writer plus internal/core reading it as a prior, and a decision about ADR-004's only-tighten rule (an opt-out loosens).

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the type opt-out mapped onto the existing unmask and fingerprint conventions with no new concept; the Opus reviewer caught that nothing in the change was tested and that a carried opt-out was honoured and expired silently; one fix round, reverify clean | went badly: a yml round-trip landed with zero tests in its first cut; the catalogue file was edited outside the listed paths on an implied reading of the brief; THREAT_MODEL T1 still owed the change (T-0216) | change next time: a task that lists docs/ERRORS.md lists internal/event/catalogue.yml and THREAT_MODEL.md beside it, and a brief that adds a yml block names the round-trip test as an acceptance line
