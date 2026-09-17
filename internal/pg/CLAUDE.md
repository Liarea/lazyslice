# internal/pg

The only package that speaks Postgres. Implements `pipeline.Source`,
`pipeline.Target` (including the gate), `pipeline.Reader` and `pipeline.Writer`
(whose fourth method, `RegisterTypes`, this package is the only real
implementation of), and registers the shape-allowlist tracers on the source
pool. No other package opens a `pgxpool.Pool`.

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
  source.system_id.privilege → source.system_id → source.rollback` trace
  (the privilege check is its own statement as of T-0222 — see this file's
  own T-0222 section below for why), and
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
- **Type registration is on the target pool and only on the target pool**
  (`types.go`, T-0083, closed). ARCHITECTURE.md §11.1 specifies load as
  "CopyFrom with explicit column lists excluding generated columns, **types
  registered in AfterConnect**", and §12 lists type registration under this
  package. It was deferred to the task that built `internal/load`, that task
  shipped without taking it, and until T-0083 nothing did it at all: a value of
  an enum array or a composite had no encode plan on the target and `CopyFrom`
  failed mid-table — **54000** and **42804**, measured on `postgres:16` — with
  THREAT_MODEL.md T8's per-table transaction cleaning up after a run that still
  failed.
  **`writer.RegisterTypes(ctx, *pipeline.Schema)` is the entry point, and the
  only one** (`pipeline.Writer`'s fourth method since T-0093, `pipeline.TypeRegistrar`
  before that; `internal/load` holds a `Writer` and never a `Target`).
  ARCHITECTURE.md §2 prints all four methods (reconciled 2026-09-09). It takes
  the source's enum, domain and composite names out
  of the schema `internal/introspect` read; the loader calls it between §11.1
  item 3 and the first `CopyFrom`, which is the only moment it can be called —
  before it the target has none of those types, and after the copy is too late.
  `Target` had an exported `RegisterTypes` of its own until this was reviewed,
  with no caller but a unit test, and this file described that dead door first;
  it is gone, and `registerTypes` in `types.go` is the private half both would
  have shared.
  **Domains are registered for the sake of arrays over them.** A scalar domain
  column is reported as its base type on the wire and copies unregistered, so on
  its own it would not be in the set — but `postal[]` fails **54000**
  unregistered and loads registered (measured, one column at a time, alongside
  the composite and enum-array numbers above).
  **The array types are read from the target, never spelled.** Postgres truncates
  the array type's name at 63 characters and prepends underscores when the
  obvious spelling is taken, and `LoadTypes` returns *no error* for a name that
  matches nothing — so a guessed `_mood` would leave the column unregistered and
  silent. `sqlTargetUserTypes` resolves each name and its `typarray` on the
  target connection instead.
  **`RegisterTypes` resets the pool.** `AfterConnect` runs once per connection,
  and the gate's, the drops' and the DDL's own connections were all made before
  the types existed; without the reset one of them comes back out of the pool
  for the first `CopyFrom` carrying a type map built when there was nothing to
  register. `TestRegisteringTypesRetiresConnectionsMadeBeforeTheDDL` is the
  guard. A schema with no user-defined type resets nothing, because the hook is
  a no-op without names and a reset would cost the gate's connection for free.
  **A type pgx cannot build a codec for is skipped, never fatal** (`loadTypes`).
  pgx's `LoadTypes` is all-or-nothing over the list it is given: it walks each
  name's dependency closure and ends the call on a dependency that is neither
  user-defined nor in its own default type map. Returning that error from
  `afterConnect` failed the connection, which failed every acquire, which killed
  the pool and the run — *after* the drop and the pre-data DDL, so the operator
  was left with an empty target and a message from inside the driver, and
  `FinishRun` could not even close the marker. It takes one type to do it:
  measured on a stock `postgres:16`, `CREATE DOMAIN d AS money`, `AS pg_lsn` and
  `AS tsquery` each do, and so does anything over `citext` — a domain over it or
  a composite with a field of it — which is a type §11.1 item 2 recreates and
  `internal/plan/keyset.go` special-cases, so a schema lazyslice supports. The
  whole list is still tried first (one round trip, the answer for every schema
  that resolves); a failure retries one name at a time and skips the ones that
  fail, which leaves those columns in exactly the state they were in before any
  of this existed — a scalar domain still loads, a composite still fails 42804
  at `CopyFrom`, and every other table loads. `typeRegistry.skipped()` is where
  the skipped names are readable;
  `TestATypeWithAnUnresolvableDependencyIsSkippedNotFatal` is the guard, over a
  `citext` domain and a `citext`-fielded composite beside an enum array that
  must still register. An error out of `afterConnect` is now reserved for the
  connection itself failing — the catalog read, or a cancelled context.
  **This is the one `AfterConnect` in lazyslice, and `Connect` refuses it on a
  source pool** — `withAfterConnect` is unexported, and a tracer-carrying config
  that reaches `Connect` with a hook is an error, not a convention
  (`TestConnectRefusesAnAfterConnectHookOnASourcePool`). T-0076 is why: a hook on
  a pooled source sets state on a server connection the pooler shares with other
  applications.
- **Registering the OIDs is not enough for a composite, and `compositeCodec` is
  the other half.** The source pool runs in `pgx.QueryExecModeExec`, so every
  value comes back in text format and `internal/extract` scans each cell into an
  `any`: a composite arrives as the Go string `(1234.50,GBP)`. pgx's
  `CompositeCodec.PlanEncode` takes a `CompositeIndexGetter` and nothing else, so
  a string gets no plan; pgx's one fallback for this shape
  (`tryScanStringCopyValueThenEncode`) scans the string in text format into an
  `any` and re-encodes it in binary, and that scan runs the codec's `DecodeValue`,
  which returns a `map[string]any` — not a getter either, so the fallback
  dead-ends with "cannot find encode plan". `compositeCodec` embeds pgx's codec
  and replaces `DecodeValue` alone, returning a value that *is* a
  `CompositeIndexGetter`; the fields are read with pgx's own text and binary
  composite scanners and encoded by pgx's own composite encoder. It carries
  arrays of composites with it, because `ArrayCodec` decodes and encodes each
  element through the map by OID. It is registered by mutating the
  `*pgtype.Type` `LoadTypes` returned **in place**: an array type built over it
  holds a pointer to that very struct, so replacing the map entry would leave the
  array's element on the plain codec.
  **`DecodeValue` is overridden for the text format only**, and the binary format
  is left on pgx's own `map[string]any`. Text is the whole of what the COPY
  fallback reads, and nothing but that fallback has a reason to see a type
  private to this package.
  **The reason first given for that restriction was false and is worth stating
  so**, because a paragraph of this file and a paragraph of
  `testdata/README.md` were built on it: it said a binary read of a composite is
  `internal/verify` reading the target *through this pool*. `internal/verify`
  does not read through this pool. `internal/core` opens a **second** target
  pool with `pg.Connect(ctx, d, nil)` — no `AfterConnect`, no registration — and
  its `readableWriter.Query` is what verify's target reader resolves to.
  Measured on an unregistered pool against `postgres:16`, a composite scans into
  an `any` as the string `(1234.50,GBP)` and an enum array as `{pending,active}`,
  which is what the source side gives too, so §6 item 5 compares like with like
  and this codec is not on verify's path at all. **T-0092 was filed on the false
  reading and should be withdrawn; T-0095 says so** — as does the residual-scan
  worry raised beside it, which reads through the same unregistered pool.
  **`compositeCodec` rests on pgx internals, measured against pgx v5.10.0.**
  `tryScanStringCopyValueThenEncode` is unexported, its text-format scan is not
  documented, and neither is the `ArrayCodec`-holds-the-`*pgtype.Type`-pointer
  behaviour the in-place mutation depends on.
  `TestPgxStillFallsBackFromAStringToACompositeEncode` asserts the fallback's
  three steps directly, through the exported API, so an upgrade that removes it
  fails there by name instead of failing trap 27 with "cannot find encode plan".
  ARCHITECTURE.md §11.1's sentence stops at "types registered in AfterConnect"
  and owes this half; T-0091 records it, and `testdata/README.md` trap 27 states
  it meanwhile.
  **A composite column loads now, and nothing can mask one (T-0094).** That is
  the consequence of this change that matters for THREAT_MODEL.md T1, and it is
  recorded rather than fixed: before this, a composite column failed `CopyFrom`
  at 42804 and could not reach a target at all. `mask.TypeTag` has no tag for a
  composite, so `internal/transform` gives it `famOther`, no rule-pack category
  but `special_category` accepts `famOther`, and a name or value hit on such a
  column drops to `low` with a `type_conflict` reason and is copied;
  `internal/verify`'s second net skips `famOther` and the residual scan walks
  only masked columns. T-0094 owns the plan-time decision (refuse the column, or
  mask a composite field-wise) and the THREAT_MODEL.md T1 entry that must record
  it; both files were outside this package's paths.

**Test.** `go test ./internal/pg/...`; the gate, tracer, identity,
read-only-transaction, type-registration and pooler behaviour need
`go test -tags integration ./internal/pg/...` (the `TestGate*` suite in
THREAT_MODEL.md T2, `TestAWriteInsideASourceTransactionIsRefusedByTheServer`
and `TestSystemIDRunsInsideAReadOnlyTransaction` for T9, and
`pooler_integration_test.go` for ADR-005's pooled endpoints, for
`TestAPooledRunLeavesThePoolerWritableForOtherClients`, which is the only test
that can see the leak T-0076 fixed, and for
`TestAPooledTargetLeaseIsReleasedForTheNextRun`, which is the only test that can
see the same shape on the target's run lease, together with
`TestALeaseOnAPoolerWithOneServerConnectionIsRefusedByName`;
`lease_integration_test.go` is the lease's other server-side half). Every branch of the gate has a case there: the three
ARCHITECTURE.md §9 names them — `TestGateRefusesRLSTable`,
`TestGateRefusesTargetAboveTableCap`, `TestGateRefusesRemoteTargetWithoutFlag` —
plus `TestGateRefusesTheSourceUnderAnotherName` for the half of rule 1 that
`SameEndpoint` cannot answer. Do not delete one because it is slow; the table
cap builds 2,001 tables on purpose, against the real constant.

`types_integration_test.go` is the type-registration half, and it is written
as a control and its answer: `TestATargetWithoutTypeRegistrationCannotCopyAnEnumArray`
copies with nothing registered and requires 54000 and an empty target, so the
failure the mechanism exists for is proven to still exist, and
`TestATargetConnectionCarriesTheSourcesUserTypes` then requires the target to
hold the source's rows exactly — with the target's own types created after three
throwaway ones, so its OIDs cannot be the source's.
`TestATypeWithAnUnresolvableDependencyIsSkippedNotFatal` is the third: a schema
pgx cannot fully resolve must leave the pool alive, register what it can, and
say what it skipped.

**Never:** open a `pgxpool.Pool` for the source or target anywhere but here;
let the gate return `Eligible` for a `NotProbed` table; add a rung to the
identity ladder that treats "not probed" as safe; leave session state on a
source connection — no `AfterConnect`, no `SET` outside a transaction (T-0076);
send a source statement outside a transaction, or teach `check` an exception
that lets one through (T-0082); put an `AfterConnect` hook on the source pool —
`Connect` refuses one, and `withAfterConnect` exists for the target alone
(T-0083); return an error from `afterConnect` for a type pgx cannot build a
codec for — one such type kills every target connection and the run with it, and
an unregistered type is only ever one column that fails the way it failed
before.

## Extension base types (T-TORTURE)

`types.go` registers the source's enums, domains and composites on every target
connection (ARCHITECTURE.md §11.1, ADR-005). An extension's own **base** type —
`citext`, `hstore`, `ltree`, pgvector's `vector` — is none of those three, is in
none of `Schema.Enums`, `Schema.Domains` or `Schema.Composites`, and cannot be
named from the source schema at all. `registerExtensionBaseTypes` resolves them
on the target instead, from `Schema.Extensions`.

- **It only claims two things, and refuses to guess a third.** A
  string-category type is a varlena whose binary form is its bytes, which is what
  `pgtype.TextCodec` writes; `hstore` gets pgx's own `HstoreCodec`. Everything
  else — a category-'U' type pgx ships no codec for — is left unregistered and
  keeps whatever behaviour it had.
- **Why it matters is the array.** A scalar `citext` column loaded before any of
  this; `citext[]` did not, because pgx builds the array codec over the element
  and could not find one, so the binary `COPY` wrote nonsense and the server
  answered `08P01`. Plausible's `monthly_reports.recipients citext[]` is the live
  case (`testdata/regressions/005-array-of-extension-type-not-registered.sql`).
  A scalar `hstore` column could never be loaded at all, before or after; it is
  fixed in the same place.
- **The source pool still carries no `AfterConnect` hook** (T-0076), so none of
  this happens on the source — which is why a `citext[]` there is *sampled* as
  one opaque string and the classifier never sees inside it. That is T-0103, and
  it is a different problem in a different direction.

## The run lease (T-0130)

`lease.go` is the target's other half of ARCHITECTURE.md §9: the gate is a
decision, and `Lease` is the ownership that decision rests on. `AcquireLease`
takes one connection out of the target pool, opens a `READ COMMITTED READ ONLY`
transaction on it, names the run inside that transaction
(`application_name = 'lazyslice run <run_id>'`, `set_config` with
`is_local = true`), and takes `pg_try_advisory_xact_lock` over
`LeaseKey(current_database())`. The transaction stays open for the life of the
lease. `internal/core` calls it **before** `Gate`'s first probe and releases it
in `run.close`, so the lock is held for the whole interval between the verdict
and the last thing the run writes.

- **The key is the server's name for the database, not the endpoint.** Two runs
  can reach one database through a loopback address, a container alias and a
  pooler; all three must collide. The read is `sqlLeaseIdentity`, which is rule
  1's `current_database()` and `pg_backend_pid()` in one round trip: the name the
  key is built from, and the backend `Release` checks it is still talking to.
- **The name goes on before the lock, not after.** A second run reads the
  holder out of `pg_locks` joined to `pg_stat_activity`; a holder that named
  itself only after locking would leave that window unattributable. A holder
  whose `application_name` cannot be read — a role that may not see another
  session's row — is still a refusal, with `Holder` empty and `internal/core`
  printing "another lazyslice run".
- **The connection is held, never returned, and its transaction is never
  committed.** A transaction-level advisory lock lives on its transaction, so
  the whole mechanism *is* holding that transaction open on a connection that
  does not go back. It costs one connection out of a pool that needs the gate,
  one drop transaction and one `CopyFrom` at a time.
- **The lock is transaction-scoped because the target can be behind a pooler,
  and the first version of this was T-0076 again** (review round 1). It took
  `pg_try_advisory_lock` — a *session* lock — on a pooled connection outside any
  transaction, and set `application_name` session-wide. ARCHITECTURE.md §9 names
  a pooler as a path to the target, and nothing refuses one. In transaction mode
  the pooler takes its server connection back when the statement's implicit
  transaction commits, so: the name was set on a backend PgBouncer then handed to
  another application; the lock was left on a backend lazyslice no longer owned;
  `pg_advisory_unlock` in `Release` was routed to whatever backend came next,
  returned false, and had its result discarded by an explicit `//nolint:errcheck`
  — so the orphan sat on the shared connection until `server_lifetime` (3600 s by
  default) recycled it, refusing every later run at exit 4 naming a holder that
  does not exist, with no command to clear it. **Measured, and worse than that
  in one direction**: with a real PgBouncer the second run's client was handed
  the same idle server connection the first run's lock was sitting on, and
  `pg_try_advisory_lock` *succeeded* for it — the same session re-locking its own
  key — so the lease permitted exactly the thing it exists to refuse.
  `pg_try_advisory_xact_lock` inside a held-open transaction closes both: a
  transaction lock is released by `COMMIT`, by `ROLLBACK`, by the backend dying
  and by the client going away and by nothing else, and an open transaction pins
  the pooler's server connection to this client for the run, so the backend is
  ours while we hold it. `TestAPooledTargetLeaseIsReleasedForTheNextRun`
  (`pooler_integration_test.go`) is the guard, and it fails on the old mechanism
  by name in both places: the lock is held by a session with no open transaction,
  and the second run is not refused.
