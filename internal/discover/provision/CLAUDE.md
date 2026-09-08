# internal/discover/provision

The `--create-target` path, and the only code in the tree that creates or
starts a container. ARCHITECTURE.md §9 "Provisioning" is the spec: a free
loopback port from 5433 upward, `postgres:<source major>` pulled if absent,
`lazyslice-target-<project>` with a random `POSTGRES_PASSWORD` and a named
volume, started, then a bounded wait for the server to accept a connection
(60 s by default, `$LAZYSLICE_PROVISION_READY_TIMEOUT` where that is set).
`Start` is the same wait applied to a container that already exists, which is
ADR-008 §6's Q1′.

**Contract.** ARCHITECTURE.md §9 "Provisioning" and ADR-008 §6 (Q1, Q1′).
`Provisioner` is called only by `internal/discover`, and only behind
`--create-target` or a "yes" to Q1/Q1′. Its precondition is ADR-008 §3's: the
Docker endpoint is **local and reachable**, established by the caller before a
provisioner is built (THREAT_MODEL.md T2).

**Rules.**
- The container survives the run and is never removed: `Provision` on an
  existing container starts it if stopped and reuses it if running, and there
  is no code path here that removes a container, an image or a volume.
- The generated password is written to the machine-local state dir
  (`internal/config`) **before** the container is created, so a state dir that
  cannot be written is a refusal with nothing created rather than a container
  whose credential was lost. It never reaches `lazyslice.yml` (ADR-004,
  THREAT_MODEL.md A3). It is also never *overwritten*: `secretFor` reuses a
  remembered password rather than minting a second one, because the volume
  outlives the container and the cluster inside it only accepts the first.
  `Password` is the reader — the state dir is the credential's only home, so
  the second run has nowhere else to get it — and neither it nor `remember`
  accepts a name that is not a single path element.
- The published binding is `127.0.0.1:<port>` explicitly, never `0.0.0.0`: a
  target on every interface is the opposite of the locality the gate requires.
- Readiness is a Postgres startup handshake, not a TCP connect: docker-proxy
  binds the host port the moment the container starts, and the `postgres`
  image runs `initdb` against a unix socket for seconds after that. No
  statement is sent, so there is no allowlist shape to register.
- `crypto/rand` for the password, `0o600` for the file, `0o700` for the
  directory — this is a credential.
  The mode is asserted on unix only: Windows has no POSIX mode bits, `os.Chmod`
  there sets the read-only attribute and nothing else, so a file written with
  `0o600` stats as `-rw-rw-rw-` and the assertion would be about the operating
  system rather than about this package. **Owed:** restricting the file by ACL
  on Windows — the state dir there is under the user's profile, which is not
  world-readable by default, but that is the platform's doing and not ours.
- **The readiness budget is 60 s — the number ARCHITECTURE.md §9 and ADR-008 §6
  both carry — and `$LAZYSLICE_PROVISION_READY_TIMEOUT` where that names a Go
  duration** (`ReadyBudgetEnv`), and it runs from the *container start* rather
  than from the first dial. 60 s is a laptop with a warm cache; a CI runner
  pulls the image cold and then runs several `initdb`s at once, so
  `.github/workflows/ci.yml` sets the variable to `3m`. The default stays at
  what the two frozen documents say on purpose: raising it in code would be a
  change to an accepted decision, which takes a superseding ADR and not a task,
  while an environment override is a new affordance and changes neither. A
  value that does not parse, or that is not positive, is the default: a
  mistyped timeout must not be what stops a run, or what turns the wait off.
  `TestReadyBudgetIsSixtySecondsUnlessTheEnvironmentSaysOtherwise` pins all
  three.
