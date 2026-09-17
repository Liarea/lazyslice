# internal/tui

The two `--tui` Bubble Tea screens (ADR-002): classification reasons and the
plan. Builds the same `core.Request` the CLI flags build; never reaches a
stage directly, never contains logic a flag doesn't already expose.

**Contract.** ARCHITECTURE.md §1 diagram: `internal/tui` is one more sink on
the same event channel as `render.Lines`/`render.NDJSON` (each `event.Event`
arrives as a `tea.Msg`). §2's `Request` is what the model must build from
keystrokes, field for field with `cmd/lazyslice`'s flag-built one. `Collector`
is the sink and sits *in front of* the line printer, never instead of it;
`model.Update` takes an `event.Event` as a `tea.Msg`.

The exported surface is small on purpose, and nothing outside it is a
contract: `Run` with its `Input`, `Result` and `ErrNoTerminal`;
`Collector`/`NewCollector`, the sink `cmd/lazyslice` puts in front of the line
printer; and `Bindings`/`Binding`, what `TestEveryTUIActionHasFlag` walks. The
model, its screens, its keymap and its rows are unexported: nothing outside
this package calls them, and an exported name is one two future developers have
to treat as a promise.

**Rules.**
- Root CLAUDE.md's hardest rule for this package: "every TUI action must be
  reachable by a CLI flag first." A screen must never offer a choice that has
  no `--flag` equivalent in ARCHITECTURE.md §8 — add the flag first, in
  `cmd/`, then the screen. `Binding.Flag` records which one, `Binding.Nav`
  marks the bindings that change no request field, and
  `TestEveryTUIActionHasFlag` in `cmd/lazyslice` fails on any action whose
  flag the real command tree does not register.
- A `?` at Q2 (ADR-008 §6, `internal/core`'s `rootQuestion`) prints the ranked
  top five root candidates with their score components through the line
  printer and `$PAGER`, exactly as ADR-008 §7 states, and re-asks — it does
  not open either Bubble Tea screen. `--tui` is still the only way into the
  two screens this package owns, because both need a plan (the reasons screen
  needs a classification, the plan table needs a plan) and Q2 is asked before
  either exists: there is nothing yet for `?` at that prompt to page into.
  The non-TTY and `--json` fallbacks must keep working whatever else lands.
- Never reach a `pipeline` stage interface directly; only ever construct a
  `core.Request` and hand it to `core.Run`. The two classify event codes are
  spelled out in `collect.go` rather than imported from `internal/classify`
  for that reason, and `TestEveryCodeTheScreensReadIsInTheCatalogue` is the
  price of it.
- A binding that cannot fire on the selected row is struck through in the
  footer, never hidden (ADR-002, research/SQLIT_STUDY.md §2.4). Cancel is not
  rebindable.

**Test.** `go test ./internal/tui/...` (model update logic against recorded
`tea.Msg` sequences, plus two whole-program runs through `tea.NewProgram` over
a reader and a writer); this package has no integration tier of its own.

**Never:** add a TUI-only action with no flag; call a stage package directly;
duplicate flag-parsing logic that already lives in `cmd/`; add a dependency for
a widget — `bubbles/textinput` pulls in a system-clipboard module and
`input.go` exists because of it.
