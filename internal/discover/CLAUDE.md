# internal/discover

The discovery ladder (rungs 0-5: yml, env/`.env`, libpq, running container,
stopped container, compose name) and candidate verification/de-duplication.
Subpackages: `dockerctx/` resolves the Docker endpoint; `provision/` is the
`--create-target` path. The target *gate* is not here — that's
`internal/pg` (`Target.Gate`), because gating needs a live connection this
package's 1s dial budget does not allow for.

**Contract.** ARCHITECTURE.md §2 "discover" and §9: implements
`pipeline.Discoverer.Discover(ctx, workdir, sink) ([]Candidate, error)`.
`provision/` implements `provision.Provisioner`, a stage seam declared in that
package rather than in `internal/pipeline` — see that package's CLAUDE.md for
why. `pipeline.Provisioner` is removed: it was dead, nothing implemented it.

**Rules.**
- 2s total listing budget, 1s per-candidate dial (ARCHITECTURE.md §9) — do not
  add a probe that can exceed either.
- Inside the dial: at most three *reads* per candidate (version, the
  `pg_class` count/hint, `to_regclass('lazyslice_meta')`), inside one
  `REPEATABLE READ READ ONLY` transaction — so five statements on the wire,
  counting the `BEGIN` and the `ROLLBACK` that carry them (T-0081). Never probe
  emptiness table by table here; that is the gate's job, after discovery
  (THREAT_MODEL.md T2).
  **This is drift from the spec, not a reading of it (T-0085).**
  ARCHITECTURE.md §9's `Discoverer` contract says "at most three *statements*
  per candidate" and names the three reads; the transaction T-0081 added makes
  five. The rail stays and the amendment is owed — the sentence should read
  "at most three reads per candidate, inside one `REPEATABLE READ READ ONLY`
  transaction". ARCHITECTURE.md is outside this package's paths, so it is filed
  rather than edited here; do not resolve the difference by rereading
  "statements" as "reads".
- `provision/` is the *only* code in the tree that creates or starts a
  container, and only behind `--create-target` or a "yes" to Q1/Q1′ — never
  called unconditionally. `Options.dial` builds a **read-only** Docker client
  (`dockerAPI` has `Ping`, `ContainerList`, `ContainerInspect` and nothing
  else); `Options.provisioner` is the separate seam that can write.
- One blocking question per run, and Q1 and Q1′ are mutually exclusive
  branches of the same no-target state. A prompt is written to stderr and read
  from the controlling terminal; `os.Stdin` is never touched.
  `TestPipedStdinIsNotAnAnswer` is ADR-008 §7's own owed test and the one that
  matters — a pipe holding `y`, a terminal that answers nothing, and the
  assertions that nothing was created and the pipe was not drained;
  `TestPromptsNeverReadStandardInput` is the cheap static guard beside it,
  parsing every non-test file in this package.
- Source selection (most-local reachable candidate with the most tables, never
  a question) lives here; do not turn it into a question without an
  ARCHITECTURE.md §9 change.

**Test.** `go test ./internal/discover/...`; container-touching parts need
`go test -tags integration ./internal/discover/...`.

**Never:** create or start a container outside `provision/` or outside
`--create-target`/Q1/Q1′; run an unbounded emptiness probe inside the dial; let
discovery itself decide a target is eligible — only `Target.Gate` does that;
read an answer from `os.Stdin`.

## Decisions made during implementation

Recorded here because ARCHITECTURE.md is silent on them, or because this
package had to deviate from it (root CLAUDE.md). Each is a deviation a
reviewer should see rather than discover. This build ships rungs 0 to 4 and
provisioning (T-PROVISION); rung 5 is a naming source and contributes no
candidate.

- **`Resolve` is the first-run entry point, beside `Discover`.**
  `pipeline.Discoverer.Discover` returns candidates and decides nothing, which
  is right for a stage interface and not enough for first run: §9's selection
  rules, the one blocking question, the controlling terminal and an ADR-005
  exit code have no carrier on that interface. `Resolve(ctx, Options, sink)
  (Result, error)` is that carrier, and `Discover` is implemented in terms of
  the same ladder walk.
- **`internal/core` calls `Resolve`; `cmd/lazyslice` calls neither.** The ladder
  is walked by `core`'s discover stage (`resolveEndpoints`), which is what keeps
  `cmd/CLAUDE.md`'s "call no `internal/<stage>` package directly" true. It ran
  in `cmd/lazyslice`'s `firstRun` for one task, because `internal/core` was
  outside T-DISCOVER's paths; **T-0061** moved it, and the move changed nothing
  in this package. `Resolve` is called for `lazyslice` with no arguments only —
  the five stage subcommands reach `Discover`, which walks the ladder to print
  it and chooses nothing before `core` stops at exit 3 — because giving them a
  first run of their own is a widening no task has asked for.
- **`Refusal`, not `core.Stop`.** `core` imports this package, so the
  dependency cannot go the other way; `Refusal` carries the same three things
  (`Code`, `Exit`, `Args`) and `cmd/lazyslice`'s `report` maps it the same way.