- **The wait reads the container's own log and its liveness once per poll**
  (`bootWatch`), for a container this run started, and every daemon call it
  makes is bounded the way the dial is. Two things come out of it: a container
  that is **not running any more** ends the wait immediately, naming the exit
  status and `docker logs <name>` — the failure a dial cannot tell from a slow
  start, and the one that cost T-0075 a whole CI leg — and, while the server
  has not yet printed `database system is ready to accept connections`, the
  first few polls are not spent dialling a server that is still running
  `initdb`.
  **The log is a grace and never a gate** (`dialGrace`, capped at a quarter of
  the budget). A server whose readiness line this package cannot see — a
  `postgresql.conf` with `logging_collector` on, `log_destination` set to
  `csvlog` or `jsonlog`, a non-English `lc_messages`, a log the daemon serves
  from somewhere other than the container's stdout — is a server that answers,
  and treating the line as a precondition refused those runs after the whole
  budget without spending a single attempt. That lands hardest on Q1′, which
  starts the developer's *own* container: the one most likely to carry a tuned
  configuration, and a path that worked before because it only ever dialled.
  `TestALogWithoutTheReadinessLineStillGetsDialled` pins it.
  Where the line *is* readable, **one occurrence is enough, not the second**,
  even though the second is the one that means TCP: the entrypoint pre-starts
  on a unix socket only when it has an `initdb` and init scripts to run, so a
  container coming back up on a volume that already holds a cluster prints the
  line exactly *once* — waiting for two made
  `TestARecreatedContainerCanStillLogIntoItsSurvivingVolume` sit out the entire
  budget in front of a server that had been accepting connections for the
  length of it (measured: 92 s). A log this daemon will not serve is not a
  failure either; the wait goes back to being the dial alone.
- **The poll is bounded, because the budget is only checked between polls.**
  The caller's context carries no deadline — `internal/discover` hands the
  run's context straight through and the moby client sets no HTTP timeout of
  its own — so a daemon that accepts an inspect or a log read and never answers
  would make the budget unenforceable, which is the hang the budget exists to
  prevent. Each poll's daemon calls run under their own
  `context.WithTimeout`, and a poll that expires has simply learned nothing.
  `TestADaemonThatNeverAnswersDoesNotOutlastTheBudget` pins it.
- **The published binding is read IPv4-first** (`published`). A daemon that
  publishes 5432 on both families lists both bindings, and an IPv4-only publish
  — which is what `0.0.0.0` and an explicit `127.0.0.1` both are — is not
  reachable on `::1` at all: every attempt inside the budget is refused for the
  same reason. An IPv6-only publish is still used, bracketed by `ConnString`.
  `TestPublishedPrefersTheIPv4Binding` pins both orders and both single-family
  cases.
