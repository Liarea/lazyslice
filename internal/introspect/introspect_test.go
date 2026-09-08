// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What is here needs no database. What a catalog query actually returns is a
// statement about a real Postgres and is asserted in
// introspect_integration_test.go against both fixtures; these are the pieces
// that are ours rather than the server's.

func TestShapesAreNamedAndDistinct(t *testing.T) {
	t.Parallel()

	seenName := map[string]bool{}
	seenSQL := map[string]bool{}
	for _, s := range Shapes() {
		if s.Name == "" || s.SQL == "" {
			t.Fatalf("shape %+v has an empty name or statement", s)
		}
		if !strings.HasPrefix(s.Name, "introspect.") {
			t.Errorf("shape %q is not named for the stage that sends it", s.Name)
		}
		if seenName[s.Name] {
			t.Errorf("shape %q is registered twice", s.Name)
		}
		if seenSQL[s.SQL] {
			t.Errorf("shape %q repeats a statement already registered", s.Name)
		}
		seenName[s.Name], seenSQL[s.SQL] = true, true
		// A backslash anywhere makes internal/pg's trace record the statement
		// as its leading keyword alone, because the elision cannot be shown to
		// have worked (internal/pg/tracer.go). No statement here needs one.
		if strings.Contains(s.SQL, `\`) {
			t.Errorf("shape %q carries a backslash", s.Name)
		}
	}
	// Every statement this package sends has a shape: the count is the sixteen
	// catalog statements, the sample, and the savepoint the sampler takes and
	// rolls back to when the source refuses one relation.
	if got, want := len(Shapes()), 19; got != want {
		t.Errorf("Shapes() has %d entries, want %d; a new statement needs a shape", got, want)
	}
}

func TestPartitionKeyColumns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		def  string
		want []string
	}{
		{"RANGE (occurred_at)", []string{"occurred_at"}},
		{"RANGE (payment_date)", []string{"payment_date"}},
		{"LIST (tenant_id)", []string{"tenant_id"}},
		{"HASH (a, b)", []string{"a", "b"}},
		// An expression member: partattrs carries a zero for it, so reading the
		// catalog's own deparsed text is what keeps it out of nowhere.
		{"LIST ((a + b), c)", []string{"(a + b)", "c"}},
		// A quoted identifier may contain the separator.
		{`RANGE ("Odd, Name")`, []string{`"Odd, Name"`}},
		{"", nil},
		{"RANGE", nil},
	}
	for _, c := range cases {
		got := partitionKeyColumns(c.def)
		if len(got) != len(c.want) {
			t.Errorf("partitionKeyColumns(%q) = %q, want %q", c.def, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("partitionKeyColumns(%q) = %q, want %q", c.def, got, c.want)
				break
			}
		}
	}
}

func TestSamplePercentIsAFractionOfPages(t *testing.T) {
	t.Parallel()

	cases := []struct {
		what          string
		approx, pages int64
		num, den      int64
	}{
		// Nothing analysed: reltuples was -1 and relpages 0, which is a freshly
		// restored dump. There is nothing to derive a fraction from, so it asks
		// for the lot and the statement's LIMIT is the bound.
		{"unanalysed", 0, 0, 100, 1},
		{"negative, defensive", -1, -1, 100, 1},
		{"nasty.sql's people", 5, 1, 100, 1},
		// 100 rows a page: six pages hold the oversampled target of 600.
		{"two million rows", 2_000_000, 20_000, 600, 20_000},
		// One row a page, so 600 pages are wanted rather than six: the row
		// fraction and the page fraction are the same number only when the row
		// is narrow, and the page count is what is actually read.
		{"one row a page", 600_000, 600_000, 60_000, 600_000},
		{"pagila's rental", 16_049, 200, 800, 200},
		// relpages known and reltuples not: a page is assumed to hold one row,
		// which asks for the most pages the LIMIT will pay for.
		{"pages without rows", 0, 10_000, 60_000, 10_000},
	}
	for _, c := range cases {
		num, den := samplePercent(c.approx, c.pages)
		if num != c.num || den != c.den {
			t.Errorf("samplePercent(%d, %d) = %d/%d, want %d/%d (%s)",
				c.approx, c.pages, num, den, c.num, c.den, c.what)
		}
		if num > 100*den {
			t.Errorf("samplePercent(%d, %d) = %d/%d, which is over 100%% (%s)",
				c.approx, c.pages, num, den, c.what)
		}
	}

	// The fraction must still cover the rows the classifier wants: 600/20000 is
	// 0.03% of two million rows, which is about 600 of them.
	num, den := samplePercent(2_000_000, 20_000)
	if rows := 2_000_000 * (float64(num) / float64(den)) / 100; rows < SampleRows {
		t.Errorf("a two-million-row table would be sampled for %.0f rows, fewer than the %d wanted", rows, SampleRows)
	}
	// And it must not read the table to get them: 0.03% of 20,000 pages is six.
	if pages := 20_000 * (float64(num) / float64(den)) / 100; pages > sampleLimit {
		t.Errorf("a two-million-row table would be read for %.0f pages", pages)
	}
}

func TestQuoteIdent(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"people":         `"people"`,
		"LegacyCustomer": `"LegacyCustomer"`,
		`we"ird`:         `"we""ird"`,
		"bıgınt":         `"bıgınt"`,
	}
	for in, want := range cases {
		if got := quoteIdent(in); got != want {
			t.Errorf("quoteIdent(%q) = %s, want %s", in, got, want)
		}
	}
	want := `"public"."LegacyCustomer"`
	if got := quoteTable(ref.TableRef{Schema: "public", Name: "LegacyCustomer"}); got != want {
		t.Errorf("quoteTable = %s, want %s", got, want)
	}
}

func TestColumnFingerprintSeparatesItsFields(t *testing.T) {
	t.Parallel()

	base := pipeline.Column{TypeOID: 25, TypMod: -1, Nullable: true, Domain: ""}
	fp := columnFingerprint(base)
	if len(fp) != 8 {
		t.Fatalf("columnFingerprint = %q, want eight hex characters", fp)
	}
	if columnFingerprint(base) != fp {
		t.Error("columnFingerprint is not a function of its input")
	}

	changed := []pipeline.Column{
		{TypeOID: 1043, TypMod: -1, Nullable: true},
		{TypeOID: 25, TypMod: 19, Nullable: true},
		{TypeOID: 25, TypMod: -1, Nullable: false},
		{TypeOID: 25, TypMod: -1, Nullable: true, Domain: "public.year"},
	}
	for _, c := range changed {
		if columnFingerprint(c) == fp {
			t.Errorf("%+v fingerprints the same as %+v; an --unmask opt-out would survive the change", c, base)
		}
	}

	// The length prefix is what stops one field's tail from being read as the
	// next field's head: 1|2 and 12|"" must not hash alike.
	a := pipeline.Column{TypeOID: 1, TypMod: 2, Domain: ""}
	b := pipeline.Column{TypeOID: 12, TypMod: 0, Domain: ""}
	if columnFingerprint(a) == columnFingerprint(b) {
		t.Error("the encoding is ambiguous across field boundaries")
	}
}

func TestCoversLeading(t *testing.T) {
	t.Parallel()

	cases := []struct {
		index, want []string
		covered     bool
	}{
		{[]string{"tenant_id", "user_id"}, []string{"tenant_id", "user_id"}, true},
		// Order within the leading columns does not matter for an equality lookup.
		{[]string{"user_id", "tenant_id"}, []string{"tenant_id", "user_id"}, true},
		{[]string{"tenant_id", "user_id", "seen_at"}, []string{"tenant_id", "user_id"}, true},
		// A trailing position is not a leading one.
		{[]string{"seen_at", "tenant_id", "user_id"}, []string{"tenant_id", "user_id"}, false},
		{[]string{"tenant_id"}, []string{"tenant_id", "user_id"}, false},
		// An expression index reports no columns and covers nothing.
		{nil, []string{"entry_uid"}, false},
	}
	for _, c := range cases {
		if got := coversLeading(c.index, c.want); got != c.covered {
			t.Errorf("coversLeading(%q, %q) = %v, want %v", c.index, c.want, got, c.covered)
		}
	}
}

// errReader is a Reader whose every statement fails, which is what a refused
// statement looks like to this package (internal/pg/source.go turns a shape
// violation back into an error at the call).
type errReader struct{ err error }

func (e errReader) Query(context.Context, string, ...any) (pipeline.Rows, error) {
	return nil, e.err
}
func (errReader) Close(context.Context) error { return nil }

// sampleReader answers the sample statement: one row per table, except for the
// tables named in denied, which fail the way pgx surfaces a server error in
// QueryExecModeExec — on Rows.Err, not on Query.
type sampleReader struct {
	denied map[string]error
	sent   []string
}

func (s *sampleReader) Query(_ context.Context, sql string, _ ...any) (pipeline.Rows, error) {
	s.sent = append(s.sent, sql)
	for name, err := range s.denied {
		if strings.Contains(sql, `"`+name+`"`) {
			return &sampleRows{err: err}, nil
		}
	}
	return &sampleRows{remaining: 1}, nil
}

func (s *sampleReader) Close(context.Context) error { return nil }

type sampleRows struct {
	remaining int
	err       error
}

func (r *sampleRows) Next() bool {
	if r.remaining <= 0 {
		return false
	}
	r.remaining--
	return true
}

func (r *sampleRows) Scan(dest ...any) error {
	for i := range dest {
		if p, ok := dest[i].(*any); ok {
			*p = i
		}
	}
	return nil
}

func (r *sampleRows) Err() error { return r.err }
func (r *sampleRows) Close()     {}

// A source role that lacks SELECT on one relation is answered at plan
// (ARCHITECTURE.md §3.6): an unreadable child-only table is dropped to
// SchemaOnly and the run continues, an unreadable parent or the root stops with
// exit 12 naming the GRANT. Neither outcome is reachable if introspect fails on
// the sample first, so the table is left without samples instead.
func TestASampleTheSourceRefusesIsNotARunFailure(t *testing.T) {
	t.Parallel()

	denied := ref.TableRef{Schema: "public", Name: "people"}
	readable := ref.TableRef{Schema: "public", Name: "orders"}
	c := &catalog{
		schema: &pipeline.Schema{},
		order:  []ref.TableRef{denied, readable},
		tables: map[ref.TableRef]*pipeline.Table{
			denied:   {Ref: denied, Columns: []pipeline.Column{{Name: "person_id"}}},
			readable: {Ref: readable, Columns: []pipeline.Column{{Name: "order_id"}}},
		},
		pages: map[ref.TableRef]int64{},
	}
	r := &sampleReader{denied: map[string]error{
		"people": &pgconn.PgError{Code: "42501", Message: "permission denied for table people"},
	}}
	if err := c.readSamples(context.Background(), r); err != nil {
		t.Fatalf("readSamples = %v, want the run to continue past a table the role cannot read", err)
	}
	if got := c.tables[denied].Samples; got != nil {
		t.Errorf("the unreadable table has %d sample rows, want none", len(got))
	}
	if got := len(c.tables[readable].Samples); got != 1 {
		t.Errorf("the readable table has %d sample rows, want 1; the run stopped at the refusal", got)
	}
}

// Every other error stays fatal: a sample statement that is systematically
// broken would otherwise classify the whole database on names and types alone
// and say nothing about it (THREAT_MODEL.md T1).
func TestASampleThatFailsForAnyOtherReasonStopsTheRun(t *testing.T) {
	t.Parallel()

	people := ref.TableRef{Schema: "public", Name: "people"}
	c := &catalog{
		schema: &pipeline.Schema{},
		order:  []ref.TableRef{people},
		tables: map[ref.TableRef]*pipeline.Table{
			people: {Ref: people, Columns: []pipeline.Column{{Name: "person_id"}}},
		},
		pages: map[ref.TableRef]int64{},
	}
	sentinel := &pgconn.PgError{Code: "42601", Message: "syntax error"}
	r := &sampleReader{denied: map[string]error{"people": sentinel}}
	err := c.readSamples(context.Background(), r)
	if !errors.Is(err, sentinel) {
		t.Fatalf("readSamples = %v, want it to wrap the source's error", err)
	}
	if !strings.Contains(err.Error(), "public.people") {
		t.Errorf("readSamples error = %q, want it to name the table", err)
	}
}

func TestIntrospectFailsOnASourceError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("the source refused a statement")
	schema, err := New().Introspect(context.Background(), errReader{err: sentinel})
	if schema != nil {
		t.Error("Introspect returned a schema alongside an error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Introspect error = %v, want it to wrap the source's", err)
	}
	if !strings.Contains(err.Error(), "the server version") {
		t.Errorf("Introspect error = %q, want it to name the statement that failed", err)
	}
}