- **The gate decides eligibility, and target-shape is only two things.**
  ADR-008 §5's tie-break is written "among eligible targets", which cannot be
  evaluated before `Target.Gate` runs — and §9 says the gate runs *after*
  discovery. So `chooseTarget` applies §5's order among the candidates that are
  reachable and are not the source, hands the winner to `internal/core`, and
  the gate's own refusal stands verbatim if it does not pass. `EmptyHint` is
  **not** a filter and must not become one: it is
  `coalesce(bool_and(relpages = 0), true)`, and `relpages` is a planner
  statistic that `TRUNCATE` and `DELETE` do not reset, so a database the gate's
  own `SELECT EXISTS` probe finds empty can carry `EmptyHint` false until
  something ANALYZEs it. Filtering on it was discovery deciding ineligibility
  from a statistic, which this file's Never list forbids.
- **A marker does not outrank an empty database in the tie-break.** ADR-008 §5's
  third step is "the one carrying our *bound* marker", and boundness is the
  gate's rule 4 — it needs the source's `system_identifier` and catalog, which
  a dial cannot see. So `plausibleTarget` (marked *or* apparently empty) is one
  rank and the byte order of `(host, port, database)` settles ties inside it.
  Another project's marked, full target therefore cannot displace an unmarked
  empty one. Only the winner reaches `internal/core`, and **that is the
  behaviour, not a gap** (**T-0062**): when the gate refuses it the run stops at
  exit 4 with the gate's own refusal and the runner-up is never tried, because
  `core.Request` carries one target and not a list. §5's ranking therefore runs
  *before* the gate and one refusal ends the run — a fall-through would load
  into a database the operator was never shown a decision line for.
  `internal/core`'s `TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp` pins
  it. The runner-up is still printed in the target decision's provenance, which
  is what tells the operator what to pass to `--target`. **Owed:** ADR-008 §5
  and ARCHITECTURE.md §9 still say the tie-break runs "among eligible targets",
  and eligibility is `Target.Gate`'s verdict, so both still read as a ranking
  over gate-eligible candidates. They need the amendment this paragraph
  describes; neither file has been inside the paths of the tasks that built
  this, and a reader who takes §5 literally would add the fall-through.
- **Q1, Q1′, the controlling terminal and provisioning (T-0063, T-PROVISION).**
  T-DISCOVER left this as a stated seam: `Options` carried no `Provisioner`, no
  `Prompter` and no `Yes`, and `ask.go`/`tty_unix.go`/`tty_windows.go` were
  removed rather than left unreachable. They are back, and the shape differs
  from what that note predicted:
  - `pipeline.Provisioner` was **not** widened: `internal/pipeline` was outside
    T-PROVISION's paths, so the widened contract is
    `provision.Provisioner`/`provision.Result`, carrying the `Candidate` and
    the `DSN` the generated `POSTGRES_PASSWORD` lives in. `pipeline.Provisioner`
    itself is gone — T-0071 deleted the dead interface once
    `provision.Provisioner` was confirmed to be the type nothing else needed to
    share; see `provision/CLAUDE.md`.
  - **`Options.Yes` is set by `internal/core`'s `resolveEndpoints`, as of
    T-0071.** `core.Request.Yes` is copied into `Options` alongside
    `CreateTarget` (`internal/core/run.go`), which is what lets ADR-008 §7's
    headless path fire under an allocated TTY with no controlling terminal
    answer available — `docker run -t`, `script(1)`, `tmux` — instead of
    `prompt.Confirm` opening `/dev/tty` and blocking with no timeout.
  - **Whether there is anybody to ask is settled before the question is
    built.** Q1 names `postgres:<major>` and a free port, both of which cost a
    dial and a bind; a headless run takes the hard failure without paying for
    either. `prompterFor` is that split.
  - **Q1 headless names `--create-target`, but only where a container could be
    created.** ADR-008 §6's table gives Q1's headless failure `--create-target`
    unconditionally; on an endpoint that is not local or does not answer, that
    flag is the refusal two lines above (`target.refused.docker_not_local`), so
    the run names `--target` there instead. Telling an operator to pass a flag
    that is about to refuse them is not a next step.
- **`--create-target` answers Q1 and does nothing else.** An earlier build of
  this package made the flag short-circuit the target tie-break and outrank the
  committed yml's `target:`, on the argument that a laptop with any other
  Postgres container running would otherwise ignore the flag. That is a change
  to ADR-008, which is accepted and frozen: §6 scopes the flag to Q1's headless
  answer, Q1 fires only where nothing target-shaped was found, and §1's list of
  what short-circuits the ladder — `--source`, `--target`, the positional DSN,
  rung 0 — does not contain it. Root CLAUDE.md requires a superseding ADR for
  that, not a task, so the widening is reverted and
  `TestCreateTargetDoesNotOutrankTheLadderOrTheYml` pins the ADR's semantics.
  **Owed:** if the widening is wanted, it needs a tracker task for a superseding
  ADR; the operator's answer today is `--target`, which does name a database.
  **ADR-016 (T-0327) is that ADR for exactly one record:** a committed target
  of lazyslice's own container (`recordedContainer`, `recorded.go`), where the
  flag provisions or reuses this directory's own container instead. Every
  other record and the ladder's tie-break keep the semantics above, and the
  yml case of `TestCreateTargetDoesNotOutrankTheLadderOrTheYml` still pins
  them.
