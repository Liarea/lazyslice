# supabase-auth

GoTrue, the authentication service behind Supabase: 27 tables, all of them in the
`auth` schema, and the densest personal data in the set.

| | |
|---|---|
| Upstream | https://github.com/supabase/auth |
| Commit | `0907af9bd6be3c76f472c40a7dcc0dc34abeffaf` |
| Artifact | `migrations/*.sql`, all 75 of them |
| SHA-256 of the concatenation | `8114b241b64d4633c1fe6a0f5987db325e1ccf6b86f79f037c03c68bcdcbdb47` |
| Built with | `postgres:16` (digest `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`) |
| `schema.sql` | 27 tables, 24 foreign keys, 271 columns |
| Root | `auth.users` |

**How `schema.sql` was derived.** The 75 migrations are replayed in name order
into an empty database and the result dumped with
`pg_dump --schema-only --no-owner --no-privileges`. Two mechanical substitutions
are needed and `../build.sh supabase-auth` makes them:

1. GoTrue's migrations are Go templates: every identifier is written
   `{{ index .Options "Namespace" }}.users`. The namespace is substituted with
   `auth`, which is what a Supabase project uses.
2. A statement is terminated with `;` between files. Two migrations end without
   one, and concatenation then runs the next file's first statement into the
   previous file's last.

The roles the migrations grant to (`postgres`, `supabase_auth_admin`,
`authenticated`, `anon`, `service_role`, `dashboard_user`, `supabase_admin`) are
created first, with no attributes at all — never as superusers — and none of them
survives into `schema.sql`, which is dumped `--no-owner --no-privileges`.

## What it is here for

* **Nothing at all in `public`.** It is the one fixture that says a run works
  when every table is in another schema.
* **Personal data inside JSON.** `users.raw_user_meta_data` and
  `identities.identity_data` are where the provider's profile lands, so the name,
  the address, the phone number and the avatar URL are values inside a jsonb
  document rather than columns.
* **A reference with no foreign key.** `refresh_tokens.user_id` is
  `varchar(255)` holding a uuid as text; upstream never declared the constraint.
* **A generated column.** `identities.email` is
  `GENERATED ALWAYS AS (lower(identity_data ->> 'email'))`.
* **A partial unique index.** `confirmation_token_idx` is
  `UNIQUE (confirmation_token) WHERE confirmation_token::text !~ '^[0-9 ]*$'`,
  which is `testdata/regressions/007-partial-unique-index-masked-column.sql`.
* **Six unique token columns** that cannot be masked at all today, which is the
  single biggest finding of the whole exercise. See docs/TORTURE.md and T-0098.