// A foreign key declared on a leaf partition, and one referencing a leaf, both
// have conparentid = 0 (verified on postgres:16), so neither is one of the
// copies Postgres clones onto every partition: they are constraints somebody
// wrote, over columns that live on the root. §11.1 recreates the root as one
// plain table and no leaf, and §3.3 says nothing addresses a leaf by name, so
// the edge is moved to the root rather than dropped — dropping it lost a real
// dependency of the root's data with nothing printed.
//
// The parent end moves only onto a key the root actually carries. A leaf may
// carry a unique key the root cannot — a partitioned table's unique constraint
// must include the partition key and a leaf's need not — and §11.1 recreates no
// leaf constraint or index, so an edge referencing one is marked
// NotRecreatable and left on the leaf for the planner to refuse (exit 13),
// rather than re-pointed into an ADD CONSTRAINT that fails after every user
// table in the target has been dropped.
//
// A key here is a primary key, a unique constraint, or a bare unique index —
// Postgres accepts all three as a referenced key — and never a partial index, an
// expression index, or a deferrable key, all three of which it refuses. Six
// edges cover the five answers so that no one of them alone decides the test:
// hasKeyOver reduced to a test of Table.PK, or to a test of a constraint's
// backing index, or one that dropped the partial guard, or one that dropped the
// indimmediate guard, each fails a case below.
func TestAPartitionLocalForeignKeyMovesToTheRoot(t *testing.T) {
	t.Parallel()

	var (
		root  = ref.TableRef{Schema: "public", Name: "ev"}
		leaf1 = ref.TableRef{Schema: "public", Name: "ev_2024"}
		leaf2 = ref.TableRef{Schema: "public", Name: "ev_2025"}
		dev   = ref.TableRef{Schema: "public", Name: "dev"}
		usr   = ref.TableRef{Schema: "public", Name: "usr"}
		other = ref.TableRef{Schema: "public", Name: "other"}
	)
	c := &catalog{
		schema: &pipeline.Schema{FKs: []pipeline.ForeignKey{
			// In the order sqlForeignKeys returns them: by constraint name.
			{Name: "ev24_dev_fk", Child: leaf1, ChildCols: []string{"dev_id"}, Parent: dev, ParentCols: []string{"id"}},
			{Name: "ev25_dev_fk", Child: leaf2, ChildCols: []string{"dev_id"}, Parent: dev, ParentCols: []string{"id"}},
			{Name: "ev_owner_fk", Child: root, ChildCols: []string{"owner_id"}, Parent: usr, ParentCols: []string{"id"}},
			// The root is partitioned by at, so PRIMARY KEY (id, at) is the
			// narrowest key it can carry: this edge references exactly it.
			{Name: "other_ev24_fk", Child: other, ChildCols: []string{"ev_id", "ev_at"}, Parent: leaf1, ParentCols: []string{"id", "at"}},
			// A bare `CREATE UNIQUE INDEX ev_code_at_uidx ON ev (code, at)`, with
			// no constraint of that name: Postgres takes it as a referenced key
			// and §11.1 item 6 creates it before any foreign key.
			{Name: "other_ev24_bare_fk", Child: other, ChildCols: []string{"ev_code", "ev_at"}, Parent: leaf1, ParentCols: []string{"code", "at"}},
			// The root's UNIQUE (kind, at) is DEFERRABLE, which Postgres refuses
			// as a referenced key: "cannot use a deferrable unique constraint for
			// referenced table". Its backing index is unique, non-partial and
			// non-expression like any other, so only indimmediate tells them
			// apart. The source accepts this edge because the leaf carries its
			// own non-deferrable UNIQUE (kind, at).
			{Name: "other_ev24_defer_fk", Child: other, ChildCols: []string{"ev_kind", "ev_at"}, Parent: leaf1, ParentCols: []string{"kind", "at"}},
			// And this one references `ev_2024_id_key UNIQUE (id)`, which the
			// leaf may declare and the root may not.
			{Name: "other_ev24_local_fk", Child: other, ChildCols: []string{"ev_id"}, Parent: leaf1, ParentCols: []string{"id"}},
			// The root's index over (tag, at) is partial, which Postgres refuses
			// as a referenced key however unique it is.
			{Name: "other_ev24_partial_fk", Child: other, ChildCols: []string{"ev_tag", "ev_at"}, Parent: leaf1, ParentCols: []string{"tag", "at"}},
			// The root's UNIQUE (owner_id, at) — a constraint, not the primary
			// key, so Table.PK alone cannot answer this one.
			{Name: "other_ev24_uq_fk", Child: other, ChildCols: []string{"ev_owner", "ev_at"}, Parent: leaf1, ParentCols: []string{"owner_id", "at"}},
		}},
		tables: map[ref.TableRef]*pipeline.Table{
			root: {
				Ref: root, Partitioned: true, Partitions: []ref.TableRef{leaf1, leaf2},
				PartitionKey: []string{"at"},
				PK:           []string{"id", "at"},
				Constraints: []pipeline.Constraint{
					{Name: "ev_pkey", Kind: 'p', Def: "PRIMARY KEY (id, at)"},
					{Name: "ev_owner_uq", Kind: 'u', Def: "UNIQUE (owner_id, at)"},
					{Name: "ev_defer_uq", Kind: 'u', Def: "UNIQUE (kind, at) DEFERRABLE"},
				},
				Indexes: []pipeline.Index{
					{Name: "ev_pkey", Columns: []string{"id", "at"}, Unique: true, Immediate: true},
					{Name: "ev_owner_uq", Columns: []string{"owner_id", "at"}, Unique: true, Immediate: true},
					{Name: "ev_code_at_uidx", Columns: []string{"code", "at"}, Unique: true, Immediate: true},
					{Name: "ev_tag_at_uidx", Columns: []string{"tag", "at"}, Unique: true, Partial: true, Immediate: true},
					// The index behind the DEFERRABLE constraint: everything a
					// usable key is, but indimmediate false.
					{Name: "ev_defer_uq", Columns: []string{"kind", "at"}, Unique: true},
				},
			},
			leaf1: {
				Ref: leaf1, Parent: &root,
				Constraints: []pipeline.Constraint{
					{Name: "ev_2024_id_key", Kind: 'u', Def: "UNIQUE (id)"},
					{Name: "ev_2024_kind_key", Kind: 'u', Def: "UNIQUE (kind, at)"},
				},
				Indexes: []pipeline.Index{
					{Name: "ev_2024_id_key", Columns: []string{"id"}, Unique: true, Immediate: true},
					{Name: "ev_2024_kind_key", Columns: []string{"kind", "at"}, Unique: true, Immediate: true},
				},
			},
			leaf2: {Ref: leaf2, Parent: &root},
			dev:   {Ref: dev},
			usr:   {Ref: usr},
			other: {Ref: other},
		},
	}
	c.repointPartitionForeignKeys()

	// The two leaves declare the same constraint over the same columns, so on
	// the root they are one edge and one ALTER TABLE, not two.
	want := []pipeline.ForeignKey{
		{Name: "ev24_dev_fk", Child: root, ChildCols: []string{"dev_id"}, Parent: dev, ParentCols: []string{"id"}},
		{Name: "ev_owner_fk", Child: root, ChildCols: []string{"owner_id"}, Parent: usr, ParentCols: []string{"id"}},
		{Name: "other_ev24_bare_fk", Child: other, ChildCols: []string{"ev_code", "ev_at"}, Parent: root, ParentCols: []string{"code", "at"}},
		// Not re-pointed: the root's only key over (kind, at) is deferrable, and
		// Postgres will not reference one however unique it is.
		{Name: "other_ev24_defer_fk", Child: other, ChildCols: []string{"ev_kind", "ev_at"}, Parent: leaf1, ParentCols: []string{"kind", "at"}, NotRecreatable: true},
		{Name: "other_ev24_fk", Child: other, ChildCols: []string{"ev_id", "ev_at"}, Parent: root, ParentCols: []string{"id", "at"}},
		// Not re-pointed: the root has no unique key over (id) to reference.
		{Name: "other_ev24_local_fk", Child: other, ChildCols: []string{"ev_id"}, Parent: leaf1, ParentCols: []string{"id"}, NotRecreatable: true},
		// Nor over (tag, at): the only index that covers them is partial, so
		// Postgres would refuse the ADD CONSTRAINT.
		{Name: "other_ev24_partial_fk", Child: other, ChildCols: []string{"ev_tag", "ev_at"}, Parent: leaf1, ParentCols: []string{"tag", "at"}, NotRecreatable: true},
		{Name: "other_ev24_uq_fk", Child: other, ChildCols: []string{"ev_owner", "ev_at"}, Parent: root, ParentCols: []string{"owner_id", "at"}},
	}
	if len(c.schema.FKs) != len(want) {
		t.Fatalf("%d edges after re-pointing, want %d: %+v", len(c.schema.FKs), len(want), c.schema.FKs)
	}
	for i, w := range want {
		got := c.schema.FKs[i]
		if got.Name != w.Name || got.Child != w.Child || got.Parent != w.Parent {
			t.Errorf("edge %d = %q %s -> %s, want %q %s -> %s",
				i, got.Name, got.Child, got.Parent, w.Name, w.Child, w.Parent)
		}
		if strings.Join(got.ChildCols, ",") != strings.Join(w.ChildCols, ",") ||
			strings.Join(got.ParentCols, ",") != strings.Join(w.ParentCols, ",") {
			t.Errorf("edge %q carries %q -> %q, want %q -> %q",
				got.Name, got.ChildCols, got.ParentCols, w.ChildCols, w.ParentCols)
		}
		if got.NotRecreatable != w.NotRecreatable {
			t.Errorf("edge %q NotRecreatable = %v, want %v; §11.1 item 6 %s",
				got.Name, got.NotRecreatable, w.NotRecreatable,
				map[bool]string{true: "cannot recreate it and the planner must refuse at plan",
					false: "recreates it against the root"}[w.NotRecreatable])
		}
	}
}

