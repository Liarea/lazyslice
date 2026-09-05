# internal/dsn

Connection-string parsing and the two types that keep passwords contained:
`DSN` (the whole string, password included) and `Ref` (the redacted identity —
host, port, database, user). Nothing else parses a connection string anywhere
in the tree.

**Contract.** ARCHITECTURE.md §2 "source and target handles" references
`dsn.Ref` throughout (`Candidate.Ref`, `Config.SourceRef`/`TargetRef`); §9
"Target" identity and locality rules (same `host:port/database`,
`system_identifier`) are computed from `Ref` here.

**Rules.**
- `DSN` is the one type in the whole codebase allowed to hold a credential.
  `TestNoValueBearingFieldSerialised` asserts it can never be reached from
  `event.Event`, `pipeline.Config`, `pipeline.Plan` or `pipeline.Report`
  (THREAT_MODEL.md T4, T5) — never add a field of type `DSN` to anything
  outside this package without checking that test first.
- `Ref.String()` never prints a password, because `Ref` never holds one — not
  "redacts on the way out," structurally cannot.
- Identity comparison (THREAT_MODEL.md T2) normalises `host:port/database`
  before comparing; loopback and cluster normalisation live here so the gate
  in `internal/pg` doesn't reimplement it.

**Test.** `go test ./internal/dsn/...`.

**Never:** add a `String()`, `MarshalYAML` or log method on `DSN` itself; let
`Ref` grow a field that could hold a value derived from a password; loosen the
identity/locality comparison without a THREAT_MODEL.md T2 update.
