# discourse

The forum. 370 tables, 29 foreign keys — the fixture for a schema that keeps its
referential integrity in the application and not in the server.

| | |
|---|---|
| Upstream | https://github.com/discourse/discourse |
| Commit | `7b13572c76fa2c32e86267c91c238b0a3d8084f9` |
| Artifact | `db/structure.sql` (Rails `schema_format = :sql`, so this is pg_dump's own output) |
| SHA-256 of the artifact | `6187a423071fb5b190c680713c7389dc70f63fd15a9aaae27fc6af61f6d9291c` |
| Built with | postgres 16 + pgvector (`pgvector/pgvector:pg16`, digest `sha256:ccc6e83d6e35e931dc7c5def2022729d5a6c370318d099181995567ff1fb4d6b`) |
| `schema.sql` | 370 tables, 29 foreign keys, 3,372 columns |
| Root | `public.users` |

```
curl -sSL https://raw.githubusercontent.com/discourse/discourse/7b13572c76fa2c32e86267c91c238b0a3d8084f9/db/structure.sql | shasum -a 256
```

**How `schema.sql` was derived.** The artifact was loaded into a
`pgvector/pgvector:pg16` container and dumped back with
`pg_dump --schema-only --no-owner --no-privileges`, with pg_dump's two
`\restrict` / `\unrestrict` lines removed (they are per-session psql
meta-commands, and testdata/torture/README.md's loader is not psql). Nothing else
was changed: the file is upstream's schema, round-tripped.

**Why pgvector.** `db/structure.sql` declares `CREATE EXTENSION vector` and three
`halfvec` columns (`ai_*.embeddings`), so a stock `postgres:16` cannot load it —
the extension is not available and the columns' type does not exist. The
catalogue (`internal/invariants/torture_catalogue_test.go`) therefore names
`pgvector/pgvector:pg16` for this schema alone, for both its source and its
target. It is the one schema in the ten that needs a non-stock image, and
carrying an extension type through the pipeline is worth having in the set.

**What it is here for.**

* **29 foreign keys across 370 tables.** Discourse's references are integer
  columns with an index and nothing else. A run rooted at `public.users` reaches
  four tables and leaves 360 schema-only, and the interesting question is whether
  the plan *says so* rather than whether it guesses.
* **`public.user_emails` has no foreign key to `public.users`.** Every address in
  a Discourse install is in that table, and a slice from users does not reach it.
  That is correct and it is the sharpest example in the set of what "follow the
  declared edges" costs.
* **`public.poll_votes` has no key at all** — no primary key, no unique index over
  NOT NULL columns — so §3.4's identity ladder runs out on it. It is the one
  table in the ten that the run rescues with `--key` rather than dropping with
  `--skip-table`: it is the only edge in Discourse that carries a person two
  tables deep.
* **hstore, pg_trgm and unaccent** are all installed, and `hstore` is why
  testdata/regressions/005 carries an hstore column.
