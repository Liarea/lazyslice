# internal/

Every Go package except `cmd/lazyslice` and the nested `mask` module (ADR-006
keeps masking as its own module so it can be reviewed as a unit). Nothing here
opens a database connection except `internal/pg`; nothing here formats a
sentence except `internal/render`; nothing here is reachable from outside this
module by Go's own rule.

**Contract.** `internal/pipeline` declares every stage interface and every
shared type from ARCHITECTURE.md §2 (`Source`, `Target`, `Schema`,
`Classification`, `Plan`, `RowBatch`, `Config`, ...); every other directory
here implements one or more of those interfaces or is a leaf the interfaces
depend on. See each subdirectory's own CLAUDE.md for which.

**Rules.**
- Import graph is fixed and acyclic (ARCHITECTURE.md §2 "Import graph"):
  `ref` imports nothing; `event` imports only `ref`; `textsig` imports only `ref` and `pipeline` (a leaf shared by classify and verify); `pipeline` imports `ref`,
  `event`, `dsn` and `mask`; the stage packages import `pipeline`. `event`
  never imports `pipeline`. `TestImportGraph` enforces this; do not add an
  edge that fails it.
- A new package under here gets its own `CLAUDE.md` before it gets a second
  file of code. This is a convention of this directory, established when the
  per-directory files were written (tracker T-0023) — root CLAUDE.md does not
  state it, and an earlier version of this file wrongly said it did.
- Package doc comments carry a "Scaffold status" line while the package is a
  no-op; update or remove that line in the same commit that ends the no-op.

**Test.** `go build ./internal/...` then `go test -race ./internal/...`. Test
one package with `go test ./internal/<pkg>/...`.

**`mask/` reaches this binary through `go.work` while you edit it, and
through `go.mod`'s tagged `require` for everyone else** (T-0285). The repo
root's `go.work` (`use ./ ./mask`) makes a local edit under `mask/` visible
here immediately, with no `replace` and no tag needed; `go install`, CI's
`install-proof` job and anyone who has not run `go work init` all build with
`GOWORK=off` and see only the version `go.mod` requires. So a change under
`mask/` that this package needs is not done at edit time: it needs its own
`mask/vX.Y.Z` tag and a bump of the root `require` before it reaches anyone
outside the workspace — see `mask/CLAUDE.md`'s "Workspace and release" for
the exact steps.

**Never:** put pipeline logic in `cmd/`; let a stage package reach another
stage package directly instead of through `pipeline`'s types; let a
value-bearing field (a row, a DSN with a password) leave the type it is
declared on without an explicit, reviewed carrier.
