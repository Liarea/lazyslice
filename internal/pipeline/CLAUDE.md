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

**`TypeRegistrar` is in `source.go` and not yet in §2 (T-0083, owed as
T-0091).** It is a second interface a `Writer` may also implement —
`RegisterTypes(ctx, *Schema) error` — so `Writer` still has exactly the three
methods §2 gives it and no existing implementation broke at compile time. It is
separate rather than a fourth method on `Writer` because the schema is the
loader's argument and registration can only happen once the DDL has created the
types in the target, which is halfway through the load. `internal/pg`'s `writer`
implements it, and `internal/load` calls it between §11.1 item 3 and the first
`CopyFrom`. ARCHITECTURE.md §2 was outside the paths of the task that added it,
so the rule above is honoured by the tracker item and not by the commit.
**"Optional" is the wrong word for it, and it is not how the loader treats it**:
a `Writer` that is not a `TypeRegistrar` fails the load, named. The original
justification — a test double or a second engine loads everything that needs no
codec — was reviewed and did not survive: there is one real `Writer` and no
second engine in the v1 cut, `internal/core` already wraps that same writer in a
`readableWriter` (which embeds the `Writer` *interface*, so it is not a
registrar) for verify, and an assertion that misses is a load step that vanishes
with no compile error. T-0093 is the compiler-enforced shape — a fourth method
on `Writer` and four test doubles updated, two of them in `internal/verify`.

**Test.** `go build ./internal/pipeline/...` (no logic to unit test on its
own); `TestNoValueBearingFieldSerialised`, `TestReasonGrammar` and
`TestConfigHasNoSecretField` live in this package or one that imports it.

**Never:** add an implementation; import a stage package; add a field that
`TestNoValueBearingFieldSerialised` would need to special-case rather than walk.
