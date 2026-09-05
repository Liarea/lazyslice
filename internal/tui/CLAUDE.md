# internal/tui

The two `--tui` Bubble Tea screens (ADR-002): classification reasons and the
plan. Builds the same `core.Request` the CLI flags build; never reaches a
stage directly, never contains logic a flag doesn't already expose.

**Contract.** ARCHITECTURE.md §1 diagram: `internal/tui` is one more sink on
the same event channel as `render.Lines`/`render.NDJSON` (each `event.Event`
arrives as a `tea.Msg`). §2's `Request` is what `Model` must build from
keystrokes, field for field with `cmd/lazyslice`'s flag-built one.

**Rules.**
- Root CLAUDE.md's hardest rule for this package: "every TUI action must be
  reachable by a CLI flag first." A screen must never offer a choice that has
  no `--flag` equivalent in ARCHITECTURE.md §8 — add the flag first, in
  `cmd/`, then the screen.
- Until these screens ship, `?` at a prompt prints the same table through the
  line printer and `$PAGER` (ARCHITECTURE.md §14's v1 cut line) — that
  fallback must keep working even after the real screens land, for `--json`/
  non-TTY runs.
- Never reach a `pipeline` stage interface directly; only ever construct a
  `core.Request` and hand it to `core.Run`.

**Test.** `go test ./internal/tui/...` (model update logic against recorded
`tea.Msg` sequences); this package has no integration tier of its own.

**Never:** add a TUI-only action with no flag; call a stage package directly;
duplicate flag-parsing logic that already lives in `cmd/`.