// An ordinary table that genuinely declares the same foreign key twice keeps
// both, because a target lazyslice wrote reads back what was hashed: only an
// edge that moved is ever collapsed into one it duplicates.
func TestARepeatedEdgeBetweenPlainTablesIsKept(t *testing.T) {
	t.Parallel()

	child := ref.TableRef{Schema: "public", Name: "film"}
	parent := ref.TableRef{Schema: "public", Name: "language"}
	c := &catalog{
		schema: &pipeline.Schema{FKs: []pipeline.ForeignKey{
			{Name: "film_language_fk", Child: child, ChildCols: []string{"language_id"}, Parent: parent, ParentCols: []string{"language_id"}},
			{Name: "film_language_fk2", Child: child, ChildCols: []string{"language_id"}, Parent: parent, ParentCols: []string{"language_id"}},
		}},
		tables: map[ref.TableRef]*pipeline.Table{child: {Ref: child}, parent: {Ref: parent}},
	}
	c.repointPartitionForeignKeys()
	if len(c.schema.FKs) != 2 {
		t.Errorf("%d edges, want both; nothing moved, so nothing may be collapsed", len(c.schema.FKs))
	}
}

// The retry that answers stale-high statistics is bounded in pages, not only in
// rows: widening straight to 100% would be a sequential scan that stops when it
// has accumulated its LIMIT, or never on a bloated relation holding nothing,
// while the run holds the source snapshot (THREAT_MODEL.md T7).
func TestSampleRetryIsBoundedInPages(t *testing.T) {
	t.Parallel()

	// The reviewer's worked example: analysed at ten million rows over a
	// million pages, since deleted down to a few thousand live rows.
	const pages = 1_000_000
	num, den := samplePercent(10_000_000, pages)
	first := float64(pages) * float64(num) / float64(den) / 100
	num, den = sampleRetryPercent(10_000_000, pages)
	retry := float64(pages) * float64(num) / float64(den) / 100
	if retry <= first {
		t.Errorf("the retry reads %.0f pages, no more than the first sample's %.0f", retry, first)
	}
	if retry > float64(sampleWidenFactor)*first {
		t.Errorf("the retry reads %.0f pages, over %d times the first sample's %.0f",
			retry, sampleWidenFactor, first)
	}
	if num == 100 && den == 1 {
		t.Error("the retry asks for the whole of a million-page relation")
	}

	// A relation small enough to be worth reading whole still is: ten pages is
	// inside the budget, so the retry asks for all of it.
	if num, den := sampleRetryPercent(1_000_000, 10); num != 100 || den != 1 {
		t.Errorf("sampleRetryPercent(1000000, 10) = %d/%d, want 100/1", num, den)
	}
	// And where the first fraction was already the whole relation there is no
	// second read to make: sampleTable compares the two and stops.
	if num, den := sampleRetryPercent(0, 0); num != 100 || den != 1 {
		t.Errorf("sampleRetryPercent(0, 0) = %d/%d, want 100/1", num, den)
	}
}

