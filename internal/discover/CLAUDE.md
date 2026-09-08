# internal/discover

The discovery ladder (rungs 0-5: yml, env/`.env`, libpq, running container,
stopped container, compose name) and candidate verification/de-duplication.
Subpackages: `dockerctx/` resolves the Docker endpoint; `provision/` is the
`--create-target` path. The target *gate* is not here — that's
`internal/pg` (`Target.Gate`), because gating needs a live connection this
package's 1s dial budget does not allow for.

**Contract.** ARCHITECTURE.md §2 "discover" and §9: implements
`pipeline.Discoverer.Discover(ctx, workdir, sink) ([]Candidate, error)`.
`provision/` implements `pipeline.Provisioner`.

**Rules.**
- 2s total listing budget, 1s per-candidate dial (ARCHITECTURE.md §9) — do not
  add a probe that can exceed either.
- Inside the dial: at most three statements per candidate (version, the
  `pg_class` count/hint, `to_regclass('lazyslice_meta')`). Never probe
  emptiness table by table here; that is the gate's job, after discovery
  (THREAT_MODEL.md T2).
- `provision/` is the *only* code in the tree that creates or starts a
  container, and only behind `--create-target` or a "yes" to Q1 — never called
  unconditionally.
- Source selection (most-local reachable candidate with the most tables, never
  a question) lives here; do not turn it into a question without an
  ARCHITECTURE.md §9 change.

**Test.** `go test ./internal/discover/...`; container-touching parts need
`go test -tags integration ./internal/discover/...`.

**Never:** create or start a container outside `provision/` or outside
`--create-target`/Q1; run an unbounded emptiness probe inside the dial; let
discovery itself decide a target is eligible — only `Target.Gate` does that.

## Decisions made during implementation

Recorded here because ARCHITECTURE.md is silent on them, or because this
package had to deviate from it (root CLAUDE.md). Each is a deviation a
reviewer should see rather than discover. This build ships rungs 0 to 3;
rung 4 and provisioning are phase 5 (ARCHITECTURE.md §14).

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
- **Q1, the controlling terminal and provisioning are all phase 5.** ADR-008 §6
  makes Q1 fire only where a container can be created, and nothing in this
  build creates one, so the state Q1 exists for — a source, no target — takes
  the stop §9's one-question rule prescribes for an item with no safe default:
  exit 4 naming `--target`. `Options` therefore carries no `Provisioner`, no
  `Prompter` and no `Yes` — there is no question for `--yes` to answer, and an
  unread field suggests a wiring that does not exist — and `ask.go`,
  `tty_unix.go` and `tty_windows.go` were removed
  rather than left as an unreachable seam a later reader would mistake for live
  code. **T-0063** lands them together with `pipeline.Provisioner`, which has to
  widen first: it returns a `Candidate` whose `Ref` holds no password, while §9
  "Provisioning" gives the container a random `POSTGRES_PASSWORD`.
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
- **Container credentials come from the container's own environment.**
  `POSTGRES_USER`, `POSTGRES_DB` and `POSTGRES_PASSWORD` are read through
  `ContainerInspect`; THREAT_MODEL.md A3 names "container env" as one of the
  places a source credential lives. Compose YAML is still never read for any
  of them — `composeProject` has no field one could arrive in.
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