- **Nothing is left on the session, on the target side either.** Both settings
  the lease makes — `application_name` and
  `idle_in_transaction_session_timeout` — are `set_config(..., true)`, local to
  the lease's transaction and put back when it ends. The timeout is disarmed
  because this transaction is idle from the moment the lock is taken until the
  run is over, and a target that sets that timeout — a managed service, or an
  operator bitten once by a forgotten `psql` — would terminate the lease's
  session mid-run and leave the run writing a target it no longer owned, with no
  error anywhere, because the connection is not touched again until `Release`.
  That is a reach the session-level lock did not have and the open transaction
  does, so it is tested rather than asserted:
  `TestTheLeaseOutlivesAnIdleInTransactionTimeout` (`lease_integration_test.go`)
  sets one second on the database, leaves the lease idle for three, and requires
  a second run to still be refused.
- **The room check is the cost of the open transaction, paid where it can be
  named** (`leaseLeavesRoom`). The lease occupies a server connection for the
  whole run, so a pooler with a single server connection cannot serve the rest of
  it. Without the check the gate's very next statement waits out the pooler's
  `query_wait_timeout` and comes back as an opaque `08P01` from inside the
  driver; with it, the lease spends one round trip on a second pooled connection
  and returns an error that says what happened, which `internal/core` renders as
  the target refusal it already has. The wait is the same wait; the sentence is
  what is bought. `TestALeaseOnAPoolerWithOneServerConnectionIsRefusedByName` is
  the guard, and it asserts what the refusal is *not* as well as what it is: a
  target nobody holds must never be refused as `LeaseHeld`, which would send an
  operator looking for a run that does not exist.
