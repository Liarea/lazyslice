# internal/invariants

The black-box suite. Six test functions, one per invariant in ARCHITECTURE.md
§12's line `internal/invariants/  I1–I6 as tests against any produced target,
both fixtures`, each run against both `testdata/` fixtures on two PostgreSQL
containers from `internal/testutil`.

| Test | Invariant | How it is checked |
|---|---|---|
| `TestI1ForeignKeysResolve` | every foreign key in the target resolves | `pg_constraint` for the edges, an anti-join per edge, plus every source edge between two tables the target has |
| `TestI2NothingFlaggedSurvives` | nothing flagged survives masking | `classify --json` on the target scoped to the columns the run neither masked nor opted out, a grep for every source email and phone, and a source-against-target distinct-value comparison over every masked column |
| `TestI3SameInputsSameTarget` | same source, secret and config → byte-identical target | `pg_dump --data-only` twice, normalised, compared, with one loaded table emptied in between |
| `TestI4SourceUnchanged` | the source is unchanged | row count and an ordered-row md5 per table, plus a catalog fingerprint (relations, indexes, columns, constraints, trigger state and every sequence's position), before and after |
| `TestI5EmittedConfigReproducesTheSnapshot` | the emitted yml reproduces the snapshot | re-run from a fresh directory holding only the yml, with `--config` and no plan flags, compared as I3 |
| `TestI6RootHoldsTakeRows` | the root holds `--take` rows | `count(*)` on the root of the target |

Not in the table, and load-bearing for all six: `fixture.mustHoldRows`, asserted
by the harness immediately after every snapshot of the fixture's own slice.
Each fixture names tables the slice is required to reach — two children and a
grandchild at least, and for nasty a table in a second schema — and each must
be in the target with more than zero rows. Without
it the whole suite is satisfied by an **upward-only** snapshot: take the root's
`--take` rows, follow foreign keys to their parents so referential integrity
holds, and never create or load a child table at all. I1 skips a source edge
whose child is absent, I6 counts only the root, I2's guard is about masked
columns, and I3/I5 compare a target with itself. §6 item 5 states the property
in general (`every step with mode ChildOK or ParentOnly has exactly Keys.Len()
rows in the target`); this is the part of it phase 3 can check without a plan.

`fixture.mustBeSubset` is the second, shorter list, and it is deliberately not
every entry of the first. "The target holds some of this table's rows" and "the
target holds fewer than all of them" are two claims and only the first is true
of every table a correct slice reaches. nasty is the proof: its run is
`--root public.people --take 3`, `people.manager_id` is an *incoming* edge, so
the child step pulls the seed's reports as well and all five people are
selected at depth 1 — after which `orders`, `order_items` and
`billing.invoices` arrive whole, correctly, at every value of `--take`.
Requiring a proper subset of those fails a pipeline that did exactly what §3
says, and the cheapest way to make it green again is to delete the guard.
`public.attachments` is nasty's one genuinely subset table (row 839's
`uploaded_by_person_id` is NULL, so no person reaches it); pagila's are
`rental` and `payment`. A fixture with an empty `mustBeSubset` is a hard
failure, and a name in it that is not in `mustHoldRows` is too.

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
  predicate flags nothing on the *source*, or when a column the emitted yml
  records as masked arrived in the target with no value in it while the source
  has one. I3 and I5 fail when the target's dump holds no COPY block or no
  data row — two empty databases dump identically — and when `lazyslice_meta`
  records no `run_id` that was not there before the second invocation. I6
  fails if `--take` is not smaller than the root. And every fixture's
  `mustHoldRows` fails a slice that reached no child table.
- **A guard the implementation can satisfy by describing itself is not proof.**
  `assertSecondRunHappened` reads `lazyslice_meta`, and §11.2 is what says the
  marker goes in before the first drop — so a second invocation that inserted
  its marker, found its own bound fingerprint, short-circuited and exited 0
  passes it while touching no data, which is exactly the case it was written
  to catch. I3 and I5 therefore **empty one loaded table between the two
  runs**: the comparison against the first dump then passes only if the second
  run truly truncated and reloaded. Keep both signals; the marker is the cheap
  one and the mutation is the proof.
- **I5 runs the second invocation from a fresh directory holding only the
  emitted yml**, and asserts that the file **parses** with `root:` and `take:`
  as top-level scalars holding the run's own values before feeding it back. Not
  a text search: `strings.Contains(text, "public.customer")` is satisfied by
  any `columns:` key beginning `public.customer.`, and `Contains(text, "3")` by
  `depth: 3` or a hex fingerprint, so the very implementation the guard exists
  to catch would pass it. Re-running in the first run's own working directory
  leaves two channels that can carry the plan instead of the file — anything
  the tool caches beside itself, and the target's `lazyslice_meta`, which
  §11.2 already has carrying `classification_fingerprint` and `tool_version`.
  A yml holding nothing but a `columns:` map would otherwise pass, and the
  first person to commit it and run on a clean checkout gets a different
  snapshot.
- **I4's fingerprint has two halves, and the second is not about rows.** An
  index created to make an extract tractable and left behind takes locks,
  consumes disk and outlives the run, and it changes no row of any ordinary
  table; so does an advanced sequence, a refreshed materialised view or a
  disabled trigger. The catalog fingerprint covers relations, indexes,
  columns, constraints, trigger state and every sequence's `last_value`, and
  reports each class in its own message. `pg_temp` is out of reach from
  another session, and the comment in the file says so rather than implying
  coverage.
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
- **I2's third half is the one the run cannot scope, and it is why the other
  two can be trusted.** The classifier half is scoped by the run's own yml and
  the grep half knows two categories, email and phone; a pipeline that
  implemented those two maskers, wrote `masker:` against every column and
  copied the rest through verbatim passes both, with pagila's `first_name`,
  `last_name`, `address`, `postal_code` and `staff.password` landing in the
  target byte-identical to production. So for every column the run itself
  recorded as masked, the distinct values in the target and in the source must
  not overlap. Widening the masked set only adds assertions. `masker: "null"`
  is excluded from the "must hold a value" guard, because its correct output is
  NULL, and so is `masker: derived_text`, because ADR-010's correct output is
  the *empty* tsvector — pagila's `public.film.fulltext` arrives holding
  nothing, and reading that as "the masker's output was never loaded" fails a
  run that did exactly what the ADR says (T-0058). All three exemptions — the
  two maskers and a source column that is NULL or `''` everywhere — count as
  skipped, so the guard's own vacuity check still fires when every masked
  column is one of them. `masker: "null"` was not counted when the
  `derived_text` exemption was added, which left a run whose whole masked set
  was `null` columns passing I2 having examined nothing; counting it is the
  fix.
- **I2's third half has one known coincidental failure, it can land on either
  of two columns, and the fix is not in this package.** nasty has two masked
  inet columns: `public.tenant_user_sessions.origin`
  (`203.0.113.7`, `203.0.113.8`, `198.51.100.22`, `198.51.100.23`) and
  `public.audit_log.client_ip` (`203.0.113.7`, `203.0.113.8`,
  `198.51.100.44`). `mask`'s IPv4 generator draws from those same three RFC
  5737 blocks (`mask/words.go` `docPrefixes`, 768 addresses) under a key that is
  random per run. So a masked address can equal a source address with nothing
  wrong, disjointness fails, and the run is red — four source values against
  four generated ones on `origin` and three against three on `client_ip`, a few
  percent of runs of this fixture for the two together. Their `2001:db8::`
  values are not in it: the v6 generator fills 96 random bits. Seen once in
  eight runs by T-0058's first reviewer; not seen in the twenty consecutive runs
  that answered that review, nor in the second reviewer's full `make
  integration` and fifteen further runs, nor in the full run that answered the
  second review. The assertion is right and stays as it
  is: the fix is to move the fixture's values out of the generator's output
  space or to give the generator a block the fixtures never use, in
  `testdata/nasty.sql` and `mask/gen_net.go`, which T-0058's paths did not
  include. It is recorded here, in `.github/workflows/ci.yml` beside the removed
  allowlist, and returned to the orchestrator; a red on `nasty/values` naming
  only `origin`, only `client_ip`, or only those two is this, not a masking
  regression.
- **What I2's third half excludes is exactly what §5 says must reuse an
  admissible value, and nothing else.** `NULL` and `''` (§5 preserves both;
  `scanCells` drops them). An empty array and an empty JSON document, which are
  the same rule for a collection and which arrive as the non-empty text `{}`
  (trap 15's `'{}'::text[]`, trap 16b's collapsed `events.payload`) — dropped
  by `preservedEmpty`. And a **small admissible domain**: §5 says a masked
  column with `d < 2 × distinct(samples)` is "a stable substitution over a
  small alphabet", and that a special category at small `d` is "collapsed to
  one fixed label (the first enum label...)". `people.marital_status` is
  collapsed to `single`, which is row 90007's real value (trap 24), and any
  boolean masker's output is in the source's own `{true, false}` (trap 19), so
  disjointness is *unsatisfiable* there and asserting it fails a correct run.
  Those columns are excluded by two signals — the yml's own top-level
  `small_domain:` list (§10), and the target catalogue's count of the values
  the type admits (`admissibleDomains`: enum labels, boolean, `varchar(n)` at
  tiny `n`) — so a run that simply omitted the column from `small_domain:`
  cannot turn the exclusion into a failure, nor the failure into an exclusion.
  Excluded columns stay in the classifier half, the grep half and
  `assertTargetHoldsMaskedRows`; the exclusions are printed; and a run where
  *every* masked column is excluded is a hard failure. What no invariant here
  can check is the complement — that the column was listed under
  `small_domain:` and in the plan, and that a special category was collapsed
  rather than substituted — because that needs the sample the classifier took.
  It is a `lazyslice verify` assertion for phase 4.
- **A column is compared as a parsed `columnRef`, never as a string.** The
  emitted yml's `columns:` keys, the `--json` stream's table and column fields
  and the catalogue all spell an identifier differently: §10's examples are
  unquoted lower case, a generated statement has to write
  `public."LegacyCustomer"."EmailAddress"` quoted (trap 9), and pg_attribute
  holds it bare. And §3.3 makes a partitioned table's *root* the step, so the
  source's rows are in `public.events_2024` while the yml and the target say
  `public.events` — `scanCells` attributes a leaf's cells to its root
  (`partitionRoots`) for that reason. A masked column whose key matches no
  column of either database is a hard failure naming the key
  (`assertMaskedColumnsExist`), never a quiet entry in `skipped`: the two
  guards would otherwise report clean by omission over exactly the columns
  traps 9, 16b and 23 exist for.
- **`internal/verify` is not covered by anything in this package, and the two
  `verify.` entries in `flaggedCodePrefixes` are unreachable.** I2 points
  `classify` at the target; no command this suite runs emits a `verify.` code.
  A `verify` stage that landed as `return nil` passes all six invariants. The
  check that would catch it is a phase 4 negative control against the gate 4
  verify stage — write a known source email literal into a target column the
  run recorded as masked, run `lazyslice verify`, assert exit 9 naming table
  and column — and the comment beside `flaggedCodePrefixes` says so.
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
- I6 uses `fixture.countedRoot`, which is neither fixture's `fixture.root`: the
  invariant only holds for a root no selected row reaches as a parent, because
  §3 lets a `PARENT_ONLY` push widen any step's key set and the root is a step.
  `public.people` is reached through its own `manager_id` (`testdata/README.md`
  trap 1). `public.customer` is reached the long way round — a selected
  customer's rentals pull their payments, and `payment.customer_id` then pushes
  a customer `--take` never selected, which is the 101 rows for `--take 100`
  T-0058 answered. The two counted roots are `public.tenant_users` and
  `public.film_actor`, and the criterion they meet is the one above — a root no
  selected row reaches *as a parent* — not the stricter "no incoming foreign
  key". `public.film_actor` does meet the stricter one; `public.tenant_users`
  does not, and does not have to: two foreign keys reference it, from
  `tenant_user_sessions` and `tenant_user_flags`, and both of those tables are
  reached only as its own children, so neither can push a key back onto it. The
  reasoning for both is in `harness_test.go` beside the field, which is where a
  third fixture's choice should be read from rather than from this line.
- A connection URL carries a password. It goes to the binary as a flag and to
  `pgx`; it never goes into a test name, a `t.Log` or a failure message, and
  `result.String()` redacts it. `pg_dump` gets the password through
  `PGPASSWORD` (forwarded into the container with `docker exec -e PGPASSWORD`)
  and a URL with the password stripped, because argv is world-readable and
  `exec.ExitError` quotes it.
- **The child environment is built by removing variables, never by setting
  them empty** (`cleanEnv`). An implementation keyed on presence would run
  with an empty masking key under `LAZYSLICE_SECRET=`, and I3's "same secret,
  same target" would then be asserting something else. The list covers every
  discovery rung and every libpq variable that can redirect a connection —
  `PGPASSFILE`, `PGSSLMODE`, `PGOPTIONS`, `PGSERVICEFILE` included — and
  everything with a `LAZYSLICE_` prefix, by prefix so a phase 4 variable is
  covered the day it exists.

**Test.** `go test -tags integration -count=1 -timeout 30m ./internal/invariants/...`
(needs a Docker endpoint; `pg_dump` on `PATH` is a fallback, not a
requirement). Each test starts its own pair of containers per fixture, so the
whole package is twelve pairs; run one invariant with `-run TestI3`.

**Never:** skip for anything but Docker; assert through `core.Run` or a stage
package instead of through the binary; loosen an assertion because the
pipeline is a no-op — every test here fails today, naming the run's exit code,
and that is the phase 3 gate.