- **The mount point depends on the major** (`dataDirFor`).
  `/var/lib/postgresql/data` below 18, `/var/lib/postgresql` at 18 and above:
  postgres:18 keeps PGDATA in a major-version-specific directory it owns, and
  its entrypoint *exits 1* rather than start when anything is mounted at the old
  path (docker-library/postgres#1259). Below 18 the path cannot move — the
  volume outlives the container, and a run that mounted it elsewhere would find
  an empty PGDATA beside the developer's rows.

**Test.** `go test ./internal/discover/provision/...` covers the create-and-
adopt logic against a fake daemon; `go test -tags integration
./internal/discover/provision/...` runs it against a real one and skips
cleanly when Docker is absent.

**Never:** remove a container, an image or a volume; publish on anything but
loopback; create a container on an endpoint the caller has not established is
local; put the password in an event, in `lazyslice.yml` or in a `dsn.Ref`;
read a compose file for a host, port, user, password or database name.

## Decisions made during implementation

- **`provision.Provisioner`, not `pipeline.Provisioner`.** T-0063 asked for
  `pipeline.Provisioner` to be widened, because it returns a
  `pipeline.Candidate` whose `Ref` is redacted by construction and so cannot
  carry the `POSTGRES_PASSWORD` this package generates, while the run has to
  connect with it. `internal/pipeline` was outside T-PROVISION's writable
  paths, so the widened contract is declared here instead: `Result` carries
  the `Candidate` **and** the `DSN`, and this package no longer implements
  `pipeline.Provisioner` at all. `internal/pipeline/discover.go`'s
  `Provisioner` interface — the one nothing implemented — is deleted (T-0071):
  this package's `Provisioner` is the only one in the tree.
- **No `event.Sink` anywhere in this package.** It emits no event of its own:
  the provisioned container reaches the candidate list through
  `internal/discover`, which probes it and prints it with
  `discover.candidate.found` like any other candidate, and the gate then runs
  on it unchanged. A sink here would have needed codes
  `internal/event/catalogue.yml` does not carry.
- **The pull and the start print through `Request.Progress`, not through the
  event sink.** ARCHITECTURE.md §9 asks for the pull to be printed and there
  is no catalogue row for it (`internal/event/catalogue.yml` was outside this
  task's paths), and an event under a code with no row renders as
  "no row in internal/event/catalogue.yml" rather than a sentence. ADR-008 §7
  already puts prompt text on this channel — stderr — so the two waits that
  can outlast a developer's patience go there too. **Owed:** rows for
  `discover.provision.pulling`, `.created` and `.waiting`, plus an `ArgKey`
  for an image, after which `Progress` is deleted and these become sink
  sends. `event.ArgContainer` already exists and `target.refused.start_timeout`
  names its container and `docker logs <name>` with it (T-0071).
- **`Request.Port` is chosen by the caller.** Q1's prompt names the port it is
  about to use (`... on port <free>`), so the port has to exist before the
  question is asked; `FreePort` is exported for that and the chosen value is
  passed back in so the question and the container cannot disagree.
- **Labels are `lazyslice.project` and `lazyslice.working_dir`, not compose
  labels.** Rung 3's project filter reads
  `com.docker.compose.project.working_dir`; stamping that on our container
  would make it appear inside a compose project no compose file describes.
  `internal/discover` reads our label instead when it applies the filter.
- **A named volume is not removed by `docker rm -v`, and that is a credential
  problem as well as a rows problem.** ARCHITECTURE.md §9 asks for a named
  volume *and* prints `docker rm -v lazyslice-target-<project>` as the command
  that would remove a refused target; `-v` removes anonymous volumes only, so
  that command leaves `lazyslice-target-<project>-data` behind and the
  recreated container comes back with the same rows. It also comes back with
  the *old* cluster: the postgres image skips `initdb` on a non-empty PGDATA
  and ignores `POSTGRES_PASSWORD`, so minting a fresh password there produced a
  container that failed authentication for the whole 60 s budget on every
  retry — and overwriting the state-dir file destroyed the one credential that
  worked. `secretFor` is the answer: reuse what is remembered, mint only when
  nothing is, and refuse — naming `docker volume rm
  lazyslice-target-<project>-data` — when a volume exists and no password for
  it does. `TestARecreatedContainerCanStillLogIntoItsSurvivingVolume` pins it
  against a real daemon. The *refusal text for a non-empty target* still lives
  in `internal/pg`, outside this task's paths; **owed:** it should name `docker
  rm -f lazyslice-target-<project> && docker volume rm
  lazyslice-target-<project>-data`, or ARCHITECTURE.md should drop the named
  volume.
- **`EnvMap`, `Credentials` and `ConnString` are exported for
  `internal/discover`.** `containers.go` and `settle` both turn a container's
  environment and binding into a connection string, and the two copies
  diverged: this one defaulted `POSTGRES_DB` to the literal `postgres` while
  rung 3 and rung 4 defaulted it to `POSTGRES_USER`, which is what the postgres
  image does. They live here rather than in `internal/discover` only because
  the import runs that way; a container's endpoint now has one spelling, which
  is what `ConnString`'s doc comment claimed before it had two.

- **T-0075: postgres:18 never started, and every symptom pointed somewhere
  else.** The whole `integration (18)` leg failed with "did not accept a
  connection within 1m0s" on all four provisioning tests while a laptop on
  postgres:16 was green, which reads exactly like a slow runner: a cold image
  pull inside the readiness budget. It was not. The container was created,
  started and dead within a second — postgres:18 refuses the
  `/var/lib/postgresql/data` mount — and this package could not tell a dead
  container from a slow one, because the only question it asked was whether the
  port answered yet. Three things came out of that and all three are above: the
  mount point per major (the fix), the log-and-liveness wait (which turns the
  next one of these into a sentence in one second), and `NotReadyError.Error`
  finally printing the cause it had been carrying in `Err` all along. The
  budget itself was not the bug and the default stays at the 60 s those two
  documents carry; CI raises it through `ReadyBudgetEnv` instead. **Owed:**
  ADR-008 §6 step 3 says "The container is left running", which is no longer
  true of the one case the exit-during-startup path names — the container is
  left *exited*, which is what makes the `docker logs <name>` in that step's
  own sentence answerable — and `target.refused.start_timeout`'s catalogue row
  is written from that step. An amendment to §6 step 3, or an ADR that
  supersedes it, is owed for that sentence; nothing about the wait's duration
  is.
- **The `integration (14)` leg failed on something else again**, and it is not
  fixed here: `FreePort` binds a port, closes it, and hands the number back, so
  two test binaries running in parallel — `internal/discover` and
  `internal/discover/provision` — both chose 5433 and the second container's
  start failed with "Bind for 127.0.0.1:5433 failed: port is already allocated".
  That race is in the product and not only in the tests: two `lazyslice`
  runs started at once on one machine hit it too. **Owed:** a tracker task —
  `create` should retry on the next free port when a start fails for an
  allocated binding.
