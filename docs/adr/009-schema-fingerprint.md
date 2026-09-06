# ADR-009: One definition of the schema fingerprint

Status: accepted, 2026-09-06

## Context

ARCHITECTURE.md §11.2 binds the marker in a target to the source by a schema fingerprint, recomputed over the target's own catalog on the next run. Two implementations of that fingerprint landed: `internal/introspect` hashes selected catalog fields (`schemaFingerprint`), and `internal/load/ddl` hashes the generated pre-data and post-data DDL text (`ddl.Fingerprint`). They disagree. The catalog hash counts every non-virtual foreign key where the DDL keeps only edges whose ends are both recreated, and it counts `lazyslice_meta`, so a target lazyslice wrote never fingerprints equal to its source and the marker never binds. The DDL hash round-trips, proved by `TestFingerprintIsStableAndMovesWithTheSchema` and by the live marker-bound assertion in the load integration suite.

## Decision

The schema fingerprint is `sha256` over the DDL text that `internal/load/ddl` generates for a `Schema`: exactly the objects §11.1 recreates, in the order it recreates them. It is the only definition. `internal/introspect` does not fingerprint; `Schema.Fingerprint` is filled by `internal/core` after introspection by calling `ddl.Fingerprint`, and the gate's marker-binding end in `internal/pg` obtains the target's value through `load.GateFingerprint`, which runs the same function over the target's introspected schema inside a transaction that `internal/pg` opens and closes itself. `internal/introspect/fingerprint.go` and its tests are deleted.

## Consequences

Any change to what §11.1 recreates changes the fingerprint, which is correct: a target loaded with a different object set is a different target. A version bump of lazyslice that changes DDL emission also changes the fingerprint; §11.2's `tool_version` line already announces that. Introspect's package is smaller and pure.

## Reversal condition

A second engine (ADR-003) whose DDL generator cannot be made deterministic, at which point a catalog-derived fingerprint per engine is specified in a superseding ADR.
