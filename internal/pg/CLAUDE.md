# internal/pg

The only package that speaks Postgres. Implements `pipeline.Source`,
`pipeline.Target` (including the gate), `pipeline.Reader`, `pipeline.Writer`,
and registers the shape-allowlist tracers on the source pool. No other
package opens a `pgxpool.Pool`.

**Contract.** ARCHITECTURE.md §2 "source and target handles" in full —
`Source.Snapshot`/`.Reader`/`.Short`/`.Release`/`.Trace`,
`Target.Gate`/`.Writer` — and §9 for the gate's exact order: reachability as
a precondition, not a rule (unreachable ⇒ exit 4, `target.refused.unreachable`,
`Eligibility.Verdict` stays `NotProbed`), then 1 identity → 2 locality →
3 `has_schema_privilege(...,'CREATE')` → 4 marker → 5 emptiness (ADR-008 §5).
`CREATE` sits after locality, not folded into the reachability precondition —
a remote target whose role lacks `CREATE` is refused as remote, naming the
host and `--allow-remote-target` (THREAT_MODEL.md T2's locality control), not
for the privilege.

**Rules.**
- Every source transaction is `REPEATABLE READ READ ONLY`. All five pgx
  tracers are registered on the source pool; a statement whose shape is not
  registered gets a cancelled context from `TraceQueryStart` and a recorded
  violation that fails the run (THREAT_MODEL.md T9) — no first-keyword
  allowlist (`WITH x AS (DELETE ...)` defeats that).
- `TestSourceNeverCopiesOrBatches` must keep passing: no `CopyFrom`,
  `SendBatch` or `Prepare` on a `Source` connection, ever.
- The gate's `Eligibility.Verdict` is tri-state; `NotProbed` must never be
  treated as eligible. Any probe error or timeout is `Refused`, never
  skipped-as-ok (THREAT_MODEL.md T2). RLS-enabled tables and probe errors
  count as **not empty**, never `pg_stat_user_tables`.
- The 2,000 cap is on the **number of user tables in the target**, not on any
  one table's rows: a target with more than 2,000 user tables is refused
  outright, exit 4 printing the table count (ARCHITECTURE.md §9 rule 5). It is
  never a sample of 2,000 and never a pass, because `Verdict` has no state in
  which "not probed" reads as eligible.
- Target identity is a disjunction over sameness (same `host:port/database`,
  or same `system_identifier` + `current_database()`), not a conjunction over
  difference — refuse on any match, exit 2 before any write-side connection is
  used.
- A marker is **bound** only when `source_fingerprint` (and `source_system_id`
  when both sides have it) and `schema_fingerprint` all match; an unbound
  marker authorises nothing and falls through to the emptiness check (§11.2).

**Decisions made during implementation.**
- `OpenSource` and `OpenTarget` replace the scaffold's `NewSource(pool)` /
  `NewTarget(pool)`. A source pool and its tracer are built together and cannot
  be separated: a source pool handed in without its allowlist is a source with
  no allowlist, and nothing downstream would notice. `Connect` stays, and now
  takes the tracer (nil for the target).
- The source pool runs in `pgx.QueryExecModeExec` with both statement caches
  off, so pgx never prepares a statement of its own — which is what lets
  `TracePrepareStart` refuse unconditionally.
- **The allowlist is a template match, not a text match.** `Shape.SQL` is
  matched with whitespace collapsed and case ignored, and admits `{ident}`,
  `{idents}`, `{int}` and `{snapshot}` and nothing else. A stage needing a
  shape this grammar cannot express adds a placeholder in `tracer.go`, where it
  is reviewed once; it does not register a looser template.
- A refused statement is recorded with its string and numeric **literals
  elided**, so the trace of a statement a bug interpolated a value into cannot
  carry that value (THREAT_MODEL.md T4).
- `reader.Query` turns a refusal back into an error at the call. In
  `QueryExecModeExec` pgx defers the error of a query that never ran to
  `Rows.Err`, where a refused statement would look like a table with no rows;
  the reader compares `Tracer.Violations()` across the call instead. The count
  is the tracer's, so a refusal on another connection in the same instant is
  attributed to this call — the direction that fails the run rather than
  continuing it.
- **`Eligibility.RowCounts` holds `RowsNotCounted` (-1), not a count.** Rule 5
  proves emptiness with `SELECT EXISTS`, and the gate does not go on to count
  rows in a database it is refusing to touch. The map is the set of offending
  tables; the renderer says "not empty", not a number.
- **`lazyslice_meta` is exempt from the emptiness probe**, alongside the
  migration-bookkeeping tables. Without the exemption an unbound marker could
  never reach any verdict but "not empty", and §9 rule 4's "falls through to
  rule 5" would be dead text. The exemption matches **schema `public` only**:
  it is the only thing in the emptiness rule that widens what counts as empty,
  so it is spelled as narrowly as the case it exists for, and a
  `reporting.schema_migrations` is somebody's data wearing a familiar name.
- The table cap is applied to the **post-exemption** list — the tables the gate
  would actually probe.
- The gate takes its **schema fingerprint from an injected
  `CatalogFingerprinter`** (`WithCatalogFingerprint`), because
  `Schema.Fingerprint` is defined over what `internal/introspect` reads and
  this package does not introspect. `Gate`'s signature is ARCHITECTURE.md §2's
  and has nowhere to pass it. **A `Target` with no fingerprinter can never find
  a marker bound** — the fail-closed direction — so wiring introspect's
  fingerprint in is owed by the task that builds the loader.
- Locality is decided from the endpoint (`dsn.Ref.Loopback`) plus
  `--allow-remote-target`; `WithLocal` carries the case the connection string
  cannot show, a container whose compose `working_dir` is the cwd.
- The refusal codes are declared here (`CodeSameDatabase`, `CodeRemote`,
  `CodeTableCap`, `CodeNotEmpty`, `CodeProbeFailed`).
  `internal/event/catalogue.yml` carries only the three ADR-008 added and owes
  rows for the rest — with the exit codes §9 assigns each — before any renderer
  can print these refusals. `catalogue.yml` is not a file this package's task
  may write; the missing rows are reported to the orchestrator as an open task.
- `uuidV4` is local rather than a dependency: `go.mod` has no direct one, and
  this is the only UUID lazyslice makes.
- **`Eligibility.Local` records whether the target is local, never whether the
  flag permitted it.** `--allow-remote-target` decides only whether rule 2
  refuses; a flag-admitted remote target is still `Local: false`, because the
  decision header (§9) is the operator's only visible signal that the write is
  leaving this machine. `pipeline.Eligibility` has no field for "remote,
  admitted by the flag"; adding one is the renderer's task, not this package's.
- **The serialised fallback is serialised here, not merely reported.** On a
  pooled endpoint every `Reader` is the holder connection, and `pgxpool.Conn`
  is not safe for concurrent use, so `Source.serial` is a one-token semaphore
  that the reader holds and `reader.Close` returns. A second `Reader` waits.
  `Serialised()` still cannot be consulted *before* the first fallback — an
  endpoint says nothing about whether it can import a snapshot until one is
  offered to it — so the waiting is the guarantee and the flag is the report.
- **A driver error from the write side is withheld, not wrapped.** pgtype
  formats a value it cannot encode with `%#v`, pgx returns that text unchanged
  through `CopyFrom`, and `cmd/lazyslice` prints a non-`*pgconn.PgError` as
  `err.Error()`. `copyFrom` therefore replaces such an error's text with one
  naming the table; the driver's own words sit behind `withheldError`, which
  has no `Unwrap`, and `RenderAnyError` is the only thing that shows them, with
  `--show-row-values-in-errors` (THREAT_MODEL.md T4). A `*pgconn.PgError` and a
  context error pass through, because `RenderError` already covers the first
  and "context canceled" is not a value.
- **The statement trace is checked for structure, not only matched against
  patterns.** `elideLiterals` cannot close over `E'a\'b'` or `$$a$$`, so a
  statement whose text carries a backslash, or which after the two
  substitutions still shows an unpaired quote or a dollar-quote delimiter, is
  recorded as its leading keyword plus `<elided>`. The backslash is detected
  directly and not through quote parity: parity is restorable, and two escaped
  literals in one statement leave an even number of quotes with a whole value
  standing between them. lazyslice generates no statement containing a
  backslash. The trace is what `--debug` prints.
- **No type registration on either pool, and this package does not owe it.**
  ARCHITECTURE.md §11.1 specifies load as "CopyFrom with explicit column lists
  excluding generated columns, types registered in AfterConnect", and §12 lists
  type registration under this package. It is **deferred to the task that
  builds `internal/load`**, alongside the `CatalogFingerprinter` wiring:
  registering the source's enum, domain, composite and user-defined array OIDs
  on each target connection needs the catalog `internal/introspect` reads, and
  `Connect` has no access to it. Until it exists, a value of such a type has no
  encode plan on the target and `CopyFrom` fails mid-table (THREAT_MODEL.md
  T8); the failure is now at least printable (see the withheld-error entry
  above). This is a recorded debt, not an oversight.

**Test.** `go test ./internal/pg/...`; the gate, tracer and identity behaviour
need `go test -tags integration ./internal/pg/...` (the `TestGate*` suite in
THREAT_MODEL.md T2). Every branch of the gate has a case there: the three
ARCHITECTURE.md §9 names them — `TestGateRefusesRLSTable`,
`TestGateRefusesTargetAboveTableCap`, `TestGateRefusesRemoteTargetWithoutFlag` —
plus `TestGateRefusesTheSourceUnderAnotherName` for the half of rule 1 that
`SameEndpoint` cannot answer. Do not delete one because it is slow; the table
cap builds 2,001 tables on purpose, against the real constant.

**Never:** open a `pgxpool.Pool` for the source or target anywhere but here;
let the gate return `Eligible` for a `NotProbed` table; add a rung to the
identity ladder that treats "not probed" as safe.
