# internal/ref

`TableRef` and `ColumnRef` only. Nothing else — no helpers, no formatting
beyond `String()`, no imports at all (ARCHITECTURE.md §2 "Import graph": `ref`
is the leaf every other package sits above).

**Contract.** ARCHITECTURE.md §2, "identifiers": `type TableRef struct{
Schema, Name string }`, `type ColumnRef struct { Table TableRef; Column
string }`. `pipeline.TableRef` and `pipeline.ColumnRef` are aliases of these —
this package is where they are actually declared.

**Rules.**
- Zero imports, forever. Adding one import here (even `fmt`, even `strings`)
  breaks the reason this package exists: nothing can form an import cycle
  around a package that imports nothing. `TestImportGraph` (in a package that
  can see this one) fails on any import added here.
- `Less` orders by `(Schema, Name)`, then `Column` — this is the ordering the
  planner and the emitter rely on for determinism (ARCHITECTURE.md §3
  "Determinism"). Do not change the sort key without checking every caller
  that assumes it.
- Every table and column reference elsewhere in the codebase is one of these
  two types, never a bare string.

**Test.** `go test ./internal/ref/...`.

**Never:** add an import; add a field that could hold a value (a sample, a
password) — this type is safe to put in an event, in `lazyslice.yml`, and in
an error message precisely because it never carries anything else.
