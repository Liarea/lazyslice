---
id: T-0042
title: "T-LOAD: load stage"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: load committed; ddl generation of §11.1 object classes, COPY in plan order, NOT VALID then validate, setval, marker, empty-or-complete on kill -9 proved by test"
---

# T-0042 · T-LOAD: load stage

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: load committed; ddl generation of §11.1 object classes, COPY in plan order, NOT VALID then validate, setval, marker, empty-or-complete on kill -9 proved by test

## Post-mortem

Went well: three rounds of review found the fingerprint disagreement before core wired either end; the developer refused to delete the one definition that round-trips. Went badly: the conflict needed an ADR the task could not write; one integration test flakes on container teardown. Change: ADR-009 picks the DDL fingerprint; a follow-up task aligns introspect and pg; the flake goes to hardening.
