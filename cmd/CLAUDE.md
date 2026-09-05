# cmd/

`cmd/lazyslice/main.go` only: the cobra command tree, flag binding, exit-code
mapping, renderer selection. No pipeline logic lives here and no stage is
reached directly — this file builds `core.Request` and calls `core.Run`
(ARCHITECTURE.md §2, §8), exactly the struct `internal/tui` builds from
keystrokes, so the TUI cannot do anything a flag cannot (ADR-002, root
CLAUDE.md "the TUI is a thin layer").

**Contract.** The flag table in ARCHITECTURE.md §8 is the spec. `req` fields
mirror it field for field; `docs/FLAGS.md` is generated from this file and CI
fails on drift. Exit codes are the `Exit*` constants, which are ADR-005's and
never change meaning.

**Rules.**
- Every flag in §8 must exist here before any TUI screen may offer the
  equivalent action, never after.
- Never add a flag matching `no-mask|disable-mask|skip-mask|unsafe`, and never
  add `--allow-nonempty-target`, `--replace`, `--allow-ctid` or `--rules` —
  `make forbidden` and ADR-005/006 both watch for these; there is no flag that
  turns masking off.
- `PgError.Detail`, `.Where` and `.Hint` are dropped unless
  `--show-row-values-in-errors`; `renderSafe` is the one place that decision is
  made (THREAT_MODEL.md T4) — do not print an error any other way.
- `report()` maps errors to exit codes; an unmapped error is `ExitInternal`,
  never silently reused as an ADR-005 code.

**Test.** `go test ./cmd/...`. `make forbidden` after touching any flag name.
Full check: `make check`.

**Never:** call an `internal/<stage>` package directly (only `core.Run`); print
a value-bearing field; add a flag absent from ARCHITECTURE.md §8 without also
updating that table.
