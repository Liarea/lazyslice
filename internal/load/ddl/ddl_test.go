// SPDX-License-Identifier: Apache-2.0

package ddl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What the target ends up holding is a statement about a real Postgres and is
// asserted in internal/load's integration suite. What is asserted here is the
// part that is a statement about ARCHITECTURE.md §11.1: which object classes are
// emitted, in which order, and which are left out.

func tref(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

// fixture is a schema with one of everything §11.1 names: a schema, an
// extension, an enum, a domain, a sequence reached only through a default, a
// table with a default, a generated column, a collation and a primary key, and a
// second table with a foreign key onto the first.
func fixture() *pipeline.Schema {
	orders := pipeline.Table{
		Ref: tref("public", "orders"),
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "integer", Default: "nextval('public.orders_id_seq'::regclass)"},
			{Name: "code", TypeName: "text", Collation: "en_US", Nullable: true},
			{Name: "status", TypeName: "public.order_status", Nullable: true},
			{Name: "total", TypeName: "numeric(10,2)", Nullable: true},
			{Name: "label", TypeName: "text", Generated: "upper(code)", Nullable: true},
		},
		PK: []string{"id"},
		Constraints: []pipeline.Constraint{
			{Name: "orders_pkey", Kind: 'p', Def: "PRIMARY KEY (id)"},
			{Name: "orders_total_check", Kind: 'c', Def: "CHECK ((total >= (0)::numeric))"},
		},
		Indexes: []pipeline.Index{
			{Name: "orders_pkey", Unique: true, Columns: []string{"id"},
				Def: "CREATE UNIQUE INDEX orders_pkey ON public.orders USING btree (id)"},
			{Name: "orders_code_idx", Columns: []string{"code"},
				Def: "CREATE INDEX orders_code_idx ON public.orders USING btree (code)"},
		},
		Sequences: []pipeline.SequenceDef{
			{Name: "public.orders_id_seq", Start: 1, Increment: 1, Min: 1, Max: 2147483647, Cache: 1},
		},
	}
	items := pipeline.Table{
		Ref: tref("public", "order_items"),
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", Identity: "a",
				IdentitySeq: &pipeline.SequenceDef{Name: "public.order_items_id_seq", Start: 1,
					Increment: 1, Min: 1, Max: 9223372036854775807, Cache: 1, Column: "id"}},
			{Name: "order_id", TypeName: "integer"},
		},
		PK: []string{"id"},
		Constraints: []pipeline.Constraint{
			{Name: "order_items_pkey", Kind: 'p', Def: "PRIMARY KEY (id)"},
			{Name: "order_items_order_id_fkey", Kind: 'f',
				Def: "FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE"},
		},
		Sequences: []pipeline.SequenceDef{
			{Name: "public.order_items_id_seq", Column: "id", Start: 1, Increment: 1,
				Min: 1, Max: 9223372036854775807, Cache: 1},
		},
	}
	return &pipeline.Schema{
		Schemas:    []string{"public"},
		Extensions: []pipeline.Extension{{Name: "citext", Schema: "public"}},
		Enums:      map[string][]string{"public.order_status": {"new", "paid"}},
		Domains: []pipeline.NamedDef{
			{Name: "public.money_amount", Def: `CREATE DOMAIN "public"."money_amount" AS numeric(10,2)`},
		},
		Tables: []pipeline.Table{orders, items},
		FKs: []pipeline.ForeignKey{{
			Name: "order_items_order_id_fkey", Child: tref("public", "order_items"),
			ChildCols: []string{"order_id"}, Parent: tref("public", "orders"),
			ParentCols: []string{"id"}, Validated: true,
		}},
	}
}

func predata(t *testing.T, s *pipeline.Schema) []string {
	t.Helper()
	out, err := PreData(s, nil)
	if err != nil {
		t.Fatalf("PreData: %v", err)
	}
	return out
}

