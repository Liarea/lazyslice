# r/webdev draft

Draft only. Nothing here is posted; the maintainer edits and posts it.

## Title

Tired of seed data that doesn't look like production? I built a CLI that
snapshots a small, safe slice of your real Postgres DB

## Post body

You know the moment: a bug only reproduces with real data shapes, a new
teammate needs a working local database today, or CI passes on your seed
data and then fails in staging because your seed data stopped looking like
production a year ago. The usual fix is either hand-maintaining seed
scripts that always drift, or somebody quietly copying prod, customer
emails and all, onto a laptop.

lazyslice is a one-command CLI: it points at your production Postgres
database, picks a root table (your `customers` or `users` table, say),
follows foreign keys out from there so the copy stays referentially
complete — a `customer` with all their `orders` and `line_items`, not
orphaned rows — masks anything that looks like personal data with a
deterministic hash, and loads the result into an empty local or CI
database. Zero config: it asks at most one question on first run and
writes a `lazyslice.yml` afterwards as a record, not something you write
by hand up front.

To be upfront about it: **v0.1.0 only supports PostgreSQL** (a second
engine is a later phase), and it **pseudonymises, not anonymises** — a few
specific things aren't hidden, row identifiers being the main one, and
they're listed plainly in the README rather than glossed over. It's also
young — two earlier attempts at this exact idea, Snaplet and Neosync, both
shut down, so I'd rather undersell it than oversell it.

For the backend/full-stack folks here: right now, when a bug only shows up
against real data shapes, what do you actually do to get that data onto
your machine?