- **A provisioned target's credential is recovered at rung 0
  (`rung0Target`).** `lazyslice.yml` records a reference and never a password
  (ADR-004), and the ordinary password sources — `PGPASSWORD`, `~/.pgpass`,
  `--password-command` — cover a database the developer administers. They do
  not cover the one lazyslice creates itself: `--create-target` mints a random
  `POSTGRES_PASSWORD` whose only home is the machine-local state dir, so the
  second run dialled its own container with no password at all and stopped at
  exit 4 with "password authentication failed", which contradicts
  ARCHITECTURE.md §9's "the container survives the run" and ADR-004's
  zero-question second run. `provision.Password` is the reader; the container
  name is computed from the working directory and compared with the file's
  label rather than taken from it, because the label is committed text and must
  not become a path this reads.
  **Since ADR-016 (T-0327) this is the fallback, not the first answer.** Keyed
  by this directory's project, it found nothing for a `lazyslice.yml` copied
  to another directory (dogfood session 2) and nothing on a second machine, and
  the run stopped on a bare SQLSTATE 28P01. `recorded.go` now looks the
  recorded container up by name on a local Docker endpoint (read-only: one
  list, one inspect), requires `provision.LabelProject` on it before reading
  its environment, and connects on its live binding with its own
  `POSTGRES_PASSWORD`; stopped is Q1′, absent is Q1 naming it (headless:
  `target.refused.container_missing`, exit 4). `rung0Target` runs only when no
  local endpoint answers. The lookup runs after the source is known, and
  without a walk when the source is named, because the two ways it ends in a
  new container need the source's major. `askQ1` takes its lead sentence and
  its refusal as parameters for that state, so Q1 stays one code path.
- **Rung 4's port comes from `HostConfig.PortBindings`, not from the summary.**
  A container that is not running publishes nothing, so `Summary.Ports` is
  empty for every stopped container; the configured binding is what the daemon
  will publish when it starts. A binding with an empty `HostPort` — a dynamic
  publish whose port is chosen at start — yields **no** candidate, because the
  candidate list would have to name a port that does not exist yet.
- **A rung-4 candidate is never dialled**, and `walk` skips `probe` for it. It
  carries `Reachable = false` with "the container is stopped" as its
  `ConnectErr`, which is exactly why ADR-008 §6 asks Q1′ *before* the gate:
  reachability is the precondition of every gate rule, so "the eligible
  target's container is exited" is a state that cannot exist after the gate has
  run.
- **A started container is `FromContainer`, not `FromStoppedContainer`.** Once
  Q1′ has started it, it *is* a running container, and that is the rung the
  next run finds it at; stamping rung 4 into `lazyslice.yml` would record a
  fact that stopped being true during the run that recorded it.
- **`Options.dialCandidate` is a second test seam beside `Options.dial`.** Q1
  chooses `postgres:<source major>` and hands the container it started back
  through the same dial every candidate gets, and neither fact can be produced
  in a unit test without a running server. It is unexported for the same reason
  `dial` is.
- **An unknown source major is a refusal, not a guess.** A source that
  short-circuited the ladder was never probed, so the provisioning path dials it
  once; if it still does not answer, the run stops at exit 4 naming `--target`
  rather than creating a container running a version chosen at random. A target
  a major behind the source is a load that fails halfway through.
- **The pull and the start print outside the event catalogue.**
  `internal/event/catalogue.yml` was outside this task's paths and carries no
  row for a pull, a creation or a wait, and a code with no row renders as
  "no row in internal/event/catalogue.yml". So those lines go to the channel
  ADR-008 §7 already puts the prompt on — stderr, through
  `provision.Request.Progress`. `target.refused.start_timeout` is emitted, and
  its row now names the container and the `docker logs` command through
  `event.ArgContainer` (T-0071): `target container {container} did not become
  ready after {seconds}s: docker logs {container}`.

- **`--create-target` against a non-local or unreachable Docker endpoint is a
  refusal, and that guard is not phase 5.** ADR-008 §3 requires it "before any
  container work" and THREAT_MODEL.md T2 is what forces it, so `refuseNoTarget`
  answers it first, ahead of every other answer to the no-target state:
  `target.refused.docker_not_local`, exit 4, naming the endpoint and
  `--docker-host`. The endpoint and its locality leave rung 3 as a
  `dockerEndpoint` rather than being re-derived, so there is one answer to
  "is this daemon local" per run.
- **Verification runs before the collapse.** ADR-008 §4 gives the lower-numbered
  rung the *printed provenance*; the provenance and the credential are not the
  same fact. A rung-3 container carries `POSTGRES_PASSWORD` from its own
  environment and a rung-1 `$DATABASE_URL` naming the same
  `(host, port, database)` may carry none, so collapsing first and dialling the
  survivor dropped databases rung 3 could have used. `walk` therefore probes
  every candidate and then collapses, and `adoptReachable` gives the survivor
  the endpoint that answered while the lower rung keeps the printed name. The
  cost is one dial per duplicate, inside the same 1 s per-candidate budget.
