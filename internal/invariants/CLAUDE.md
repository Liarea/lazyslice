# internal/invariants

The black-box suite. Six test functions, one per invariant in ARCHITECTURE.md
§12's line `internal/invariants/  I1–I6 as tests against any produced target,
both fixtures`, each run against both `testdata/` fixtures on two PostgreSQL
containers from `internal/testutil`.

| Test | Invariant | How it is checked |
|---|---|---|
| `TestI1ForeignKeysResolve` | every foreign key in the target resolves | `pg_constraint` for the edges, an anti-join per edge, plus every source edge between two tables the target has |
| `TestI2NothingFlaggedSurvives` | nothing flagged survives masking | `classify --json` on the target, scoped to the columns the run neither masked nor opted out, plus a grep for every source email and phone |
| `TestI3SameInputsSameTarget` | same source, secret and config → byte-identical target | `pg_dump --data-only` twice, normalised, compared |
| `TestI4SourceUnchanged` | the source is unchanged | row count and an ordered-row md5 per table, before and after |
| `TestI5EmittedConfigReproducesTheSnapshot` | the emitted yml reproduces the snapshot | re-run with `--config` and no plan flags, compared as I3 |
| `TestI6RootHoldsTakeRows` | the root holds `--take` rows | `count(*)` on the root of the target |

**Contract.** This package is compiled against the *binary*, never against a
stage. It builds `./cmd/lazyslice`, runs it with flags, and then talks to the
two databases. Of ours it imports `internal/testutil` and nothing else — a
suite that reached into the pipeline could be made to pass by the same mistake
that made the pipeline wrong. Outside ours it uses `pgx` and `goccy/go-yaml`,
the latter only to read the *emitted* `lazyslice.yml` (`config_test.go`), which
is output under test and not one of our types.

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
- **Each test asserts against something it cannot vacuously satisfy, and every
  one of the six says how.** I1 fails when the target is missing any foreign
  key the source declares between two tables the target has (a zero-FK target
  is the special case, not the whole check). I2 fails when the source held no
  personal literal, when `classify --json` named no column, when the same
  predicate flags nothing on the *source*, or when no column the emitted yml
  records as masked arrived in the target with a value in it. I3 and I5 fail
  when the target's dump holds no COPY block or no data row — two empty
  databases dump identically — and when `lazyslice_meta` records no `run_id`
  that was not there before the second invocation, which is what separates
  "the second run reproduced the target" from "the second run did nothing".
  I6 fails if `--take` is not smaller than the root.
- **I1 reads `pg_constraint`, never `information_schema`.** A constraint name
  is unique per table, not per schema, so joining `referential_constraints` to
  `table_constraints` on (schema, name) pairs a partitioned table's cloned
  constraint with the wrong header and merges the columns of two same-named
  constraints; and `unique_constraint_name` is NULL for a foreign key backed by
  a bare `CREATE UNIQUE INDEX`, so an inner join drops that edge in silence.
  Both shapes are in `testdata/`. `conparentid = 0` keeps one row per declared
  edge.
- **I2's classifier half is scoped by §6 item 4:** "every unmasked,
  non-opted-out column". Two kinds of column are outside the net. A masked one,
  because a masked email still classifies as an email (§4, §5). An opted-out
  one, because an `unmask:` block keeps the source's own values in the target by
  design (§10's `public.film.description`, `masker: free_text` with a reason),
  and `verify` exits 0 on it. Asserting "nothing flagged anywhere", or
  "nothing flagged outside the masked set", is a false failure on a correct
  run. Both sets are read out of the emitted yml (`readColumnScope`), and they
  are kept apart: the vacuity guard ("not one column masked") counts the
  masker-backed set alone, so a run that opted everything out still fails. A
  `warn` or an `error` is not by itself evidence of surviving personal data and
  is not treated as one.
- **`classify` in this suite is always pointed at a config path that cannot
  exist.** `--no-config` suppresses only the *write* (§8); the read still
  defaults to `./lazyslice.yml`, which in the run's working directory is the
  file the run under test just emitted, and a classifier fed the run's own
  opt-outs is an echo rather than a second opinion.
- **`flaggedCodePrefixes` is a claim about a catalogue phase 4 writes, and it
  cannot fail silently.** A column-scoped code in neither it nor
  `copiedCodePrefixes` is a hard failure naming the code, and the same
  predicate must flag something on the source before a clean target is
  believed. When the classify codes land, that list is what changes.
- **`pg_dump` runs inside the server's own container** (`docker exec`, the
  container found by the published port), because pg_dump refuses to dump a
  server newer than itself and the CI matrix runs postgres:14 and postgres:18
  with whatever client the runner carries. The fallback to a `pg_dump` on PATH
  compares the two majors first and fails naming both numbers.
- **Dumps are normalised before comparison.** pg_dump has emitted a
  per-session `\restrict <token>` line since the August 2025 security releases
  (14.19, 15.14, 16.10, 17.6), so two dumps of one untouched database differ in
  bytes at line 5. The token lines and the version banners are replaced in
  place, keeping line numbers; any *other* backslash line outside a COPY block
  is a hard failure, so the next meta-command pg_dump invents is loud rather
  than a mystifying permanent diff.
- I6 uses `fixture.countedRoot`, which is not always `fixture.root`: the
  invariant only holds for a root no selected row reaches as a parent, and
  `public.people` is reached through its own `manager_id`
  (`testdata/README.md` trap 1).
- A connection URL carries a password. It goes to the binary as a flag and to
  `pgx`; it never goes into a test name, a `t.Log` or a failure message.

**Test.** `go test -tags integration -count=1 -timeout 30m ./internal/invariants/...`
(needs a Docker endpoint; `pg_dump` on `PATH` is a fallback, not a
requirement). Each test starts its own pair of containers per fixture, so the
whole package is twelve pairs; run one invariant with `-run TestI3`.

**Never:** skip for anything but Docker; assert through `core.Run` or a stage
package instead of through the binary; loosen an assertion because the
pipeline is a no-op — every test here fails today, naming the run's exit code,
and that is the phase 3 gate.
