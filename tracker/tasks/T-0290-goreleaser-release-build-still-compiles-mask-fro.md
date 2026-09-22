---
id: T-0290
title: "goreleaser release build still compiles mask/ from go.work, not the tagged version go.mod requires"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0290 · goreleaser release build still compiles mask/ from go.work, not the tagged version go.mod requires

## Goal

T-0285 review (Makefile install-proof fix): .goreleaser.yaml's builds[].env has no GOWORK=off (outside T-0285's authorized paths, so not fixed there), so the release binary goreleaser builds — and the archives/checksums it publishes — still resolve github.com/Liarea/lazyslice/mask from this tree's committed go.work, not from the version go.mod requires. install-proof's own scratch build (Makefile) now proves 'go install' resolves the tagged mask version, and ci.yml's install-proof job now also runs the test suite once with GOWORK=off, but neither touches the goreleaser build path (.goreleaser.yaml, release.yml) or tools/release/tag.sh, both outside T-0285's paths. If mask/ is changed and a new mask/vX.Y.Z tag is cut without go.mod's require being bumped and landed first, a tool tag cut in that window ships a binary built against mask/'s working-tree state while 'go install' of the same tag resolves the older tagged mask version -- two different maskers behind one version number. Fix: (a) add GOWORK=off to .goreleaser.yaml's builds[].env and to release.yml's build step env, and (b) add a check to tools/release/tag.sh for a vX.Y.Z tag that fails when mask/ at HEAD differs from the mask version go.mod requires (e.g. diff mask/ against a checkout of that required tag). mask/CLAUDE.md's 'Workspace and release' section and docs/RUNBOOK.md's release section describe the intended landing order in prose only; nothing enforces it mechanically outside go.mod's own require line.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 moved to E6 phase 6
- 2026-09-22 closed: done

## Post-mortem

went well: e0dc25f: global GOWORK=off in .goreleaser.yaml (covers the go mod hooks as well as builds), make snapshot green; tag.sh check 8 proven both ways (mask/ code unchanged since mask/v0.1.0 passes, an edited mask.go refuses) | went badly: the first cut compared all of mask/ and refused over T-0285's own CLAUDE.md edit, so Markdown is excluded | change next time: a release guard should be tried against today's tree before it is committed, not just against a crafted failure
