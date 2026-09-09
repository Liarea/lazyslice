# calcom

The scheduling app: 102 tables, 179 foreign keys, 46 enum types, and every
identifier in the application's own case. **It needs fewer flags than any other
schema of its size in the set**: one, `"Booking".uid`, the opaque reference in a
booking's public URL. `"Booking".oneTimePassword` was the second until T-0112
(T-0098).

| | |
|---|---|
| Upstream | https://github.com/calcom/cal.com |
| Commit | `1251ba5be567d26a7f922452fe7797642376476e` |
| Artifact | `packages/prisma/migrations/*/migration.sql`, all 595 of them |
| SHA-256 of the concatenation | `47c84e0e2eb64a8491078c7fa2d389c6f4ad04fadf6c15dab58158ce54da0802` |
| Built with | `postgres:16` (digest `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`) |
| `schema.sql` | 102 tables, 179 foreign keys, 1,092 columns |
| Root | `public.users` (48 incoming foreign keys) |

**How `schema.sql` was derived.** Prisma Migrate writes plain PostgreSQL DDL, one
`migration.sql` per migration, so the schema is those 595 files replayed in name
order into an empty database and dumped with
`pg_dump --schema-only --no-owner --no-privileges`. `../build.sh calcom` appends
a `;` between files for the same reason supabase-auth's build does: a handful end
without one. Nothing else is changed, and in particular `schema.prisma` is not
read at all — the migrations are the authority on what the database looks like,
and they are already SQL.

## What it is here for

* **Quoting, at scale.** `"EventType"`, `"Booking"."startTime"`,
  `"Attendee"."email"` — every identifier in every statement lazyslice generates
  has to be quoted or the run fails at the first `SELECT`. `testdata/nasty.sql`
  trap 9 is one table shaped like this on purpose; this is a hundred of them,
  written by somebody who was not thinking about us.
* **46 enum types**, more than the rest of the set together.
* **A trigger with a hard-coded foreign key.** `Membership` carries
  `update_membership_custom_role`, which rewrites `customRoleId` to the literal
  `'owner_role'`, `'admin_role'` or `'member_role'` — and `customRoleId` has a
  foreign key to `"Role"`. Those three rows have to exist before a single
  membership can be inserted, whatever the insert says. `generate.sql` writes
  them by hand and says why.
* **People who never signed up.** A booking's attendees have their own names,
  addresses, phone numbers and timezones, two foreign keys from the root.
