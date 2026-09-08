# internal/discover/provision

The `--create-target` path, and the only code in the tree that creates or
starts a container. ARCHITECTURE.md §9 "Provisioning" is the spec: a free
loopback port from 5433 upward, `postgres:<source major>` pulled if absent,
`lazyslice-target-<project>` with a random `POSTGRES_PASSWORD` and a named
volume, started, then up to 60 s for the server to accept a connection.
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
  `pipeline.Provisioner` at all. **Owed:** `internal/pipeline/discover.go`
  still declares a `Provisioner` interface nothing implements; it should
  either be deleted or restated as this one, in a task whose paths include
  `internal/pipeline`.
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
  for a container name and one for an image, after which `Progress` is deleted
  and these become sink sends. The same catalogue note already records that
  `target.refused.start_timeout` cannot name its container or `docker logs
  <name>` for want of that `ArgKey`.
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