- **That the pool has a second connection is enforced, not assumed**
  (`targetPoolFloor`, `withMinMaxConns`, pg.go). `Connect` takes whatever
  `pgxpool.ParseConfig` makes of the operator's DSN and `pool_max_conns` is a
  legal connection-string parameter, so `--target '...?pool_max_conns=1'` would
  have handed the lease the pool's only connection and left `Gate`'s very next
  `pool.Acquire` waiting for a connection that cannot come back until the run it
  is blocking has finished — a hang on startup rather than a refusal, and a
  failure mode the lease itself introduced. `OpenTarget` puts a floor of 2 under
  `MaxConns` (the lease plus one worker) and raises only, never lowers.
  `TestATargetPoolHasRoomForTheRunLeaseAndAWorker` is the guard. The source pool
  holds no lease and gets no floor.
- **`Release` is quiet and idempotent**, and it is quiet because every way it
  can fail still ends the lock: the `ROLLBACK` succeeds and the transaction's end
  releases it; the `ROLLBACK` fails and the connection is closed here (`discard`,
  source.go's `endTx` discipline), which ends the session and the lock with it;
  or `Release` is never reached because the process died, and the lock goes with
  the connection. `pgxpool`'s own `Release` destroys a connection whose
  transaction status is not idle, which is a fourth net under the same
  guarantee. There is no outcome in which a lock survives for a later run to
  trip over — which is exactly what the session-level lock could not say.
  `Release` also compares `pg_backend_pid()` against the pid it recorded at
  acquire and closes the connection instead of returning it to the pool on a
  mismatch: under an open transaction the backend cannot move, so the check is
  the assertion of that invariant rather than a remedy for it.
- **A lease that cannot be taken is not a lease that is free.** Every non-`LeaseHeld`
  error is `internal/core`'s `unreachableTarget` — the gate's reachability
  precondition arriving one statement earlier than it used to, with the same
  code, exit and `{host}`/`{reason}`.
- **`NewRunID` exists because the id is needed before the marker row is.**
  `StartRun` still makes one when the caller brought none, which is what a direct
  caller with no lease gets; `internal/core` brings one, so the refusal a second
  run prints and the row this one writes carry the same id.
- **`Alive` re-asks the question `AcquireLease` answered once, and only that
  question** (T-0252, `docs/reviews/2026-09-15-redteam/round5-still-leaking.json`).
  Everything above this bullet is what makes the lease's transaction unable to
  outlive its own connection; none of it makes the connection itself unable to
  be closed from *outside* — a server-side idle-session reaper with a shorter
  fuse than `idle_in_transaction_session_timeout` disarms, a pooler restart, a
  NAT timeout on a long-lived idle connection, or an operator's own
  `pg_terminate_backend`. The red team's replay was exactly that: the pid
  holding the lease terminated from a second session, a second lazyslice run
  then took the target and loaded into it in full, and the first run resumed
  and dropped and recreated the very tables the second had just filled,
  ending in a raw `SQLSTATE 23505` neither run was refused over. `Alive(ctx)`
  runs `sqlLeaseAlive` — `SELECT EXISTS (... WHERE ... pid = pg_backend_pid()
  ...)` — on the lease's own connection, so it is the connection answering
  about itself rather than a second connection asking whether *anyone* holds
  the key (`leaseHolder`'s question, and a different one). A query that fails
  outright is read as `false`, not propagated: a terminated backend fails the
  query, not the boolean, and a connection unable to say whether it holds the
  lock does not hold it, as far as a caller deciding whether to touch the
  target is concerned. `internal/load`'s `checkLeaseAlive` is the caller —
  before the whole-target recheck (T-0242 above) and before each table's own
  lock-and-recheck (T-0130) — and it is a refusal, `load.refused.lease_lost`,
  never a fall-through, the same direction the bullet above already takes for
  a lease that cannot be taken at all. `Alive` is a point-in-time answer with
  the same residual every lock-and-recheck here already has: a lease
  terminated in the instant after it returns true is not caught by that call,
  only by whichever one runs next.

`Eligibility.MarkerRunID` and `.MarkerStatus` are filled by rule 4 for the same
reason: `internal/load` re-reads that row under the `ACCESS EXCLUSIVE` lock it
takes before each drop, because by then the gate's verdict is a remembered fact
(ARCHITECTURE.md §11.2, THREAT_MODEL.md T2, both amended 2026-09-14). **Never**
treat the gate's verdict as continuing ownership of the target, and never make
the lease optional behind a flag: it is a T2 rail.

## Rule 1 under the role §9 recommends (the 2026-09-15 red team)

ARCHITECTURE.md §9 rule 1 is a disjunction over sameness, and only the
`system_identifier` half survives **aliasing**: the same physical server reached
under two published ports, two host spellings or a pooler name normalises to a
different endpoint every time. `EXECUTE` on `pg_control_system` is **not granted
to `PUBLIC`**, so under the SELECT-only source role §9 itself tells operators to
create, `systemIdentifier` answers `""` and the endpoint comparison stands
alone. The red team published two ports onto one production container, pointed
`--source` at one and `--target` at the other, and the run printed "dropping
public.customers in the target" and did it. With a superuser source the same run
is refused `same_database`, which is what makes the gap specific to the
recommended role.

Two things closed it, and they are deliberately separate values:

- **`Source.ClusterID` / `sqlClusterID`** — the postmaster start time rendered
  in UTC with `to_char` (never `::text`: a `timestamptz` renders in the
  *session's* `TimeZone`, which is set per role, per database, per DSN and by
  `PGTZ`, so the cast made one postmaster answer two sessions with two
  identities — R2-06 through a session-dependent input), the oid of the
  maintenance database `postgres` (`initdb`-assigned only below PostgreSQL 15;
  from 15 on it is **pinned at 5**, so the field distinguishes nothing there),
  `data_directory` when the role may read it (through `pg_settings`, because
  `current_setting()` *raises* on that superuser-only GUC), the server version,
  and `system_identifier` appended as a last field when the role may execute
  `pg_control_system`. **Under the SELECT-only role §9 recommends, on PG 15+,
  that is the start time and the version and nothing else** — say so rather
  than claim the oid separates two sibling compose containers, which it does
  not. `Target.SetSourceCluster` is how `internal/core` hands
  the source's over, between `OpenTarget` and `Gate`, because the read needs a
  source connection the target does not have.

  **Every value here must be the same for every session on the postmaster**
  (ARCHITECTURE.md §9 and THREAT_MODEL.md T2, both amended 2026-09-15 for
  R2-06/T-0190). The first version of this read `inet_server_addr()` and
  `inet_server_port()`, which are properties of the **connection**: `NULL` over
  a unix socket and different again behind anything that re-dials, so one
  cluster reached over two transports had two identities and the arm fell to
  the spelling a second time, which cost a production database. **Never** put
  anything from `inet_server_*`, `inet_client_*`, the backend pid or
  `pg_stat_activity` in it, and add a field only at the **end** — the fields
  are positional.

- **The identity SQL lives here and only here.**
  `internal/load/load_integration_test.go` still re-types the pre-T-0190
  version of it to build a source identity for the gate; it passes on the
  first field and is asserting against a shape the product no longer produces
  (**T-0199**). A caller that needs the identity calls `Source.ClusterID`.

- **`sameClusterIdentity` is hierarchical, not string equality and not a
  vote.** When both sides filled `system_identifier` it **decides alone** —
  equal is one cluster, unequal is two — because it is the only value that
  survives a restart; giving every field an equal veto meant a postmaster
  restart between the source read and the gate's read (start time moves,
  system identifier does not) made the gate stop recognising the source's own
  cluster, and any rendering difference in a weaker field outvoted a matching
  identifier. Only when one side could not read it do the weaker positional
  fields decide, each skipped when either side left it empty, because the two
  sides are read by two roles: `system_identifier` and `data_directory` are
  readable by a superuser target and not by the `SELECT`-only source role §9
  recommends, and a privilege one side lacks must never read as a difference.
  Its second answer, `comparable`, is false when no field was filled on both
  sides, and that is what `clusterUnknown` now means by unknown.
  `clusterIDSystemIDField` is the position, and it moves if a field is ever
  added before it.
- **`clusterUnknown` fails closed.** When neither identity can be compared —
  no system identifier on one side *and* no cluster identity — a target whose
  `current_database()` is the source's own database name is **refused**. The
  endpoint spelling is exactly what an alias changes, so "the endpoints differ"
  is not evidence of anything there. In practice this arm is nearly
  unreachable, because the cluster read succeeds for any role; the caller it
  does catch is one that supplies neither, which is why
  `internal/load/load_integration_test.go` now supplies what `internal/core`
  supplies.

**The cluster identity is not a fallback inside `SystemID`, and must not
become one.** §11.2's marker binding is recorded against the system identifier,
and a value that changes when the source cluster restarts would make the gate
refuse a target lazyslice itself wrote on the next run.

**What actually pins the transport rule is the statement's own text.**
`TestTheClusterIdentityReadsNothingFromTheConnection` blacklists the
connection-scoped functions and
`TestTheClusterIdentityRendersTheStartTimeInAFixedZone` forbids a bare
`timestamptz` cast and requires the UTC `to_char`; that is the only level at
which a container suite can state the unix-socket case, because it reaches its
server over TCP both times.
`TestOneClusterReachedOverTwoEndpointsHasOneClusterIdentity` is the end-to-end
half — one container reached over its own endpoint and over a second one
(`testutil.SecondEndpoint`), one identity, and the gate refuses the source named
the second way — and it **does not discriminate the original defect**: the proxy
dials the same backend `host:port`, so `inet_server_addr()`/`inet_server_port()`
answer identically over both routes and the pre-fix SQL passes it unchanged. Do
not cite it as the evidence for the transport rule. Reaching one server over a
genuinely different server-side socket, so that the pre-fix statement fails, is
**T-0200**.
`TestOneClusterReadUnderTwoSessionTimeZonesHasOneClusterIdentity` is the
integration half that *does* discriminate: one cluster read with
`ALTER DATABASE ... SET TimeZone` in between, one identity required.

`TestGateRefusesAnAliasedSourceWithNoSystemIdentifier` and
`TestGateAcceptsASameNamedDatabaseOnADifferentCluster` are the two directions:
the alias is refused, and a database of the same name on a genuinely different
cluster stays eligible — which is the ordinary case (`app` on production, `app`
locally) and what the fail-closed arm must not take down.

## Decisions made for T-0222 (2026-09-16, R2-06's R3 replay)

- **`has_function_privilege` cannot guard a call inside the same statement,
  and the section above's "in practice this arm is nearly unreachable" was
  written before this landed and is corrected by it.** The round-3 replay
  (`docs/reviews/2026-09-15-redteam/round3-still-leaking.json`) denied the
  source role `pg_postmaster_start_time()` too — a hardened cluster that
  revokes monitoring functions from `PUBLIC`, the same class of hardening the
  original finding already assumed for `pg_control_system` — and the old
  `sqlClusterID` was one `SELECT` concatenating four fields with `||`, so the
  permission error on that one call failed the whole row: `ClusterID`
  collapsed to `""` even though the maintenance database's oid and the server
  version needed no privilege that role lacked. The fix the replay's own JSON
  suggested — `CASE WHEN has_function_privilege(...) THEN
  pg_postmaster_start_time() ... ELSE '' END` in one statement — **does not
  work**: measured by hand against `postgres:16`, a role
  denied `EXECUTE` still gets `permission denied for function
  pg_postmaster_start_time` out of a `CASE`, a scalar subquery around the
  call, and a CTE with a `WHERE has_function_privilege(...)` guard alike.
  `TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied` runs none
  of those three shapes and asserts no permission error, so it is not the
  evidence for this; `TestTheClusterStartTimeIsGuardedByASeparateStatement`
  is the regression pin, and fails if the guard and the guarded call are ever
  recombined into one statement. Postgres checks `EXECUTE` for every function call in a compiled plan at
  executor-initialisation time, before any branch, clause or subquery around
  it is evaluated — the guard has to be a **separate statement that carries no
  reference to the guarded function at all**, checked before the guarded
  statement is ever sent. `sqlClusterID` is now three constants
  (`sqlCanReadClusterStartTime`, `sqlClusterIDStartTime`, `sqlClusterIDRest`)
  and `sqlSystemID` is two (`sqlCanReadSystemID`, `sqlSystemID`); `readSystemID`
  and `readClusterID` (source.go) run the check-then-call sequence and are
  shared by `Source.SystemID`/`Source.ClusterID` and target.go's
  `systemIdentifier`/`clusterIdentity`, so both sides of the comparison
  degrade the same way. `TestSystemIDRunsInsideAReadOnlyTransaction`'s traced
  shape list grew a `source.system_id.privilege` entry ahead of
  `source.system_id` for the same reason.
- **`EXECUTE` on `pg_control_system()` is granted to `PUBLIC` by default on
  stock PostgreSQL 16** — verified directly against a fresh container, a
  freshly created `NOSUPERUSER` role, and no grant beyond `CONNECT` on the
  database. ARCHITECTURE.md §9, THREAT_MODEL.md T2 and this file (the
  "Decisions made for T-0190" section above, and the `sqlClusterID` bullet
  before it) all state the opposite — "`EXECUTE` on `pg_control_system` is not
  granted to `PUBLIC`" — repeatedly, and none of them cites a source; every
  existing test that wanted a role without it simulated the absence by passing
  `""` by hand rather than by creating a real restricted role and measuring
  it. This task's own integration test needed the function actually denied to
  exercise the code path it is testing, so it revokes `EXECUTE` on both
  `pg_control_system()` and `pg_postmaster_start_time()` from `PUBLIC`
  explicitly rather than relying on either being denied by default. Whether
  the documented claim was ever true on a version this project still supports,
  or is corrected prose debt from the start, is outside this task's paths to
  resolve across ARCHITECTURE.md, THREAT_MODEL.md and this file's own history
  — filed as **T-0224**.
- **`sameClusterVerdict` (target.go) replaces `Gate`'s inline switch**, whose
  `default` arm used to fall back to `targetRef.SameCluster(source)` — a
  comparison of the two normalised endpoint spellings — exactly when neither
  identity could be compared. That is precisely the case rule 1's identity
  check exists to cover for: one cluster reached over two transports (a
  published TCP port and the unix socket, a pooler, an SSH tunnel) normalises
  to two different endpoints by construction, so falling back to the endpoint
  comparison when the identity check has nothing to go on answers "different
  cluster" from the one signal that is guaranteed to be wrong in exactly this
  case. The R3 replay's mid-migration shape — a target database named
  differently from the source's (`app` reached directly, `newprod` over a
  second route to the same server) — also does not reach `sameCatalog`'s own
  fail-closed arm a few lines below, because that arm is gated on
  `currentDB == source.Database`. `sameClusterVerdict` now answers `true`
  (possibly the same cluster) whenever nothing could be compared, matching the
  direction `clusterUnknown`'s fail-closed refusal already took for the
  narrower case it covers. `TestSameClusterVerdict` (cluster_test.go) pins the
  four cases as a pure function, with no database needed.