- **The dial runs under the source allowlist, inside a read-only
  transaction** (T-0081). THREAT_MODEL.md T9's control is that every statement
  lazyslice sends to the source is allowlisted and read-only, and discovery
  dials the source like any other candidate. So `probe` registers the three
  statements on a `pg.Tracer` and passes it to `pg.Connect`; dialling with a nil
  tracer made the invariant false and left invariant I4's trace with no record
  of the connection at all. **Read-only is the transaction and not the
  connection.** `pg.Connect` used to set `default_transaction_read_only = on` on
  every source connection as it was opened, and T-0076 removed that: through a
  transaction-pooling PgBouncer the setting stayed on the pooler's *shared*
  server connection and left unrelated applications read-only after lazyslice
  exited. For the interval between the two, this dial sent three catalog reads
  to production candidates with no read-only rail at all — the allowlist alone.
  So `probe` acquires **one** connection (a transaction lives on the connection
  that opened it, and `pool.QueryRow` may acquire a different one each time),
  wraps the three reads in `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY` …
  `ROLLBACK`, and takes both literals from `pg.SourceShapes()` by name rather
  than writing them out a second time. `internal/pg`'s tracer refuses any
  statement that reaches an idle source connection (T-0082), so a dial that
  loses its `BEGIN` fails as an unreachable candidate instead of as a paragraph
  nobody re-read; `TestTheDialRunsInsideAReadOnlyTransaction` is the proof, and
  it is what `probe` returns its `*pg.Tracer` for.
  **And the dial acts on that verdict rather than only carrying it.** The
  tracer refuses a statement *by* cancelling the context pgconn checks before it
  writes, so a broken rail reaches `connectErr` as "context canceled" — which
  matched none of its arms and rendered as a driver line, or, from a pool
  acquire, as words a reader takes for a timeout. A candidate whose dial lost
  its `BEGIN` therefore came back `Reachable: false` with a message pointing at
  the operator's database, and discovery moved on to another endpoint. `probe`
  now asks `Tracer.Violation()` before it writes any `ConnectErr` and again
  after the three reads answer (`dialErr`, `dialViolation`), and says a source
  rail broke, in those words, naming which of the two. The candidate is still
  left unreachable — that is the fail-safe direction, and it is the only lever
  `probe` has, since it returns no error — but it is no longer fail-*silent*.
  `connectErr` has a `context canceled` arm as the backstop, and it does not say
  "did not answer".
  **Shapes are looked up once.** `transactionShapes()` is `probe`'s call and
  `probe`'s guard; `dialShapes(begin, rollback)` takes them as arguments. It
  used to look them up a second time and carry a second branch for the missing
  case, which `probe`'s own guard made unreachable — and the two disagreed about
  what to do, one dialling under a three-shape allowlist that would refuse the
  dial's own `BEGIN`.
- **"No password" is reserved for having none.** SQLSTATE 28P01 renders as
  "password authentication failed for user ...", so matching the word alone
  reported a *wrong* password as a *missing* one and sent the operator to
  `~/.pgpass` for an entry that is already there. `passwordAvailable` asks
  `pgconn.ParseConfig` — the same resolution the dial does, covering the DSN,
  `$PGPASSWORD` and the password file — and 28000 keeps the server's own line,
  because "no `pg_hba.conf` entry" is not a password problem.
- **A rung-1 name is claimed only by a value that could be used.** `.env`
  holding `DATABASE_URL=${PROD_URL}` — ADR-008 §2's own example — beside
  `.env.local` holding the real URI under the same name is the standard dotenv
  override. Marking the name seen on the unusable value dropped the one value
  that connects, so `take` marks it only after a candidate is produced.
- **`Result` carries the provenance and the label of each endpoint** (T-0060).
  §10's `source:` block records the rung an endpoint came from, and only the
  ladder knows it: `internal/core` builds its `pipeline.Candidate` from these
  four fields and `internal/emit` writes them, so an argument-free re-run
  re-emits a committed `from: compose` / `service: db` unchanged. Building both
  candidates as `pipeline.FromFlag`, which is what `internal/core` did before,
  rewrote those two lines on every such run — reachable only once the ladder
  existed, because before it the run stopped at exit 3 and never reached `emit`.
  Rung 0 carries the *file's* provenance forward rather than `FromYml`: the file
  records where the endpoint was found, and stamping `yml` on it would lose the
  rung on the second run instead of the first.
- **Rung 1 has four names, not six.** ARCHITECTURE.md §9 lists
  `DATABASE_URL`, `POSTGRES_URL`, `PG_URL`, `DB_URL`; ADR-008 §2 says "one of
  the six names at rung 1" without listing them. The four §9 names are
  implemented, because §9 is the one that enumerates.
- **A `.env` value must be a `postgres://` or `postgresql://` URI.** A libpq
  keyword string (`host=... port=...`) parses in `pgconn` and is refused here
  anyway: ADR-008 §2 says a `.env` variable contributes "a complete URI or
  nothing", and admitting the keyword form would reopen the composition case
  one spelling further along.