func postdata(t *testing.T, s *pipeline.Schema) []string {
	t.Helper()
	out, err := PostData(s, nil)
	if err != nil {
		t.Fatalf("PostData: %v", err)
	}
	return out
}

func joined(stmts []string) string { return strings.Join(stmts, ";\n") }

func indexOf(t *testing.T, all []string, want string) int {
	t.Helper()
	for i, s := range all {
		if strings.Contains(s, want) {
			return i
		}
	}
	t.Fatalf("no statement contains %q; got:\n%s", want, strings.Join(all, ";\n"))
	return -1
}

// §11.1 numbers the object classes 1 to 5 before the data and says "in this
// order". A CREATE TABLE whose column type is an enum the statement before it
// did not create is a 42704 against a target whose tables have already been
// dropped, so the order is the specification and not a preference.
func TestPreDataFollowsTheObjectClassOrder(t *testing.T) {
	pre := predata(t, fixture())
	order := []string{
		"CREATE SCHEMA IF NOT EXISTS \"public\"",
		"CREATE EXTENSION IF NOT EXISTS \"citext\"",
		"CREATE TYPE \"public\".\"order_status\" AS ENUM",
		"CREATE DOMAIN \"public\".\"money_amount\"",
		"CREATE SEQUENCE \"public\".\"orders_id_seq\"",
		"CREATE TABLE \"public\".\"orders\"",
	}
	last := -1
	for _, want := range order {
		at := indexOf(t, pre, want)
		if at <= last {
			t.Errorf("%q is at %d, which is not after the class before it (%d)", want, at, last)
		}
		last = at
	}
}

// §11.1 item 1 creates the schemas, and items 2 to 5 are what needs them. An
// extension installed into a schema of its own (CREATE SCHEMA extensions;
// CREATE EXTENSION pg_trgm SCHEMA extensions is the common convention), a
// sequence or a type outside every table's schema all name a schema that holds
// no table, and CREATE EXTENSION ... SCHEMA into a schema that does not exist is
// a 3F000 at the first pre-data statement — after the drop has already emptied
// the target.
func TestASchemaIsCreatedForEveryObjectAndNotOnlyForTables(t *testing.T) {
	s := fixture()
	s.Extensions = []pipeline.Extension{{Name: "pg_trgm", Schema: "extensions"}}
	s.Domains = append(s.Domains, pipeline.NamedDef{
		Name: "types.postcode", Def: `CREATE DOMAIN "types"."postcode" AS text`,
	})
	s.Tables[0].Sequences = append(s.Tables[0].Sequences, pipeline.SequenceDef{
		Name: "seqs.counter", Column: "id", Start: 1, Increment: 1, Min: 1, Max: 2147483647, Cache: 1,
	})

	pre := predata(t, s)
	for _, c := range []struct{ schema, used string }{
		{"extensions", `CREATE EXTENSION IF NOT EXISTS "pg_trgm" SCHEMA "extensions"`},
		{"types", `CREATE DOMAIN "types"."postcode"`},
		{"seqs", `CREATE SEQUENCE "seqs"."counter"`},
	} {
		create := indexOf(t, pre, `CREATE SCHEMA IF NOT EXISTS "`+c.schema+`"`)
		used := indexOf(t, pre, c.used)
		if create > used {
			t.Errorf("%q is created at statement %d and used at %d", c.schema, create, used)
		}
	}
}

// The identity column carries its own sequence, so §11.1 item 4 must not create
// one for it: CREATE SEQUENCE public.order_items_id_seq followed by a table
// declaring GENERATED ALWAYS AS IDENTITY is a duplicate name.
func TestAnIdentitySequenceIsNotCreatedTwice(t *testing.T) {
	pre := joined(predata(t, fixture()))
	if strings.Contains(pre, `CREATE SEQUENCE "public"."order_items_id_seq"`) {
		t.Error("the identity column's sequence is created in its own right as well as by the column")
	}
	if !strings.Contains(pre, "GENERATED ALWAYS AS IDENTITY") {
		t.Error("the identity column does not declare its identity")
	}
	if !strings.Contains(pre, `CREATE SEQUENCE "public"."orders_id_seq"`) {
		t.Error("a sequence that is not an identity column's is not created")
	}
}

