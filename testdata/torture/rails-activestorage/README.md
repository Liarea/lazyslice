# rails-activestorage

`rails new` with ActiveStorage installed and two models: seven tables, three
foreign keys. The smallest schema in the set, and the one carrying the shape
phase 5 owes an answer to.

| | |
|---|---|
| Upstream | https://github.com/rails/rails |
| Pin | Rails 8.0.2 from RubyGems, on `ruby:3.3-slim` (digest `sha256:79f7a07363931fde1a5b312dee281fd62ddf56c33bdba0c2622af9b4621182fb`) |
| Built with | that image against `postgres:16` |
| `schema.sql` | 7 tables, 3 foreign keys, 36 columns |
| Root | `public.active_storage_blobs` |

**How `schema.sql` was derived.** `../build.sh rails-activestorage` runs, in a
`ruby:3.3-slim` container, what a person starting a Rails project runs:

```
gem install rails -v 8.0.2 pg
rails new blog -d postgresql --skip-git --skip-test ...
bin/rails active_storage:install
bin/rails generate scaffold Post title:string body:text author_email:string
bin/rails generate model Comment post:references author_name:string author_email:string body:text
bin/rails db:migrate
```

then dumps the database with `pg_dump --schema-only --no-owner --no-privileges`.
The two generated models are there so the schema has something for
ActiveStorage's attachments to point *at*; everything else — `active_storage_*`,
`ar_internal_metadata`, `schema_migrations` — is Rails'.

Pinning the Rails version rather than a git commit is deliberate, for the same
reason as django's.

## What it is here for

* **The polymorphic reference.** `active_storage_attachments` points at whatever
  it is attached to through `(record_type, record_id)`, with no foreign key
  behind it — ARCHITECTURE.md §3.2's inference is what has to notice, and
  `generate.sql` makes the pair point at real `posts` rows so there is something
  to notice.
* **A four-column unique index whose masked column is a literal.**
  `index_active_storage_attachments_uniqueness` is
  `(record_type, record_id, name, blob_id)` and `name` is `'cover'` in every row;
  raising it as a unique column refused a run that could not have collided, which
  is
  `testdata/regressions/003-composite-unique-index-is-not-a-unique-column.sql`.
* **`active_storage_blobs.key`** is an opaque storage key under a single-column
  unique index, classified `credential` — and `credential`'s only masker emits
  one fixed literal, which is
  `testdata/regressions/001-unique-index-masking-collision.sql` seen from a third
  angle.
