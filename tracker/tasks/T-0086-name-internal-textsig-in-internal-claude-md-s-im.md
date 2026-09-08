---
id: T-0086
title: "Name internal/textsig in internal/CLAUDE.md's import graph, ARCHITECTURE.md section 2 and section 12"
epic: E9
phase: ""
status: done
owner: opus
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: textsig named in ARCHITECTURE.md §2 import graph and §12 layout and in internal/CLAUDE.md"
---

# T-0086 · Name internal/textsig in internal/CLAUDE.md's import graph, ARCHITECTURE.md section 2 and section 12

## Goal

T-0055 added the leaf package internal/textsig (the value-only validators and the name dictionary, imported by internal/classify and internal/verify). internal/CLAUDE.md's import-graph rule and ARCHITECTURE.md section 2 'Import graph' name each package one by one and do not name it; ARCHITECTURE.md section 12's layout does not list it. Neither file was in T-0055's paths, so the drift is recorded in internal/textsig/CLAUDE.md instead of where a developer reads the graph.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: textsig named in ARCHITECTURE.md §2 import graph and §12 layout and in internal/CLAUDE.md

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
