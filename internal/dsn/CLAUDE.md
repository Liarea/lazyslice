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

**Decisions made during implementation.**
- `Parse` is `pgconn.ParseConfig`, not a second reading of the text. Both the
  URI form and the keyword/value form are accepted because both are already in
  a developer's environment, and using the driver's own parser means `Ref`
  describes the connection that will actually be made — libpq's `PG*` defaults
  and service file included, which is rung 2 of the ladder.
- **A multi-host connection string is refused, not reduced to its first host**
  (`ErrMultipleHosts`). libpq accepts `host=a,b` and
  `postgres://u@a:5432,b:5432/db` and connects to whichever answers; pgconn
  reports the first as `Config.Host` and carries the rest in `Config.Fallbacks`.
  A `Ref` built from the first host would describe a connection the driver may
  not make, and the gate compares that `Ref`: a loopback host written first and
  a production host written second would read as local and not-the-source while
  the write went to production (ARCHITECTURE.md §9 rules 1 and 2,
  THREAT_MODEL.md T2). A multi-host `DATABASE_URL` is exactly what discovery
  rung 1 picks up verbatim, so this fails closed. pgconn also repeats the
  primary host in `Fallbacks` for the TLS downgrade `sslmode=prefer` implies;
  that repeat is one endpoint and is allowed, so the comparison is over the
  distinct `host:port` set. To support multi-host later, carry the fallbacks on
  `Ref` and make the gate require *every* one of them to be non-source and
  local — do not go back to taking the first.
- `Redact` **rebuilds** the string from the parsed `Ref` rather than cutting the
  password out of the text. A textual redaction has to find the password to
  remove it and misses any spelling it did not anticipate. The rebuilt string
  therefore drops every connection parameter (`sslmode`, `application_name`,
  …): none of them is what a caller wanted to show, and one of them could hold
  a secret.
- **`localhost` counts as loopback** in `Ref.Loopback`, although
  ARCHITECTURE.md §9 refuses the same name for a *Docker endpoint*. The two are
  different questions: there, "local" authorises reading a daemon's containers
  over TCP and `tcp://localhost:2375` is a plausible tunnel; here, "local" is
  the ordinary spelling of the target in every compose file, and refusing it
  would make `--allow-remote-target localhost` the normal case and teach the
  flag away. Revisit with THREAT_MODEL.md T2 if that stops being true.
- A **Unix socket directory** normalises to `unix:<clean path>` and is never
  folded into loopback: `/var/run/postgresql` and `/tmp` are two clusters.
- `Fingerprint` is computed over the *normalised* host and port, so a source
  reached as `localhost` on one run and `127.0.0.1` on the next keeps one
  fingerprint and the marker stays bound (ARCHITECTURE.md §11.2). A `Ref` whose
  host will not normalise is still fingerprinted, because a fingerprint is a
  label and never a gate; the gate's own comparison is `SameEndpoint`, which
  returns an error instead.
- `SameCluster` is new here and is not a refusal. It answers "same host:port,
  any database" for the header's `same cluster as source` line when
  `system_identifier` is unreadable.

**Test.** `go test ./internal/dsn/...`.

**Never:** add a `String()`, `MarshalYAML` or log method on `DSN` itself; let
`Ref` grow a field that could hold a value derived from a password; loosen the
identity/locality comparison without a THREAT_MODEL.md T2 update.
