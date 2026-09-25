---
id: T-0333
title: "--create-target's container name doubles the prefix for a directory named lazyslice-*"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0333 · --create-target's container name doubles the prefix for a directory named lazyslice-*

## Goal

Dogfood session 3: run from a directory named lazyslice-dogfood3, the tool proposed and created the container 'lazyslice-target-lazyslice-dogfood3'. The project name is the directory's basename (ADR-008); the container name prefixes it with lazyslice-target-, so a directory that already starts with lazyslice- reads twice. Strip a leading 'lazyslice-' (or 'lazyslice_') from the project name when building the container name, keep the yml's project name as is, and pin it in the discover tests.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 80a93ce after one review round (one medium, three low): --create-target's container name no longer doubles the prefix for a directory named lazyslice-*, and the fix round closed the collision this opened, two directories (shop and lazyslice-shop) mapping to one name, by making --create-target refuse a same-named container that is not this directory's, as the Q1 path already did | went badly: the rename changes the container name a committed yml from before this landing recorded for a lazyslice-* directory, so the remembered container and password no longer match; filed as an E6 follow-up together with the section 9 amendment and an end-to-end test | change next time: a brief that renames anything a committed file records says how the old name is recognised
