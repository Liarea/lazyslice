---
id: T-0020
title: "Repository scaffold per ARCHITECTURE.md sections 12 and 13"
epic: E3
phase: 3
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-05
outcome: "done: 8254d0f; 20 packages, interfaces, Makefile, CI, goreleaser, SECURITY, CONTRIBUTING; make check green"
---

# T-0020 · Repository scaffold per ARCHITECTURE.md sections 12 and 13

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-05 Low findings carried forward: --cap accepts empty table name; --unmask lhs unvalidated; main_test checks flag names only, not defaults or types; core.Request has no mode for subcommands (introspect/classify/verify/doctor indistinguishable), add a Mode enum in T-CORE; mask.Canonical doc promises a type tag it does not return, resolve in T-MASK; goreleaser check needs a remote; cosign absent locally.

- 2026-09-05 closed: done: 8254d0f; 20 packages, interfaces, Makefile, CI, goreleaser, SECURITY, CONTRIBUTING; make check green

## Post-mortem

Went well: one fix round; the reviewer caught a goreleaser token spelling that would have broken the first release. Went badly: go.mod deviated from ARCHITECTURE.md section 13 (charm.land paths) without reporting it; prerelease: auto contradicted its own comment. Change: developer prompts must say 'report every deviation from ARCHITECTURE.md in concerns'; orchestrator corrects ARCHITECTURE.md the same day.
