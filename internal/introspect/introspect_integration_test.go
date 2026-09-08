// SPDX-License-Identifier: Apache-2.0

//go:build integration

package introspect

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// What a catalog query returns is a statement about a real Postgres, so these
// run against the two fixtures. testdata/README.md is the specification: every
// trap in it that introspection is responsible for has an assertion here, named
// for the trap it covers, so that a fixture change and a regression fail
// separately and legibly.
//
// The reader is a real internal/pg.Source with this package's shapes registered
// on its allowlist, so the suite also proves that every statement Introspect
// sends matches a registered shape: Source.Violation is checked after each run
// (THREAT_MODEL.md T9).

func introspectFixture(ctx context.Context, t *testing.T, load func(context.Context, string) error) *pipeline.Schema {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	if err := load(ctx, url); err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	analyse(ctx, t, url)

	shapes := make([]pg.Shape, 0, len(Shapes()))
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	src, err := pg.OpenSource(ctx, dsn.DSN(url), shapes...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)
	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	first := introspectOnce(ctx, t, src, id)
	second := introspectOnce(ctx, t, src, id)
	if err := src.Violation(); err != nil {
		t.Fatalf("the source allowlist refused a statement Introspect sent: %v", err)
	}
	// Two introspections of one snapshot must describe it identically (ADR-004),
	// and this is the only place that property is asserted against a real
	// catalog. ARCHITECTURE.md §11.2's marker stands on it: the fingerprint is
	// sha256 over the DDL internal/load/ddl renders from this Schema, so a field
	// whose order comes out of a Go map or an unordered catalog query makes the
	// hash differ run to run, the marker lazyslice wrote stops binding, and
	// every reload of that target is refused with exit 4 — fail-closed, and the
	// reload path dead. ddl.TestFingerprintIsStableAndMovesWithTheSchema runs
	// over a hand-built pipeline.Schema and cannot see a catalog-ordering bug at
	// all, which is why the assertion is here and not only there.
	assertSameSchema(t, first, second)
	// ADR-009: this package does not fingerprint. Schema.Fingerprint is sha256
	// over the DDL internal/load/ddl generates, so what is asserted here is that
	// Introspect leaves it alone — a value set here would be the second,
	// disagreeing definition ADR-009 removed, and §11.2's marker would bind
	// against whichever end read it. The hash's own properties are asserted
	// where it is defined (ddl.TestFingerprintIsStableAndMovesWithTheSchema) and
	// end to end in internal/load's TestLoadPagilaIntoAMarkedTarget, which
	// compares the fingerprint of a source with that of the target lazyslice
	// wrote from it.
	for _, s := range []*pipeline.Schema{first, second} {
		if s.Fingerprint != "" {
			t.Errorf("Introspect filled Schema.Fingerprint with %q; ADR-009 leaves it to the caller",
				s.Fingerprint)
		}
	}
	return first
}

// assertSameSchema fails unless two introspections of one snapshot describe it
// identically. It compares the whole *pipeline.Schema rather than a hash, so it
// is not tied to what any one fingerprint definition covers; only Table.Samples
// is excluded, because sample values are production data (THREAT_MODEL.md T4),
// never reach the DDL the fingerprint is taken over, and would be printed by a
// failure here.
func assertSameSchema(t *testing.T, first, second *pipeline.Schema) {
	t.Helper()

	a, b := withoutSamples(first), withoutSamples(second)
	if reflect.DeepEqual(a, b) {
		return
	}
	if len(a.Tables) != len(b.Tables) {
		t.Fatalf("two introspections of one snapshot returned %d and %d tables",
			len(a.Tables), len(b.Tables))
	}
	for i := range a.Tables {
		if !reflect.DeepEqual(a.Tables[i], b.Tables[i]) {
			t.Fatalf("two introspections of one snapshot describe %s differently:\n %+v\n %+v",
				a.Tables[i].Ref, a.Tables[i], b.Tables[i])
		}
	}
	a.Tables, b.Tables = nil, nil
	t.Fatalf("two introspections of one snapshot differ outside the table list:\n %+v\n %+v", *a, *b)
}

// withoutSamples is a shallow copy of s whose tables carry no sample rows.
func withoutSamples(s *pipeline.Schema) *pipeline.Schema {
	out := *s
	out.Tables = append([]pipeline.Table(nil), s.Tables...)
	for i := range out.Tables {
		out.Tables[i].Samples = nil
	}
	return &out
}

func introspectOnce(ctx context.Context, t *testing.T, src *pg.Source, id pipeline.SnapshotID) *pipeline.Schema {
	t.Helper()
	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader on the snapshot: %v", err)
	}
	defer func() { _ = r.Close(context.WithoutCancel(ctx)) }()

	schema, err := New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	return schema
}

// analyse fills pg_class.reltuples. It is a precondition of the sampler's
// "largest leaf partition by reltuples" rule, not part of what is under test:
// LoadPagila already does it, and nasty.sql analyses only stream_rows.
func analyse(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to analyse the fixture: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
		t.Fatalf("analysing the fixture: %v", err)
	}
}

// ---------- nasty.sql ----------

