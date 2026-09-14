# internal/pipeline

Every stage interface and every shared type from ARCHITECTURE.md §2, one file
per stage (`discover.go`, `introspect.go`, `classify.go`, `plan.go`,
`extract.go`, `transform.go`, `load.go`, `verify.go`, `config.go`,
`source.go`, `pipeline.go`). No implementations — a `New()` constructor, a
SQL query, a masker, a renderer all belong in a stage package or in
`internal/pg`, never here.

**Contract.** This package *is* ARCHITECTURE.md §2. `Config` and every type in
that section live here and nowhere else; `internal/emit` implements `Emitter`
over `pipeline.Config` and holds no type of its own — that pattern (types
here, behaviour in the stage package) is the one to follow for any new type.

**Rules.**
- Imports only `ref`, `event`, `dsn` and `mask` (ARCHITECTURE.md §2 "Import
  graph"). It must never import a stage package — that would be the cycle
  `TestImportGraph` exists to catch.
- `Config` may hold no field that could carry a secret; a `secret:"true"` tag
  and `TestConfigHasNoSecretField` are the enforcement (THREAT_MODEL.md T5).
- `Decision.Reason` and `Step.Why` are rendered from fixed template sets
  (`internal/classify/reasons.go` and the equivalent for `plan`), never
  free-form — a sample value must never be constructible into one of these
  fields.
- Changing a stage's interface signature here is an ARCHITECTURE.md change
  first: update §2, then this file, in the same commit.

§2's `KeySet` block matches `plan.go` (Len, Bytes, Chunks, FirstChunk, EachChunk) as of 2026-09-08; a change here is a change to §2 in the same commit.

**`Writer` has four methods and `TypeRegistrar` is gone (T-0093).**
`RegisterTypes(ctx, *Schema) error` is `Writer`'s fourth method, in `source.go`,
and ARCHITECTURE.md §2 prints all four (reconciled 2026-09-09). It was an optional second
interface, `TypeRegistrar`, from T-0083 until T-0093: separate because the schema
is the loader's argument and registration can only happen once the DDL has
created the types in the target, halfway through the load. That justification did
not survive review. **"Optional" was the wrong word for it and it was not how the
loader treated it**: `internal/load` refused a `Writer` that was not a
`TypeRegistrar`, by name, at run time — and a type assertion that misses is a
load step that vanishes with no compile error. `internal/core` already wraps that
same writer in a `readableWriter` (which embeds the `Writer` *interface*), so one
refactor stood between the assertion and a run that registers nothing and fails
mid-copy on a composite column. The method is the compiler-checked shape:
`internal/pg`'s `writer` implements it, the doubles in `internal/load` and
`internal/verify` answer nil, and the by-name refusal in `internal/load` is gone
with the assertion that needed it.

**Test.** `go build ./internal/pipeline/...` (no logic to unit test on its
own); `TestNoValueBearingFieldSerialised`, `TestReasonGrammar` and
`TestConfigHasNoSecretField` live in this package or one that imports it.

**Never:** add an implementation; import a stage package; add a field that
`TestNoValueBearingFieldSerialised` would need to special-case rather than walk.

**`Tx` has a `Query` (T-0130, 2026-09-14).** ARCHITECTURE.md §2 prints it, and
this file records it in the same commit, as the rule above requires. `Writer`
still has no read and should not get one: verify reads the target through a
reader `internal/core` opens for it. The exception is the lock-and-recheck of
§11.2 — `LOCK TABLE ... NOWAIT`, re-verify what the gate approved, `DROP` — whose
read has to happen inside the transaction the drop commits in. A read made
anywhere else is an answer about a moment that has already passed, which is the
defect the recheck exists to close. `Eligibility` grew `MarkerRunID` and
`MarkerStatus` for the same reason: they are what the loader re-verifies, and
both are identifiers rather than values.