- **Container credentials come from the container's own environment, and one
  helper spells them.** `POSTGRES_USER`, `POSTGRES_DB` and `POSTGRES_PASSWORD`
  are read through `ContainerInspect`; THREAT_MODEL.md A3 names "container env"
  as one of the places a source credential lives. Compose YAML is still never
  read for any of them — `composeProject` has no field one could arrive in.
  `provision.EnvMap`, `provision.Credentials` and `provision.ConnString` are
  that one spelling, used by rung 3, rung 4 and `provision.settle` alike. They
  live in `provision/` because the dependency runs that way — this package
  imports it and not the reverse — and they exist because the second copy
  diverged: it defaulted `POSTGRES_DB` to the literal `postgres` instead of to
  `POSTGRES_USER`, which is what the postgres image does, so a Q1′ start of a
  container with `POSTGRES_USER=app` and no `POSTGRES_DB` printed
  `app@host/app` and handed the run `.../postgres`.
- **`dockerctx.Dial` calls `client.New`, not `client.NewClientWithOpts`.**
  ADR-008 §3 names the latter, but in the pinned `moby/moby/client v0.6.0` it
  is documented "Deprecated: use [New]" and carries a `//go:fix inline`
  directive, so govet's inline analyzer fails `make lint` on it — the same
  situation the ADR itself records for `WithAPIVersionNegotiation`. The two
  options and their order are unchanged.
- **`go.mod` moves `github.com/moby/moby/api` from indirect to direct.**
  `client.ContainerList` returns `[]container.Summary` from that module, so
  naming the type makes it a direct requirement. No module was added; `make
  lint` requires the file to be tidy.
- **`refDSN` carries `Ref.Params` as query parameters (T-0135,
  docs/reviews/2026-09-09/REVIEW.md finding 6).** Rung 0 rebuilds a dialable
  string from the committed `source_ref`/`target_ref`, and it used to carry
  only host, port, database and user — so a first run with
  `sslmode=verify-full` reran at pgx's default of `prefer`. `refDSN` does not
  re-check `dsn.AllowedParams`: `Ref.Params` already went through
  `dsn.FilterAllowedParams` on the way in, at `internal/emit`'s `refOf`, so by
  the time this package sees it, it already carries only what that let
  through.
- **A dropped param is a warning on `progress`, not an `event.Sink` send
  (`warnDroppedParams`).** `dsn.ExtractParams` names every connection
  parameter a string carried that `Ref.Params` did not — the allowlist is
  short, and `sslcompression=1` beside `sslmode=verify-full` is a real
  connection setting an operator wrote and lazyslice silently drops. Sending
  it as an event needs a `Code` with a row in `internal/event/catalogue.yml`
  (`internal/event/CLAUDE.md`: "every `Code` used anywhere in the tree has a
  row in `catalogue.yml`; CI fails on a code missing from the catalogue"), and
  that file is outside this task's paths — exactly the situation "the pull and
  the start print outside the event catalogue" above already names, so this
  follows the same precedent: `progressOf(o)`, stderr by default.
  - **Coverage is `Resolve`'s `o.Source`/`o.Target` and `walk`'s collapsed
    candidates — not every ladder rung individually.** The two flag-named
    strings are checked once each, at the top of `Resolve`, because
    `Resolve`'s early return (both endpoints already named) means `walk` is
    never called at all in that case (ADR-008 §1) and would otherwise warn on
    nothing. `walk`'s own loop covers rung 0 (rebuilt by `refDSN`, and so never
    carries a drop — this is belt and suspenders), rung 1 (`.env`/environment,
    the one rung a developer's own arbitrary params realistically reach
    through), and rungs 2-4 (libpq defaults and `provision.ConnString`, which
    construct their own strings and so never carry a drop either).
  - **Owed:** `internal/core`'s own `dsn.Parse(r.req.Source)` /
    `dsn.Parse(r.req.Target)` (`internal/core/run.go`) and `internal/pg`'s two
    `dsn.Parse` calls (`internal/pg/source.go`, `internal/pg/target.go`) parse
    connection strings this package never sees and get no warning wired at
    all. Both packages were outside this task's paths; filed as **T-0166**.
