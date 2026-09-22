---
id: T-0285
title: "go install works: the mask module gets its own tag and go.mod requires it by version, with go.work for local development"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0285 · go install works: the mask module gets its own tag and go.mod requires it by version, with go.work for local development

## Goal

README.md's install section (T-0279, 2026-09-22) records that go install github.com/Liarea/lazyslice/cmd/lazyslice@v0.1.0 fails from an empty module cache because go.mod points the nested module github.com/Liarea/lazyslice/mask at a local replace directive, which go install cannot resolve outside the tree. ROADMAP.md 'Versioning and releases' already names mask/vX.Y.Z as the mask module's tag. Do it in this order: tools/release/tag.sh accepts mask/vX.Y.Z (same preconditions, no README check for it, no release run to watch: mask has no goreleaser pipeline) so the orchestrator can cut mask/v0.1.0 with make tag; then go.mod drops the replace and requires github.com/Liarea/lazyslice/mask at that tag, a go.work with use ./ ./mask keeps local edits to mask/ visible to the root module during development (go.work.sum committed, CI unchanged since go build honours go.work), and CI gains a job that runs go install ...@${GITHUB_SHA} equivalent, that is builds the root module with GOFLAGS=-mod=mod and GOWORK=off from a clean module cache, so the replace can never come back unnoticed. Prove it here with GOMODCACHE and GOPATH in a scratch directory; then correct README's install section. Landing order matters: the mask tag must exist before the go.mod change merges.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 2026-09-22 T-0279's post-mortem calls this task T-0284; the id was taken by a filing during the run, so this is the go install task it means.
- 2026-09-22 2026-09-22 2026-09-22 the tag half is done by the orchestrator: make tag cuts mask/vX.Y.Z (9a9d0e1), mask/v0.1.0 is on origin at 1b92e66 (mask/ is unchanged since v0.1.0, so it is exactly what the binary shipped), and the module proxy resolves github.com/Liarea/lazyslice/mask@v0.1.0 from an empty cache. The developer's half: go.mod requires it by version, go.work for local development, the install proof in CI, README's install paragraph.
- 2026-09-22 closed: done

## Post-mortem

went well: b9999a2; the developer found that macOS's GNU Make 3.81 silently ignores .SHELLFLAGS, so install-proof's recipe passed a broken build, and fixed it with an explicit set -e; the orchestrator re-ran make check, make install-proof and a GOWORK=off build and vet, all green | went badly: blocked after two rounds on a medium finding outside its paths (goreleaser still built mask from go.work), which was the orchestrator's to schedule, not the developer's to fix | change next time: a brief that adds go.work should include the release config (.goreleaser.yaml, tag.sh) in its paths, because a workspace changes what every go command in the release builds
