---
id: T-0155
title: "v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0155 · v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version

## Goal

The tap is empty and no tag has ever been cut, so gate 3 item "brew install from your tap installs a binary that prints its version" was never actually run. After the repository is public (T-0154): check .goreleaser.yaml against goreleaser v2 with goreleaser check, confirm release.yml refuses a tag whose commit has no green ci run (T-0140) and passes one that has, tag v0.0.1 on a green commit, watch the release workflow, and prove on a machine or container that did not build it that brew install Liarea/tap/lazyslice prints the version. Record what broke and what was changed in docs/RUNBOOK.md "Cutting a release". Note that prerelease: true with a tap set to skip pre-releases would leave the tap empty; decide and record which setting v0.0.x uses.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 2026-09-14 correction: the go-public prerequisite is T-0156, not T-0154

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