// catalogReader answers the catalog read with one table of one column and
// records every statement it is handed. Every other statement returns no rows,
// which is a database holding that one table and nothing else.
type catalogReader struct{ sent []string }

func (c *catalogReader) Query(_ context.Context, sql string, _ ...any) (pipeline.Rows, error) {
	c.sent = append(c.sent, sql)
	switch {
	case sql == sqlTables:
		return &fixedRows{rows: [][]any{{
			"public", "people", "r", false, false, false, int64(0), int64(0), "", 0, true,
		}}}, nil
	case sql == sqlColumns:
		return &fixedRows{rows: [][]any{{
			"public", "people", "person_id", "integer", int64(23), int32(-1), "", false, "", "", "", "",
		}}}, nil
	case strings.Contains(sql, "TABLESAMPLE"):
		return &fixedRows{rows: [][]any{{"a production value"}}}, nil
	}
	return &fixedRows{}, nil
}

func (c *catalogReader) Close(context.Context) error { return nil }

// sampled reports the statements only the sampler sends: the TABLESAMPLE
// statements themselves and the savepoint it takes around them.
func (c *catalogReader) sampled() []string {
	var out []string
	for _, sql := range c.sent {
		if strings.Contains(sql, "TABLESAMPLE") || strings.Contains(sql, "SAVEPOINT") {
			out = append(out, sql)
		}
	}
	return out
}

