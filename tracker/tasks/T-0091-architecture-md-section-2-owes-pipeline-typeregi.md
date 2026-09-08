---
id: T-0091
title: "ARCHITECTURE.md section 2 owes pipeline.TypeRegistrar, and section 11.1 owes the composite text-form note"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0091 · ARCHITECTURE.md section 2 owes pipeline.TypeRegistrar, and section 11.1 owes the composite text-form note

## Goal

T-0083 added pipeline.TypeRegistrar (RegisterTypes(ctx, *Schema) error), an optional second interface a Writer may implement, and internal/pg's Target/writer implement it; ARCHITECTURE.md section 2 lists Writer's three methods and does not mention it, and internal/pipeline/CLAUDE.md's rule says an interface change here is an ARCHITECTURE.md change in the same commit. ARCHITECTURE.md was outside T-0083's paths. Two edits are owed: section 2 gains TypeRegistrar beside Writer with the reason it is separate (the schema is the loader's argument, and registration can only happen once the DDL has created the types); section 11.1's 'types registered in AfterConnect' sentence gains the half that registration alone does not buy - the source pool runs in QueryExecModeExec so a composite arrives as its text form, pgx's CompositeCodec encodes only a CompositeIndexGetter, and internal/pg/types.go's compositeCodec replaces DecodeValue so pgx's own string fallback completes. Both are recorded in internal/pg/CLAUDE.md, internal/load/CLAUDE.md, internal/pipeline/CLAUDE.md and testdata/README.md trap 27 meanwhile.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
