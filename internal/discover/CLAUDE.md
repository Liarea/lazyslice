# internal/discover

The discovery ladder (rungs 0-5: yml, env/`.env`, libpq, running container,
stopped container, compose name) and candidate verification/de-duplication.
Subpackages: `dockerctx/` resolves the Docker endpoint; `provision/` is the
`--create-target` path. The target *gate* is not here — that's
`internal/pg` (`Target.Gate`), because gating needs a live connection this
package's 1s dial budget does not allow for.

**Contract.** ARCHITECTURE.md §2 "discover" and §9: implements
`pipeline.Discoverer.Discover(ctx, workdir, sink) ([]Candidate, error)`.
`provision/` implements `provision.Provisioner`, which is the widened
`pipeline.Provisioner` — see that package's CLAUDE.md for why it is declared
there.

**Rules.**
- 2s total listing budget, 1s per-candidate dial (ARCHITECTURE.md §9) — do not
  add a probe that can exceed either.
- Inside the dial: at most three statements per candidate (version, the
  `pg_class` count/hint, `to_regclass('lazyslice_meta')`). Never probe
  emptiness table by table here; that is the gate's job, after discovery
  (THREAT_MODEL.md T2).
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
    the `DSN` the generated `POSTGRES_PASSWORD` lives in. `internal/pipeline`
    now declares an interface nothing implements; see
    `provision/CLAUDE.md`, "Owed".
  - **`Options.Yes` is read here and not set by `internal/core`. This is a
    merge blocker, not a debt.** `core.Request.Yes` exists and
    `resolveEndpoints` does not copy it into `Options` (`internal/core/run.go`,
    the literal ending `CreateTarget: r.req.CreateTarget` — it needs
    `Yes: r.req.Yes,`); `internal/core` is outside T-PROVISION's writable paths
    and the fix is that one line. Until it lands, `lazyslice --yes` on a machine
    that *has* a controlling terminal opens `/dev/tty` and blocks in
    `prompt.Confirm` with no timeout at all — the 60 s budget covers the
    container start, not the question — so automation under an allocated TTY
    (`docker run -t`, `script(1)`, `tmux`) hangs instead of taking ADR-008 §7's
    headless path. Without a controlling terminal — CI, cron — the headless
    path is taken anyway, which is the case ADR-004 relies on. This is the only
    piece of T-0063's acceptance this task could not finish.
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
  its existing row still cannot name the container or `docker logs <name>` for
  want of an `ArgKey`; the catalogue's own comment already records that debt.

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
- **The dial runs under the source allowlist.** THREAT_MODEL.md T9's control is
  that every statement lazyslice sends to the source is allowlisted and
  read-only, and discovery dials the source like any other candidate. So
  `probe` registers the three statements on a `pg.Tracer` and passes it to
  `pg.Connect`, which is also what brings `default_transaction_read_only = on`
  on the connection. Dialling with a nil tracer made the invariant false and
  left invariant I4's trace with no record of the connection at all.
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