func TestIntrospectNasty(t *testing.T) {
	ctx := context.Background()
	// LoadNastyNotRecreatable, not testutil.LoadNasty directly: trap 25's
	// price_list_notes.list_id -> price_lists_eu (list_id) edge is what
	// Trap25_DeferrableRootKeyLeafOnlyKey below is about, and LoadNasty leaves
	// it out so that a Plan call over the default fixture does not refuse
	// unconditionally on it (testdata/README.md trap 25).
	s := introspectFixture(ctx, t, testutil.LoadNastyNotRecreatable)

	t.Run("TableList", func(t *testing.T) {
		want := []string{
			"billing.invoices",
			"public.LegacyCustomer", "public.attachments", "public.audit_log",
			"public.click_stream", "public.device_readings", "public.devices",
			"public.events", "public.events_2024", "public.events_2025",
			"public.order_items", "public.orders", "public.organisations",
			"public.people", "public.price_list_notes", "public.price_lists",
			"public.price_lists_eu", "public.price_lists_us",
			"public.projects", "public.sites", "public.stream_rows",
			"public.teams", "public.tenant_user_flags", "public.tenant_user_sessions",
			"public.tenant_users",
		}
		var got []string
		for _, tbl := range s.Tables {
			got = append(got, tbl.Ref.String())
		}
		if !sameStrings(got, want) {
			t.Errorf("tables = %q,\nwant %q", got, want)
		}
	})

	// Trap 8: a table outside public. Every table is named (schema, name) all
	// the way through and the edge carries the schema of both ends.
	t.Run("Trap8_SchemaQualified", func(t *testing.T) {
		if !contains(s.Schemas, "billing") {
			t.Errorf("Schemas = %q, want billing among them", s.Schemas)
		}
		fk := fkFrom(t, s, tref("billing", "invoices"), "person_id")
		if fk.Parent != tref("public", "people") {
			t.Errorf("billing.invoices.person_id points at %s, want public.people", fk.Parent)
		}
	})

	// Trap 1: a self-referencing foreign key.
	t.Run("Trap1_SelfReference", func(t *testing.T) {
		fk := fkFrom(t, s, tref("public", "people"), "manager_id")
		if fk.Child != fk.Parent {
			t.Errorf("people.manager_id is %s -> %s, want a self-reference", fk.Child, fk.Parent)
		}
		if !sameStrings(fk.ParentCols, []string{"person_id"}) {
			t.Errorf("people.manager_id references %q, want person_id", fk.ParentCols)
		}
	})

	// Trap 2: a three-table cycle, and trap 3: a two-table one.
	t.Run("Trap2and3_Cycles", func(t *testing.T) {
		edges := [][3]string{
			{"organisations", "primary_team_id", "teams"},
			{"teams", "lead_project_id", "projects"},
			{"projects", "owner_organisation_id", "organisations"},
			{"people", "preferred_order_id", "orders"},
			{"orders", "person_id", "people"},
		}
		for _, e := range edges {
			fk := fkFrom(t, s, tref("public", e[0]), e[1])
			if fk.Parent != tref("public", e[2]) {
				t.Errorf("%s.%s points at %s, want public.%s", e[0], e[1], fk.Parent, e[2])
			}
		}
	})

	// Trap 4: a composite primary key with a composite foreign key to it, and
	// trap 5: the same pair of columns again under MATCH FULL.
	t.Run("Trap4and5_CompositeKeysAndMatchType", func(t *testing.T) {
		users := table(t, s, "public", "tenant_users")
		if !sameStrings(users.PK, []string{"tenant_id", "user_id"}) {
			t.Errorf("tenant_users PK = %q, want [tenant_id user_id] in the declared order", users.PK)
		}
		sessions := fkFrom(t, s, tref("public", "tenant_user_sessions"), "tenant_id", "user_id")
		if !sameStrings(sessions.ParentCols, []string{"tenant_id", "user_id"}) {
			t.Errorf("the sessions edge references %q, want [tenant_id user_id]", sessions.ParentCols)
		}
		if sessions.MatchFull {
			t.Error("tenant_user_sessions' edge reports MATCH FULL; confmatchtype is 's' there")
		}
		flags := fkFrom(t, s, tref("public", "tenant_user_flags"), "tenant_id", "user_id")
		if !flags.MatchFull {
			t.Error("tenant_user_flags' edge does not report MATCH FULL; confmatchtype is 'f' there")
		}
		if flags.Name == sessions.Name {
			t.Error("two constraints over one pair of columns came back as one edge")
		}
		// tenant_user_sessions.user_id is nullable, which is what makes the
		// MATCH SIMPLE rule observable at plan.
		col := column(t, table(t, s, "public", "tenant_user_sessions"), "user_id")
		if !col.Nullable {
			t.Error("tenant_user_sessions.user_id is not nullable; trap 4's NULL-component case is gone")
		}
	})

	// Trap 6: a polymorphic pair with no constraint behind it. Introspection
	// must not invent the edge, and must still report the declared one.
	t.Run("Trap6_PolymorphicPairIsNotAnEdge", func(t *testing.T) {
		for _, fk := range s.FKs {
			if fk.Child == tref("public", "attachments") && contains(fk.ChildCols, "owner_id") {
				t.Errorf("attachments.owner_id came back as foreign key %q; Postgres knows nothing about that pair", fk.Name)
			}
			if fk.Virtual {
				t.Errorf("%s came back Virtual; introspection reads constraints and infers no edge", fk.Name)
			}
		}
		fk := fkFrom(t, s, tref("public", "attachments"), "uploaded_by_person_id")
		if fk.Parent != tref("public", "people") {
			t.Errorf("attachments.uploaded_by_person_id points at %s, want public.people", fk.Parent)
		}
		attachments := table(t, s, "public", "attachments")
		if column(t, attachments, "owner_type").TypeName == "" {
			t.Error("attachments.owner_type has no type")
		}
	})

	// Trap 7: a partitioned table with two partitions.
	t.Run("Trap7_Partitions", func(t *testing.T) {
		events := table(t, s, "public", "events")
		if !events.Partitioned {
			t.Error("public.events is not reported as partitioned")
		}
		if !sameStrings(events.PartitionKey, []string{"occurred_at"}) {
			t.Errorf("events partition key = %q, want [occurred_at]", events.PartitionKey)
		}
		want := []ref.TableRef{tref("public", "events_2024"), tref("public", "events_2025")}
		if len(events.Partitions) != len(want) {
			t.Fatalf("events partitions = %v, want %v", events.Partitions, want)
		}
		for i := range want {
			if events.Partitions[i] != want[i] {
				t.Fatalf("events partitions = %v, want %v", events.Partitions, want)
			}
		}
		if !sameStrings(events.PK, []string{"event_id", "occurred_at"}) {
			t.Errorf("events PK = %q, want [event_id occurred_at]; it has to contain the partition key", events.PK)
		}
		if events.Parent != nil {
			t.Errorf("the partitioned root reports a parent of %s", events.Parent)
		}
		for _, leaf := range want {
			l := table(t, s, leaf.Schema, leaf.Name)
			if l.Parent == nil || *l.Parent != tref("public", "events") {
				t.Errorf("%s reports parent %v, want public.events", leaf, l.Parent)
			}
			if l.Partitioned {
				t.Errorf("%s is a leaf and reports itself partitioned", leaf)
			}
			if l.SampledFrom != nil {
				t.Errorf("%s is a regular table and must sample itself, not %s", leaf, l.SampledFrom)
			}
		}
		// TABLESAMPLE is refused on a partitioned table, so the root is sampled
		// through its largest leaf by reltuples: events_2024 holds five rows and
		// events_2025 two.
		if events.SampledFrom == nil || *events.SampledFrom != tref("public", "events_2024") {
			t.Fatalf("events.SampledFrom = %v, want public.events_2024", events.SampledFrom)
		}
		if len(events.Samples) == 0 {
			t.Error("the partitioned root has no samples, so the classifier would see only its name and types")
		}
		if len(events.Samples) > 0 && len(events.Samples[0]) != len(events.Columns) {
			t.Errorf("a sample row has %d values for %d columns", len(events.Samples[0]), len(events.Columns))
		}
		// One edge for the partitioned root, not one per partition.
		var edges int
		for _, fk := range s.FKs {
			if fk.Child.Name == "events" || fk.Child.Name == "events_2024" || fk.Child.Name == "events_2025" {
				edges++
				if fk.Child != tref("public", "events") {
					t.Errorf("edge %q is declared on the leaf %s; nothing addresses a leaf by name", fk.Name, fk.Child)
				}
			}
		}
		if edges != 1 {
			t.Errorf("the events component has %d edges, want 1", edges)
		}
	})

	// Trap 25: a partitioned root's own key is DEFERRABLE, a leaf carries a
	// key the root cannot hold, and price_list_notes.list_id references the
	// leaf directly. hasKeyOver must find no key on public.price_lists over
	// (list_id) alone -- its only key is the DEFERRABLE (list_id, region),
	// which Postgres refuses as a referenced key outright, and no key lacking
	// region can ever exist on a table partitioned by it -- so the edge is
	// left pointed at the leaf and marked ForeignKey.NotRecreatable rather
	// than silently re-pointed at a root that cannot carry it.
	t.Run("Trap25_DeferrableRootKeyLeafOnlyKey", func(t *testing.T) {
		root := table(t, s, "public", "price_lists")
		if !root.Partitioned {
			t.Error("public.price_lists is not reported as partitioned")
		}
		if root.Parent != nil {
			t.Errorf("the partitioned root reports a parent of %s", root.Parent)
		}

		leaf := table(t, s, "public", "price_lists_eu")
		if leaf.Parent == nil || *leaf.Parent != tref("public", "price_lists") {
			t.Errorf("public.price_lists_eu reports parent %v, want public.price_lists", leaf.Parent)
		}

		rootKey := index(t, root, "price_lists_list_id_region_key")
		if rootKey.Immediate {
			t.Error("price_lists_list_id_region_key reports Immediate; the constraint behind it is DEFERRABLE")
		}
		leafKey := index(t, leaf, "price_lists_eu_list_id_key")
		if !leafKey.Immediate {
			t.Error("price_lists_eu_list_id_key reports not Immediate; the constraint behind it is a plain, non-deferrable UNIQUE")
		}
		if !sameStrings(leafKey.Columns, []string{"list_id"}) {
			t.Errorf("price_lists_eu_list_id_key columns = %q, want [list_id]: the root cannot hold a key missing its partition column region", leafKey.Columns)
		}

		fk := fkFrom(t, s, tref("public", "price_list_notes"), "list_id")
		if fk.Parent != tref("public", "price_lists_eu") {
			t.Errorf("price_list_notes.list_id points at %s, want public.price_lists_eu: introspect must leave an edge it cannot re-point on the leaf, never silently drop or re-point it", fk.Parent)
		}
		if !fk.NotRecreatable {
			t.Error("price_list_notes.list_id's edge reports NotRecreatable = false, want true")
		}

		// The exception to "an edge in Schema.FKs never names a partition":
		// only a NotRecreatable edge may still name price_lists_eu, and every
		// other edge in the fixture must still be re-pointed at a root or name
		// an ordinary table.
		for _, other := range s.FKs {
			if other.Name == fk.Name {
				continue
			}
			if other.Parent == tref("public", "price_lists_eu") || other.Parent == tref("public", "price_lists_us") {
				t.Errorf("%s points at a leaf partition (%s) and is not marked NotRecreatable", other.Name, other.Parent)
			}
		}
	})

	// Trap 9: a quoted, mixed-case identifier. The proof that the sample
	// statement quotes is that it returned rows rather than failing with 42P01.
	t.Run("Trap9_QuotedIdentifiers", func(t *testing.T) {
		legacy := table(t, s, "public", "LegacyCustomer")
		for _, name := range []string{"CustomerID", "MigratedFromPersonID", "EmailAddress", "ContactNumber", "MobileNumber", "Notes"} {
			column(t, legacy, name)
		}
		if len(legacy.Samples) != 3 {
			t.Errorf(`public."LegacyCustomer" returned %d sample rows, want its 3; an unquoted identifier fails here with 42P01`, len(legacy.Samples))
		}
		fk := fkFrom(t, s, tref("public", "LegacyCustomer"), "MigratedFromPersonID")
		if fk.Parent != tref("public", "people") {
			t.Errorf(`"MigratedFromPersonID" points at %s, want public.people`, fk.Parent)
		}
	})

	// Trap 10: keys that are not integers.
	t.Run("Trap10_KeyTypes", func(t *testing.T) {
		sites := table(t, s, "public", "sites")
		if !sameStrings(sites.PK, []string{"site_code"}) {
			t.Errorf("sites PK = %q, want [site_code]", sites.PK)
		}
		if got := column(t, sites, "site_code").TypeName; got != "text" {
			t.Errorf("sites.site_code type = %q, want text", got)
		}
		devices := table(t, s, "public", "devices")
		if got := column(t, devices, "device_id").TypeName; got != "uuid" {
			t.Errorf("devices.device_id type = %q, want uuid", got)
		}
		readings := table(t, s, "public", "device_readings")
		if !sameStrings(readings.PK, []string{"device_id", "taken_at"}) {
			t.Errorf("device_readings PK = %q, want [device_id taken_at]", readings.PK)
		}
		if got := column(t, readings, "taken_at").TypeName; got != "timestamp with time zone" {
			t.Errorf("device_readings.taken_at type = %q, want timestamp with time zone", got)
		}
	})

	// Trap 11: no primary key, three unique indexes, one of them usable as an
	// identity. Introspection is what tells them apart.
	t.Run("Trap11_UniqueIndexes", func(t *testing.T) {
		log := table(t, s, "public", "audit_log")
		if len(log.PK) != 0 {
			t.Errorf("audit_log PK = %q, want none", log.PK)
		}
		usable := index(t, log, "audit_log_entry_uid_key")
		if !usable.Unique || usable.Partial || usable.Expression {
			t.Errorf("audit_log_entry_uid_key = %+v, want unique, not partial, not an expression", usable)
		}
		if !sameStrings(usable.Columns, []string{"entry_uid"}) {
			t.Errorf("audit_log_entry_uid_key columns = %q, want [entry_uid]", usable.Columns)
		}
		partial := index(t, log, "audit_log_recent_action_key")
		if !partial.Unique || !partial.Partial {
			t.Errorf("audit_log_recent_action_key = %+v, want unique and partial", partial)
		}
		expr := index(t, log, "audit_log_lower_entry_uid_key")
		if !expr.Unique || !expr.Expression {
			t.Errorf("audit_log_lower_entry_uid_key = %+v, want unique and an expression index", expr)
		}
		if len(expr.Columns) != 0 {
			t.Errorf("audit_log_lower_entry_uid_key reports columns %q; an expression index has none, and a partial list would read as a plain index", expr.Columns)
		}
		if !strings.Contains(expr.Def, "lower") {
			t.Errorf("the expression index def %q did not come from pg_get_indexdef", expr.Def)
		}
	})

	// Trap 12: no row identity at all. The refusal is the planner's; what
	// introspection owes is that there is nothing here to mistake for a key.
	t.Run("Trap12_NoIdentity", func(t *testing.T) {
		clicks := table(t, s, "public", "click_stream")
		if len(clicks.PK) != 0 {
			t.Errorf("click_stream PK = %q, want none", clicks.PK)
		}
		for _, idx := range clicks.Indexes {
			if idx.Unique {
				t.Errorf("click_stream carries unique index %q", idx.Name)
			}
		}
	})

	// Traps 13 and 24: two enum types, one the classifier ignores and one it
	// flags. Both must reach the target before the table that uses them.
	t.Run("Trap13and24_Enums", func(t *testing.T) {
		account := s.Enums["public.account_status"]
		if !sameStrings(account, []string{"pending", "active", "suspended", "closed"}) {
			t.Errorf("public.account_status labels = %q, want them in declaration order", account)
		}
		marital := s.Enums["public.marital_status"]
		if len(marital) != 6 {
			t.Errorf("public.marital_status has %d labels, want 6; trap 24's small_domain rests on the count", len(marital))
		}
		people := table(t, s, "public", "people")
		for _, name := range []string{"status", "marital_status"} {
			col := column(t, people, name)
			if !strings.HasSuffix(col.TypeName, name) && !strings.HasSuffix(col.TypeName, "account_status") {
				t.Errorf("people.%s type = %q, want the enum type", name, col.TypeName)
			}
			if col.TypeOID == 0 {
				t.Errorf("people.%s has no type OID", name)
			}
		}
	})

	// Trap 14: a generated column. The loader must not name it in a COPY column
	// list, which it can only know from here.
	t.Run("Trap14_GeneratedColumn", func(t *testing.T) {
		col := column(t, table(t, s, "public", "people"), "display_name")
		if col.Generated == "" {
			t.Fatal("people.display_name is not reported as generated")
		}
		if !strings.Contains(col.Generated, "given_name") {
			t.Errorf("display_name generated expression = %q, want the catalog's own deparse", col.Generated)
		}
		if col.Default != "" {
			t.Errorf("display_name also reports a default of %q; the expression is in pg_attrdef and must be read as one thing or the other", col.Default)
		}
	})

	// Trap 15: an array of email addresses. The classifier reaches the element
	// type from the type name it is given here.
	t.Run("Trap15_Array", func(t *testing.T) {
		col := column(t, table(t, s, "public", "people"), "alt_emails")
		if col.TypeName != "text[]" {
			t.Errorf("people.alt_emails type = %q, want text[]", col.TypeName)
		}
		if !col.Nullable {
			t.Error("people.alt_emails is not nullable; Katherine's NULL column is one of trap 15's five edges")
		}
	})

	// Trap 16: jsonb, both the leaf-masked and the collapsed column.
	t.Run("Trap16_Jsonb", func(t *testing.T) {
		if got := column(t, table(t, s, "public", "people"), "contact").TypeName; got != "jsonb" {
			t.Errorf("people.contact type = %q, want jsonb", got)
		}
		if got := column(t, table(t, s, "public", "events"), "payload").TypeName; got != "jsonb" {
			t.Errorf("events.payload type = %q, want jsonb", got)
		}
	})

	// Trap 18: type signals, with and without a name.
	t.Run("Trap18_TypeSignals", func(t *testing.T) {
		sessions := table(t, s, "public", "tenant_user_sessions")
		if got := column(t, sessions, "origin").TypeName; got != "inet" {
			t.Errorf("tenant_user_sessions.origin type = %q, want inet", got)
		}
		if got := column(t, sessions, "adapter").TypeName; got != "macaddr" {
			t.Errorf("tenant_user_sessions.adapter type = %q, want macaddr", got)
		}
		if got := column(t, table(t, s, "public", "audit_log"), "client_ip").TypeName; got != "inet" {
			t.Errorf("audit_log.client_ip type = %q, want inet", got)
		}
	})

	// Trap 19: a name that lies about its type. The classifier's type gate needs
	// the type, and this is where it comes from.
	t.Run("Trap19_TypeUnderALyingName", func(t *testing.T) {
		col := column(t, table(t, s, "public", "people"), "email_verified")
		if col.TypeName != "boolean" {
			t.Errorf("people.email_verified type = %q, want boolean", col.TypeName)
		}
	})

	// Trap 21: identity columns with non-default starts and increments. Both
	// the kind and the sequence parameters are load-bearing.
	t.Run("Trap21_Identity", func(t *testing.T) {
		cases := []struct {
			schema, table, column, kind string
			start, increment            int64
		}{
			{"public", "people", "person_id", "a", 90000, 7},
			{"public", "LegacyCustomer", "CustomerID", "a", 42, 1},
			{"public", "orders", "order_id", "d", 200000, 3},
			{"public", "tenant_user_sessions", "session_id", "d", 5000, 11},
			{"public", "tenant_user_flags", "flag_id", "d", 9000, 2},
			{"public", "organisations", "organisation_id", "d", 500, 3},
			{"public", "teams", "team_id", "d", 600, 3},
			{"public", "projects", "project_id", "d", 700, 3},
			{"public", "attachments", "attachment_id", "d", 800, 13},
			{"billing", "invoices", "invoice_id", "d", 3000, 5},
			{"public", "stream_rows", "stream_row_id", "d", 1, 1},
		}
		for _, c := range cases {
			tbl := table(t, s, c.schema, c.table)
			col := column(t, tbl, c.column)
			if col.Identity != c.kind {
				t.Errorf("%s.%s.%s identity = %q, want %q", c.schema, c.table, c.column, col.Identity, c.kind)
			}
			if col.IdentitySeq == nil {
				t.Errorf("%s.%s.%s has no identity sequence", c.schema, c.table, c.column)
				continue
			}
			if col.IdentitySeq.Start != c.start || col.IdentitySeq.Increment != c.increment {
				t.Errorf("%s.%s.%s sequence = start %d step %d, want start %d step %d",
					c.schema, c.table, c.column,
					col.IdentitySeq.Start, col.IdentitySeq.Increment, c.start, c.increment)
			}
			if col.IdentitySeq.Column != c.column {
				t.Errorf("%s.%s's identity sequence is owned by %q, want %q",
					c.schema, c.table, col.IdentitySeq.Column, c.column)
			}
			if !hasSequence(tbl, col.IdentitySeq.Name) {
				t.Errorf("%s.%s.%s's sequence is not in Table.Sequences", c.schema, c.table, c.column)
			}
		}
	})

	// Trap 22: two million rows, on demand. Loaded without the gate, the table
	// is empty, and an empty table samples to nothing rather than erroring.
	t.Run("Trap22_StreamRowsEmptyByDefault", func(t *testing.T) {
		stream := table(t, s, "public", "stream_rows")
		if len(stream.Samples) != 0 {
			t.Errorf("stream_rows returned %d samples from an empty table", len(stream.Samples))
		}
	})

	// Trap 23: a unique index, a varchar(n) and a CHECK, all on masked columns.
	// §5's domain machinery reads all three from here.
	t.Run("Trap23_MaskingDomains", func(t *testing.T) {
		legacy := table(t, s, "public", "LegacyCustomer")
		contact := column(t, legacy, "ContactNumber")
		if contact.TypeName != "character varying(15)" {
			t.Errorf(`"ContactNumber" type = %q, want character varying(15)`, contact.TypeName)
		}
		// atttypmod for varchar(n) is n + 4.
		if contact.TypMod != 19 {
			t.Errorf(`"ContactNumber" atttypmod = %d, want 19`, contact.TypMod)
		}
		for _, name := range []string{"LegacyCustomer_EmailAddress_key", "LegacyCustomer_ContactNumber_key"} {
			idx := index(t, legacy, name)
			if !idx.Unique || idx.Partial || idx.Expression {
				t.Errorf("%s = %+v, want a plain unique index", name, idx)
			}
		}
		email := column(t, legacy, "EmailAddress")
		if len(email.Checks) == 0 {
			t.Fatal(`"EmailAddress" carries no CHECK; §5's "preserve what the application checks" has nothing to read`)
		}
		if !strings.Contains(email.Checks[0], "~~") && !strings.Contains(email.Checks[0], "LIKE") {
			t.Errorf(`"EmailAddress" check = %q, want the catalog's deparse of the LIKE constraint`, email.Checks[0])
		}
		if contact.Nullable != true {
			t.Error(`"ContactNumber" is not nullable; trap 23's "NULL stays NULL under a unique index" case is gone`)
		}
	})

	t.Run("ForeignKeyIndexing", func(t *testing.T) {
		// The composite primary key of tenant_users covers the composite edge
		// into it; click_stream.person_id has no index at all.
		sessions := fkFrom(t, s, tref("public", "tenant_user_sessions"), "tenant_id", "user_id")
		if sessions.Indexed {
			t.Error("the sessions edge reports itself indexed; tenant_user_sessions has no index on (tenant_id, user_id)")
		}
		self := fkFrom(t, s, tref("public", "people"), "manager_id")
		if self.Indexed {
			t.Error("people.manager_id reports itself indexed; nasty.sql declares no index on it")
		}
		readings := fkFrom(t, s, tref("public", "device_readings"), "device_id")
		if !readings.Indexed {
			t.Error("device_readings.device_id is the leading column of its primary key and must report as indexed")
		}
	})

	t.Run("NotRecreated", func(t *testing.T) {
		kinds := kindCounts(s)
		// nasty.sql declares public.fill_stream_rows and nothing else v1 leaves
		// behind; if it grows a view or a trigger, this says so.
		if kinds["function"] == 0 {
			t.Errorf("NotRecreated = %v, want public.fill_stream_rows counted as a function", kinds)
		}
		for _, o := range s.NotRecreated {
			if o.Kind == "" || o.Name == "" {
				t.Errorf("NotRecreated carries %+v", o)
			}
		}
	})

	// §2's Constraints, in creation order, with the def from
	// pg_get_constraintdef. This is the sole input to §11.1 item 5's unique,
	// check and exclusion DDL, and nothing else in the suite reads it.
	t.Run("Constraints", func(t *testing.T) {
		legacy := table(t, s, "public", "LegacyCustomer")
		want := []pipeline.Constraint{
			{Name: "LegacyCustomer_EmailAddress_check", Kind: 'c'},
			{Name: "LegacyCustomer_pkey", Kind: 'p'},
			{Name: "LegacyCustomer_MigratedFromPersonID_fkey", Kind: 'f'},
		}
		if len(legacy.Constraints) != len(want) {
			t.Fatalf(`"LegacyCustomer" constraints = %+v, want %d`, legacy.Constraints, len(want))
		}
		for i, w := range want {
			got := legacy.Constraints[i]
			if got.Name != w.Name || got.Kind != w.Kind {
				t.Errorf("constraint %d = %q (%c), want %q (%c); the order is pg_constraint.oid, which is creation order",
					i, got.Name, got.Kind, w.Name, w.Kind)
			}
		}
		if def := legacy.Constraints[0].Def; !strings.HasPrefix(def, "CHECK") || !strings.Contains(def, "EmailAddress") {
			t.Errorf("the check def = %q, want pg_get_constraintdef's own text", def)
		}
		if def := legacy.Constraints[1].Def; def != `PRIMARY KEY ("CustomerID")` {
			t.Errorf("the primary key def = %q", def)
		}
		// nasty.sql: one CHECK, 28 foreign keys, 20 primary keys and 4 unique
		// constraints (the partition clones included in every count, which
		// sqlConstraints does not filter), on every supported major. Trap 25
		// adds five of the foreign keys — price_list_notes.person_id ->
		// people (unconditional, testdata/README.md trap 25's reachability
		// fix) plus, loaded through LoadNastyNotRecreatable as this test is,
		// the declared price_list_notes.list_id -> price_lists_eu edge and
		// two clones of price_lists.owner_person_id -> people onto its
		// leaves — one primary key (price_list_notes) and all four unique
		// constraints (price_lists' own DEFERRABLE key, its clone on each
		// leaf, and price_lists_eu's own non-deferrable one) — the first
		// contype 'u' rows this fixture has ever produced. Unfiltered,
		// PostgreSQL 18 adds 79 more of contype 'n' — the NOT NULLs it now
		// stores in pg_constraint — and the same schema would fingerprint
		// differently per major (§11.2's marker could never bind across them).
		assertConstraintKinds(t, s, map[byte]int{'c': 1, 'f': 28, 'p': 20, 'u': 4})
	})

	t.Run("CatalogFieldsNothingElseReads", func(t *testing.T) {
		assertCatalogFields(t, s)
	})

	t.Run("SamplesAreShapedLikeTheirTable", func(t *testing.T) {
		for _, tbl := range s.Tables {
			if len(tbl.Samples) > SampleRows {
				t.Errorf("%s returned %d samples, above the cap of %d", tbl.Ref, len(tbl.Samples), SampleRows)
			}
			for _, row := range tbl.Samples {
				if len(row) != len(tbl.Columns) {
					t.Fatalf("%s: a sample row has %d values for %d columns", tbl.Ref, len(row), len(tbl.Columns))
				}
			}
		}
		// public.people holds five rows and every one of them must be here, or
		// the classifier's validators are looking at a third of the table.
		if got := len(table(t, s, "public", "people").Samples); got != 5 {
			t.Errorf("public.people returned %d samples, want its 5 rows", got)
		}
	})
}

