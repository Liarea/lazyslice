---
id: T-0071
title: "Apply T-PROVISION's core patch: --yes reaches the ladder, delete the dead pipeline.Provisioner, ArgContainer in the not-ready refusal, unreachable-target args at all three sites, CodeTargetNone for the no-target refusal"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-08
started: 2026-09-08
closed: 2026-09-08
outcome: "done: 7a6b96d; --yes reaches the ladder, dead Provisioner interface removed, not-ready refusal names the container and a docker logs command, unreachable refusals carry host and cause at all three sites, no-target refusal has its own code"
---

# T-0071 · Apply T-PROVISION's core patch: --yes reaches the ladder, delete the dead pipeline.Provisioner, ArgContainer in the not-ready refusal, unreachable-target args at all three sites, CodeTargetNone for the no-target refusal

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 started

- 2026-09-08 closed: done: 7a6b96d; --yes reaches the ladder, dead Provisioner interface removed, not-ready refusal names the container and a docker logs command, unreachable refusals carry host and cause at all three sites, no-target refusal has its own code

- 2026-09-08 Lows for T-PIN: ladder_options_test must assert each Options field's value is r.req.<Name> and fail on more than one Options literal; add a core test for openTarget with an empty target asserting CodeTargetUnset, exit 4, no placeholder; the password assertion is vacuous (dsn.Ref cannot hold one), test ArgReason redaction against a PgError with Detail instead; three dangling back-references to target.refused.start_timeout's deleted comment (catalogue.yml ~246 and ~307, internal/load/ddl/recreatable.go:53).

## Post-mortem

Went well: applying a reviewer-verified patch was mechanical; the reviewer then caught that the reason helper dropped the dial cause. Went badly: the options-literal test pins keys not values, so a stub copy would pass. Change: fold the strengthened assertions into T-PIN, which owns the same file.
