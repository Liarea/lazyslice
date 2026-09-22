---
id: T-0284
title: "go install of lazyslice fails: go.mod's mask replace directive rejects @v0.1.0/@latest"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: cancelled
---

# T-0284 · go install of lazyslice fails: go.mod's mask replace directive rejects @v0.1.0/@latest

## Goal

go.mod:91 has 'replace github.com/Liarea/lazyslice/mask => ./mask'. Verified 2026-09-22 with GOMODCACHE and GOPATH pointed at a scratch dir: 'go install github.com/Liarea/lazyslice/cmd/lazyslice@v0.1.0' and the same with @latest both fail with 'The go.mod file for the module providing named packages contains one or more replace directives. It must not contain directives that would cause it to be interpreted differently than if it were the main module.' No mask/vX.Y.Z tag exists yet (git tag -l has only v0.0.1-v0.1.0), so the replace can't be swapped for a pinned require. Fix: tag mask/vX.Y.Z (ROADMAP.md 'Versioning and releases' already names this scheme), point go.mod's require at that version, and drop the replace line; then re-verify 'go install .../cmd/lazyslice@vX.Y.Z' from an empty module cache before the README documents it as an install method. README.md's install section (T-0279) omits go install for this reason until it is fixed.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 cancelled: duplicate of T-0285, which carries the plan (mask/v0.1.0 tag through make tag, go.mod requires it by version, go.work for local development, a CI job that builds with GOWORK=off from a clean module cache); the developer filed this one during T-0279 for the same failure

## Post-mortem

Cancelled. Reason: duplicate of T-0285, which carries the plan (mask/v0.1.0 tag through make tag, go.mod requires it by version, go.work for local development, a CI job that builds with GOWORK=off from a clean module cache); the developer filed this one during T-0279 for the same failure