// ---------- a table the source role cannot read ----------

// A source role that lacks SELECT on one relation is a condition of that table,
// not of the run. ARCHITECTURE.md §3.6 answers it at plan — an unreadable table
// that is unreachable or child-only is dropped to SchemaOnly and the run
// continues, an unreadable parent or root stops there with exit 12 naming the
// table, the role and the GRANT to run, and "There is no mid-extract permission
// failure by design". Introspect runs before plan and takes no privileges, so if
// a failing sample ended the run here neither of those outcomes could ever be
// reached and the developer would see a wrapped 42501 instead of the command
// that fixes it.
//
// pg_class is world-readable, so the table is still described in full: it is the
// sample, and only the sample, that is missing.
func TestIntrospectSurvivesATableTheRoleCannotRead(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadNasty(ctx, url, false); err != nil {
		t.Fatalf("loading nasty.sql: %v", err)
	}
	analyse(ctx, t, url)

	shapes := make([]pg.Shape, 0, len(Shapes()))
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	src, err := pg.OpenSource(ctx, dsn.DSN(withoutSelectOnPeople(ctx, t, url)), shapes...)
	if err != nil {
		t.Fatalf("opening the source as the narrow role: %v", err)
	}
	t.Cleanup(src.Close)
	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	s := introspectOnce(ctx, t, src, id)
	if err := src.Violation(); err != nil {
		t.Fatalf("the source allowlist refused a statement Introspect sent: %v", err)
	}

	people := table(t, s, "public", "people")
	if len(people.Columns) == 0 {
		t.Error("public.people has no columns; the catalog is readable even where the table is not")
	}
	if n := len(people.Samples); n != 0 {
		t.Errorf("public.people returned %d samples from a role that cannot SELECT it", n)
	}
	if people.SampledFrom != nil {
		t.Errorf("public.people reports SampledFrom %v; nothing was sampled", *people.SampledFrom)
	}
	sampled := 0
	for _, tbl := range s.Tables {
		if len(tbl.Samples) > 0 {
			sampled++
		}
	}
	if sampled == 0 {
		t.Error("no table was sampled at all, so tolerating one refusal proves nothing")
	}
}

