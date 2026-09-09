---
id: T-0123
title: "ARCHITECTURE.md section 2: Writer has four methods now, and pipeline.TypeRegistrar is gone"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0123 · ARCHITECTURE.md section 2: Writer has four methods now, and pipeline.TypeRegistrar is gone

## Goal

T-HARD-C (T-0093) moved RegisterTypes(ctx, *Schema) error onto pipeline.Writer and deleted the optional pipeline.TypeRegistrar interface, so the load's type registration is compiler-checked rather than reached through a type assertion that can miss. ARCHITECTURE.md was outside T-HARD-C's paths and two places are now wrong: section 2's Writer block still lists three methods (Exec, CopyFrom, Begin), and the note recorded after T-0083 at section 11.1 ('pipeline.TypeRegistrar is the writer-side interface through which load registers ... a writer that cannot register types is refused by name') names a type that no longer exists and a by-name refusal internal/load no longer has. Owed: section 2's Writer gains RegisterTypes with the reason it is a method and not a second interface (an optional interface that misses is a load step that vanishes with no compile error; internal/core's readableWriter embeds the Writer interface and was one refactor from being handed to the loader), and the section 11.1 note is rewritten to say the same. internal/pipeline/CLAUDE.md's rule is that an interface change here is an ARCHITECTURE.md change in the same commit; this task is how that rule is honoured, and the note in internal/pipeline/CLAUDE.md points at it.

## Acceptance



## Log

- 2026-09-09 created

- 2026-09-09 Fix round on T-HARD-C confirmed the code half is done and this doc half is all that remains; recording the second half of the edit so it lands in one commit. Once §2's Writer block prints four methods, the seven citation sites that currently say '§2 still prints the three it had, owed as T-0123' become false and must be stripped back to a plain 'ARCHITECTURE.md §2' citation in the same commit: internal/verify/verify.go:25-28, internal/verify/verify_test.go:292, internal/verify/verify_integration_test.go:78, internal/load/load.go:255-257, internal/pg/CLAUDE.md:483-488, internal/pipeline/CLAUDE.md:30-33, internal/load/CLAUDE.md:198-201. Those files are inside T-HARD-C's paths but the strip cannot precede the ARCHITECTURE.md edit without re-creating the misleading citation it was raised for, so it belongs to this task.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