func (c *catalogReader) unsampled() []string {
	var out []string
	for _, sql := range c.sent {
		if !strings.Contains(sql, "TABLESAMPLE") && !strings.Contains(sql, "SAVEPOINT") {
			out = append(out, sql)
		}
	}
	return out
}

// fixedRows scans a fixed table of values into whatever destinations the caller
// passes, the way a driver converts a column to the field it is scanned into.
type fixedRows struct {
	rows [][]any
	cur  int
	err  error
}

func (r *fixedRows) Next() bool {
	if r.cur >= len(r.rows) {
		return false
	}
	r.cur++
	return true
}

func (r *fixedRows) Scan(dest ...any) error {
	row := r.rows[r.cur-1]
	if len(dest) != len(row) {
		return fmt.Errorf("the statement scanned %d values into %d destinations", len(row), len(dest))
	}
	for i := range dest {
		d := reflect.ValueOf(dest[i]).Elem()
		v := reflect.ValueOf(row[i])
		switch {
		case v.Type().AssignableTo(d.Type()):
			d.Set(v)
		case d.Kind() != reflect.String && v.Type().ConvertibleTo(d.Type()):
			d.Set(v.Convert(d.Type()))
		default:
			return fmt.Errorf("value %d is a %s and the destination is a %s", i, v.Type(), d.Type())
		}
	}
	return nil
}