func TestAColumnCarriesItsTypeCollationDefaultAndGeneration(t *testing.T) {
	pre := joined(predata(t, fixture()))
	for _, want := range []string{
		`"code" text COLLATE "en_US"`,
		`"id" integer DEFAULT nextval('public.orders_id_seq'::regclass) NOT NULL`,
		`"label" text GENERATED ALWAYS AS (upper(code)) STORED`,
		`CONSTRAINT "orders_pkey" PRIMARY KEY (id)`,
		`CONSTRAINT "orders_total_check" CHECK ((total >= (0)::numeric))`,
	} {
		if !strings.Contains(pre, want) {
			t.Errorf("the table definition does not carry %q; got:\n%s", want, pre)
		}
	}
}

// A foreign key is item 6 and never a column or table constraint: it is added
// after the data so that no edge is deferred and load order inside a cycle does
// not matter (ADR-005).
func TestForeignKeysAreNotInThePreDataAndAreValidatedInThePost(t *testing.T) {
	pre := joined(predata(t, fixture()))
	if strings.Contains(pre, "FOREIGN KEY") {
		t.Errorf("a foreign key is in the pre-data DDL:\n%s", pre)
	}
	post := postdata(t, fixture())
	add := indexOf(t, post, "ADD CONSTRAINT \"order_items_order_id_fkey\"")
	validate := indexOf(t, post, "VALIDATE CONSTRAINT \"order_items_order_id_fkey\"")
	if !strings.HasSuffix(post[add], " NOT VALID") {
		t.Errorf("the foreign key is not added NOT VALID: %s", post[add])
	}
	if validate < add {
		t.Error("the constraint is validated before it is added")
	}
	if !strings.Contains(post[add], "ON DELETE CASCADE") {
		t.Errorf("the referential action was dropped: %s", post[add])
	}
}

// An unvalidated foreign key in the source holds in the target exactly as much
// as it holds in the source, so it is added NOT VALID and never validated:
// validating it would fail the run over rows the source itself does not check.
func TestAnUnvalidatedSourceForeignKeyIsNotValidated(t *testing.T) {
	s := fixture()
	s.FKs[0].Validated = false
	s.Tables[1].Constraints[1].Def += " NOT VALID"
	post := postdata(t, s)
	for _, stmt := range post {
		if strings.Contains(stmt, "VALIDATE CONSTRAINT") {
			t.Errorf("an unvalidated source constraint is validated in the target: %s", stmt)
		}
	}
	add := post[indexOf(t, post, "ADD CONSTRAINT")]
	if strings.Count(add, "NOT VALID") != 1 {
		t.Errorf("NOT VALID is appended to a definition that already carries it: %s", add)
	}
}

// pg_get_indexdef prints CREATE UNIQUE INDEX for the index behind a primary key,
// and the CREATE TABLE has already created that constraint, so replaying it is a
// 42P07 on the constraint's own index name.
func TestTheIndexBehindAConstraintIsNotCreatedAgain(t *testing.T) {
	post := postdata(t, fixture())
	for _, stmt := range post {
		if strings.Contains(stmt, "INDEX orders_pkey") {
			t.Errorf("the primary key's own index is created a second time: %s", stmt)
		}
	}
	indexOf(t, post, "INDEX orders_code_idx")
}

