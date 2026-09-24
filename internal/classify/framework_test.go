// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0314: dogfood session 1 found schema_migrations.version masked (its
// digit strings pass the Luhn check) and ar_internal_metadata copied
// verbatim, environment row and all. This file pins internal/classify's
// half: a framework metadata table is never masked, whatever its columns'
// own signals say, and stays that way through every later pass.

// TestFrameworkMetadataTableNeverMasked builds a real bookkeeping column
// (schema_migrations.version, on pipeline.IsFrameworkMetadataColumn's own
// allowlist for this table) whose values would mask under the classifier's
// ordinary value-signal rules — they pass the national_id checksum, the exact
// dogfood session 1 shape (internal/classify/CLAUDE.md's own T-0314 entry) —
// and asserts the table wins: NeverMasked, unmasked, and the reason names
// why. It is deliberately the *real* column name, not merely a column that
// happens to sit on a recognised table: since the T-0314 review round's
// second finding, the exemption is scoped to the table's own bookkeeping
// allowlist, not to every column of it, and a column outside that allowlist
// is exactly what TestFrameworkMetadataTableColumnNotOnAllowlistIsMasked
// pins instead.
func TestFrameworkMetadataTableNeverMasked(t *testing.T) {
	t.Parallel()
	migrations := ref.TableRef{Schema: "public", Name: "schema_migrations"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "schema_migrations", []string{"version"},
				tc("version", "text"),
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: migrations, Column: "version"}: anyOf(
			"078-05-1120", "219-09-9999", "457-55-5462"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: migrations, Column: "version"}]
	if !d.NeverMasked {
		t.Errorf("schema_migrations.version NeverMasked = false, want true: a framework metadata "+
			"table's own bookkeeping column is never masked regardless of what its values look like (%+v)", d)
	}
	if d.Masked {
		t.Errorf("schema_migrations.version Masked = true, want false: %+v", d)
	}
	if !strings.Contains(d.Reason, "framework metadata table") {
		t.Errorf("schema_migrations.version reason = %q, want it to name the table as framework "+
			"metadata rather than only report a suppressed hit", d.Reason)
	}
}

// TestFrameworkMetadataTableIsCaseInsensitive pins
// pipeline.IsFrameworkMetadataTable's own case rule against the one table on
// T-0314's list whose real catalogue spelling is mixed case
// (__EFMigrationsHistory).
func TestFrameworkMetadataTableIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	table := ref.TableRef{Schema: "public", Name: "__EFMigrationsHistory"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "__EFMigrationsHistory", []string{"MigrationId"},
				tc("MigrationId", "text"),
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: table, Column: "MigrationId"}: anyOf("a@fixture.test", "b@fixture.test"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: table, Column: "MigrationId"}]
	if d.Masked || !d.NeverMasked {
		t.Errorf("__EFMigrationsHistory.MigrationId = %+v, want NeverMasked and unmasked: the catalogue "+
			"spells this table mixed-case and the match must not depend on it", d)
	}
}