// withoutSelectOnPeople returns a connection URL for a role that can see the
// catalog and read every table of nasty.sql except public.people.
func withoutSelectOnPeople(ctx context.Context, t *testing.T, url string) string {
	t.Helper()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to grant: %v", err)
	}
	defer func() { _ = admin.Close(context.WithoutCancel(ctx)) }()
	for _, stmt := range []string{
		`CREATE ROLE narrow LOGIN PASSWORD 'narrow'`,
		`GRANT USAGE ON SCHEMA public, billing TO narrow`,
		`GRANT SELECT ON ALL TABLES IN SCHEMA public, billing TO narrow`,
		`REVOKE SELECT ON public.people FROM narrow`,
	} {
		if _, execErr := admin.Exec(ctx, stmt); execErr != nil {
			t.Fatalf("%s: %v", stmt, execErr)
		}
	}

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatalf("parsing the fixture URL: %v", err)
	}
	return "postgres://narrow:narrow@" + cfg.Host + ":" + strconv.Itoa(int(cfg.Port)) + "/" + cfg.Database
}

// ---------- an extension used only by a view ----------

// Schema.Extensions is what §11.1 item 2 recreates, and §11.1 recreates no
// view: an extension only a view depends on must not be in it. Both halves of
// that cost something. An extension the target cannot create fails the load at
// DDL after item 1 has dropped every user table; and Schema.Fingerprint hashes
// the list, so an extension the source has and a target lazyslice wrote (which
// holds no views) does not would stop §11.2's marker from ever binding and
// every second run against that target would be refused with exit 4.
//
// The trap is that a view's row type is a composite — typtype 'c', with
// typrelid pointing at the view — so the walk's composite branch matched it and
// took the view's columns for a recreated object's. Both databases here are on
// one server: the second is the control, without which "no extension" would
// pass on a server where citext does not resolve at all.
func TestAnExtensionUsedOnlyByAViewIsNotCollected(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	viewOnly := database(ctx, t, url, "view_only", `
CREATE EXTENSION citext;
CREATE TABLE public.plain (id int PRIMARY KEY, addr text);
CREATE VIEW public.v_plain AS SELECT id, addr::citext AS addr FROM public.plain;
CREATE MATERIALIZED VIEW public.m_plain AS SELECT id, addr::citext AS addr FROM public.plain;
`)
	column := database(ctx, t, url, "citext_column", `
CREATE EXTENSION citext;
CREATE TABLE public.emails (id int PRIMARY KEY, addr citext);
`)

	s := introspectDatabase(ctx, t, viewOnly)
	for _, e := range s.Extensions {
		t.Errorf("Extensions carries %s in %s; only a view uses citext here, and §11.1 recreates no view",
			e.Name, e.Schema)
	}
	views := 0
	for _, o := range s.NotRecreated {
		if o.Kind == "view" || o.Kind == "matview" {
			views++
		}
	}
	if views != 2 {
		t.Errorf("NotRecreated holds %d views; the fixture declares a view and a materialised view, "+
			"so an empty extension list here would prove nothing", views)
	}

	// The control: the same extension, used by a column of a recreated table,
	// is still collected.
	control := introspectDatabase(ctx, t, column)
	found := false
	for _, e := range control.Extensions {
		if e.Name == "citext" {
			found = true
		}
	}
	if !found {
		t.Errorf("Extensions = %+v for a table with a citext column, want citext among them",
			control.Extensions)
	}
}

