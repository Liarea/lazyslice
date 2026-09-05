# internal/plan

The subset planner: the client-side monotone FIFO worklist with provenance
tags. Row identity fallback, root-table default, unreadable-table handling,
SCC/topological load order, budget checks, and the printed estimate. No SQL
pushdown, ever, and no extraction — this package decides which keys, not
which rows' bytes.

**Contract.** ARCHITECTURE.md §3 (the algorithm in full) and §2 "plan":
`Planner.Plan(ctx, Reader, *Schema, *Classification, PlanRequest) (*Plan,
error)`.

**Rules.**
- Determinism is load-bearing, not a nicety: FIFO queue, tables in
  `(schema, name)` order, edges in constraint-name order, every `KeySet`/
  `Chunk`/SQL result in identity-column order, never a Go map iteration.
  `TestPlanIsDeterministic` checks two runs over one snapshot produce
  byte-identical `selected` sets — this is what makes `lazyslice.yml` "a
  record of what happened" (ADR-004).
- A row's mode (`ChildOK`/`ParentOnly`) is fixed the first time it is popped
  and never revisited — this is both the size control and the privacy control
  (THREAT_MODEL.md T11, the Jailer #126 case).
- `RowBudget`/`MemoryBudget` are checked during the walk, not after
  (THREAT_MODEL.md T11): exceeding either is exit 11 naming the table.
- `internal/pipeline.KeySet.Bytes()` is the single source of truth for memory
  estimates — do not compute a separate estimate here.
- Unreadable tables are resolved via `RolePrivileges.Unreadable` before any
  key is fetched (§3.6); a parent the role cannot read is exit 12, never
  silently dropped.

**Test.** `go test ./internal/plan/...`, including `TestPlanIsDeterministic`
and the cycle/composite-key fixtures in `testdata/nasty.sql`.

**Never:** push SQL predicates down to the source instead of walking client-side;
let a table's mode change after its first pop; skip a budget check to "plan
faster"; iterate a Go map where output order matters.