- **`sameClusterIdentity`'s field-by-field fallback is not "any two fields
  present and equal is one cluster" (T-0222 fix round, review).** The
  maintenance database's oid (`clusterIDMaintenanceOIDField`) is pinned to `5`
  for every cluster from PostgreSQL 15 on, and the server version
  (`clusterIDServerVersionField`) is shared by every cluster built from the
  same image, so agreement on either is not evidence of one cluster — only
  disagreement is. Under the role denied both `pg_control_system` and
  `pg_postmaster_start_time` this section already describes, those two are the
  only fields either side can fill; before this fix, agreeing on both reported
  `known=true, same=true`, so two unrelated clusters from the same image
  compared as one and `Eligibility.SameCluster` came back confidently true for
  a legitimately separate target. `clusterIDWeakField` names the two
  positions; the comparison loop now tracks whether any *non*-weak field
  contributed, and reports `known=false` — the same "unknown" as no common
  field at all — when only weak fields agreed, while a weak field's
  disagreement still decides "different cluster" on its own, because
  disagreement needs no specificity to be believed. The `cluster_test.go` case
  this covers is named for what it now asserts:
  `"start time denied too: oid and version alone are not decisive, so this is
  unknown"`.
- **`readClusterID`'s recovery from a denied start time only works inside an
  open transaction if the failed statement does not leave it aborted**
  (T-0222 fix round, review). `Source.ClusterID` runs `sqlCanReadClusterStartTime`,
  `sqlClusterIDStartTime` and `sqlClusterIDRest` inside one `REPEATABLE READ
  READ ONLY` transaction; if the privilege check answers yes but the guarded
  read still fails — the privilege revoked between the two statements, or any
  other server error — the transaction is left aborted, and `sqlClusterIDRest`
  sent on it next would itself fail with `25P02` and be swallowed as
  "unreadable", losing the whole identity rather than the one field. This was
  invisible on `target.go`'s `clusterIdentity`, which runs the same three
  statements on the gate's autocommit connection, where a failed statement
  never touches the next one — so the two callers of `readClusterID` degraded
  differently despite sharing the helper. `readClusterID` now takes an
  `inTransaction bool`; when true it takes `SAVEPOINT cluster_start_time`
  before the guarded read and releases or rolls back to it afterward, so a
  failure there costs one field and not the transaction. `Source.ClusterID`
  passes `true` and registers the three new fixed-text shapes this needs
  (`source.cluster_id.savepoint`, `.release_savepoint`,
  `.rollback_to_savepoint`); `target.go`'s `clusterIdentity` passes `false` and
  registers nothing new, because its connection carries no tracer to register
  against and needs no savepoint.
  **The exact race — a privilege `REVOKE`d by another session between the two
  statements — cannot be reproduced by actually revoking**: a `REPEATABLE
  READ` transaction's catalog reads use the snapshot taken at `BEGIN`, so a
  `REVOKE` another session commits after that is not visible inside it, and
  the guarded read would keep succeeding.
  `TestReadClusterIDRecoversFromAFailedStartTimeReadInsideATransaction`
  (`gate_integration_test.go`) forces the same failure shape a different way —
  a role that genuinely holds `EXECUTE` on `pg_postmaster_start_time()`, with
  the built-in `to_char(timestamp, text)` `sqlClusterIDStartTime` calls
  shadowed by one that raises, using `search_path = public, pg_catalog` to put
  a function of our own ahead of the one it is shadowing — and asserts both
  that the maintenance oid and server version still come back and that the
  transaction is still usable afterward, not only that `readClusterID` itself
  returns no error.
