// SPDX-License-Identifier: Apache-2.0

package pipeline

import "strings"

// frameworkMetadataTableNames is the bare, case-folded name of every
// migration/version bookkeeping table lazyslice recognises by name alone
// (tracker T-0314). An ORM or migration tool reads this table before the
// application does anything else, so a partial or masked copy of it does not
// make the target *safer* the way masking an ordinary lookup table does — it
// makes the target *wrong*: Rails re-running every migration against a schema
// that already has them (schema_migrations missing or short), or refusing a
// destructive rake task because the one row it reads back says "production"
// (ar_internal_metadata).
//
// root CLAUDE.md's "when in doubt, mask it" does not reach these tables the
// way it reaches an ordinary one: there is no doubt to resolve. The name is
// the framework's own, fixed across every application that uses it, and the
// rows are bookkeeping the tool wrote and reads back — never end-user data.
//
// dogfood session 1 (T-0314) found this the hard way: schema_migrations had
// no foreign key reaching it, so §3's walk never selected it and it planned
// SchemaOnly — ddl only, zero rows — and ar_internal_metadata went the other
// way, copied verbatim with its source `environment` row intact.
var frameworkMetadataTableNames = map[string]bool{
	"schema_migrations":      true, // Rails (and others that copied the name)
	"ar_internal_metadata":   true, // Rails
	"flyway_schema_history":  true, // Flyway
	"django_migrations":      true, // Django
	"alembic_version":        true, // SQLAlchemy / Alembic
	"knex_migrations":        true, // Knex.js
	"goose_db_version":       true, // Goose
	"__efmigrationshistory":  true, // EF Core (catalog spelling __EFMigrationsHistory)
	"atlas_schema_revisions": true, // Ariga Atlas
	"_prisma_migrations":     true, // Prisma
	"sequelizemeta":          true, // Sequelize (catalog spelling SequelizeMeta)
	"databasechangelog":      true, // Liquibase
	"databasechangeloglock":  true, // Liquibase
}

// IsFrameworkMetadataTable answers whether ref names one of them.
//
// The match is on the bare table name only, never the schema — every one of
// these tools creates its table in whatever schema the migration ran
// against, and none of them are schema-qualified by convention — and it is
// case-insensitive, because the tools do not agree on case: EF Core's own
// catalog name is the mixed-case, quoted __EFMigrationsHistory, and every
// other one on this list is unquoted lower.
func IsFrameworkMetadataTable(name string) bool {
	return frameworkMetadataTableNames[strings.ToLower(name)]
}

// frameworkMetadataColumns is the per-table allowlist internal/classify reads
// (T-0314 review round, finding 2): which of a recognised table's own columns
// are the tool's actual bookkeeping — a migration id, a checksum, a
// timestamp, a boolean flag — rather than a column that merely happens to sit
// on the same table.
//
// The premise IsFrameworkMetadataTable's own doc comment states — "bookkeeping
// the framework itself wrote and reads back, never end-user data" — holds for
// every column listed here and does not hold for every column of the table:
// Liquibase's DATABASECHANGELOG carries AUTHOR, the identity of whichever
// developer ran the changeset, and Flyway's flyway_schema_history carries
// INSTALLED_BY, the database role or OS user that applied it — either can be a
// real name, a real username or an email address, and neither is on this
// list. Every entry here is a well-known column of the tool's own migration
// schema as that tool ships it; nothing here is inferred from one operator's
// database.
//
// A column absent from its table's set is not "maybe exempt": it reaches the
// ordinary classifier exactly as any other column of any other table would,
// so a name or value signal on it still masks it (root CLAUDE.md, "when in
// doubt, mask it"). The table itself is still forced to a Lookup step and
// copied whole regardless (internal/plan's own T-0314 entry) — this map only
// answers the narrower question of which of its columns travel unmasked.
var frameworkMetadataColumns = map[string]map[string]bool{
	"schema_migrations": {"version": true},
	"ar_internal_metadata": {
		"key": true, "value": true, "created_at": true, "updated_at": true,
	},
	// installed_by deliberately absent: the role or user that ran the migration.
	"flyway_schema_history": {
		"installed_rank": true, "version": true, "description": true, "type": true,
		"script": true, "checksum": true, "installed_on": true, "execution_time": true,
		"success": true,
	},
	"django_migrations":     {"id": true, "app": true, "name": true, "applied": true},
	"alembic_version":       {"version_num": true},
	"knex_migrations":       {"id": true, "name": true, "batch": true, "migration_time": true},
	"goose_db_version":      {"id": true, "version_id": true, "is_applied": true, "tstamp": true},
	"__efmigrationshistory": {"migrationid": true, "productversion": true},
	"atlas_schema_revisions": {
		"version": true, "description": true, "type": true, "applied": true, "total": true,
		"executed_at": true, "execution_time": true, "error": true, "error_stmt": true,
		"hash": true, "partial_hashes": true, "operator_version": true,
	},
	"_prisma_migrations": {
		"id": true, "checksum": true, "finished_at": true, "migration_name": true,
		"logs": true, "rolled_back_at": true, "started_at": true, "applied_steps_count": true,
	},
	"sequelizemeta": {"name": true},
	// author deliberately absent: the developer identity that ran the changeset.
	"databasechangelog": {
		"id": true, "filename": true, "dateexecuted": true, "orderexecuted": true,
		"exectype": true, "md5sum": true, "description": true, "comments": true,
		"tag": true, "liquibase": true, "contexts": true, "labels": true,
		"deployment_id": true,
	},
	// lockedby deliberately absent: the identity holding the lock.
	"databasechangeloglock": {"id": true, "locked": true, "lockgranted": true},
}

// IsFrameworkMetadataColumn answers whether column, on a table
// IsFrameworkMetadataTable already recognises, is one of that tool's own
// known bookkeeping columns. The match is case-insensitive on both arguments,
// for the identical reason IsFrameworkMetadataTable's own is.
//
// A table IsFrameworkMetadataTable does not recognise has no entry here
// either, so this reports false for it; callers check IsFrameworkMetadataTable
// first, the way internal/classify's markNeverMasked does, and this function
// does not repeat that check.
func IsFrameworkMetadataColumn(table, column string) bool {
	return frameworkMetadataColumns[strings.ToLower(table)][strings.ToLower(column)]
}

// ArInternalMetadataEnvironmentColumns finds Rails's ar_internal_metadata
// key/value columns on t, case-insensitively, so that internal/plan can say
// the environment rewrite is coming and internal/load can perform it against
// the exact spelling the catalog holds — one answer, read by both, the same
// reason this file's rest is here (internal/pipeline/CLAUDE.md).
//
// ar_internal_metadata is an EAV table (Rails' own migration creates it with
// a string primary key "key" and a "value" column): the row with key =
// "environment" is what a fresh clone's `bin/rails` reads before it will run
// a destructive task, and a snapshot's copy of it verbatim carries the
// source's own "production" straight into a development checkout.
//
// ok is false when the table does not carry both columns — a same-named
// table of a different shape, which nothing here can rule out by name alone
// — and both callers then leave the table alone rather than send a statement
// that names a column that is not there.
func ArInternalMetadataEnvironmentColumns(t Table) (keyCol, valueCol string, ok bool) {
	for _, c := range t.Columns {
		switch strings.ToLower(c.Name) {
		case "key":
			keyCol = c.Name
		case "value":
			valueCol = c.Name
		}
	}
	return keyCol, valueCol, keyCol != "" && valueCol != ""
}