// §11.1: a partitioned source table becomes one plain table holding the root's
// columns and constraints; its partitions are not recreated. pg_get_indexdef
// prints ON ONLY for an index on the root's own relation, which a plain table
// cannot be indexed with.
func TestAPartitionedTableBecomesOnePlainTable(t *testing.T) {
	root := tref("public", "payment")
	leaf := tref("public", "payment_p2022_01")
	s := &pipeline.Schema{
		Tables: []pipeline.Table{
			{
				Ref:          root,
				Partitioned:  true,
				PartitionKey: []string{"paid_at"},
				Partitions:   []ref.TableRef{leaf},
				Columns:      []pipeline.Column{{Name: "id", TypeName: "integer"}},
				Indexes: []pipeline.Index{{Name: "payment_id_idx",
					Def: "CREATE INDEX payment_id_idx ON ONLY public.payment USING btree (id)"}},
			},
			{
				Ref:     leaf,
				Parent:  &root,
				Columns: []pipeline.Column{{Name: "id", TypeName: "integer"}},
				Indexes: []pipeline.Index{{Name: "payment_p2022_01_id_idx",
					Def: "CREATE INDEX payment_p2022_01_id_idx ON public.payment_p2022_01 USING btree (id)"}},
			},
		},
	}
	pre := joined(predata(t, s))
	if strings.Contains(pre, "PARTITION") {
		t.Errorf("the partitioning was recreated:\n%s", pre)
	}
	if strings.Contains(pre, "payment_p2022_01") {
		t.Errorf("a leaf partition was recreated as a table:\n%s", pre)
	}
	post := joined(postdata(t, s))
	if strings.Contains(post, "ON ONLY") {
		t.Errorf("an index was recreated ON ONLY on a plain table:\n%s", post)
	}
	if strings.Contains(post, "payment_p2022_01_id_idx") {
		t.Errorf("a leaf partition's index was recreated:\n%s", post)
	}
}

// THREAT_MODEL.md T8 and research/HARD_PROBLEMS.md §4.1: the strict-NULL form,
// never a literal 0. pagila 3.1.0 declares no sequence ownership at all — every
// one of its sequences is reached through a column default calling nextval — so
// a setval that read SequenceDef.Column alone would reset nothing in the
// project's own friendly fixture.
func TestSetvalIsStrictNullAndFindsTheColumnThroughTheDefault(t *testing.T) {
	post := postdata(t, fixture())
	stmt := post[indexOf(t, post, "orders_id_seq")]
	want := `SELECT pg_catalog.setval('"public"."orders_id_seq"', coalesce(max("id"), 1), max("id") IS NOT NULL) ` +
		`FROM "public"."orders"`
	if stmt != want {
		t.Errorf("setval is\n  %s\nand should be\n  %s", stmt, want)
	}
	for _, s := range post {
		if strings.Contains(s, "setval") && strings.Contains(s, ", 0)") {
			t.Errorf("a sequence is reset to a literal 0: %s", s)
		}
	}
}

// setval takes a regclass, and text is cast to regclass by the rules that parse
// an identifier: an unquoted name is folded to lower case. A sequence whose name
// needs quoting — mixed case, a space, a keyword — therefore resolves to a
// relation that does not exist, and because setval is item 6 the failure lands
// after every table has been copied: a target fully loaded with its sequences
// unreset, which is the THREAT_MODEL.md T8 outcome the strict-NULL form exists
// to prevent. testdata/nasty.sql's public."LegacyCustomer"."CustomerID" is the
// fixture; pagila has no mixed-case sequence, which is why the integration suite
// is not on its own enough to catch this.
func TestSetvalQuotesASequenceNameThatNeedsQuoting(t *testing.T) {
	table := pipeline.Table{
		Ref: tref("public", "LegacyCustomer"),
		Columns: []pipeline.Column{
			{Name: "CustomerID", TypeName: "integer", Identity: "a"},
		},
		Sequences: []pipeline.SequenceDef{
			{Name: `public.LegacyCustomer_CustomerID_seq`, Column: "CustomerID",
				Start: 42, Increment: 1, Min: 1, Max: 2147483647, Cache: 1},
		},
	}
	setvals := Setvals(table)
	if len(setvals) != 1 {
		t.Fatalf("expected one setval, got %d", len(setvals))
	}
	want := `SELECT pg_catalog.setval('"public"."LegacyCustomer_CustomerID_seq"', ` +
		`coalesce(max("CustomerID"), 1), max("CustomerID") IS NOT NULL) FROM "public"."LegacyCustomer"`
	if setvals[0].SQL != want {
		t.Errorf("setval is\n  %s\nand should be\n  %s", setvals[0].SQL, want)
	}
	if setvals[0].Sequence != `public.LegacyCustomer_CustomerID_seq` {
		t.Errorf("the sequence is reported as %q, which is not the name the catalog holds", setvals[0].Sequence)
	}
}