- **Two of this task's own regression pins asserted a shape that correlated
  with the fix rather than the property the surrounding prose claimed they
  pinned (T-0222 fix round, review).**
  `TestTheClusterStartTimeIsGuardedByASeparateStatement` used to assert only
  that `sqlClusterIDRest` does not mention the guarded function and a
  compound condition on `sqlCanReadClusterStartTime` that a straight
  `has_function_privilege` guard could never trip — so rewriting
  `sqlClusterIDStartTime` as `SELECT CASE WHEN
  has_function_privilege('pg_postmaster_start_time()','EXECUTE') THEN
  to_char(...) ELSE '' END` (precisely the shape measured not to work) passed
  it unchanged, and so did deleting the privilege check from `readClusterID`
  outright. It now asserts the property by name: the guard contains
  `has_function_privilege` and none of the guarded call's own shape
  (`to_char(`, `AT TIME ZONE`); the guarded statement calls
  `pg_postmaster_start_time` and never `has_function_privilege`; and the two
  constants are not equal. `sqlCanReadSystemID`/`sqlSystemID` had no pin of
  this kind at all before this round; `TestTheSystemIdentifierIsGuardedByASeparateStatement`
  is the same shape for that pair.
  `TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied`'s central
  assertion (`!e.SameCluster`) is the other one: it is satisfied by
  `sameClusterVerdict`'s unknown-defaults-true arm on its own, because the
  fixture's two clusters are one cluster read twice and the only fields the
  restricted role can compare — the maintenance oid and the server version —
  are `clusterIDWeakField`s that agree, which the third T-0222 amendment above
  reports as unknown rather than as evidence of sameness; a reverted
  `has_function_privilege` split reaches that same unknown state by a
  different route (`sourceCluster == ""`) and passes the old assertion
  identically. The test's own failure message claimed a discrimination the
  fixture does not exercise, and so did ARCHITECTURE.md §9's "still
  contributes the rest, on both sides of the comparison" — both corrected to
  say the surviving weak fields prove difference only, never sameness.
  `TestGateDistinguishesAGenuinelyDifferentClusterFromWeakFieldsAlone` adds
  the one case that does discriminate: a second, real cluster (a
  `postgres:14` container, so its server version disagrees with the
  `postgres:16` source's on its own) reached under the same restricted role,
  where `sameClusterIdentity` must read the weak fields' disagreement as
  "different cluster" — the only outcome the split can produce that the
  default arm cannot.

## A read replica: the postmaster start time is decisive one-directionally (T-0241, the 2026-09-17 round-4 red team)

`docs/reviews/2026-09-15-redteam/round4-still-leaking.json`, "a read
replica": `--source` a streaming standby under the `SELECT`-only role §9
recommends, `--target` a database on that standby's own primary, stock
privileges. A primary and its hot standby share `system_identifier` but never
the postmaster start time — the standby's postmaster started strictly after
the primary's, when `pg_basebackup` finished, not when the cluster did — and
under the restricted role that field was the one surviving decisive field
once `system_identifier` was unreadable, so `sameClusterIdentity` read
"different cluster" with total confidence and the gate let the write through
with no `same cluster as source` line. Two independent changes, deliberately
kept separate the way ClusterID and SystemID already are.

- **`clusterIDDifferenceUnreliableField` is the mirror image of
  `clusterIDWeakField`, and so far names exactly one field: the postmaster
  start time (position 0).** A weak field (the maintenance oid, the server
  version) can prove two clusters different but never that they are the
  same; the start time is the opposite — a match remains as decisive as any
  other non-weak field (two independent postmasters starting in the same
  microsecond is not a case), but a *mismatch* no longer decides anything on
  its own, because a standby's start time is guaranteed to mismatch its own
  primary's. `sameClusterIdentity`'s comparison loop now branches on
  agreement before it branches on which field disagreed: an agreeing field
  still sets `known` (and `decisive` when it is not weak) exactly as before;
  a disagreeing field is checked against
  `clusterIDDifferenceUnreliableField` first, and the start time's
  disagreement is skipped — read exactly as a field neither side filled —
  while every other field's disagreement still sets `same = false` and
  `known = true` as before. **Corrected below (T-0255, round-5 red team):**
  `data_directory` was *not* left unaffected — `data_directory` joins the
  start time, but only when the source is a standby.
  `TestADataDirectoryDisagreementStillProvesTwoClusters`
  (cluster_test.go) pins that the non-standby case stays decisive, so the
  carve-out cannot silently widen into "no field's disagreement matters
  regardless of standby." `internal/core/CLAUDE.md`'s own T-0241 section is
  now stale about this package's shape (the call site's shape and the "can
  still be fooled by `data_directory`" claim this fix disproved) — filed as
  **T-0261**, since that file is outside this package's paths.
  **Consequence, stated rather than hidden:** a weaker role that could
  previously distinguish two genuinely different clusters from a start-time
  mismatch alone now reads that shape as unknown too —
  `TestSameClusterIdentitySkipsFieldsOneSideCouldNotRead`'s former "two
  clusters, weaker role" case is renamed and its expectation changed
  (`(false, true)` → `(false, false)`) to say so. That is the fail-closed
  direction: an honest "unknown" that costs a warning line (or ADR-013's
  headless refusal) is preferred to a confident "different" that a standby
  can produce and a genuinely different cluster can too, with nothing here
  able to tell which. `TestGateDistinguishesAGenuinelyDifferentClusterFromWeakFieldsAlone`
  (this file, T-0222 section above) still proves a weaker role can
  distinguish two real clusters — through the server version, a field this
  fix does not touch.
  `TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown`
  (cluster_test.go) pins the round-4 shape end to end, over hand-written
  identity strings for the reason `gate_integration_test.go`'s own T-0222
  cases already give: a primary-and-standby integration fixture needs
  `pg_basebackup` run inside a second container on a Docker network the two
  share, which is more than `internal/testutil`'s `Postgres`, `PgBouncer` and
  `SecondEndpoint` build (`internal/testutil/CLAUDE.md` lists all three; none
  stands up a second *server*). That gap is recorded in THREAT_MODEL.md T2's
  2026-09-17 amendment rather than silently left implicit.
- **`Source.Replica` is a new, independent, positive check — not a repair of
  the identity comparison above, and it does not wait on that comparison's
  answer.** `pg_is_in_recovery()` is executable by `PUBLIC` on every
  supported version, unlike `pg_control_system` and (on a hardened cluster)
  `pg_postmaster_start_time()`, so it needs no grant and no fallback. When it
  answers true, `pg_stat_wal_receiver`'s `sender_host`/`sender_port` are read
  too, scanned as `sql.NullString` because that view restricts those two
  columns to a superuser or a role holding `pg_read_all_stats` — an ordinary
  role sees `NULL` there, not a permission error, which is why
  `readReplicaStatus` treats an unreadable sender the same way `SystemID`
  and `ClusterID` treat an unreadable field: the zero value, never an error.
  It runs inside its own `REPEATABLE READ READ ONLY` transaction, registered
  against the source tracer, for the same T-0076/T-0082 reason `SystemID`
  does — no statement this package sends may reach the source outside a
  transaction. `target.go` has no equivalent: nothing here ever asks whether
  a *target* is in recovery, because a run refuses a remote or a same-cluster
  target on other grounds long before replication status would matter.
  `internal/core`'s `discover` calls it once, alongside `SystemID`, and
  prints `source.warn.standby` whenever it is true — interactive runs
  included, because an operator watching is exactly who should be told the
  database being read is not itself the primary — and refuses a headless run
  with no `--target` outright (`source.refused.standby_no_target`), *whether
  or not* `Eligibility.SameCluster` ends up true: that signal can still be
  fooled by a standby whose `data_directory` genuinely differs from its
  primary's, which the identity fix above does not and must not change, and
  the round-4 red team's second reproduction reached its target through a
  discovered `$DATABASE_URL` on a ladder whose cheap `clusterKey` check never
  flags a standby and its primary as one cluster at all — they ordinarily
  publish on different ports. `TestReplicaOnAnOrdinarySourceIsNotAStandby`
  (`replica_integration_test.go`) is the one thing proven against a real
  server: an ordinary, non-standby source answers `pg_is_in_recovery()`
  false with no error and no second statement sent, under the same role §9
  recommends. `internal/core/names_test.go`'s `TestStandbyNoTargetRefusal`
  pins the headless-refusal decision as a pure function, the way
  `TestSameClusterVerdict` (this package) pins the identity comparison one
  layer down; the `Standby == true` half is the same untested-against-a-real-
  server gap the paragraph above names.
- **The recommended-role snippet, and `internal/core`'s `CodeRoleWritable`
  message that renders the same statement, both gain
  `GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro;`.**
  ARCHITECTURE.md §9's "the identity works without it" claim (the T-0190
  section above) was false for exactly this shape: without the grant, the
  identity degrades to the postmaster start time and the server version, and
  the start time is now the one field that can never settle a standby
  against its own primary. Granting `EXECUTE` restores `system_identifier`,
  which decides the comparison outright and needs none of this section's
  reasoning at all.