// database creates a database on the server url points at, runs ddl in it and
// returns a connection URL for it. Two databases on one server are two
// catalogs at the price of one container start.
func database(ctx context.Context, t *testing.T, url, name, ddl string) string {
	t.Helper()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to create %s: %v", name, err)
	}
	defer func() { _ = admin.Close(context.WithoutCancel(ctx)) }()
	if _, execErr := admin.Exec(ctx, "CREATE DATABASE "+name); execErr != nil {
		t.Fatalf("creating database %s: %v", name, execErr)
	}

	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatalf("parsing the server URL: %v", err)
	}
	own := "postgres://" + cfg.User + ":" + cfg.Password + "@" +
		cfg.Host + ":" + strconv.Itoa(int(cfg.Port)) + "/" + name

	conn, err := pgx.Connect(ctx, own)
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, execErr := conn.Exec(ctx, ddl); execErr != nil {
		t.Fatalf("loading %s: %v", name, execErr)
	}
	return own
}

// introspectDatabase runs Introspect once against url through a real Source
// with this package's shapes registered, and fails if the allowlist refused a
// statement it sent.
func introspectDatabase(ctx context.Context, t *testing.T, url string) *pipeline.Schema {
	t.Helper()

	shapes := make([]pg.Shape, 0, len(Shapes()))
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	src, err := pg.OpenSource(ctx, dsn.DSN(url), shapes...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)
	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	s := introspectOnce(ctx, t, src, id)
	if err := src.Violation(); err != nil {
		t.Fatalf("the source allowlist refused a statement Introspect sent: %v", err)
	}
	return s
}