- **`refDSNValidated` retries without an unreadable file-valued param instead
  of dropping the endpoint (docs/reviews, 2026-09-14, findings 2 and, on the
  first landing, the re-review below).** `refDSN` rebuilds a connection
  string from a committed `Ref`, and `dsn.Parse` reads
  `sslrootcert`/`sslcert`/`sslkey` off disk *at parse time* —
  `pgconn.ParseConfig` builds the `tls.Config` immediately, not at connect
  time. A `sslrootcert` path that does not exist on this machine (a
  per-developer certificate directory is exactly `ARCHITECTURE.md` §9's own
  worked example for why the file is shared) used to make `dsn.Parse` fail
  outright.
  - **First landing (rung0 only) missed the ordinary case.** The retry was
    built into `rung0`'s own loop, but `Resolve` short-circuits rung 0 before
    `walk`/`rung0` is ever reached whenever the committed yml supplies what
    the run needs — `refDSN(o.Config.SourceRef)` directly at what is now
    `Resolve`'s call into the shared helper, and `rung0Target` likewise — so
    the ordinary committed `lazyslice.yml` (both sides recorded, the case
    this task exists to support) got the unread cert string handed straight
    through with no retry, and failed far from here: `internal/core`'s own
    `dsn.Parse` of that string aborted the run with exit-code-2's "--source is
    not a Postgres connection string", naming a flag the operator never
    passed.
  - **The fix is now the retry itself, factored out.** `refDSNValidated(w
    io.Writer, r dsn.Ref) (string, error)` wraps `refDSN` with the retry and
    the warning, and it is the only way any of the three callers — `Resolve`'s
    rung-0 short-circuit, `rung0Target`, and `rung0`'s own walk of both sides
    — turns a `Ref` into a dialable string. It retries once, through
    `withoutFileParams`, with every `filePathParams` entry stripped from the
    `Ref` before the second `refDSN`/`dsn.Parse`, and warns the dropped keys
    by name on `progressOf(o)` (`warnUnreadableParams`) rather than staying
    silent. A `Ref` whose non-file params (`sslmode`, `connect_timeout`,
    `application_name`, a validated `options`) still parse without the
    unreadable file keeps the endpoint.
  - `refDSNValidated` takes an `io.Writer` rather than `Options` so `rung0`,
    `rung0Target` and `Resolve` can each pass their own `progressOf(o)` without
    the helper reaching back into `Options` itself.
  - **A reference that is not usable at all is `("", nil)`; one that fails to
    parse for a reason the retry cannot fix is a loud refusal, not
    `("", false)` (docs/reviews, 2026-09-14, finding 2, second half).** The
    first landing above closed the retry gap and left the non-recoverable
    branch exactly as it was: an empty `Host`/`Database` and a genuine parse
    failure both returned `("", false)`, which every caller read the same
    way — "nothing at rung 0" — and `Resolve`'s own short-circuit fell
    through to `walk` and discovery on either. A `sslmode=verify-ful` typo or
    a non-numeric `connect_timeout` in a committed, otherwise-ordinary
    `lazyslice.yml` therefore never refused: the run silently loaded whatever
    the ladder found next, which can be a different database than the file
    named (THREAT_MODEL.md T2 is exactly the control this restores). The two
    cases are not the same thing and the return type now says so: an unusable
    `Ref` (no host or no database — a `--plan` run with no target, a yml one
    side of which was never recorded) is `("", nil)`, still meaning "nothing
    here"; a `Ref` that does carry a host and a database but will not parse
    even after `withoutFileParams`'s retry is `("", err)`.
  - **`dsn.ParseError(s)` is the error `refDSNValidated` returns** — pgconn's
    own text, which `dsn.Parse` itself discards and returns a generic message
    instead of, because `Parse` also runs on an operator-typed connection
    string that can hold a password and pgconn's error quotes the string it
    failed on. `s` here was built by `refDSN` from a `Ref` and nothing else, so
    it structurally cannot hold one (THREAT_MODEL.md T4, T5) — see
    `dsn.ParseError`'s own doc comment in `internal/dsn/dsn.go`, which states
    that precondition and restricts the function to a caller that can meet it.
    pgconn's message names the parameter it objected to (`"invalid
    connect_timeout"`, `"failed to configure TLS (sslmode is invalid)"`),
    which is what lets the refusal name the offending parameter rather than
    only the fact that something in the file did not parse.
  - **`Resolve` refuses through the new `refuseInvalidRef`, exit 2** —
    `CodeSourceRefInvalid` / `CodeTargetRefInvalid` — naming the yml field
    (`source_ref` or `target_ref`), `dsn.ParseError`'s reason, and the fix:
    edit the file's `source:`/`target:` block, delete it to let lazyslice
    rediscover that side, or pass `--source`/`--target` to override it for one
    run. `rung0Target` carries the error back the same way `refDSNValidated`
    does, ahead of the provisioned-password lookup, so a bad `target_ref`
    refuses before that lookup ever runs.
  - **`rung0`'s own loop still swallows the error**, unlike `Resolve`'s two
    direct calls. It reads `cfg.SourceRef` and `cfg.TargetRef`
    unconditionally, including the side the operator overrode with
    `--source`/`--target` this run — and `Resolve`'s short-circuit never
    looks at that side's yml value at all when a flag named it (ADR-008 §1),
    so the only way `rung0` can see a real parse error is on a value this run
    is not using. `Resolve`'s own two calls already refuse loudly, before
    `walk` (and so `rung0`) is ever reached, for the side that *is* in play;
    letting `rung0` refuse a second time over an unused field would make an
    override fail on a file it was explicitly told to ignore.

## What the ladder will not pick as a target (the 2026-09-15 red team)

With `--source` pointing at production and no `--target`, the ladder chose the
production container and wrote the masked slice into its `postgres` maintenance
database — a different database on the same cluster, which ARCHITECTURE.md §9
rule 1 makes eligible by design — under `--yes`, exit 0, and without the
`same cluster as source` warning §9 promises (that half was `internal/core`'s
and is wired now, `CodeTargetSameCluster`).

Two changes in `chooseTarget`, and one deliberate non-change.

- **A maintenance database is never target-shaped** — except in lazyslice's
  own container for this project (T-0334, below). `postgres`, `template0`
  and `template1` are the databases a cluster is created with; `postgres` exists
  so that a client has something to connect to in order to create another one.
  Ranking it is what let the ladder land a slice there.
- **A candidate off the source's cluster outranks one on it**, ahead of every
  other sort key, so a genuinely separate server always wins where one is
  reachable. `clusterKey` is `collapseKey` without the database.