// TestFrameworkMetadataTableFKChildIsMaskedWhenParentMasks is the T-0314
// review round's finding 2, and it inverts what this test used to assert.
// pipeline.IsFrameworkMetadataTable matches a bare table name in any schema,
// so the exemption's premise — "bookkeeping the framework itself wrote and
// reads back, never end-user data" — is not guaranteed for a same-named
// application table; and a *real* migration table never carries a foreign key
// at all (dogfood session 1's own shape), so treating a framework-metadata
// column exactly like an ordinary surrogate key here — lifted when its
// validated FK parent turns out to be masked — never costs a genuine
// framework table anything. Before this round, propagateKeys' unconditional
// cw.neverMask = false was instead stopped dead by cw.frameworkMetadata, which
// left the child's real SSNs in cleartext (THREAT_MODEL.md T1) and broke the
// join to the now-masked parent (T8) — this is the regression test for that
// leak. The shape is contrived — no real schema_migrations table carries a
// foreign key — but the guard it tests is general, not specific to any one
// framework's actual DDL.
func TestFrameworkMetadataTableFKChildIsMaskedWhenParentMasks(t *testing.T) {
	t.Parallel()
	people := ref.TableRef{Schema: "public", Name: "people"}
	migrations := ref.TableRef{Schema: "public", Name: "schema_migrations"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "people", []string{"ssn"},
				tc("ssn", "text"), // masks to national_id on its values alone
			),
			tt("public", "schema_migrations", []string{"version"},
				tc("version", "text"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("schema_migrations_version_fkey", migrations, []string{"version"}, people, []string{"ssn"}),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: people, Column: "ssn"}: anyOf(
			"078-05-1120", "219-09-9999", "457-55-5462"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	parent := cls.Decisions[ref.ColumnRef{Table: people, Column: "ssn"}]
	if !parent.Masked {
		t.Fatalf("people.ssn Masked = false, want true: the test proves nothing about propagation "+
			"unless the parent actually masks (%+v)", parent)
	}
	child := cls.Decisions[ref.ColumnRef{Table: migrations, Column: "version"}]
	if !child.Masked {
		t.Errorf("schema_migrations.version Masked = false, want true: FK propagation must lift a "+
			"framework metadata table's exemption when its validated parent masks, the same as an "+
			"ordinary surrogate key (%+v)", child)
	}
	if child.NeverMasked {
		t.Errorf("schema_migrations.version NeverMasked = true, want false: %+v", child)
	}
	if child.Category != parent.Category {
		t.Errorf("schema_migrations.version Category = %v, want the parent's %v: two categories across "+
			"one edge is two maskers over one set of values", child.Category, parent.Category)
	}
	if strings.Contains(child.Reason, "framework metadata table") {
		t.Errorf("schema_migrations.version reason = %q, want the framework-metadata fragment blanked "+
			"once propagation has overridden it, the same as a lifted surrogate-key fragment", child.Reason)
	}
}

// TestFrameworkMetadataTableIsMaskedByExplicitYmlPattern is the T-0314 review
// round's finding 3: a committed lazyslice.yml pattern naming this exact
// column is more specific than pipeline.IsFrameworkMetadataTable's bare-name
// match, and ADR-004 lets a file only tighten — silently ignoring the raise
// would read a column back unmasked that an earlier run, or an operator who
// knows this particular table isn't what the name suggests, recorded as
// masked.
func TestFrameworkMetadataTableIsMaskedByExplicitYmlPattern(t *testing.T) {
	t.Parallel()
	migrations := ref.TableRef{Schema: "public", Name: "schema_migrations"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "schema_migrations", []string{"version"},
				tc("version", "text"),
			),
		},
	}
	samples := mapSampler{
		// Plain, non-matching values: the column's own signals decide CatNone
		// and ConfNone, so a confidence bump to certain can only be the yml
		// pattern's doing, and the test proves the raise itself rather than a
		// value signal that would have masked the column either way.
		ref.ColumnRef{Table: migrations, Column: "version"}: anyOf("mig-alpha", "mig-beta"),
	}
	prior := &pipeline.Config{
		ExtraPatterns: []pipeline.Pattern{
			{Name: `\Aversion\z`, Category: pipeline.CatEmail, Confidence: pipeline.ConfCertain},
		},
	}
	cls, err := New().Classify(schema, samples, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: migrations, Column: "version"}]
	if !d.Masked || d.NeverMasked {
		t.Errorf("schema_migrations.version = %+v, want Masked and not NeverMasked: an explicit "+
			"lazyslice.yml pattern names this column more specifically than the framework-metadata "+
			"exemption's bare table-name match", d)
	}
	if d.Category != pipeline.CatEmail {
		t.Errorf("schema_migrations.version Category = %v, want %v", d.Category, pipeline.CatEmail)
	}
}