func (r *fixedRows) Err() error { return r.err }
func (r *fixedRows) Close()     {}

// IntrospectSchema is Introspect with the sampler left out and nothing else
// changed. It is the read the gate's end of §11.2's binding takes
// (load.GateFingerprint), and what it buys is that no production value of a
// database the gate has not yet agreed to touch is read into memory
// (THREAT_MODEL.md T4). The gate reaches it through a type assertion on
// pipeline.SchemaOnlyIntrospector that falls back silently, so this is the test
// that fails when the sampler comes back.
func TestIntrospectSchemaIsTheSameReadWithoutTheSampler(t *testing.T) {
	t.Parallel()

	in, ok := New().(pipeline.SchemaOnlyIntrospector)
	if !ok {
		t.Fatal("New() does not satisfy pipeline.SchemaOnlyIntrospector, so the gate falls back to the sampling read")
	}

	fullReader := &catalogReader{}
	full, err := in.Introspect(context.Background(), fullReader)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	schemaOnlyReader := &catalogReader{}
	schemaOnly, err := in.IntrospectSchema(context.Background(), schemaOnlyReader)
	if err != nil {
		t.Fatalf("IntrospectSchema: %v", err)
	}

	// Without this the rest is vacuous: a read that sampled nothing either way
	// would pass every assertion below.
	if len(fullReader.sampled()) == 0 {
		t.Fatal("the full read sent no sample statement, so this test asserts nothing")
	}
	if got := schemaOnlyReader.sampled(); len(got) != 0 {
		t.Errorf("the schema-only read sent %d sample statements, want none: %v", len(got), got)
	}
	if got, want := schemaOnlyReader.unsampled(), fullReader.unsampled(); !slices.Equal(got, want) {
		t.Errorf("the schema-only read sent %d catalog statements and the full read %d; the two must be the same read", len(got), len(want))
	}

	if len(full.Tables) != 1 || len(schemaOnly.Tables) != 1 {
		t.Fatalf("the full read describes %d tables and the schema-only read %d, want 1 each", len(full.Tables), len(schemaOnly.Tables))
	}
	if len(full.Tables[0].Samples) == 0 {
		t.Fatal("the full read took no samples, so this test asserts nothing")
	}
	for i := range schemaOnly.Tables {
		if got := schemaOnly.Tables[i].Samples; got != nil {
			t.Errorf("%s came back with %d sample rows from the schema-only read", schemaOnly.Tables[i].Ref, len(got))
		}
		if got := schemaOnly.Tables[i].SampledFrom; got != nil {
			t.Errorf("%s came back with SampledFrom = %v from the schema-only read", schemaOnly.Tables[i].Ref, got)
		}
	}

	// Every other field is what Introspect returns, which is what lets the two
	// ends of §11.2's binding hash the same catalog to the same value.
	for i := range full.Tables {
		full.Tables[i].Samples = nil
		full.Tables[i].SampledFrom = nil
	}
	if !reflect.DeepEqual(full, schemaOnly) {
		t.Errorf("the two reads describe the catalog differently:\n full: %+v\n only: %+v", full, schemaOnly)
	}
	if schemaOnly.Fingerprint != "" {
		t.Errorf("IntrospectSchema filled Schema.Fingerprint with %q; internal/core fills it from load.SchemaFingerprint (ADR-009)", schemaOnly.Fingerprint)
	}
}