// §11.2's binding compares the fingerprint in the marker with one recomputed
// over the target's catalog, and lazyslice writes lazyslice_meta into the target
// itself. If it entered the object classes, a target lazyslice wrote would
// fingerprint one CREATE TABLE and one ANALYZE away from its own source, no
// marker could ever bind, and §11.2's reload path would fall through to the
// emptiness check and refuse every target this package has ever written.
func TestTheMarkerTableIsNotAnObjectClass(t *testing.T) {
	s := fixture()
	marker := pipeline.Table{
		Ref:     tref("public", "lazyslice_meta"),
		Columns: []pipeline.Column{{Name: "run_id", TypeName: "uuid"}},
	}
	withMarker := fixture()
	withMarker.Tables = append(withMarker.Tables, marker)

	for _, stmt := range predata(t, withMarker) {
		if strings.Contains(stmt, "lazyslice_meta") {
			t.Errorf("the pre-data DDL recreates the marker table: %s", stmt)
		}
	}
	for _, stmt := range postdata(t, withMarker) {
		if strings.Contains(stmt, "lazyslice_meta") {
			t.Errorf("the post-data DDL touches the marker table: %s", stmt)
		}
	}

	source, err := Fingerprint(s)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	target, err := Fingerprint(withMarker)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if source != target {
		t.Errorf("a schema read back from a target we wrote fingerprints %s and its source %s, "+
			"so no marker could ever bind", target, source)
	}
}

// §11.1 recreates a partitioned source table as one plain table and none of its
// leaves, so an edge with a leaf at either end is an edge the target cannot
// carry — and must not enter the fingerprint. The catalog hash ADR-009 deleted
// counted every non-virtual foreign key, so a source holding such an edge never
// fingerprinted equal to the target lazyslice wrote from it, §11.2's marker
// never bound, and the second run was refused with exit 4.
func TestFingerprintCountsOnlyEdgesBetweenRecreatedTables(t *testing.T) {
	root := tref("public", "ev")
	leaf := tref("public", "ev_2024")
	logRef := tref("public", "log")
	logCols := []pipeline.Column{{Name: "ev_id", TypeName: "bigint"}}
	evCols := []pipeline.Column{{Name: "event_id", TypeName: "bigint"}}

	source := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: root, Partitioned: true, PartitionKey: []string{"event_id"},
				Partitions: []ref.TableRef{leaf}, Columns: evCols},
			{Ref: leaf, Parent: &root, Columns: evCols},
			{Ref: logRef, Columns: logCols, Constraints: []pipeline.Constraint{{
				Name: "log_ev_id_fkey", Kind: 'f',
				Def: "FOREIGN KEY (ev_id) REFERENCES public.ev_2024(event_id)",
			}}},
		},
		FKs: []pipeline.ForeignKey{{
			Name: "log_ev_id_fkey", Child: logRef, ChildCols: []string{"ev_id"},
			Parent: leaf, ParentCols: []string{"event_id"}, Validated: true,
		}},
	}
	for _, stmt := range postdata(t, source) {
		if strings.Contains(stmt, "log_ev_id_fkey") {
			t.Errorf("an edge onto a leaf partition was recreated: %s", stmt)
		}
	}

	// The target lazyslice wrote from that source, read back: one plain table,
	// no leaf, and no edge, because the edge was never added.
	target := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: root, Columns: evCols},
			{Ref: logRef, Columns: logCols},
		},
	}
	sourceFP, err := Fingerprint(source)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	targetFP, err := Fingerprint(target)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if sourceFP != targetFP {
		t.Errorf("a source with an edge onto a leaf partition fingerprints %s and the target "+
			"lazyslice wrote from it fingerprints %s, so §11.2's marker can never bind",
			sourceFP, targetFP)
	}

	// The counterpart, so that what is asserted above is "the leaf edge was
	// skipped" and not "foreign keys are ignored": an edge whose two ends are
	// both recreated does move the hash.
	withEdge := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: root, Columns: evCols},
			{Ref: logRef, Columns: logCols, Constraints: []pipeline.Constraint{{
				Name: "log_ev_id_fkey", Kind: 'f',
				Def: "FOREIGN KEY (ev_id) REFERENCES public.ev(event_id)",
			}}},
		},
		FKs: []pipeline.ForeignKey{{
			Name: "log_ev_id_fkey", Child: logRef, ChildCols: []string{"ev_id"},
			Parent: root, ParentCols: []string{"event_id"}, Validated: true,
		}},
	}
	edgeFP, err := Fingerprint(withEdge)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if edgeFP == targetFP {
		t.Error("an edge between two recreated tables did not change the fingerprint")
	}
}

