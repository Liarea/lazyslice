# testdata/torture

Ten PostgreSQL schemas nobody wrote with lazyslice in mind.

`testdata/pagila` is the schema a person would plausibly have designed and
`testdata/nasty.sql` is every shape that has ever broken a subsetting tool, put
in one place on purpose. Both are ours. These ten are not: they are the real
schemas of ten open-source projects, at a pinned commit or image digest, and
between them they carry 1,023 tables and 1,308 foreign keys that nobody chose
for our convenience. They are phase 5's gate (docs/BUILD_PLAN.md), and
docs/TORTURE.md is what came of them.

```
make torture
```

## What is here

| Directory | Tables | FKs | Root | Runs |
|---|---:|---:|---|---|
| `rails-activestorage/` | 7 | 3 | `public.active_storage_blobs` | clean |
| `django/` | 10 | 9 | `public.auth_user` | clean |
| `supabase-auth/` | 27 | 24 | `auth.users` | clean |
| `plausible/` | 42 | 40 | `public.sites` | clean |
| `gitlab/` | 43 | 116 | `public.namespaces` | clean |
| `metabase/` | 100 | 130 | `public.core_user` | clean |
| `calcom/` | 102 | 179 | `public.users` | clean |
| `mastodon/` | 118 | 156 | `public.accounts` | **exit 13**, see below |
| `odoo/` | 204 | 622 | `public.res_users` | clean |
| `discourse/` | 370 | 29 | `public.users` | clean |

Each directory holds three files and nothing else:

* `README.md` — the upstream URL, the pinned commit or image digest, the SHA-256
  of the artifact, how `schema.sql` was derived from it, every deviation from
  upstream, and what the schema is in the set for.
* `schema.sql` — a `pg_dump --schema-only --no-owner --no-privileges` of a real
  database built from that pin.
* `generate.sql` — the rows. A few hundred in the most-connected tables, and the
  personal data put in deliberately.

`_common/fill.sql` is the row generator all ten call, and `_common/cleanup.sql`
takes it back out again. `build.sh` rebuilds any `schema.sql` from its pin.

**Nine of ten snapshot cleanly and the tenth is `mastodon`**, which refuses at
exit 13 because its primary keys default to a function ARCHITECTURE.md §11.1 does
not recreate. That refusal is asserted as tightly as the nine successes are;
`mastodon/README.md` says why it cannot be otherwise.

## Contract

`internal/invariants/torture_catalogue_test.go` is the catalogue: for each
schema, the root, the `--take`, the flags a first run demanded, the exit code
expected, and the tables the slice has to reach. `TestTortureSchemas` runs them;
`TestTortureCatalogueMatchesTheFixtures` fails when a directory here has no
catalogue entry or a catalogue entry has no directory, so neither can be added
alone.

**The root is not a free choice.** It is the table with the most foreign-key
edges incident to it — each declared constraint counted once, in either
direction — with ties broken by incoming edges, then by row count, then by name.
`assertRootIsMostConnected` recomputes that on the loaded source at run time, so
a generator change that moves the answer fails the suite rather than quietly
slicing from somewhere else. Discourse is the schema that needs every term of the
rule: `users`, `uploads` and `ad_plugin_house_ads` all have four incoming edges
and none outgoing.

## Rules

- **Every deviation from upstream is in that schema's `README.md`, with a
  count.** There are exactly three in the ten: gitlab's subset (43 of 1,448
  tables) and the two objects in it that depend on a GitLab-defined function;
  mastodon's `timestamp_id` salt, fixed so the build is reproducible; and odoo's
  three modules. A fourth that appears without a paragraph is a fixture nobody
  can trust.
- **`schema.sql` is generated, never hand-edited.** It is `pg_dump`'s output,
  with the two `\restrict` / `\unrestrict` psql meta-commands stripped, and
  `build.sh <name>` regenerates it. Re-pin and rebuild; do not patch.
- **No file here may contain a `COPY ... FROM stdin` block or a backslash
  command.** The loader (`runScript`, in `internal/invariants`) sends each file
  to the server as one simple query and is not psql — which is the whole reason
  the `\restrict` lines are stripped. A file that broke this would fail with the
  server's own syntax error, which is loud, but it would fail.
- **A generator is deterministic in the row index.** No `random()`, no `now()`,
  no `clock_timestamp()`. Invariant I3 says the same source, secret and config
  produce a byte-identical target, and a fixture that changed under its own feet
  would make that unfalsifiable. `_common/fill.sql`'s header states it and every
  expression it emits obeys it.
- **Generated personal data stays out of the maskers' own output spaces.**
  Addresses are under `.test`, never `example.com`/`.net`/`.org`; IPv4 is in
  `10.0.0.0/8`, never an RFC 5737 documentation range; phone numbers are outside
  `555-01XX`. ARCHITECTURE.md §5 makes those the `email`, `network_id` and
  `phone` maskers' output, so a source value inside one is a documented
  false-positive surface for the residual scan (T-0059, T-0073).
  **A generated person's name is the same rule and it was learned the hard way.**
  `mask/words.go` holds the `givenWords`, `surnameWords`, `streetWords` and
  `suffixWords` the `person_name` and `address` maskers draw from, and no word in
  a generator's name pool may be in one of them. Cal.com's attendee pool shared
  four given names and four surnames with those lists, and over 500 rows a masked
  name landed on a name the source still held in that column often enough that
  `TestTortureSchemas/calcom` failed about one run in five at exit 9,
  `verify.refused.residual`. Widening a pool does not help — it adds collidable
  values; being disjoint from the lists does, and it makes the collision
  impossible rather than unlikely. The check is a grep of each `generate.sql`'s
  quoted literals against those four lists.
- **The suite masks under a fixed key.** `internal/invariants/harness_test.go`
  writes `fixedSecret` into every run's working directory, so `make torture` is
  one experiment repeated rather than a new one each time and a residual-scan
  failure is a defect rather than a draw. It is not a substitute for the rule
  above: a fixed key makes the answer stable, disjoint pools make it right.
- **A schema's flags are in the catalogue with a reason each, and in
  docs/TORTURE.md.** Every `--unmask` here is a column a first run refused and
  named; the reason says why that column is not personal data, or names the
  tracker task that would remove the need for the flag. A flag with a reason like
  "makes it pass" is the thing this directory exists to make impossible.
- **`_common/fill.sql` is shared, so a change to it changes all ten.** Treat it
  as a library: the per-schema `generate.sql` is where a schema's own peculiarity
  goes, and every table it fills by hand says in a comment why the generator
  could not.

## Test

```
make torture                       # all ten, plus testdata/regressions/
go test -tags 'integration torture' -run TestTortureSchemas/django ./internal/invariants/
```

One schema by hand, without Go:

```
psql -f testdata/torture/django/schema.sql
psql -f testdata/torture/_common/fill.sql
psql -f testdata/torture/django/generate.sql
psql -f testdata/torture/_common/cleanup.sql
```

## Never

Add a schema without a `README.md` that pins it; hand-edit a `schema.sql`; put a
`random()` or a `now()` in a generator; put a word from `mask/words.go`'s name,
street or suffix lists in a generator's name pool; add an `--unmask` to the catalogue
without a reason that would survive a stranger reading it; let this directory
grow an eleventh schema without deciding which of the ten it replaces — ten is
gate 5's number and the suite asserts it.