- **A same-cluster candidate is still chosen when it is the only one — in an
  interactive run.** That is the ordinary compose setup, rule 1 admits it, the
  decision header and the `same cluster as source` warning print before
  anything is written, and `internal/core/gate_integration_test.go`'s
  `TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp` (interactive — `Yes`
  false *and* a scripted `discover.Prompter` set on the request, not `Yes:
  false` on its own: `go test` itself has no controlling terminal, and
  headless is `--yes`, or no controlling terminal, not `--yes` alone, so a
  bare drop of `Yes: true` is still headless and still trips the refusal below
  before `chooseTarget` ever runs — the 2026-09-16 reverify's finding) pins the
  gate-refusal-ends-the-run behaviour over a run whose only two target
  candidates are on the source's cluster. **T-0184 / ADR-013** (proposed,
  2026-09-16) answers the question this bullet used to leave open: a
  **headless** run — `--yes`, or no controlling terminal: the run cannot ask —
  with no `--target` now refuses at exit 4
  (`target.refused.headless_same_cluster`) before `chooseTarget` ever runs,
  when every reachable target-shaped candidate — the whole set `chooseTarget`
  would have ranked, via the new `targetShaped`/`allOnSourceCluster` helpers —
  is on the source's own cluster, naming `--target`. A `--target` named
  explicitly (flag, positional DSN, or a committed `lazyslice.yml`) never
  reaches the check: it short-circuits `Resolve` before the ladder is even
  walked (ADR-008 §1), so it stays eligible exactly as rule 1 and ADR-008 §5
  say. `internal/core/headless_same_cluster_integration_test.go` reproduces
  the 2026-09-15 red team's run headlessly and asserts the refusal, and its
  sibling test asserts an explicit same-cluster `--target` is untouched.
  **`Result.TargetNamed` is the fact a second, later same-cluster check in
  `internal/core`'s `openTarget` needs and `TargetProvenance` cannot give it**
  (ADR-013 review finding 1). That check runs *after* the gate, on
  `Eligibility.SameCluster` — the gate's authoritative `system_identifier`
  comparison, which catches a same-cluster target this package's cheap
  `clusterKey` address comparison missed (a pooler, an SSH tunnel, a
  `host.docker.internal` vs `127.0.0.1` spelling) — and it must not fire for a
  target the operator named, same-cluster or not. `TargetProvenance` cannot
  answer "did the operator name this": rung 0 carries the *file's own*
  provenance forward rather than `pipeline.FromYml` (the doc comment on
  `Resolve`'s rung-0 branch, and on `Result` itself), so a target read back
  from a committed `lazyslice.yml` can carry the same provenance
  (`FromEnvVar`, `FromContainer`, `FromCompose`) a ladder-chosen candidate
  would. `Result.TargetNamed` is set `true` at both places that short-circuit
  the ladder for the target — `o.Target != ""` and `rung0Target`'s success
  path — and nowhere else, so `internal/core` can ask that instead of trying
  to read operator intent out of a field that does not carry it.

## `--password-command` covers a discovered candidate too (T-0213 review round)

The first landing of T-0213 wired `--password-command` into `internal/core`'s
two `discover.ResolvePassword` calls — for `--source`/`--target` named on the
command line, or resolved from a committed `lazyslice.yml` reference — and
left this package's own promise unmet: `rung0`'s doc comment ("the candidate
carries no password and the ordinary password sources ... supply one") and
`rung0Target`'s both describe the command as covering *any* candidate with no
password, and `Options` carried no `PasswordCommand` field at all. A
committed `lazyslice.yml` plus `--password-command`, with no `--source`
override, still failed authentication: `probe` dialled the password-less
candidate with none, `pg.Connect` refused it, the candidate came back
`Reachable: false`, and `chooseSource`/`chooseTarget` skipped it — the run
refused "no source" before `internal/core`'s own call was ever reached.

- **`Options.PasswordCommand`, resolved by `probe` itself, not by
  `internal/core`.** `internal/core` cannot resolve it on this package's
  behalf the way it does for a named endpoint: it does not see a candidate
  until after the ladder has already chosen one, and by then every
  password-less candidate has already been dialled and marked unreachable.
  `probe` now checks `passwordAvailable(f.dsn)` before dialling and, when it
  is false and `PasswordCommand` is set, resolves it and injects the result
  into `f.dsn` with `injectPassword` — the same helper `ResolvePassword`
  itself uses — before the dial, not after a failed one.
- **`passwordCache` (`password.go`) makes the resolution run at most once per
  walk, not once per password-less candidate.** `PasswordCommandTimeout`'s own
  doc comment already argues a credential helper should not be charged
  against the 1 s per-candidate dial budget; the same argument extends to
  running it four times because four candidates had no password. `walk`
  builds one `*passwordCache` (only when `PasswordCommand != ""`) and every
  `probe` call in that walk shares it via `Options.pwCache`, unexported so a
  caller cannot supply its own and defeat the guarantee. `sync.Once` inside
  the cache is what makes the second and later calls return the first
  result — success or failure — without a second invocation.
- **The resolution happens before `dialBudget`'s context is created, not
  inside it.** `probe` used to open its 1 s `context.WithTimeout` as its
  first line; resolving the password after that would hand the command
  whatever was left of one second rather than `PasswordCommandTimeout`'s own
  30, contradicting that constant's doc comment a second way. The check and
  the resolve call now run on the outer `ctx` `probe` was given, ahead of
  that line — costing nothing on the ordinary path, since the cache makes it
  a no-op after the first candidate.
- **A resolution failure is warned once, not sent as an event or attached to
  the candidate's `ConnectErr`.** `internal/event/catalogue.yml` has no row
  for this and adding one is outside a fix round scoped to the paths named
  above (see "the pull and the start print outside the event catalogue",
  earlier in this file, for the same precedent); `passwordCache.resolve`
  writes one line to `progressOf(o)` (ordinarily stderr) the first time it
  fails, and the candidate falls through to the ordinary `noPassword`
  refusal on `f.cand.ConnectErr` exactly as it would have with no
  `--password-command` set at all — a failed credential helper must not make
  a run that could otherwise proceed on another candidate abort the whole
  walk.
- **Two more findings from the same review, fixed alongside the field
  above.** `runPasswordCommand`'s 30 second timeout was not enforced against
  a grandchild the command backgrounds — `exec.CommandContext` kills only the
  shell, and with `c.Stdout` a `*bytes.Buffer`, `c.Run()` waits on that pipe
  closing rather than on the shell's own exit; `c.WaitDelay` now bounds that
  wait to a second past the context firing (or past the shell's own exit),
  proven by `TestPasswordCommandDoesNotBlockOnAGrandchildHoldingStdout`
  reproducing the reviewer's own `sleep 45 &` script and measuring the
  elapsed time rather than only the refusal string. And the newline trim left
  a trailing `\r` on Windows, where the command runs through `cmd /C` and
  `cmd`'s own line ending is CRLF; the trim now removes `\r` as well as `\n`.
  `TestDiscoveredCandidateAuthenticatesWithPasswordCommand` and
  `TestProbeResolvesPasswordCommandOnceAcrossCandidates`
  (`password_integration_test.go`, `discover_integration_test.go`) are the
  new tests for the field itself; `cmd/lazyslice`'s
  `TestPasswordCommandOutputReachesNoSink` gained a second run, with a canary
  that is a role's real password, so it actually reaches `emit` and greps
  `lazyslice.yml` — the sink THREAT_MODEL.md T5 cares about most — which the
  first landing's single failing-canary run never did.

## The next run chooses the container the first one made (T-0334)

Dogfood session 3: a second run from the directory whose first run answered
Q1 listed `lazyslice-target-<project>` (`0 table(s), probably empty`) and
then asked Q1 again for the same name on the next port; headless it stopped
at exit 4 `target.refused.none`. Provisioning creates the container with
`POSTGRES_DB=postgres`, and the red team's rule above dropped that database
from `targetShaped`.

- **`found.own` lifts the maintenance rule for one container.** `containers`
  sets it (`ownContainer`) when a container's `provision.LabelProject` equals
  this working directory's `projectName` *and* its `provision.LabelWorkingDir`
  is this directory (absolute and clean, or equal once symlinks resolve). The
  project label alone is the basename, so two checkouts named alike share
  `lazyslice-target-<project>`; without the working-dir half the second one
  took the first one's container with no question. `collapse` keeps it when a lower rung names
  the same database. `targetShaped` then ranks that container's `postgres`
  like any other candidate: ADR-008 §5's order is unchanged, `EmptyHint` is
  still not a filter, and the gate still judges the winner. The label is not
  trusted as proof — anyone can set it — only as the reason the name rule
  does not apply; a container that spoofs it is still gated.
- **"probably empty" needs no second probe.** The hint is the connected
  database's own `pg_class` count and `relpages` statistic; it says nothing
  about other databases in the cluster, and a load into `postgres` writes
  none of them. No read was added to the dial.
- **Q1 never proposes a name that is taken (`reuseOwn`).** When the ladder
  could not rank `lazyslice-target-<project>` (stopped beside other
  candidates, still booting past the 1 s dial, filtered out by working dir),
  `noTarget` looks it up by name before Q1: stopped and ours is Q1′ with its
  own words and headless default; running and ours is `provision.Start` with
  no question (it creates nothing); present and not `ownContainer` (no
  `LabelProject`, or another checkout's working dir) is exit 4
  `target.refused.name_taken` naming `--target`, at a terminal too, and the
  container is not read or started. A lookup that fails is not evidence the name is taken, and
  Q1 is asked as before. In ADR-016's missing-record state the same lookup
  runs at a terminal only, because headless the file names a different
  container and ADR-016's `target.refused.container_missing` stands.
- **Tests.** `own_container_test.go` pins the running container chosen with
  no question (at a terminal, headless, and carrying the marker), the stopped
  one still Q1′, and the taken name never proposed (including by another
  checkout with the same basename, reachable or not). `internal/core`'s
  `TestTwoRunsFromOneDirectoryReuseTheContainerTheFirstMade` runs two headless
  runs with nothing but `--source` against a provisioned container.
- **Not changed, and owed:** `--create-target` reuses a container named
  `lazyslice-target-<project>` whether or not lazyslice created it
  (`provision.Provision`'s `byName` checks no label), filed as **T-0373**.

