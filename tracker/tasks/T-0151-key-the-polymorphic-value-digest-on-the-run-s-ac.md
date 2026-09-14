---
id: T-0151
title: "Key the polymorphic value digest on the run's actual mask key, not the published schema fingerprint"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-14
started: ""
closed: 2026-09-14
outcome: cancelled
---

# T-0151 · Key the polymorphic value digest on the run's actual mask key, not the published schema fingerprint

## Goal

internal/plan/polymorphic.go's valueDigest HMACs under p.schema.Fingerprint, which internal/emit writes to lazyslice.yml (schema_fingerprint:) and internal/load writes into the target's lazyslice_meta (ARCHITECTURE.md section 10 and 11) -- so it is published in the same artifact set as the digest it protects, and the digest is a one-way encoding, not a secret-keyed one (2026-09-14 review finding 1, T-0131). Fixing this needs internal/core to resolve the run key (resolveKey) before calling plan.New().Plan, and pipeline.PlanRequest to carry that key through to internal/plan -- both outside internal/plan's own paths. A plan-only run (lazyslice plan / --plan-only) resolves no key today and should keep a documented unkeyed fallback rather than be made to write or require one.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 cancelled: Decision 2026-09-14: the polymorphic digest is removed altogether; unmapped type values are reported as a count per column, so no key needs threading into plan

## Post-mortem

Cancelled. Reason: Decision 2026-09-14: the polymorphic digest is removed altogether; unmapped type values are reported as a count per column, so no key needs threading into plan
