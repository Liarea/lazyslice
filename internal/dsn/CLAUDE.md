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
- **`Ref.Params` and `ExtractParams` (T-0135, docs/reviews/2026-09-09/REVIEW.md
  finding 6).** `Ref` used to carry only host, port, database and user, so a
  first run with `sslmode=verify-full` reran at pgx's default of `prefer`:
  `emit` wrote the four fields, `discover`'s rung 0 rebuilt a URI from them,
  and the certificate path and every other transport option were gone.
  `Params` is the allowlisted fix — `AllowedParams`: `sslmode`, `sslrootcert`,
  `sslcert`, `sslkey`, `connect_timeout`, `application_name`, `options`. Never
  `password`, never `sslpassword`, never a whole connection string.
  - **This *is* "a second reading of the text"**, the thing `Parse`'s own
    decision above says it deliberately is not. It has to be: `sslmode`,
    `sslcert`, `sslkey` and `sslrootcert` are consumed into a `tls.Config` by
    `configTLS` and `connect_timeout` into a `time.Duration`, and
    `pgconn.Config` keeps no string form of any of them for `Params` to read
    back — there is nothing on the parsed struct to carry them from. `Parse`'s
    own four fields (`Host`, `Port`, `Database`, `User`) are still read off
    `*pgconn.Config` exactly as before; only `Params` re-parses `s`, through
    `ExtractParams`/`readSettings`/`readURLSettings`/`readKeywordValueSettings`
    — a small, allowlist-only reader, not a second general-purpose DSN parser.
  - `ExtractParams` also returns `dropped`: every key `s` carries that is
    neither allowlisted nor one of the identity/credential keys `Ref` or `DSN`
    already carry another way (`host`, `port`, `dbname`/`database`, `user`,
    `password`, `passfile`, `sslpassword`). It is exported so a caller other
    than `Parse` can warn on a name Params silently dropped; `Parse` itself
    only uses the allowlisted half.
  - **`FilterAllowedParams` is the defensive twin**, for a `Ref` built from
    something other than a fresh `Parse` — `internal/emit` reading a committed
    `lazyslice.yml` is the one caller. The committed file is text a person can
    hand-edit, and "`Ref` never holds a credential" has to hold against a
    `params:` block someone added a `password:` key to, not just against a
    live connection string.
  - **`Ref` is no longer comparable with `==`.** A `map[string]string` field
    makes the whole struct incomparable, and `Ref` is embedded in
    `pipeline.Candidate` by value, so this reaches every package that used to
    compare either with `==`. `IsZero()` replaces the one such comparison that
    was inside this task's paths (`internal/emit/document.go`); two more, in
    `internal/dsn/dsn_test.go` and `internal/emit/emit_test.go` (and the
    `integration`-tagged `internal/discover/discover_integration_test.go`),
    became `reflect.DeepEqual`. One more is outside every one of those paths —
    `internal/core/provenance_test.go:180,187` — and was filed as **T-0165**
    rather than fixed here (root CLAUDE.md: writes outside a task's paths are
    not this task's to make). T-0165 has since landed: `pipeline.Candidate`
    grew the matching `IsZero()` (`internal/pipeline/discover.go`, field-wise,
    `Params` counted as zero when empty through `Ref.IsZero()`), and both
    lines in `provenance_test.go` call it instead of comparing with `==`.
  - `Ref.String()` shows `Params` as a `(+N params)` count, never the keys or
    values: a certificate path is a filesystem layout, which is more than an
    ordinary log line needs to carry even though it is not a credential.
- **`options` is checked by value as well as by key (docs/reviews, 2026-09-14,
  finding 5).** Every other `AllowedParams` entry is a transport or TLS setting
  `pgconn` itself consumes; `options` is libpq's own channel for passing
  arbitrary `-c name=value` GUCs to the backend startup packet, so allowlisting
  it by key alone let a hand-edited `lazyslice.yml` set `search_path` (which
  changes unqualified name resolution during introspect/extract),
  `statement_timeout` in the unsafe direction, `row_security`, or anything
  else — a new, unreviewed influence path from a committed file into the
  session lazyslice opens against a live source. `allowedOptionSettings` is a
  three-entry allowlist of timeouts only (`statement_timeout`, `lock_timeout`,
  `idle_in_transaction_session_timeout` — settings that make a session refuse
  to hang and change nothing about what it sees or does), and
  `sanitizeOptions` parses the value as `-c name=value` pairs and fails closed
  on anything it cannot fully account for: one recognised setting next to one
  it does not is not "safe half of it kept", it is dropped whole. Both
  `FilterAllowedParams` (a `Ref` from a committed file) and `ExtractParams` (a
  fresh connection string) run it; a value `ExtractParams` rejects is reported
  through the same `dropped` list an unallowlisted key already used, never with
  the value itself.
  - This does not touch the *decision header* half of the finding — showing
    `options`'s value rather than only a count so an operator can review what
    a run is about to set. No code in the tree prints `Ref.Params` beyond
    `String()`'s count today (searched for "decision header" across the
    tree — ARCHITECTURE.md's own only reference is `Ref.String`'s doc
    comment), so there is no header call site inside or outside this task's
    paths to change; a future one should print `options`'s value specifically,
    once `sanitizeOptions` has already limited it to timeouts.
