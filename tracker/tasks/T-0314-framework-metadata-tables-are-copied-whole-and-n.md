---
id: T-0314
title: "Framework metadata tables are copied whole and never masked (schema_migrations, ar_internal_metadata and their kin)"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0314 · Framework metadata tables are copied whole and never masked (schema_migrations, ar_internal_metadata and their kin)

## Goal

Dogfood session 1: schema_migrations.version was masked (its digit strings pass the Luhn check) and the table was schema_only because no foreign key reaches it, so the copy had zero migration rows and Rails would re-run every migration; ar_internal_metadata likewise, and when copied its environment=production row makes Rails refuse destructive tasks in development. lazyslice should recognise the framework metadata tables by name (schema_migrations, ar_internal_metadata, flyway_schema_history, django_migrations, alembic_version, knex_migrations, goose_db_version, __EFMigrationsHistory, atlas_schema_revisions, _prisma_migrations, sequelize meta, liquibase databasechangelog and databasechangeloglock), copy them whole as lookups regardless of reachability, never mask them, and rewrite ar_internal_metadata's environment to development (say so in the plan). Pin with a fixture holding schema_migrations and ar_internal_metadata.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: effdee3; framework metadata tables are copied whole as lookups regardless of reachability and never masked, ar_internal_metadata's environment is rewritten to development, and the plan names the rule on the table's own line; the reviewer's narrowing (only each tool's bookkeeping columns are exempt; Flyway's installed_by and Liquibase's AUTHOR are classified) landed in the fix round; make check, verify/plan/core/load integration and make torture green, regression 042 on Luhn-valid migration versions passes only with both halves | went badly: blocked twice on the verify half: the second net still refused a Luhn-valid version in the copied table (T-0348), outside the task's paths, so the developer could only file it; the orchestrator landed it by hand (a netMode case, a unit test that fails without it, 042 switched to the dogfood shape); the per-tool column lists came from the developer's memory, filed as T-0349 | change next time: a task that adds a never-mask rule must include internal/verify in its paths, since the second net is the other half of every masking exemption
