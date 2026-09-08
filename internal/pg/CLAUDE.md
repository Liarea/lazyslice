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
- Every source transaction is `REPEATABLE READ READ ONLY`, and **read-only is
  set per transaction only — never as a session default and never in the
  startup packet** (T-0076): a pooled endpoint shares its server connections
  with other applications, so session state outlives the run and is theirs to
  suffer. Nothing here may run an `AfterConnect` hook on the source. All five
  pgx tracers are registered on the source pool; a statement whose shape is not
  registered gets a cancelled context from `TraceQueryStart` and a recorded
  violation that fails the run (THREAT_MODEL.md T9) — no first-keyword
  allowlist (`WITH x AS (DELETE ...)` defeats that).
- **A statement that reaches a source connection with no transaction open is a
  violation** (T-0082): `check` reads the connection's transaction status and
  refuses anything that is not itself a `BEGIN` on an idle one, with
  `ErrOutsideTransaction` rather than `ErrRefused` so the failure says which
  rule broke. This is the pool-wide half of what T-0076 removed, put back as a
  check instead of a session GUC; a nil `*pgx.Conn` (status 0) is not idle, it
  is a shape test with no connection at all. Do not answer "this one statement
  does not need a transaction" by special-casing it here.
  **The shape miss has priority over the transaction rule.** `outsideTx` is
  asked only of a statement that matched a registered shape, so a statement that
  is *both* unallowlisted and on an idle connection is `ErrRefused`. The T9 case
  the allowlist exists for — an injected or hand-added write on the source — is
  the one most likely to arrive with no transaction under it, and computing the
  two independently reported `DROP TABLE users` as `ErrOutsideTransaction`,
  whose own doc comment says the statement was ours and only its transaction was
  missing. `TestAnUnallowlistedStatementOnAnIdleConnectionIsRefusedForItsShape`
  holds that ordering.
  `opensTransaction` is a prefix test over `sqlBeginReadOnly`'s spelling and
  nothing wider: it is reached only by a statement that already matched one of
  our shapes, and no shape admits `START TRANSACTION`, so the regexp that used
  to also match that spelling was an alternative that could not be reached. A
  shape for another opener is widened here in the same change that registers it.
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
  `{idents}`, `{int}`, `{snapshot}`, `{selectlist}`, `{casts}`, `{keypred}`
  and `{where}` and nothing else. A stage needing a shape this grammar cannot
  express adds a placeholder in `tracer.go`, where it is reviewed once; it does
  not register a looser template.
