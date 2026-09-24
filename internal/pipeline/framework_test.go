// SPDX-License-Identifier: Apache-2.0

package pipeline

import "testing"

func TestIsFrameworkMetadataTable(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"schema_migrations", true},
		{"ar_internal_metadata", true},
		{"flyway_schema_history", true},
		{"django_migrations", true},
		{"alembic_version", true},
		{"knex_migrations", true},
		{"goose_db_version", true},
		// EF Core's own catalogue spelling is mixed case; the match must not
		// depend on it (T-0314).
		{"__EFMigrationsHistory", true},
		{"__efmigrationshistory", true},
		{"atlas_schema_revisions", true},
		{"_prisma_migrations", true},
		{"SequelizeMeta", true},
		{"sequelizemeta", true},
		{"DATABASECHANGELOG", true},
		{"databasechangelog", true},
		{"databasechangeloglock", true},
		// Ordinary tables, including ones that merely sound related, are not
		// on the list — this is a fixed, narrow set of names and nothing here
		// widens it by inference.
		{"users", false},
		{"migrations_log", false},
		{"schema_migrations_backup", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsFrameworkMetadataTable(c.name); got != c.want {
			t.Errorf("IsFrameworkMetadataTable(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestIsFrameworkMetadataColumn is the T-0314 review round's second
// finding's pin: the exemption is narrower than the table match, and it is
// specifically the columns that can carry a developer's or a database role's
// own identity — Liquibase's AUTHOR, Flyway's INSTALLED_BY — that must come
// back false so internal/classify's markNeverMasked lets them fall through to
// the ordinary classifier.
func TestIsFrameworkMetadataColumn(t *testing.T) {
	cases := []struct {
		table  string
		column string
		want   bool
	}{
		{"schema_migrations", "version", true},
		{"schema_migrations", "Version", true}, // case-insensitive, both args
		{"ar_internal_metadata", "key", true},
		{"ar_internal_metadata", "value", true},
		{"ar_internal_metadata", "created_at", true},
		{"flyway_schema_history", "checksum", true},
		{"flyway_schema_history", "version", true},
		// installed_by is the role or OS user that ran the migration, not
		// bookkeeping the tool reads back — it must not be exempted.
		{"flyway_schema_history", "installed_by", false},
		{"databasechangelog", "id", true},
		{"databasechangelog", "filename", true},
		{"databasechangelog", "comments", true},
		// author is the developer who ran the changeset.
		{"databasechangelog", "author", false},
		{"DATABASECHANGELOG", "AUTHOR", false},
		{"databasechangeloglock", "locked", true},
		// lockedby is the identity holding the lock.
		{"databasechangeloglock", "lockedby", false},
		{"__EFMigrationsHistory", "MigrationId", true},
		// A table this package does not recognise at all has no entry either
		// way; IsFrameworkMetadataColumn does not repeat IsFrameworkMetadataTable's
		// own check.
		{"users", "id", false},
		{"", "version", false},
		{"schema_migrations", "", false},
	}
	for _, c := range cases {
		if got := IsFrameworkMetadataColumn(c.table, c.column); got != c.want {
			t.Errorf("IsFrameworkMetadataColumn(%q, %q) = %v, want %v", c.table, c.column, got, c.want)
		}
	}
}

func TestArInternalMetadataEnvironmentColumns(t *testing.T) {
	t.Run("Rails' own spelling", func(t *testing.T) {
		tbl := Table{Columns: []Column{
			{Name: "key", TypeName: "character varying"},
			{Name: "value", TypeName: "character varying"},
			{Name: "created_at", TypeName: "timestamp"},
		}}
		keyCol, valueCol, ok := ArInternalMetadataEnvironmentColumns(tbl)
		if !ok || keyCol != "key" || valueCol != "value" {
			t.Errorf("got (%q, %q, %v), want (\"key\", \"value\", true)", keyCol, valueCol, ok)
		}
	})
	t.Run("case-insensitive", func(t *testing.T) {
		tbl := Table{Columns: []Column{
			{Name: "Key", TypeName: "text"},
			{Name: "Value", TypeName: "text"},
		}}
		keyCol, valueCol, ok := ArInternalMetadataEnvironmentColumns(tbl)
		if !ok || keyCol != "Key" || valueCol != "Value" {
			t.Errorf("got (%q, %q, %v), want the catalogue's own spelling preserved", keyCol, valueCol, ok)
		}
	})
	t.Run("a same-named table of a different shape", func(t *testing.T) {
		tbl := Table{Columns: []Column{
			{Name: "id", TypeName: "bigint"},
			{Name: "name", TypeName: "text"},
		}}
		if _, _, ok := ArInternalMetadataEnvironmentColumns(tbl); ok {
			t.Error("ok = true, want false: neither key nor value is present")
		}
	})
	t.Run("only one of the two columns", func(t *testing.T) {
		tbl := Table{Columns: []Column{
			{Name: "key", TypeName: "text"},
		}}
		if _, _, ok := ArInternalMetadataEnvironmentColumns(tbl); ok {
			t.Error("ok = true, want false: value is missing")
		}
	})
}