- **`Ref.Params` and `Parse`'s doc comments narrowed to say plainly that Params
  is a second, literal reading of the connection string and sees only what
  that string spells out (docs/reviews, 2026-09-14, finding 4).**
  `Host`/`Port`/`Database`/`User` come off `pgconn`'s resolved `*Config` and so
  include `PGHOST`/`PGPORT`/`PGUSER`/`PGDATABASE` and a `PGSERVICE` entry, but
  `Params` cannot — it exists because `pgconn.Config` keeps no string form of
  `sslmode`/`sslcert`/`sslkey`/`sslrootcert`/`connect_timeout` to read back
  from, so it re-parses `s` itself, and a setting `s` never spells (because
  `PGSSLMODE` or the service file supplied it instead) is not in `s` to find.
  A source or target found through discovery rung 2 (libpq env / service file,
  `ARCHITECTURE.md` §9) therefore records `params: {}` even when it connected
  with `sslmode=verify-full`, and `refDSN`'s rung-0 rerun dials with no
  `sslmode` at all — the same failure finding 6 closed, surviving on one whole
  rung. Recovering the effective settings from `pgconn.Config` (`TLSConfig`,
  `Fallbacks`, `ConnectTimeout`) instead of re-parsing text would close it, but
  that is a real change and not a doc fix; it is filed as **T-0168** rather
  than built here, and the doc comments on `Ref.Params`, `Parse` and (in
  `internal/discover`) `refDSN` now say so instead of asserting the gap is
  closed.
- **`ParseError` is `Parse`'s discarded pgconn error, kept for exactly one
  caller (docs/reviews, 2026-09-14, finding 2).** `Parse` returns a generic
  "could not be parsed" on failure because the string it was given might be
  operator-typed and might hold a password, and pgconn's own error quotes the
  string it failed on. `internal/discover`'s `refDSNValidated` needed more
  than that: a committed `lazyslice.yml` `source_ref`/`target_ref` that fails
  to parse for a reason its sslrootcert retry cannot fix (a typoed `sslmode`,
  a non-numeric `connect_timeout`) used to read as "nothing at rung 0" and let
  the run fall through to discovery silently, and the refusal that replaces
  that needs to name the parameter pgconn objected to, not just say
  "malformed". `ParseError` returns pgconn's real, unredacted text — safe here
  specifically because `internal/discover` only ever calls it on a string
  `refDSN` built from a `Ref`, and a `Ref` cannot hold a password no matter
  what built the string from it (THREAT_MODEL.md T4, T5); its doc comment
  states that precondition and confines the function to a caller that can meet
  it. It is not a general-purpose replacement for `Parse`'s own error and nothing
  else in the tree should call it.

**Test.** `go test ./internal/dsn/...`.

**Never:** add a `String()`, `MarshalYAML` or log method on `DSN` itself; let
`Ref` grow a field that could hold a value derived from a password; loosen the
identity/locality comparison without a THREAT_MODEL.md T2 update; widen
`AllowedParams` without checking it against THREAT_MODEL.md T4/T5 first.