// ---------- pagila ----------

func TestIntrospectPagila(t *testing.T) {
	ctx := context.Background()
	s := introspectFixture(ctx, t, testutil.LoadPagila)

	t.Run("TableCount", func(t *testing.T) {
		// testdata/README.md: 22 tables of relkind r or p, no more and no fewer.
		if got := len(s.Tables); got != 22 {
			var names []string
			for _, tbl := range s.Tables {
				names = append(names, tbl.Ref.String())
			}
			t.Errorf("pagila has %d tables, want 22: %q", got, names)
		}
		for _, tbl := range s.Tables {
			if tbl.Ref.Name == "rental_by_category" {
				t.Error("the materialised view is in the table list")
			}
		}
	})

	t.Run("ViewsAndFunctionsAreNotRecreated", func(t *testing.T) {
		byKind := map[string][]string{}
		for _, o := range s.NotRecreated {
			byKind[o.Kind] = append(byKind[o.Kind], o.Name)
		}
		if !contains(byKind["matview"], "public.rental_by_category") {
			t.Errorf("matviews = %q, want public.rental_by_category", byKind["matview"])
		}
		for _, view := range []string{"public.actor_info", "public.customer_list", "public.film_list", "public.staff_list"} {
			if !contains(byKind["view"], view) {
				t.Errorf("views = %q, want %s among them", byKind["view"], view)
			}
		}
		if !contains(byKind["function"], "public.group_concat") || !contains(byKind["function"], "public.rewards_report") {
			t.Errorf("functions = %q, want group_concat and rewards_report", byKind["function"])
		}
		if !contains(byKind["trigger"], "public.film.film_fulltext_trigger") {
			t.Errorf("triggers = %q, want public.film.film_fulltext_trigger", byKind["trigger"])
		}
		// The same trigger is counted on its table, which is what the plan prints.
		if got := table(t, s, "public", "film").Triggers; got != 2 {
			t.Errorf("public.film has %d user triggers, want 2 (film_fulltext_trigger and last_updated)", got)
		}
	})

	t.Run("EnumAndDomains", func(t *testing.T) {
		rating := s.Enums["public.mpaa_rating"]
		if !sameStrings(rating, []string{"G", "PG", "PG-13", "R", "NC-17"}) {
			t.Errorf("public.mpaa_rating = %q", rating)
		}
		var names []string
		for _, d := range s.Domains {
			names = append(names, d.Name)
		}
		for _, want := range []string{"public.year", "public.bıgınt"} {
			if !contains(names, want) {
				t.Errorf("domains = %q, want %s among them", names, want)
			}
		}
		for _, d := range s.Domains {
			if !strings.HasPrefix(d.Def, "CREATE DOMAIN ") {
				t.Errorf("domain %s def = %q", d.Name, d.Def)
			}
			// A dotless Turkish i survives only if nothing lowercases it with a
			// locale-dependent rule; quote_ident is the catalog's own.
			if d.Name == "public.bıgınt" && !strings.Contains(d.Def, `"bıgınt"`) {
				t.Errorf("the quoted domain def is %q", d.Def)
			}
			if d.Name == "public.year" && !strings.Contains(d.Def, "CHECK") {
				t.Errorf("public.year lost its check: %q", d.Def)
			}
		}
	})

	t.Run("TsvectorAndArray", func(t *testing.T) {
		film := table(t, s, "public", "film")
		if got := column(t, film, "fulltext").TypeName; got != "tsvector" {
			t.Errorf("film.fulltext type = %q, want tsvector", got)
		}
		if got := column(t, film, "special_features").TypeName; got != "text[]" {
			t.Errorf("film.special_features type = %q, want text[]", got)
		}
		if got := column(t, film, "rating").Default; !strings.Contains(got, "mpaa_rating") {
			t.Errorf("film.rating default = %q, want the enum cast the catalog deparses", got)
		}
	})

	// Two foreign keys from one child into one parent. An edge set keyed by
	// (child, parent) silently loses one of them.
	t.Run("TwoEdgesIntoOneParent", func(t *testing.T) {
		var found []string
		for _, fk := range s.FKs {
			if fk.Child == tref("public", "film") && fk.Parent == tref("public", "language") {
				found = append(found, fk.Name)
			}
		}
		sort.Strings(found)
		if !sameStrings(found, []string{"film_language_id_fkey", "film_original_language_id_fkey"}) {
			t.Errorf("film -> language edges = %q, want both", found)
		}
	})

	// The realistic partitioned root: seven leaves with real data, and the
	// largest by reltuples is not the first by name.
	t.Run("PartitionedPaymentSamplesItsLargestLeaf", func(t *testing.T) {
		payment := table(t, s, "public", "payment")
		if !payment.Partitioned {
			t.Fatal("public.payment is not reported as partitioned")
		}
		if len(payment.Partitions) != 7 {
			t.Errorf("payment has %d partitions, want 7: %v", len(payment.Partitions), payment.Partitions)
		}
		if !sameStrings(payment.PartitionKey, []string{"payment_date"}) {
			t.Errorf("payment partition key = %q, want [payment_date]", payment.PartitionKey)
		}
		// payment_p2022_03 holds 2,713 rows, more than any other leaf, while
		// payment_p2022_01 holds 723 and sorts first. Picking the first name
		// rather than the largest leaf fails here and nowhere else.
		if payment.SampledFrom == nil || *payment.SampledFrom != tref("public", "payment_p2022_03") {
			t.Errorf("payment.SampledFrom = %v, want public.payment_p2022_03", payment.SampledFrom)
		}
		if got := len(payment.Samples); got != SampleRows {
			t.Errorf("payment returned %d samples, want the cap of %d", got, SampleRows)
		}
	})

	// pagila v3.1.0 declares every payment foreign key on the partitions and
	// none on the root: six leaves times customer, rental and staff is eighteen
	// constraints, each with conparentid = 0 because none is a clone of a root
	// constraint there is no root constraint to clone. Dropping a partition-local
	// edge left public.payment with no outgoing edge at all, so the planner never
	// pulled a customer, a rental or a staff row for a payment and the slice was
	// referentially incomplete with nothing printed. They are moved to the root,
	// where the six leaves' copies of one constraint collapse into one edge.
	t.Run("PaymentPartitionEdgesMoveToTheRoot", func(t *testing.T) {
		payment := tref("public", "payment")
		parents := map[string]string{}
		for _, fk := range s.FKs {
			if fk.Child == payment {
				if was, dup := parents[fk.Parent.Name]; dup {
					t.Errorf("payment -> %s twice: %q and %q; the leaves' copies did not collapse",
						fk.Parent.Name, was, fk.Name)
				}
				parents[fk.Parent.Name] = fk.Name
			}
		}
		for _, want := range []string{"customer", "rental", "staff"} {
			if _, ok := parents[want]; !ok {
				t.Errorf("public.payment has no edge to %s; a partition-local dependency was lost", want)
			}
		}
		if len(parents) != 3 {
			t.Errorf("public.payment has %d outgoing edges, want 3: %v", len(parents), parents)
		}
		// And nothing addresses a leaf by name, in either direction.
		for _, fk := range s.FKs {
			for _, end := range []ref.TableRef{fk.Child, fk.Parent} {
				if tbl := table(t, s, end.Schema, end.Name); tbl.Parent != nil {
					t.Errorf("edge %q names the partition %s; %s is the table §11.1 recreates",
						fk.Name, end, *tbl.Parent)
				}
			}
		}
	})

	t.Run("SequencesAreOwnedAndParameterised", func(t *testing.T) {
		actor := table(t, s, "public", "actor")
		if len(actor.Sequences) != 1 {
			t.Fatalf("public.actor has %d sequences, want 1: %+v", len(actor.Sequences), actor.Sequences)
		}
		seq := actor.Sequences[0]
		if seq.Name != "public.actor_actor_id_seq" {
			t.Errorf("actor's sequence = %q, want public.actor_actor_id_seq schema-qualified", seq.Name)
		}
		if seq.Increment != 1 || seq.Start != 1 {
			t.Errorf("actor's sequence = %+v, want start 1 step 1", seq)
		}
		// pagila v3.1.0 writes CREATE SEQUENCE and a nextval default and never
		// ALTER SEQUENCE ... OWNED BY, so the sequence is referenced rather than
		// owned. Reading ownership alone would drop every sequence in this
		// fixture and leave the target's defaults calling a sequence that does
		// not exist.
		if seq.Column != "" {
			t.Errorf("actor's sequence reports owner column %q; pagila declares no OWNED BY", seq.Column)
		}
		col := column(t, actor, "actor_id")
		if col.Identity != "" {
			t.Errorf("actor.actor_id identity = %q, want none; pagila uses a default, not an identity column", col.Identity)
		}
		if !strings.Contains(col.Default, "nextval") {
			t.Errorf("actor.actor_id default = %q, want the nextval the catalog deparses", col.Default)
		}
	})

	t.Run("Constraints", func(t *testing.T) {
		rental := table(t, s, "public", "rental")
		want := []struct {
			name string
			kind byte
		}{
			{"rental_pkey", 'p'},
			{"rental_customer_id_fkey", 'f'},
			{"rental_inventory_id_fkey", 'f'},
			{"rental_staff_id_fkey", 'f'},
		}
		if len(rental.Constraints) != len(want) {
			t.Fatalf("rental constraints = %+v, want %d", rental.Constraints, len(want))
		}
		for i, w := range want {
			got := rental.Constraints[i]
			if got.Name != w.name || got.Kind != w.kind {
				t.Errorf("constraint %d = %q (%c), want %q (%c); the order is creation order",
					i, got.Name, got.Kind, w.name, w.kind)
			}
			if got.Def == "" {
				t.Errorf("constraint %q has no def", got.Name)
			}
		}
		if def := rental.Constraints[1].Def; !strings.Contains(def, "REFERENCES customer") {
			t.Errorf("rental_customer_id_fkey def = %q, want pg_get_constraintdef's own text", def)
		}
		// pagila declares no CHECK and no unique constraint (its unique keys are
		// indexes), so 22 primary keys and 36 foreign keys is the whole of it on
		// every supported major.
		assertConstraintKinds(t, s, map[byte]int{'f': 36, 'p': 22})
	})

	t.Run("CatalogFieldsNothingElseReads", func(t *testing.T) {
		assertCatalogFields(t, s)
	})

	t.Run("NoSelfReferencingEdge", func(t *testing.T) {
		for _, fk := range s.FKs {
			if fk.Child == fk.Parent {
				t.Errorf("pagila v3.1.0 has no self-referencing foreign key, but %q is one", fk.Name)
			}
		}
	})
}

