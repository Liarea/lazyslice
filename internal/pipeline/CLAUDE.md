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

**Test.** `go build ./internal/pipeline/...` (no logic to unit test on its
own); `TestNoValueBearingFieldSerialised`, `TestReasonGrammar` and
`TestConfigHasNoSecretField` live in this package or one that imports it.

**Never:** add an implementation; import a stage package; add a field that
`TestNoValueBearingFieldSerialised` would need to special-case rather than walk.
