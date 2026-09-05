# internal/core

`Run`, `Request`, `Report`. The only place the nine stages are wired together
in order. No stage logic, no flag parsing (that's `cmd/`), no rendering (that's
`internal/render`) — `Run` calls each stage's interface method and sends
events; it never contains a SQL statement or a masking rule itself.

**Contract.** ARCHITECTURE.md §1: `core.Run(ctx, Request, event.Sink)
(*Report, error)` drives discover → introspect → classify → plan → extract →
transform → load → verify → emit, in that order, with no other entry point.
`Request` mirrors the CLI flags in §8 field for field — `cmd/lazyslice` builds
one from flags, `internal/tui` builds the same struct from keystrokes.

**Rules.**
- `Run` is the only place stages are sequenced; a stage package must never
  call another stage package directly.
- The `Default*` constants here are the single source of the v1 flag defaults
  (ARCHITECTURE.md §8) — the CLI, the yml reader and the TUI all read these
  constants rather than repeating the literal.
- A run holds the source snapshot from the start of introspect to the end of
  extract and releases it before load and verify (ARCHITECTURE.md §1) — that
  ordering is `Run`'s responsibility and must not be reordered for
  convenience.
- The line printer, NDJSON writer and TUI are three sinks on one channel; `Run`
  must never special-case which sink is attached.

**Test.** `go test ./internal/core/...`. Full pipeline behaviour needs
`make integration` once the stages are real.

**Never:** reach into a stage's internals instead of its `pipeline` interface;
change stage order without an ARCHITECTURE.md §1 update first; let `Report` or
`Request` carry a value-bearing field (THREAT_MODEL.md T4).
