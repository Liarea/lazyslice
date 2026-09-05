# internal/invariants

The black-box suite. Six test functions, one per invariant in ARCHITECTURE.md
§12's line `internal/invariants/  I1–I6 as tests against any produced target,
both fixtures`, each run against both `testdata/` fixtures on two PostgreSQL
containers from `internal/testutil`.

| Test | Invariant | How it is checked |
|---|---|---|
| `TestI1ForeignKeysResolve` | every foreign key in the target resolves | `information_schema` for the edges, an anti-join per edge |
| `TestI2NothingFlaggedSurvives` | nothing flagged survives masking | `classify --json` on the target, plus a grep for every source email and phone |
| `TestI3SameInputsSameTarget` | same source, secret and config → byte-identical target | `pg_dump --data-only` twice, compared |
| `TestI4SourceUnchanged` | the source is unchanged | row count and an ordered-row md5 per table, before and after |
| `TestI5EmittedConfigReproducesTheSnapshot` | the emitted yml reproduces the snapshot | re-run with `--config` and no plan flags, compared as I3 |
| `TestI6RootHoldsTakeRows` | the root holds `--take` rows | `count(*)` on the root of the target |

**Contract.** This package is compiled against the *binary*, never against a
stage. It builds `./cmd/lazyslice`, runs it with flags, and then talks to the
two databases. It imports `internal/testutil` and `pgx` and nothing else of
ours — a suite that reached into the pipeline could be made to pass by the
same mistake that made the pipeline wrong.

**Rules.**
- Everything is behind the `integration` build tag; `doc.go` is the only
  untagged file and holds only the package comment. `make integration`
  (`go test -tags integration ./...`) runs it.
- **The only skip is `testutil.SkipWithoutDocker`.** A missing `pg_dump`, a
  build failure, a fixture that no longer contains an email address: each is a
  hard failure naming what to install or fix. A suite that skips when the
  thing it tests is absent reports green on an empty repository.
- Every failure names the table, and the column when there is one. `%v` on a
  count is not a finding.
- Each test asserts against something it cannot vacuously satisfy: I1 fails if
  the target declares no foreign keys, I2 fails if the source held no personal
  literal or if `classify --json` named no column, I6 fails if `--take` is not
  smaller than the root.
- I6 uses `fixture.countedRoot`, which is not always `fixture.root`: the
  invariant only holds for a root no selected row reaches as a parent, and
  `public.people` is reached through its own `manager_id`
  (`testdata/README.md` trap 1).
- A connection URL carries a password. It goes to the binary as a flag and to
  `pgx`; it never goes into a test name, a `t.Log` or a failure message.

**Test.** `go test -tags integration -count=1 -timeout 30m ./internal/invariants/...`
(needs a Docker endpoint and `pg_dump` on `PATH`). Each test starts its own
pair of containers per fixture, so the whole package is twelve pairs; run one
invariant with `-run TestI3`.

**Never:** skip for anything but Docker; assert through `core.Run` or a stage
package instead of through the binary; loosen an assertion because the
pipeline is a no-op — every test here fails today, naming the run's exit code,
and that is the phase 3 gate.