// ---------- helpers ----------

// assertConstraintKinds counts Table.Constraints by contype over the whole
// schema. §2 enumerates five kinds and no others; anything else here is a
// catalog row that reached a Constraint.Kind the loader cannot render.
func assertConstraintKinds(t *testing.T, s *pipeline.Schema, want map[byte]int) {
	t.Helper()
	got := map[byte]int{}
	for _, tbl := range s.Tables {
		for _, con := range tbl.Constraints {
			got[con.Kind]++
			if !strings.ContainsRune("pucfx", rune(con.Kind)) {
				t.Errorf("%s carries constraint %q of kind %q, which ARCHITECTURE.md §2 does not define",
					tbl.Ref, con.Name, string(con.Kind))
			}
			if con.Name == "" || con.Def == "" {
				t.Errorf("%s carries constraint %+v with no name or no def", tbl.Ref, con)
			}
		}
	}
	for kind, n := range want {
		if got[kind] != n {
			t.Errorf("constraints of kind %q = %d, want %d (all kinds: %v)", string(kind), got[kind], n, got)
		}
	}
	for kind := range got {
		if _, ok := want[kind]; !ok {
			t.Errorf("constraints of kind %q = %d, want none", string(kind), got[kind])
		}
	}
}

// assertCatalogFields covers the §2 fields this package fills that no trap
// names, on both fixtures: a stub returning the zero value for them would
// otherwise pass the whole suite, which is how the PostgreSQL 18 contype 'n'
// divergence went unnoticed (T-INTROSPECT review).
func assertCatalogFields(t *testing.T, s *pipeline.Schema) {
	t.Helper()

	// The lowest major ARCHITECTURE.md §14 supports is 14.
	if s.ServerVersion < 140000 {
		t.Errorf("ServerVersion = %d, want the server_version_num of a supported major", s.ServerVersion)
	}

	// §11.1 item 2 recreates "every extension a recreated column type, default,
	// index or operator depends on ... found through pg_depend", not every
	// extension installed. Neither fixture declares one, and plpgsql — which
	// every database has — is never one of them: an extension in this list that
	// the target cannot create fails the load at DDL, and it enters
	// Schema.Fingerprint, where it would stop §11.2's marker from binding.
	for _, e := range s.Extensions {
		t.Errorf("Extensions carries %s in %s; neither fixture installs an extension a recreated object depends on",
			e.Name, e.Schema)
	}

	// Composite types. Neither fixture declares one, and an extension's own
	// composite must not appear here either: §11.1 item 2 creates the extension
	// before item 3 creates the types, so recreating dblink_pkey_results is a
	// 42710 against a target whose tables have already been dropped.
	for _, c := range s.Composites {
		t.Errorf("Composites carries %s (%q); neither fixture declares a composite type", c.Name, c.Def)
	}

	// ApproxRows is pg_class.reltuples, which the fixture ANALYZE has filled. A
	// relation nothing has analysed reports -1, which readTables carries as 0;
	// nothing here may be negative, and something must be positive or the
	// sampler's "largest leaf by reltuples" rule is choosing between zeroes.
	positive := 0
	for _, tbl := range s.Tables {
		if tbl.ApproxRows < 0 {
			t.Errorf("%s reports ApproxRows %d; reltuples of -1 is carried as 0", tbl.Ref, tbl.ApproxRows)
		}
		if tbl.ApproxRows > 0 {
			positive++
		}
	}
	if positive == 0 {
		t.Error("no table reports a row estimate, though the fixture was analysed")
	}

	// relrowsecurity and relforcerowsecurity. Neither fixture enables row-level
	// security; a policy is counted under NotRecreated, and these two say whether
	// the source table was under one, which reading as a constant false would
	// hide.
	for _, tbl := range s.Tables {
		if tbl.RLS || tbl.ForceRLS {
			t.Errorf("%s reports RLS=%v ForceRLS=%v; neither fixture enables row-level security",
				tbl.Ref, tbl.RLS, tbl.ForceRLS)
		}
	}

	// convalidated. An unvalidated edge is a hint the planner may not follow as
	// a constraint (§2), so reading the column as a constant true would make
	// every edge look enforced. Neither fixture declares NOT VALID.
	for _, fk := range s.FKs {
		if !fk.Validated {
			t.Errorf("foreign key %q reports itself unvalidated; neither fixture declares NOT VALID", fk.Name)
		}
		if fk.Child == (ref.TableRef{}) || fk.Parent == (ref.TableRef{}) {
			t.Errorf("foreign key %q has an unnamed end", fk.Name)
		}
	}

	// Column.Collation is the *non-default* collation: every text column of both
	// fixtures collates in the database default, which sqlColumns elides with
	// `co.collname <> 'default'`. Reporting "default" here would put a
	// COLLATE "default" into §11.1 item 5's DDL and into the fingerprint.
	texts := 0
	for _, tbl := range s.Tables {
		for _, col := range tbl.Columns {
			if col.TypeName == "text" || strings.HasPrefix(col.TypeName, "character varying") {
				texts++
			}
			if col.Collation != "" {
				t.Errorf("%s.%s reports collation %q; neither fixture declares COLLATE",
					tbl.Ref, col.Name, col.Collation)
			}
		}
	}
	if texts == 0 {
		t.Error("no text column was found, so the collation assertion proves nothing")
	}
}

