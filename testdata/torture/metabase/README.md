# metabase

The BI tool's own application database: 100 tables, and the clearest example in
the set of a column whose name says nothing and whose contents must not leave the
building.

| | |
|---|---|
| Upstream | https://github.com/metabase/metabase |
| Pin | the official image `metabase/metabase:v0.56.10`, digest `sha256:4b2bdce29288b8e94d73e44862644e84c940ab68e551ba9aefb1a8dc1738e9d8` |
| Built with | that image against `postgres:16`, `MB_DB_TYPE=postgres` |
| `schema.sql` | 100 tables, 130 foreign keys, 892 columns |
| Root | `public.core_user` (41 incoming foreign keys) |

**Why an image digest and not a commit.** Metabase's schema is a Liquibase
changelog — `resources/migrations/*.yaml`, 400 KB of it, thousands of changesets
that create, rename and drop over eight years. Replaying it by hand is a
migration engine; running Metabase is one command. `../build.sh metabase` starts
the image with `MB_DB_TYPE=postgres` pointed at an empty database, waits for
"Metabase Initialization COMPLETE", stops it, and dumps the result with
`pg_dump --schema-only --no-owner --no-privileges`.

## What it is here for

* **`metabase_database.details`** is a JSON blob holding the host, user and
  password of every warehouse Metabase connects to, in a column called
  `details`, encrypted only when `MB_ENCRYPTION_SECRET_KEY` is set — and it
  usually is not.
* **`view_log` and `recent_views`** point at whatever was looked at through
  `(model, model_id)`: an undeclared polymorphic reference in a table with
  hundreds of thousands of rows in a real install.
* **A renamed table's identity sequence.** `group_table_access_policy` became
  `sandboxes` and Postgres left the sequence behind under the old name; the
  target's `GENERATED AS IDENTITY` creates `sandboxes_id_seq`, and lazyslice's
  `setval` named the source's. That is
  `testdata/regressions/006-identity-sequence-renamed-table.sql`, and it is the
  only defect in the set that only a *renamed* schema could have found.
* **Two keyless tables** (`model_index_value`, `table_privileges`) that the run
  drops with `--skip-table`; both are empty in the fixture.