- **A quoted identifier inside a template is fixed text, never a placeholder**
  (`templateSegments`, T-0050). `internal/extract` and `internal/verify` build
  per-table shapes by quoting a table name into the template, precisely so that
  the shape names one relation instead of admitting every relation — and an
  identifier is arbitrary text: `CREATE TABLE public."{ident}"` is a table a
  person can make. Compiling the placeholder inside that quoted name turned
  extract's `extract.lookup.public.{ident}` into `SELECT {selectlist} FROM
  {ident} t ORDER BY {idents} LIMIT 1001`, an ordered read of anything including
  `pg_catalog.pg_authid`, which is the exact widening the per-table name exists
  to prevent; and a table called `{table}` made `NewTracer` fail on an unknown
  placeholder and took the run with it. The scan skips quoted runs (`""` is an
  embedded quote, and an unterminated quote takes the rest of the template, so a
  malformed template matches nothing rather than everything). Nothing lazyslice
  writes puts a placeholder inside quotes — every placeholder stands for a whole
  identifier, quotes included — so this can only ever narrow a shape.
  **A quoted name is matched the way Postgres reads one**: `regexp.QuoteMeta`
  inside `(?-i:...)`, so case-sensitively and space for space, while the SQL
  around it keeps the `(?is)` an unquoted (server-folded) identifier needs.
  Matching it with `quoteLiteral` under the outer `(?i)`, as the first version
  did, was the same widening in a smaller size: a space in the template stands
  for a run of *zero or more* spaces, so a shape naming `"my table"` admitted a
  read of `"mytable"`, and the case fold let a shape naming
  `"LegacyCustomer"` — `testdata/nasty.sql` ships that table — admit a read of
  `"legacycustomer"`, which on the server is a different table.
  `TestAQuotedNameInAShapeIsMatchedAsPostgresReadsOne` holds both, in both
  directions. One residual, recorded rather than closed: `normaliseSQL` collapses
  whitespace runs on both sides before either is matched, so a shape naming
  `"my table"` still admits a read of `"my  table"`. Closing it means
  normalising around quotes instead of through them, which changes how every
  statement — an operator's `--where` predicate included — is normalised.
  **A typo is the cost of this rule**: a stage that writes `FROM "{ident}"`
  meaning the placeholder now compiles a shape that matches only a table
  actually called `{ident}`, and `expandInto`'s build-time refusal cannot see
  inside quotes to say so. The two are not separable from the template string —
  a name and a typo look identical — and the widening is the one that fails
  silently, so this is the direction the ambiguity is resolved in. What catches
  the typo instead is each stage's own shapes test, which builds a statement
  with the real builder and runs it through a real `Tracer`
  (`internal/extract/shapes_test.go`, `internal/plan/shapes_test.go`,
  `TestSourceShapesCoverThisPackagesOwnStatements` here): a template that can
  never match fails there, at build time, as it did before.
  `TestAPlaceholderShapedTableNameIsAName` and
  `TestTemplateSegmentsSplitsOnQuotedIdentifiers` are the guards here;
  `internal/extract`'s
  `TestALookupShapeForATableNamedWithBracesNamesThatTableAlone` is the guard at
  the other end.
- **The four join placeholders are structures, not escape hatches.**
  `{selectlist}`, `{casts}` and `{keypred}` were added for the planner and
  extract, whose statements are built per table and per key arity
  (ARCHITECTURE.md §3 and §2 "extract"). Each admits identifiers, casts and
  equality and nothing else — no literal, no call, no subquery, no statement
  separator — so a write cannot be spelled in any of them: a select-list item
  has no place for `lo_import(...)`, a cast argument list is `$n::type[]` and
  cannot be `ARRAY[...]`, and a key predicate is column-to-column equality and
  `IS NOT NULL`. A cast's type name admits the multi-word spellings
  `format_type` writes (`timestamp(3) without time zone`) as a **closed list**
  of continuation words (`reTypeWord`), not as "a word". The earlier
  `[a-z]+` continuation was wrong and the claim above was false with it:
  `INTO evil` is two letters-only words, so
  `SELECT t."a"::int INTO evil FROM "public"."orders" t ORDER BY t."id" LIMIT
  500` matched `plan.seed`, and `SELECT ... INTO` is `CREATE TABLE AS` — a
  write on the source, refused only by the `READ ONLY` transaction. T9 makes
  the tracer a control in its own right, so a run of words after a cast is not
  admitted; a type spelling that needs another word is added to the list, once.
  Pinned by `TestKeyJoinPlaceholders` and
  `internal/plan`'s `TestStatementsOutsideTheGrammarAreStillRefused`.
- **`{where}` is the one placeholder holding text this program did not write,
  and it is the one with an exclusion list**: no `;`, no `--`, no `/*`, no `\`,
  no `$`, and parentheses that balance within the predicate to `whereMaxDepth`
  (6) levels. `--where "id > 0) --"` matches the seed shape and comments the
  template's own `ORDER BY ... LIMIT n` away, turning §3's bounded root read
  into an unbounded one inside the holder transaction (THREAT_MODEL.md T11,
  then T9); `;` ends the statement and starts another; and an unbalanced `)`
  closes the template's own `WHERE (` and appends whole clauses —
  `id > 0) UNION ALL SELECT c."pan" AS o1 FROM "public"."cards" c WHERE (true`
  matched `plan.seed_where` with every parenthesis in the statement paired,
  until balance was required. A `;`, `(` or `)` inside a string literal in a
  predicate is refused with the rest: telling a literal from structure costs a
  SQL lexer, and the refusal costs an operator a rephrasing.
- **Be exact about what that buys.** It buys one statement, no commented-out
  tail, no clause appended to the template's own, and a trace that is a
  statement rather than a leading keyword and `<elided>` (the `\` and `$`
  exclusions are the two forms `elideLiterals` cannot close over). It does
  **not** make a predicate safe: a balanced predicate is still arbitrary
  operator SQL and can carry a function call (`id = lo_import('/etc/passwd')`,
  and a `dblink(...)` that opens a connection of its own, which `READ ONLY`
  does not reach) or a subquery with an unbounded aggregate. What bounds those
  is the `READ ONLY` transaction and nothing here. The admitted cases are
  pinned by name in `internal/plan`'s `TestWhatTheWhereExclusionsDoNotStop`, so
  the guarantee a reader inherits is the one the code makes. THREAT_MODEL.md
  T9 is the v1-blocking control and still words the tracer as refusing anything
  outside "the fixed set lazyslice generates", and still offers the bounded
  count (`LIMIT 1001`) as the control against an unbounded count inside the
  holder transaction — which a balanced `--where` subquery walks straight past.
  Two amendments are owed there. **One:** `--where` is operator SQL and the
  allowlist bounds only its *form* (no `;`, `--`, `/*`, `\`, `$`, and
  parentheses balanced to depth 6); a balanced predicate may still carry a
  function call, including one that opens a connection of its own (`dblink`,
  `postgres_fdw`), and a subquery with an unbounded aggregate — so `READ ONLY`
  and not the tracer is what bounds those, and the bounded-count bullet needs
  the same qualification about its own scope. **Two:** the `INTO evil`
  near-miss above belongs in T9 as a recorded near-miss and not only as a fixed
  regex, because at the time the layer that caught it was not present on every
  path: `Connect` set no `default_transaction_read_only` on the source pool and
  `Source.SystemID` sent its statement outside any `BEGIN`, so an autocommit
  path had the allowlist and nothing else. **Both amendments have since landed
  in T9, and no statement this package sends is on an autocommit path any more:
  `Source.SystemID` opens a `REPEATABLE READ READ ONLY` transaction of its own**
  (T-0076, see the entry below), so the transaction is under every statement
  *here* rather than a session setting being. It is under every statement in the
  binary too, as of T-0081 and T-0082: `internal/discover`'s dial opens a
  transaction around its three catalog reads, and `check` refuses a statement
  that arrives on an idle source connection, so the claim is a check and not a
  reading of every call site.
- **A predicate that breaks the rule is refused at `--where`, not only here.**
  `internal/plan/where.go` checks the same characters and the same balance when
  the request arrives and returns exit 2 naming the character and its position.
  Without it an ordinary regex predicate (`email ~ '^\w+@example\.com$'`, both
  a backslash and a dollar sign) reached the operator as "the source refused a
  statement: statement does not match any registered shape", and incremented
  `Violations()`. A recorded violation now always means a bug in our own SQL
  generation. The guards are
  `TestWherePredicatesThatWouldEscapeTheSeedAreRefused` and
  `TestTheWhereCheckAndTheShapeAgree`, in `internal/plan` because that is where
  the seed statement is built and where `--where` is read.
- **`ExtractShapes()` and `shapes_extract.go` are gone** (T-EXTRACT). They were
  a review of extract's statement forms, held here because the task that
  reviewed them could write `internal/pg` and not `internal/extract`, which was
  then a no-op scaffold. `internal/extract.Shapes(plan)` now declares the
  statements that package really builds, as every other stage does
  (`introspect.Shapes()`, `plan.Shapes()`), and it is narrower than the review
  was: the lookup read names its table and writes its bound as a literal.
  `internal/plan`'s `TestTheComposedAllowlistStillRefusesAnUnboundedRead`
  compiles the union of the planner's shapes, this package's and
  `extract.Shapes(nil)`, and pins that the union still refuses an unbounded
  read; a stage that registers a shape wide enough to make another stage's
  bound optional fails there.
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
  `CatalogFingerprinter`** (`WithCatalogFingerprint`), because ADR-009 defines
  `Schema.Fingerprint` as `sha256` over the DDL `internal/load/ddl` generates
  for the schema `internal/introspect` reads, and this package can import
  neither — `internal/load` imports `internal/pg`, so the edge back is a cycle.
  `load.GateFingerprint` is the only caller. `Gate`'s signature is
  ARCHITECTURE.md §2's and has nowhere to pass it. **A `Target` with no
  fingerprinter can never find a marker bound** — the fail-closed direction.
- **This package opens the transaction the fingerprinter runs in**
  (`Target.catalogFingerprint`): `BEGIN ISOLATION LEVEL REPEATABLE READ READ
  ONLY`, the injected call, then `ROLLBACK`. It handed the fingerprinter a
  pooled connection in autocommit until T-FPR, and `introspect.Introspect`
  wraps its sampling in a `SAVEPOINT`, which outside a transaction block is
  25P01 — so the binding could never be confirmed and a target lazyslice itself
  wrote was refused with exit 4. `internal/load`'s integration test carried an
  `inTransaction` wrapper around the introspector to work around it; that is
  gone, and `TestLoadPagilaIntoAMarkedTarget` now passes `introspect.New()`
  bare. The `BEGIN` belongs here and not on the other side of the injection,
  because a caller that opened it would be ending a transaction this package
  had started. `READ ONLY` is the free half: recomputing a fingerprint reads,
  and the target pool has no tracer, so the server is what refuses a
  fingerprinter that tried to write.
  **That `BEGIN` is now pinned in this package's own suite** (T-0053):
  `TestGateRunsTheFingerprinterInsideATransaction` injects a fingerprinter that
  takes a `SAVEPOINT` through the reader it is handed and fails the test by name
  on 25P01, so the statement is guarded where it is written rather than in
  `internal/load`'s suite, where the bug was found. `load.GateFingerprint` now
  asks for a schema-only introspection, so the production fingerprinter takes no
  savepoint of its own; the transaction is still owed to the contract
  `CatalogFingerprinter` states — one version of the catalog for a read of many
  statements, and a savepoint legal for any fingerprinter that needs one — and a
  test is what keeps a contract nothing currently exercises.
  **A `ROLLBACK` that fails discards the connection**, with `source.go`'s
  `endTx` discipline through the shared `discard` helper: the connection is
  closed, and `pgxpool` throws a closed connection away when the gate's own
  deferred `Release` returns it, rather than handing an unknown transaction
  state to whoever acquires next. It is still reported as well as acted on —
  every caller turns the error into `CodeProbeFailed` and a refused run, which
  is the fail-closed direction for a binding that could not be confirmed. The
  comment that used to justify reporting it by saying "the gate has rules left
  to run on this connection" was wrong in the other direction: rule 4 returning
  an error is the end of the gate, and rule 5 is never reached on that path.
- Locality is decided from the endpoint (`dsn.Ref.Loopback`) plus
  `--allow-remote-target`; `WithLocal` carries the case the connection string
  cannot show, a container whose compose `working_dir` is the cwd.
- The refusal codes are declared here (`CodeSameDatabase`, `CodeRemote`,
  `CodeTableCap`, `CodeNotEmpty`, `CodeProbeFailed`).
  `internal/event/catalogue.yml` carries only the three ADR-008 added and owes
  rows for the rest — with the exit codes §9 assigns each — before any renderer
  can print these refusals. `catalogue.yml` is not a file this package's task
  may write; the missing rows are reported to the orchestrator as an open task.
- **Read-only is per transaction, never a session default** (T-0076, revising
  T-EXTRACT). ARCHITECTURE.md §2 makes every source transaction `BEGIN
  ISOLATION LEVEL REPEATABLE READ READ ONLY`, and that transaction is the
  enforcement THREAT_MODEL.md T9 names — the tracer is the evidence. For a
  while `Connect` also set `default_transaction_read_only=on` as a session
  `SET` in an `AfterConnect` hook, because `Source.SystemID` was the one
  statement issued outside any `BEGIN` and on that autocommit path the
  allowlist was the whole defence (the `SELECT t."a"::int INTO evil FROM ...`
  near-miss T9 records). **That hook is gone. `Source.SystemID` opens a
  read-only transaction of its own instead**, so no statement this package sends
  is left on an autocommit path and nothing is set on the session. What the hook
  gave was pool-wide, covering statements written where this package cannot see
  them, and what replaced it at first was discipline at each call site here —
  which is exactly the kind of rule this package's own history says gets broken:
  `internal/discover`'s dial had been relying on the pool-wide half and was left
  with the allowlist alone, and it was a person reading prose who noticed
  (T-0081). Both halves have since landed: that dial opens a transaction around
  its three catalog reads, and the pool-wide rail is back as a check rather than
  a session GUC — `check` refuses any statement that reaches a source connection
  with no transaction open (T-0082, and the entry below).
  **Why, measured:** a session GUC set through a transaction-pooling PgBouncer
  is set on the *shared server connection* the pooler assigned, not on anything
  lazyslice owns, and PgBouncer in transaction mode does not run
  `server_reset_query` by default (`server_reset_query_always = 0`). So it
  stayed on that server connection after lazyslice exited and every later
  client assigned it inherited it — a second, unrelated client through the same
  pooler read `on` back without issuing a `SET`, and its `CREATE TABLE` failed
  with "cannot execute CREATE TABLE in a read-only transaction" — for up to
  `server_lifetime` (3600 s by default). A run against a pooled source could
  leave other applications on that pooler read-only after it finished. A rail
  that protects the source by breaking its neighbours is damage, and this one
  bought nothing the transactions did not already give.
  `TestAPooledRunLeavesThePoolerWritableForOtherClients` is the proof: one
  server connection for the whole pooler (`MAX_DB_CONNECTIONS=1`,
  `DEFAULT_POOL_SIZE=1`), a whole source session on it, then an unrelated
  client that must be given that same connection and must be able to
  `CREATE TABLE`. Against the old `AfterConnect` exec it fails on SQLSTATE
  25006. `TestConnectSetsNoSessionStateOnASourceConnection` is the unit half —
  no `AfterConnect` hook at all, and no `SET` on the allowlist —
  `TestSystemIDRunsInsideAReadOnlyTransaction` pins the `source.begin →
  source.system_id → source.rollback` trace, and
  `TestAWriteInsideASourceTransactionIsRefusedByTheServer` registers a write
  shape on purpose so that what refuses the write can only be the server (25006).
  THREAT_MODEL.md T9's read-only bullet carries the same wording; it listed the
  session setting among T9's rails and now says the setting is per transaction
  and why.
  **The other door was already shut, and stays shut: not a startup parameter
  either.** It was written into `ConnConfig.RuntimeParams` first, which puts it
  in the PostgreSQL startup packet; PgBouncer, Odyssey and Supavisor accept only
  `client_encoding`, `DateStyle`, `TimeZone`, `standard_conforming_strings` and
  `application_name` there and **refuse the connection** for anything else
  unless it is listed in `ignore_startup_parameters`. A pooled endpoint is a
  supported v1 topology (ADR-005 "Pooled endpoints", ARCHITECTURE.md §2
  `Source.Reader`, §8's `--single-connection`), so that made `OpenSource` unable
  to open any connection at all through one — and because `pgxpool` connects
  lazily, as an opaque acquire failure rather than at `Connect`, with the
  serialised-extract fallback in `Source.Reader` unreachable behind it.
  `TestConnectAddsNoStartupParameterAPoolerWouldRefuse` is the unit guard, over
  what we send rather than over what a pooler answers: it compares
  `ConnConfig.RuntimeParams` against a hand-written list of what PgBouncer
  documents it accepts, so it cannot fail on a list that has gone stale.
  **`pooler_integration_test.go` is the half where PgBouncer answers** (T-0050):
  `internal/testutil.PgBouncer` starts a real pooler in transaction mode in
  front of a real server, `TestASourceOpensAndReadsThroughAPooler` opens a
  `Source` through it and runs identity/export/import/read, and
  `TestAStartupParameterOutsideThePoolersListRefusesTheConnection` is the
  negative control — the same parameter moved into the startup packet and no
  connection can be opened at all. That control has a positive control in front
  of it (the same pooled URL, the same pool, without the parameter) and asserts
  the *wording* of the refusal, so it can no longer pass on a pooler that was
  never reachable. Both doors being shut is why the setting lives in the
  transaction: a pooler refuses it in the startup packet and cannot be trusted
  to clear it from the session.
  `internal/invariants`'s I4 cannot see server session state either — it
  compares catalogs and row checksums — so the pooled integration test is the
  only thing standing between this package and that leak. Do not answer a
  future "one more thing to set on the source connection" with `AfterConnect`;
  put it in the transaction, or it is a neighbour's outage.
  **The dial outside this package (T-0081, closed).** Three comments outside
  this package described the session setting as present, and one of them sat on
  top of a real gap: `internal/discover`'s dial sent its three catalog reads on
  a source-mode pool with no transaction under them, on the path that dials
  production candidates. That was a rail restoration and not a comment rewrite,
  and it landed as one — `probe` acquires one connection and wraps the three
  reads in `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY` ... `ROLLBACK`,
  taking both literals from `SourceShapes()` by name rather than writing them
  out a second time (two copies of an allowlisted literal are two things to keep
  in step). If a stage ever needs those literals in a form `SourceShapes` cannot
  give, export them here; do not let a second copy of the `BEGIN` text exist.
  **The structural half (T-0082, closed).** What the `AfterConnect` hook gave
  was a pool-wide rail; per-call-site discipline plus
  `TestSystemIDRunsInsideAReadOnlyTransaction` pinned one function and nothing
  else, which is why the `internal/discover` regression was found by reading
  prose. `Tracer` is where the mechanism belongs, because it already sees every
  statement on the source pool and already refuses by cancelling the context, so
  `check` now takes the connection's transaction status and refuses a non-`BEGIN`
  statement on an idle one with `ErrOutsideTransaction`. It is a status pgx
  tracks from every `ReadyForQuery`, not a count this package keeps, so it
  cannot drift from what the server thinks. `TestASourceStatementOutsideATransactionIsRefused`
  is the unit half; `internal/discover`'s
  `TestTheDialRunsInsideAReadOnlyTransaction` is the half that proves the caller
  this check was armed against passes it.
  **`TestASourceStatementSentWithNoTransactionIsRefusedOnARealConnection` is
  what pins the rail to a real server** (`readonly_integration_test.go`, beside
  the positive half). The unit test hands `check` a status byte written out as a
  constant, and `txStatus` fails *open* — 0 for a nil `*pgx.Conn` or a nil
  `PgConn`, and 0 is deliberately not a violation — so nothing else in the tree
  asserted that an idle connection really reports `I` at the moment
  `TraceQueryStart` runs. Without it, a change in how pgx reports the status, or
  a wrapper that hands the tracer a connection with no `PgConn`, would make this
  rail a no-op with every test still green and this file still promising it. It refuses on status `I` only: `0`
  means the tracer was called with no connection, which is how the shape tests
  here and in `internal/plan`, `internal/extract` and `internal/verify` call
  `TraceQueryStart`, and `E` is a failed transaction, which is still one — the
  `ROLLBACK` that ends it must not be refused.
  **Owed (T-0084).** THREAT_MODEL.md T9 still reads "that guarantee is
  `internal/pg`'s, not yet the binary's" and names the dial as the uncovered
  path; it was outside the paths of the task that covered it. It is the binary's
  now.
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
  **The condition is "this endpoint cannot give a reader", not "this endpoint
  refused the snapshot"** (T-0050 fix round). Three answers reach it — the
  acquire failing, the `BEGIN` failing, `SET TRANSACTION SNAPSHOT` failing — and
  a cancelled or expired context reaches none of them, because the tracer
  refuses a statement *by* cancelling the context and a caller that walked away
  cancels its own; reading either as a pooler would turn a refused statement
  into a silently serialised run. The first two were added because they are what
  a restrictive pooler actually says. **What a real PgBouncer does is worth
  knowing** (measured against `edoburu/pgbouncer:v1.25.2-p0` in transaction
  mode): given a second server connection it pins one for the length of the
  holder's transaction, so the exporting transaction is still open when the
  second client connects and `SET TRANSACTION SNAPSHOT` **succeeds** — with room
  to spare, a pooled run is a normal parallel-reader run
  (`TestASourceOpensAndReadsThroughAPooler`). Given `max_db_connections=1`, the
  holder's transaction is holding the only one and the reader's acquire comes
  back `FATAL: query_wait_timeout` (SQLSTATE 08P01) after the pooler's own wait
  — which `Source.Reader` used to return as a run-ending error, leaving §8's
  `--single-connection` automatic half unreachable on the exact topology it
  exists for. It now serialises, and
  `TestAPoolerWithOneServerConnectionSerialisesTheExtract` is the proof: it fails
  with `query_wait_timeout` against the old condition.
  `TestTheSerialisedFallbackWorksThroughAPooler` drives the third answer with a
  snapshot identifier the server will not import, which is a **synthetic**
  trigger and is labelled as one in the test — no pooler produces it.
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
  type registration under this package. It was **deferred to the task that
  builds `internal/load`**, as the `CatalogFingerprinter` wiring was:
  registering the source's enum, domain, composite and user-defined array OIDs
  on each target connection needs the catalog `internal/introspect` reads, and
  `Connect` has no access to it. That task shipped without taking it, so the
  debt is now **T-0083** and not a deferral to a task that has closed. Until it
  is done, a value of such a type has no encode plan on the target and
  `CopyFrom` fails mid-table (THREAT_MODEL.md T8); the failure is at least
  printable (see the withheld-error entry above). Whatever does it registers on
  the **target** pool: an `AfterConnect` hook on the source is what T-0076
  removed and what this file's Never list forbids.

**Test.** `go test ./internal/pg/...`; the gate, tracer, identity,
read-only-transaction and pooler behaviour need
`go test -tags integration ./internal/pg/...` (the `TestGate*` suite in
THREAT_MODEL.md T2, `TestAWriteInsideASourceTransactionIsRefusedByTheServer`
and `TestSystemIDRunsInsideAReadOnlyTransaction` for T9, and
`pooler_integration_test.go` for ADR-005's pooled endpoints and for
`TestAPooledRunLeavesThePoolerWritableForOtherClients`, which is the only test
that can see the leak T-0076 fixed). Every branch of the gate has a case there: the three
ARCHITECTURE.md §9 names them — `TestGateRefusesRLSTable`,
`TestGateRefusesTargetAboveTableCap`, `TestGateRefusesRemoteTargetWithoutFlag` —
plus `TestGateRefusesTheSourceUnderAnotherName` for the half of rule 1 that
`SameEndpoint` cannot answer. Do not delete one because it is slow; the table
cap builds 2,001 tables on purpose, against the real constant.

**Never:** open a `pgxpool.Pool` for the source or target anywhere but here;
let the gate return `Eligible` for a `NotProbed` table; add a rung to the
identity ladder that treats "not probed" as safe; leave session state on a
source connection — no `AfterConnect`, no `SET` outside a transaction (T-0076);
send a source statement outside a transaction, or teach `check` an exception
that lets one through (T-0082).