func tref(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

func table(t *testing.T, s *pipeline.Schema, schema, name string) pipeline.Table {
	t.Helper()
	for _, tbl := range s.Tables {
		if tbl.Ref == tref(schema, name) {
			return tbl
		}
	}
	t.Fatalf("%s.%s is not in the schema", schema, name)
	return pipeline.Table{}
}

func column(t *testing.T, tbl pipeline.Table, name string) pipeline.Column {
	t.Helper()
	for _, col := range tbl.Columns {
		if col.Name == name {
			return col
		}
	}
	t.Fatalf("%s has no column %q", tbl.Ref, name)
	return pipeline.Column{}
}

func index(t *testing.T, tbl pipeline.Table, name string) pipeline.Index {
	t.Helper()
	for _, idx := range tbl.Indexes {
		if idx.Name == name {
			return idx
		}
	}
	t.Fatalf("%s has no index %q", tbl.Ref, name)
	return pipeline.Index{}
}

// fkFrom finds the edge by its child columns rather than by its constraint
// name, so that an assertion does not depend on a name Postgres generated.
func fkFrom(t *testing.T, s *pipeline.Schema, child ref.TableRef, cols ...string) pipeline.ForeignKey {
	t.Helper()
	for _, fk := range s.FKs {
		if fk.Child == child && sameStrings(fk.ChildCols, cols) {
			return fk
		}
	}
	t.Fatalf("no foreign key on %s over %q", child, cols)
	return pipeline.ForeignKey{}
}

func hasSequence(tbl pipeline.Table, name string) bool {
	for _, q := range tbl.Sequences {
		if q.Name == name {
			return true
		}
	}
	return false
}

func kindCounts(s *pipeline.Schema) map[string]int {
	out := map[string]int{}
	for _, o := range s.NotRecreated {
		out[o.Kind]++
	}
	return out
}

func contains(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
