---
id: T-0152
title: "ADR for T-0131's published-fingerprint value digest, or promote T-0151"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-14
started: ""
closed: 2026-09-14
outcome: cancelled
---

# T-0152 · ADR for T-0131's published-fingerprint value digest, or promote T-0151

## Goal

2026-09-14 review of T-0131 (docs/reviews/) found the polymorphic value digest keyed on the published schema fingerprint (internal/plan/polymorphic.go valueDigest) deviates from T-0131's stated acceptance rule (HMAC under the run key) with no ADR recording the deviation -- docs/adr/ still ends at 011. Reviewer offered two fixes: widen scope so internal/core resolves the run key before plan.New().Plan and pipeline.PlanRequest carries it (T-0151 already tracks this option), or write an ADR (superseding nothing, proposed until gate-5 close) stating the plan-stage digest is a published-domain-separated one-way encoding rather than secret-keyed material, and amend T-0131's acceptance text accordingly. Neither internal/pipeline nor docs/adr/ is in internal/plan's authorized paths, so whichever is chosen needs a task scoped to touch them.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 cancelled: Decision 2026-09-14: no digest, no ADR needed; recorded in internal/plan/CLAUDE.md by the T-0131 landing

## Post-mortem

Cancelled. Reason: Decision 2026-09-14: no digest, no ADR needed; recorded in internal/plan/CLAUDE.md by the T-0131 landing