func TestEveryRecreatedTableIsAnalysed(t *testing.T) {
	post := postdata(t, fixture())
	for _, want := range []string{`ANALYZE "public"."orders"`, `ANALYZE "public"."order_items"`} {
		indexOf(t, post, want)
	}
	analyze := indexOf(t, post, `ANALYZE "public"."orders"`)
	fk := indexOf(t, post, "ADD CONSTRAINT")
	if analyze < fk {
		t.Error("ANALYZE runs before the foreign keys are added")
	}
}

// The drop path is what makes a second run against a marked target possible at
// all: CREATE TYPE has no IF NOT EXISTS, and a sequence a previous run left
// behind is owned by no table.
func TestDropsCoverEverythingPreDataCreates(t *testing.T) {
	s := fixture()
	drops := DropTables(s, []ref.TableRef{tref("public", "left_behind")})
	if len(drops) != 3 {
		t.Fatalf("expected three drops, got %d", len(drops))
	}
	if drops[0].Table != tref("public", "orders") {
		t.Errorf("the drops are not in the reverse of the creation order: %v", drops[0].Table)
	}
	if drops[2].Table != tref("public", "left_behind") {
		t.Errorf("a table only the caller knows about was not dropped: %v", drops[2].Table)
	}
	for _, d := range drops {
		if !strings.HasPrefix(d.SQL, "DROP TABLE IF EXISTS ") || !strings.HasSuffix(d.SQL, " CASCADE") {
			t.Errorf("a drop is not an IF EXISTS ... CASCADE: %s", d.SQL)
		}
	}
	objects := strings.Join(DropObjects(s), ";\n")
	for _, want := range []string{
		`DROP SEQUENCE IF EXISTS "public"."orders_id_seq"`,
		`DROP DOMAIN IF EXISTS "public"."money_amount"`,
		`DROP TYPE IF EXISTS "public"."order_status"`,
	} {
		if !strings.Contains(objects, want) {
			t.Errorf("the drop path does not carry %q; got:\n%s", want, objects)
		}
	}
	if strings.Contains(objects, "CASCADE") {
		t.Errorf("a type or sequence is dropped CASCADE, which would take a stranger's column with it:\n%s", objects)
	}
}

// §11.1: the fingerprint is over the recreated object classes only, and is the
// same hash whether it is computed on the source or on a target we wrote.
func TestFingerprintIsStableAndMovesWithTheSchema(t *testing.T) {
	a, err := Fingerprint(fixture())
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	b, err := Fingerprint(fixture())
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if a != b {
		t.Errorf("two fingerprints of one schema differ: %s and %s", a, b)
	}
	changed := fixture()
	changed.Tables[0].Columns[1].TypeName = "citext"
	c, err := Fingerprint(changed)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if c == a {
		t.Error("a column's type changed and the fingerprint did not")
	}
	// A view is not recreated, so it must not enter the hash: otherwise a
	// target lazyslice wrote could never fingerprint equal to its source and
	// §11.2's marker would never bind.
	withView := fixture()
	withView.NotRecreated = append(withView.NotRecreated, pipeline.Object{Kind: "view", Name: "public.v"})
	d, err := Fingerprint(withView)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if d != a {
		t.Error("a view entered the fingerprint")
	}
}

