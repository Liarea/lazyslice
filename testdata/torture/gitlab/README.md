# gitlab

A 43-table subset of GitLab's 1,448. The fixture for *width*: `public.users` has
more than seventy columns, several of them encrypted with their IV and salt in
neighbouring columns, and `public.namespaces` is self-referencing.

| | |
|---|---|
| Upstream | https://github.com/gitlabhq/gitlabhq (the GitHub mirror of gitlab-org/gitlab) |
| Commit | `f49b990568b44b24710efabf287200733f6fa930` |
| Artifact | `db/structure.sql` |
| SHA-256 of the artifact | `1b1c0eaf0dbb4715664ca19def90cde45e5bc12de0c212893a895d3b4c572411` |
| Built with | `postgres:16` (digest `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`) |
| `schema.sql` | 43 tables, 116 foreign keys, 968 columns |
| Root | `public.namespaces` |

```
curl -sSL https://raw.githubusercontent.com/gitlabhq/gitlabhq/f49b990568b44b24710efabf287200733f6fa930/db/structure.sql | shasum -a 256
```

## Why a subset, and which one

Upstream's `db/structure.sql` is 62,857 lines and 1,448 tables, and it loads in
about six seconds — so size alone is not the reason. The reasons are that a
`--take 100` over 1,448 tables is a plan nobody can read in a review, and that
generating rows for 1,448 tables is a fixture nobody can maintain. The subset is
computed, not hand-picked, by `../build.sh gitlab`:

1. **Seed**: 21 tables named in the build script — the account and
   work-tracking core (`users`, `namespaces`, `projects`, `issues`,
   `merge_requests`, `notes`, `members`, `milestones`, `emails`, `user_details`,
   `user_preferences`, `personal_access_tokens`, `identities`,
   `project_authorizations`, `issue_assignees`, `todos`, `events`,
   `abuse_reports`, `award_emoji`, `project_settings`, `namespace_settings`).
2. **Close over required parents**, so every NOT NULL foreign key still has
   something to point at.
3. **Close over what the surviving triggers touch.** GitLab hangs a trigger on
   most of these tables and each writes into a sync-event or loose-foreign-key
   table; dropping those left `INSERT INTO projects` failing with "relation
   projects_sync_events does not exist". The closure is over the trigger
   functions' own source text, one hop at a time, to a fixed point.
4. **Drop everything else**, one `DROP TABLE ... CASCADE` per statement — a
   single transaction over 1,400 drops runs out of `max_locks_per_transaction`.

That yields 43 tables. The `gitlab_partitions_static` and
`gitlab_partitions_dynamic` schemas are dropped whole: nothing in the keep set
is partitioned once the closure settles.

## The two objects the subset removes, and why

`ARCHITECTURE.md` §11.1 does not recreate functions, and refuses (exit 13) when a
*recreated* object depends on one. Two objects in the 43-table subset do:

| Object | Depends on |
|---|---|
| `public.organizations.uuid`'s `DEFAULT gen_random_uuid_v7()` | `gen_random_uuid_v7` |
| index `index_todos_coalesced_snoozed_until_created_at` | `timestamp_coalesce` |

The subset drops both — the default, keeping the column and its `NOT NULL`; the
index, keeping the table. **That is a deviation from upstream and it is the only
one.** It is made here rather than everywhere because `mastodon` carries the same
refusal *unedited* and is the tenth schema of gate 5 for exactly that reason: the
set proves the refusal happens and says what it costs, and does not spend nine
tenths of its coverage on it. docs/TORTURE.md records both halves.

## What it is here for

* **`CHECK (col IS NOT NULL)` instead of `NOT NULL`.** GitLab adds required
  columns this way to avoid a table rewrite, and `pg_attribute.attnotnull` says
  nothing about them; `_common/fill.sql` has a `required()` helper because of
  this schema.
* **A composite unique index whose non-key column is the discriminator.**
  `issue_assignees` is `PRIMARY KEY (user_id, issue_id)` with a third foreign key
  on `namespace_id`, which is what made `_common/fill.sql` order its parent picks
  by unique-index participation.
* **Eleven columns a first run refuses to mask**, ten of them tokens. See
  docs/TORTURE.md.