// TestFrameworkMetadataTableBookkeepingColumnStaysExempt is the T-0314 review
// round's second finding's other half: databasechangelog.comments is real
// Liquibase bookkeeping (on pipeline.IsFrameworkMetadataColumn's own
// allowlist for this table) and its name also matches rules.yml's ordinary
// free_text pattern — proving the allowlist, not merely "no rule fired", is
// what keeps it unmasked.
func TestFrameworkMetadataTableBookkeepingColumnStaysExempt(t *testing.T) {
	t.Parallel()
	table := ref.TableRef{Schema: "public", Name: "databasechangelog"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "databasechangelog", []string{"id"},
				tc("id", "text"),
				tc("comments", "text"),
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: table, Column: "comments"}: anyOf("initial schema", "add index"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: table, Column: "comments"}]
	if !d.NeverMasked || d.Masked {
		t.Errorf("databasechangelog.comments = %+v, want NeverMasked and unmasked: it is on the table's own "+
			"bookkeeping allowlist even though its name matches the ordinary free_text rule", d)
	}
	if !strings.Contains(d.Reason, "framework metadata table") {
		t.Errorf("databasechangelog.comments reason = %q, want it to name the table as framework metadata", d.Reason)
	}
}

// TestFrameworkMetadataTableColumnNotOnAllowlistIsMasked is the T-0314 review
// round's second finding: the blanket table-name exemption used to reach
// every column, including Liquibase's AUTHOR (the developer who ran the
// changeset) and Flyway's INSTALLED_BY (the role or user who applied it) —
// neither is bookkeeping the tool reads back, and either can hold a real
// identity. Neither is on pipeline.IsFrameworkMetadataColumn's allowlist, so
// both fall through markNeverMasked to the ordinary classifier — and an
// author column whose values are email addresses masks like any other email
// column would.
func TestFrameworkMetadataTableColumnNotOnAllowlistIsMasked(t *testing.T) {
	t.Parallel()
	table := ref.TableRef{Schema: "public", Name: "databasechangelog"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "databasechangelog", []string{"id"},
				tc("id", "text"),
				tc("author", "text"),
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: table, Column: "author"}: anyOf(
			"alex.developer@realcorp.example", "sam.developer@realcorp.example"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: table, Column: "author"}]
	if d.NeverMasked {
		t.Errorf("databasechangelog.author NeverMasked = true, want false: AUTHOR is not on the table's own "+
			"bookkeeping allowlist and must reach the ordinary classifier (%+v)", d)
	}
	if !d.Masked {
		t.Errorf("databasechangelog.author Masked = false, want true: its values are email addresses (%+v)", d)
	}
	if d.Category != pipeline.CatEmail {
		t.Errorf("databasechangelog.author Category = %v, want %v", d.Category, pipeline.CatEmail)
	}
	if strings.Contains(d.Reason, "framework metadata table") {
		t.Errorf("databasechangelog.author reason = %q, want no framework-metadata exemption fragment", d.Reason)
	}
}

// TestFrameworkMetadataTableUnaffectedByARefusedYmlRaise is the other half of
// finding 3's fix: a yml raise that ends up refused (no usable category) must
// not unmask the column's default exemption as a side effect merely because it
// was attempted.
func TestFrameworkMetadataTableUnaffectedByARefusedYmlRaise(t *testing.T) {
	t.Parallel()
	migrations := ref.TableRef{Schema: "public", Name: "schema_migrations"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "schema_migrations", []string{"version"},
				tc("version", "text"),
			),
		},
	}
	samples := mapSampler{
		// Plain, non-matching values, so the column's own signals decide
		// CatNone and the refusal below is entirely the yml raise's doing.
		ref.ColumnRef{Table: migrations, Column: "version"}: anyOf("mig-alpha", "mig-beta"),
	}
	prior := &pipeline.Config{
		// semi_structured accepts only json, jsonb and hstore (rules.yml), so
		// naming it for a text column is refused.
		ExtraPatterns: []pipeline.Pattern{
			{Name: `\Aversion\z`, Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfCertain},
		},
	}
	cls, err := New().Classify(schema, samples, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: migrations, Column: "version"}]
	if d.Masked || !d.NeverMasked {
		t.Errorf("schema_migrations.version = %+v, want unmasked and NeverMasked: a refused yml raise "+
			"must not lift the framework-metadata exemption as a side effect", d)
	}
	if !strings.Contains(d.Reason, "framework metadata table") {
		t.Errorf("schema_migrations.version reason = %q, want the framework-metadata fragment intact", d.Reason)
	}
}