func TestRecreatableAcceptsASchemaOfOnlyRecreatedObjects(t *testing.T) {
	s := fixture()
	s.NotRecreated = []pipeline.Object{
		{Kind: "view", Name: "public.order_summary"},
		{Kind: "function", Name: "public.rewards_report"},
		{Kind: "trigger", Name: "public.orders.last_updated"},
	}
	if err := Recreatable(s); err != nil {
		t.Errorf("a schema whose only user function nothing depends on was refused: %v", err)
	}
}

// §11.1: a refusal is exit 13 at plan, naming the table, the column or index,
// the dependency and its kind — before the snapshot is used for keys and before
// anything in the target is dropped.
func TestRecreatableRefusesADefaultThatCallsAFunctionWeDoNotRecreate(t *testing.T) {
	s := fixture()
	s.NotRecreated = []pipeline.Object{{Kind: "function", Name: "public.next_code"}}
	s.Tables[0].Columns[1].Default = "public.next_code()"

	var r *Refusal
	err := Recreatable(s)
	if !asRefusal(err, &r) {
		t.Fatalf("expected a *Refusal, got %v", err)
	}
	if r.Code != CodeNotRecreatableFunction || r.Exit != ExitNotRecreatable {
		t.Errorf("the refusal is %s exit %d", r.Code, r.Exit)
	}
	if r.Table != tref("public", "orders") || r.Object != "code" || r.Dependency != "public.next_code" {
		t.Errorf("the refusal names %v.%s depending on %s", r.Table, r.Object, r.Dependency)
	}
}

func TestRecreatableRefusesAnIndexExpressionAndACollation(t *testing.T) {
	s := fixture()
	s.NotRecreated = []pipeline.Object{
		{Kind: "function", Name: "public.slugify"},
		{Kind: "collation", Name: "public.numeric_aware"},
	}
	s.Tables[0].Indexes[1].Def = "CREATE INDEX orders_code_idx ON public.orders USING btree (public.slugify(code))"
	var r *Refusal
	if !asRefusal(Recreatable(s), &r) || r.Code != CodeNotRecreatableFunction || r.Object != "orders_code_idx" {
		t.Errorf("an index expression calling a user function was not refused by name: %v", Recreatable(s))
	}

	s = fixture()
	s.NotRecreated = []pipeline.Object{{Kind: "collation", Name: "public.numeric_aware"}}
	s.Tables[0].Columns[1].Collation = "numeric_aware"
	if !asRefusal(Recreatable(s), &r) || r.Code != CodeNotRecreatableCollation {
		t.Errorf("a column using a collation we do not recreate was not refused: %v", Recreatable(s))
	}
}

// A built-in the deparser wrote without a schema must not be read as a user
// function of the same bare name unless the source really declares one: the
// refusal is a run that cannot start, so it has to be about a real dependency.
func TestRecreatableDoesNotRefuseABuiltInCall(t *testing.T) {
	s := fixture()
	s.NotRecreated = []pipeline.Object{{Kind: "function", Name: "public.rewards_report"}}
	s.Tables[0].Columns[1].Default = "now()"
	if err := Recreatable(s); err != nil {
		t.Errorf("a default calling now() was refused: %v", err)
	}
	// A string literal that happens to spell a call is not a call.
	s.Tables[0].Columns[1].Default = "'rewards_report()'::text"
	if err := Recreatable(s); err != nil {
		t.Errorf("a string literal was read as a function call: %v", err)
	}
}

func asRefusal(err error, out **Refusal) bool { return errors.As(err, out) }
