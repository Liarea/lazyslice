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

**Outstanding debt — this file is its only home.** One signature here has run
ahead of ARCHITECTURE.md §2 and is a debt, not a precedent. It is recorded
*once*, here, rather than in each affected package, so that closing it is two
edits and not five; `internal/plan`, `internal/extract` and `internal/verify`
describe the Go interface as it is and say nothing that fixing §2 makes false.

`KeySet` (`plan.go`) has five methods — `Len`, `Bytes`, `Chunks(n)`,
`FirstChunk(n) Chunk` and `EachChunk(n, f) error` — where §2's block still
lists three. The two new ones are memory contracts rather than new capability:
`Chunks` builds every chunk before it returns any and each chunk holds its own
copy of the keys, so a caller that wanted one chunk (`internal/verify`'s sample
compare) or one chunk at a time (`internal/extract`) was allocating a second
copy of a whole key set — at verify time *after* `--memory-budget` (§8, exit
11) has been checked at plan and can no longer refuse anything. T-0050 added
them; it could write this package and not ARCHITECTURE.md, and it could not
file its own tracker task (tracker/ is orchestrator-only, root CLAUDE.md), so
this paragraph is the debt's only marker in the tree. Nothing pins §2 to
`plan.go`; a drift test would have to fail today to say so.

Closing it is one commit: replace §2's `KeySet` block with

```go
// KeySet is a sorted set of key tuples ... (prose above the block unchanged)
//
// Chunks materialises a second copy of the whole set — every chunk is built
// before any is returned, and each chunk holds its own typed arrays. EachChunk
// builds one chunk at a time and drops it, and is what internal/extract walks
// a keyed step with. FirstChunk returns the first chunk only, or nil for an
// empty set, and is what internal/verify's sample compare takes.
// EachChunk stops at the first error f returns and returns it.
type KeySet interface {
    Len() int
    Bytes() int64
    Chunks(n int) []Chunk // consecutive runs of at most n tuples, in key order
    FirstChunk(n int) Chunk
    EachChunk(n int, f func(Chunk) error) error
}
```

and delete this "Outstanding debt" section in the same commit.

**Test.** `go build ./internal/pipeline/...` (no logic to unit test on its
own); `TestNoValueBearingFieldSerialised`, `TestReasonGrammar` and
`TestConfigHasNoSecretField` live in this package or one that imports it.

**Never:** add an implementation; import a stage package; add a field that
`TestNoValueBearingFieldSerialised` would need to special-case rather than walk.