## A standby's data directory is not decisive either (T-0255, the 2026-09-17 round-5 red team)

`docs/reviews/2026-09-15-redteam/round5-still-leaking.json`, "the standby
data_directory variant": the same shape the T-0241 section above closes, with
`data_directory` readable this time (`pg_read_all_settings`, a monitoring-grade
grant) instead of denied. `data_directory` was still treated as unconditionally
decisive on disagreement — this file said so outright, in the paragraph the
bullet above corrects — and a co-hosted standby's data directory disagrees with
its primary's by construction (`pg_basebackup` into a second directory beside
its source is the ordinary shape of standing one up), so under that role the
comparison read "different cluster" with total confidence exactly as the start
time alone did before T-0241.

- **`clusterIDDifferenceUnreliableField(i int, standby bool)`** now takes the
  standby answer and skips `data_directory`'s disagreement (alongside the start
  time's, which it skips unconditionally) only when `standby` is true.
  `TestADataDirectoryDisagreementStillProvesTwoClusters` gained a
  `standby == true` case with a disagreement outside the carve-out (the server
  version) so the conditional widening cannot collapse into an unconditional
  one by mutation — see this section's fix-round amendment below, which found
  the original version of that test did not actually pin this.
- **A second, cheaper and independent rail**, `senderMatchesTarget`
  (`target.go`): when the source is a standby and `pg_stat_wal_receiver`'s
  sender resolves to the target's own `host:port` (`dsn.Ref.SameCluster`,
  normalised string equality — see ARCHITECTURE.md's own amendment for what
  that does and does not see through), `Eligibility.SameCluster` is set `true`
  outright, ahead of the identity comparison. This is an opportunistic extra
  signal, not a repair of the identity comparison: a role that cannot read the
  sender columns, or a target reached by a different route than
  `primary_conninfo` spells, gets nothing from it and falls back to the
  `data_directory` carve-out and the unknown-defaults-true direction.

**Fix-round corrections (2026-09-17, this file's own review round).** The first
landing of this section, and of the `data_directory` amendment above, both
still said outright that "`data_directory` is unaffected: nothing forces a
standby's data directory to disagree with its primary's the way the start time
is forced to, so it still decides 'different cluster' on disagreement" — the
exact claim this fix disproves, left standing in the same files the fix
touched. `source.go`'s doc comment on `sameClusterIdentity` had the identical
sentence twenty lines from the code it was rewriting. Both are corrected in
this pass; the lesson generalises past this one task; do not let a `sameX,
unless Y` finding land beside prose that still asserts `sameX` unconditionally.