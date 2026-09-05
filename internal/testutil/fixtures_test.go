//go:build integration

package testutil

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The fixtures are the definition of correct (docs/BUILD_PLAN.md phase 3), so
// they get a test of their own: not "does the file parse" but "does every table
// this repository names arrive, with the rows it is supposed to have, in the
// shape testdata/README.md promises". A trap that silently stops being a trap
// is worse than no trap, because every suite downstream then passes for the
// wrong reason.
//
// Two passes. The counts below are the fixture contents; the catalogue
// assertions further down are the traps themselves, so that stripping
// GENERATED ALWAYS off a column, un-partitioning a table or dropping a
// constraint fails here rather than in a phase 4 test that says something else.
// Changing either is a deliberate act: update testdata/README.md in the same
// commit.

// pagilaTables is every ordinary and partitioned table in pagila-schema.sql at
// PagilaCommit, with the row count pagila-data.sql loads into it. payment is
// the partitioned root, so its count is the sum of its seven leaves.
var pagilaTables = map[string]int{
	"public.actor":            200,
	"public.address":          603,
	"public.category":         16,
	"public.city":             600,
	"public.country":          109,
	"public.customer":         599,
	"public.film":             1000,
	"public.film_actor":       5462,
	"public.film_category":    1000,
	"public.inventory":        4581,
	"public.language":         6,
	"public.payment":          16049,
	"public.payment_p2022_01": 723,
	"public.payment_p2022_02": 2401,
	"public.payment_p2022_03": 2713,
	"public.payment_p2022_04": 2547,
	"public.payment_p2022_05": 2677,
	"public.payment_p2022_06": 2654,
	"public.payment_p2022_07": 2334,
	"public.rental":           16044,
	"public.staff":            2,
	"public.store":            2,
}

// nastyTables is every table in nasty.sql with the rows the fixture inserts.
// public.stream_rows is empty unless LoadNasty is called with big=true, which
// is the whole point of the gate.
var nastyTables = map[string]int{
	"billing.invoices":            3,
	"public.LegacyCustomer":       3,
	"public.attachments":          4,
	"public.audit_log":            4,
	"public.click_stream":         3,
	"public.device_readings":      4,
	"public.devices":              3,
	"public.events":               7,
	"public.events_2024":          5,
	"public.events_2025":          2,
	"public.order_items":          7,
	"public.orders":               5,
	"public.organisations":        2,
	"public.people":               5,
	"public.projects":             2,
	"public.sites":                2,
	"public.stream_rows":          0,
	"public.teams":                2,
	"public.tenant_user_flags":    3,
	"public.tenant_user_sessions": 5,
	"public.tenant_users":         4,
}

func TestLoadPagila(t *testing.T) {
	ctx := context.Background()
	SkipWithoutDocker(ctx, t)

	url := Postgres(ctx, t, "")
	if err := LoadPagila(ctx, url); err != nil {
		t.Fatalf("LoadPagila: %v", err)
	}
	assertTables(ctx, t, url, pagilaTables)
	assertPagilaShapes(ctx, t, connect(ctx, t, url))
}

// assertPagilaShapes checks the claims testdata/README.md's "What Pagila
// brings" table makes about the shapes only this fixture has. It is short on
// purpose: pagila is upstream's file, so what is worth asserting is that the
// README describes the file we actually pinned.
func assertPagilaShapes(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	// Two foreign keys from one child into one parent. Following one edge and
	// forgetting the other selects too few language rows, and de-duplicating
	// edges by (child, parent) instead of by constraint name loses one of them
	// outright.
	got := scanStrings(ctx, t, conn, `
		SELECT conname FROM pg_constraint
		WHERE contype = 'f'
		  AND conrelid = 'public.film'::regclass
		  AND confrelid = 'public.language'::regclass
		ORDER BY conname`)
	want := []string{"film_language_id_fkey", "film_original_language_id_fkey"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("film -> language foreign keys are %v, want %v", got, want)
	}

	// And no self-reference: pagila v3.1.0 has none, which is why
	// people.manager_id in nasty.sql is the only fixture for that shape.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND conrelid = confrelid`); n != 0 {
		t.Errorf("pagila has %d self-referencing foreign keys, want 0: testdata/README.md says nasty.sql carries that shape", n)
	}

	// The materialised view and the seven views must not be tables, which is
	// what assertTables is really asserting when it counts 22 relations.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'm'`); n != 1 {
		t.Errorf("pagila has %d materialised views, want 1 (rental_by_category)", n)
	}
}

func TestLoadNasty(t *testing.T) {
	ctx := context.Background()
	SkipWithoutDocker(ctx, t)

	url := Postgres(ctx, t, "")
	if err := LoadNasty(ctx, url, false); err != nil {
		t.Fatalf("LoadNasty: %v", err)
	}
	assertTables(ctx, t, url, nastyTables)

	conn := connect(ctx, t, url)
	t.Run("types", func(t *testing.T) { assertNastyTypes(ctx, t, conn) })
	t.Run("structure", func(t *testing.T) { assertNastyStructure(ctx, t, conn) })
	t.Run("identity", func(t *testing.T) { assertNastyIdentity(ctx, t, conn) })
	t.Run("gate", func(t *testing.T) { assertNastyGateOff(ctx, t, conn) })
}

// TestLoadNastyBig is the other half of the gate. Without it a gate stuck off
// looks exactly like a gate that works: TestLoadNasty asserts stream_rows is
// empty either way, and every streaming test downstream would then measure an
// empty table.
//
// It costs about six seconds and a couple of hundred megabytes on top of the
// container, so -short skips it.
func TestLoadNastyBig(t *testing.T) {
	if testing.Short() {
		t.Skip("the 2,000,000-row fill costs about six seconds and 200MB")
	}
	ctx := context.Background()
	SkipWithoutDocker(ctx, t)

	url := Postgres(ctx, t, "")
	if err := LoadNasty(ctx, url, true); err != nil {
		t.Fatalf("LoadNasty(big): %v", err)
	}

	conn := connect(ctx, t, url)
	if n := scanInt(ctx, t, conn, `SELECT count(*) FROM public.stream_rows`); n != StreamRows {
		t.Errorf("public.stream_rows has %d rows, want %d", n, StreamRows)
	}
	// Every row hangs off the lowest person_id, which is what makes a slice
	// rooted at anyone else pull none of them.
	if n := scanInt(ctx, t, conn, `
		SELECT count(DISTINCT person_id) FROM public.stream_rows`); n != 1 {
		t.Errorf("public.stream_rows references %d people, want 1", n)
	}
	// The fill advances the identity's sequence past the last id it wrote. A
	// sequence left behind the rows collides on the first insert into the copy.
	last, called := sequenceState(ctx, t, conn, "public", "stream_rows", "stream_row_id")
	if last != StreamRows || !called {
		t.Errorf("stream_rows_stream_row_id_seq is at (%d, is_called=%v), want (%d, true)", last, called, StreamRows)
	}
}

// nastyColumnTypes is every column in nasty.sql whose type is the trap, keyed
// by schema.table.column and holding format_type's spelling.
var nastyColumnTypes = map[string]string{
	// Types (testdata/README.md traps 13 to 18).
	"public.people.status":                  "account_status",
	"public.people.display_name":            "text",
	"public.people.email_verified":          "boolean",
	"public.people.ref":                     "text",
	"public.people.alt_emails":              "text[]",
	"public.people.contact":                 "jsonb",
	"public.events.payload":                 "jsonb",
	"public.tenant_user_sessions.client_ip": "inet",
	// Keys that are not integers (trap 10). These are the types that decide
	// which Chunk encoding and which cast the plan uses.
	"public.sites.site_code":           "text",
	"public.devices.device_id":         "uuid",
	"public.device_readings.device_id": "uuid",
	"public.device_readings.taken_at":  "timestamp with time zone",
}

func assertNastyTypes(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	for _, key := range sortedKeys(nastyColumnTypes) {
		parts := strings.SplitN(key, ".", 3)
		got := scanString(ctx, t, conn, `
			SELECT format_type(a.atttypid, a.atttypmod)
			FROM pg_attribute a
			JOIN pg_class c ON c.oid = a.attrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = $1 AND c.relname = $2 AND a.attname = $3`,
			parts[0], parts[1], parts[2])
		if got != nastyColumnTypes[key] {
			t.Errorf("%s is %s, want %s", key, got, nastyColumnTypes[key])
		}
	}

	// The enum itself, not just a column that uses it.
	if got := scanString(ctx, t, conn, `
		SELECT t.typtype FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'public' AND t.typname = 'account_status'`); got != "e" {
		t.Errorf("public.account_status has typtype %q, want \"e\"", got)
	}
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid
		WHERE t.typname = 'account_status'`); n != 4 {
		t.Errorf("public.account_status has %d labels, want 4", n)
	}

	// The generated column, which the loader must not name in a COPY column
	// list. attgenerated 's' is STORED; '' is an ordinary column.
	if got := scanString(ctx, t, conn, `
		SELECT a.attgenerated FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'people' AND a.attname = 'display_name'`); got != "s" {
		t.Errorf("people.display_name has attgenerated %q, want \"s\" (GENERATED ALWAYS ... STORED)", got)
	}

	// Personal data two levels down in the JSON, below the one level of key
	// collection the v1 rule pack reaches.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM public.people WHERE contact #>> '{profile,contact,email}' IS NOT NULL`); n != 5 {
		t.Errorf("%d of 5 people have contact.profile.contact.email; the JSON depth trap is gone", n)
	}
	// The array trap: three of five rows have values, two are NULL.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM public.people WHERE alt_emails IS NOT NULL`); n != 3 {
		t.Errorf("%d of 5 people have alt_emails, want 3", n)
	}
}

func assertNastyStructure(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	// Trap 1: the self-referencing foreign key, which is the shape pagila does
	// not have.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_constraint
		WHERE contype = 'f' AND conrelid = 'public.people'::regclass AND confrelid = conrelid`); n != 1 {
		t.Errorf("people has %d self-referencing foreign keys, want 1 (manager_id)", n)
	}

	// Traps 4 and 5: the two composite foreign keys, and their match types.
	// 's' is MATCH SIMPLE (a NULL component references nothing); 'f' is MATCH
	// FULL (all NULL or none). ForeignKey.MatchFull is a field in
	// ARCHITECTURE.md section 2 and this is the only fixture for it.
	composite := map[string]string{
		"tenant_user_flags_tenant_user_fkey":          "f",
		"tenant_user_sessions_tenant_id_user_id_fkey": "s",
	}
	for _, name := range sortedKeys(composite) {
		var cols int
		var match string
		if err := conn.QueryRow(ctx, `
			SELECT array_length(conkey, 1), confmatchtype FROM pg_constraint
			WHERE contype = 'f' AND conname = $1`, name).Scan(&cols, &match); err != nil {
			t.Errorf("looking for the composite foreign key %s: %v", name, err)
			continue
		}
		if cols != 2 {
			t.Errorf("%s covers %d columns, want 2", name, cols)
		}
		if match != composite[name] {
			t.Errorf("%s has confmatchtype %q, want %q", name, match, composite[name])
		}
	}

	// The partially-NULL child row MATCH SIMPLE is about: it references no
	// tenant_users row at all, so the parent step must select nothing for it.
	if notNull := scanBool(ctx, t, conn, `
		SELECT a.attnotnull FROM pg_attribute a
		WHERE a.attrelid = 'public.tenant_user_sessions'::regclass AND a.attname = 'user_id'`); notNull {
		t.Error("tenant_user_sessions.user_id is NOT NULL; the MATCH SIMPLE partial-NULL case has no fixture")
	}
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM public.tenant_user_sessions WHERE tenant_id IS NOT NULL AND user_id IS NULL`); n != 1 {
		t.Errorf("%d sessions have a tenant and no user, want 1", n)
	}
	// The all-NULL row MATCH FULL allows.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM public.tenant_user_flags WHERE tenant_id IS NULL AND user_id IS NULL`); n != 1 {
		t.Errorf("%d flags have both referencing columns NULL, want 1", n)
	}

	// Trap 7: the partitioned root and its two leaves, with events_2024 the
	// larger, which is what makes "sample the largest partition" have one
	// answer. reltuples is set by the ANALYZE at the end of the fixture.
	if !scanBool(ctx, t, conn, `
		SELECT EXISTS (SELECT 1 FROM pg_partitioned_table WHERE partrelid = 'public.events'::regclass)`) {
		t.Error("public.events is not partitioned")
	}
	leaves := scanStrings(ctx, t, conn, `
		SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'public.events'::regclass
		ORDER BY c.reltuples DESC, c.relname`)
	if strings.Join(leaves, ",") != "events_2024,events_2025" {
		t.Errorf("events leaves by reltuples are %v, want [events_2024 events_2025]", leaves)
	}

	// Trap 8: the cross-schema edge.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_constraint
		WHERE contype = 'f' AND conrelid = 'billing.invoices'::regclass AND confrelid = 'public.people'::regclass`); n != 1 {
		t.Errorf("billing.invoices has %d foreign keys into public.people, want 1", n)
	}

	// Trap 9: the quoted identifier still needs quoting. Unquoted, this name
	// does not resolve at all.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_attribute
		WHERE attrelid = 'public."LegacyCustomer"'::regclass AND attname = 'EmailAddress'`); n != 1 {
		t.Error(`public."LegacyCustomer"."EmailAddress" is gone; the quoting trap is not a trap`)
	}

	// Traps 11 and 12: the two rungs of the identity ladder below a primary
	// key, and the table where there is no rung left and the run must refuse.
	assertIdentityLadder(ctx, t, conn)
}

// assertIdentityLadder covers the two rungs of ARCHITECTURE.md section 3.4
// below a primary key: a table whose identity must come from a unique index,
// and a table where there is no identity to find and the run must refuse.
func assertIdentityLadder(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	for _, table := range []string{"public.audit_log", "public.click_stream"} {
		if n := scanInt(ctx, t, conn, `
			SELECT count(*) FROM pg_constraint WHERE contype = 'p' AND conrelid = $1::regclass`, table); n != 0 {
			t.Errorf("%s has a primary key; it is the fixture for a table without one", table)
		}
	}

	// audit_log: exactly one unique index that is neither partial nor on an
	// expression, plus one of each kind that must be passed over.
	kinds := map[string]string{}
	rows, err := conn.Query(ctx, `
		SELECT ci.relname,
		       CASE WHEN ix.indpred IS NOT NULL THEN 'partial'
		            WHEN ix.indexprs IS NOT NULL THEN 'expression'
		            ELSE 'plain' END
		FROM pg_index ix
		JOIN pg_class ci ON ci.oid = ix.indexrelid
		WHERE ix.indrelid = 'public.audit_log'::regclass AND ix.indisunique
		ORDER BY ci.relname`)
	if err != nil {
		t.Fatalf("listing audit_log indexes: %v", err)
	}
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			t.Fatalf("scanning audit_log indexes: %v", err)
		}
		kinds[name] = kind
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing audit_log indexes: %v", err)
	}
	want := map[string]string{
		"audit_log_entry_uid_key":       "plain",
		"audit_log_lower_entry_uid_key": "expression",
		"audit_log_recent_action_key":   "partial",
	}
	for _, name := range sortedKeys(want) {
		if kinds[name] != want[name] {
			t.Errorf("audit_log unique index %s is %q, want %q", name, kinds[name], want[name])
		}
	}
	if len(kinds) != len(want) {
		t.Errorf("audit_log has %d unique indexes, want %d", len(kinds), len(want))
	}

	// click_stream: no unique index at all, and two rows identical in every
	// column, so no pseudo-key probe can find a candidate either. The required
	// behaviour is exit 12 naming --key, and that is only testable while this
	// table genuinely has no identity.
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_index WHERE indrelid = 'public.click_stream'::regclass AND indisunique`); n != 0 {
		t.Errorf("public.click_stream has %d unique indexes; it is the fixture for a table with no identity", n)
	}
	if n := scanInt(ctx, t, conn, `
		SELECT count(*) - count(DISTINCT (person_id, url, clicked_at)) FROM public.click_stream`); n != 1 {
		t.Errorf("public.click_stream has %d duplicate rows, want 1: a pseudo-key probe would otherwise succeed", n)
	}
}

// nastyIdentity is the table testdata/README.md tabulates: every identity
// column, whether it is GENERATED ALWAYS ('a') or BY DEFAULT ('d'), its START
// and INCREMENT, and the value its sequence will hand out next.
var nastyIdentity = []struct {
	schema, table, column string
	always                bool
	start, increment      int64
	next                  int64
}{
	{"billing", "invoices", "invoice_id", false, 3000, 5, 3015},
	{"public", "LegacyCustomer", "CustomerID", true, 42, 1, 45},
	{"public", "attachments", "attachment_id", false, 800, 13, 852},
	{"public", "orders", "order_id", false, 200000, 3, 200015},
	{"public", "organisations", "organisation_id", false, 500, 3, 506},
	{"public", "people", "person_id", true, 90000, 7, 90035},
	{"public", "projects", "project_id", false, 700, 3, 706},
	{"public", "stream_rows", "stream_row_id", false, 1, 1, 1},
	{"public", "teams", "team_id", false, 600, 3, 606},
	{"public", "tenant_user_flags", "flag_id", false, 9000, 2, 9006},
	{"public", "tenant_user_sessions", "session_id", false, 5000, 11, 5055},
}

// assertNastyIdentity covers README trap 21. Both halves matter: GENERATED
// ALWAYS is what forces OVERRIDING SYSTEM VALUE out of the loader, and the
// sequence's own position is state the load has to carry, or the first insert
// a developer makes into their copy collides with a loaded row.
func assertNastyIdentity(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	if n := scanInt(ctx, t, conn, `
		SELECT count(*) FROM pg_attribute WHERE attidentity <> '' AND attnum > 0`); n != len(nastyIdentity) {
		t.Errorf("nasty.sql has %d identity columns, want %d", n, len(nastyIdentity))
	}

	for _, want := range nastyIdentity {
		name := want.schema + "." + want.table + "." + want.column
		var identity string
		var start, increment int64
		if err := conn.QueryRow(ctx, `
			SELECT a.attidentity, s.seqstart, s.seqincrement
			FROM pg_attribute a
			JOIN pg_class c ON c.oid = a.attrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_sequence s ON s.seqrelid = pg_get_serial_sequence(
			         quote_ident(n.nspname) || '.' || quote_ident(c.relname), a.attname)::regclass
			WHERE n.nspname = $1 AND c.relname = $2 AND a.attname = $3`,
			want.schema, want.table, want.column).Scan(&identity, &start, &increment); err != nil {
			t.Errorf("reading the identity of %s: %v", name, err)
			continue
		}
		wantIdentity := "d"
		if want.always {
			wantIdentity = "a"
		}
		if identity != wantIdentity {
			t.Errorf("%s has attidentity %q, want %q", name, identity, wantIdentity)
		}
		if start != want.start || increment != want.increment {
			t.Errorf("%s starts at %d step %d, want %d step %d", name, start, increment, want.start, want.increment)
		}

		last, called := sequenceState(ctx, t, conn, want.schema, want.table, want.column)
		next := last
		if called {
			next = last + increment
		}
		if next != want.next {
			t.Errorf("%s's sequence will hand out %d next, want %d", name, next, want.next)
		}
	}
}

// assertNastyGateOff checks that a load without big leaves the machinery the
// gate needs behind. TestLoadNastyBig is the other half: without it, a gate
// stuck off passes this test too.
func assertNastyGateOff(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()

	if !scanBool(ctx, t, conn, `
		SELECT EXISTS (SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		               WHERE n.nspname = 'public' AND p.proname = 'fill_stream_rows')`) {
		t.Error("public.fill_stream_rows is missing; the 2,000,000-row gate has nothing to call")
	}
}

// sequenceState returns the last value written to the identity sequence behind
// a column, and whether that value has been handed out.
//
// The sequence relation is read directly because pg_sequences.last_value and
// pg_sequence_last_value() are both NULL after ALTER TABLE ... RESTART WITH,
// which is exactly the state nasty.sql leaves nine of its ten sequences in.
func sequenceState(ctx context.Context, t *testing.T, conn *pgx.Conn, schema, table, column string) (last int64, called bool) {
	t.Helper()

	seq := scanString(ctx, t, conn, `
		SELECT pg_get_serial_sequence(quote_ident($1) || '.' || quote_ident($2), $3)`, schema, table, column)
	if seq == "" {
		t.Fatalf("%s.%s.%s has no sequence", schema, table, column)
	}
	// seq comes back from the catalog already quoted where it needs to be.
	if err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT last_value, is_called FROM %s`, seq)).Scan(&last, &called); err != nil {
		t.Fatalf("reading %s: %v", seq, err)
	}
	return last, called
}

// assertTables compares the tables actually present, and their row counts,
// against want. It reports the whole difference rather than the first one,
// because a fixture that has drifted has usually drifted in several places.
func assertTables(ctx context.Context, t *testing.T, url string, want map[string]int) {
	t.Helper()

	conn := connect(ctx, t, url)

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		ORDER BY n.nspname, c.relname`)
	if err != nil {
		t.Fatalf("listing tables: %v", err)
	}
	type table struct{ schema, name string }
	var found []table
	for rows.Next() {
		var tb table
		if err := rows.Scan(&tb.schema, &tb.name); err != nil {
			t.Fatalf("scanning table list: %v", err)
		}
		found = append(found, tb)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listing tables: %v", err)
	}

	got := make(map[string]int, len(found))
	for _, tb := range found {
		key := tb.schema + "." + tb.name
		var n int
		q := fmt.Sprintf(`SELECT count(*) FROM %s.%s`, quote(tb.schema), quote(tb.name))
		if err := conn.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("counting %s: %v", key, err)
		}
		got[key] = n
	}

	if len(got) != len(want) {
		t.Errorf("loaded %d tables, want %d", len(got), len(want))
	}
	for _, key := range sortedKeys(want) {
		n, ok := got[key]
		switch {
		case !ok:
			t.Errorf("%s is missing", key)
		case n != want[key]:
			t.Errorf("%s has %d rows, want %d", key, n, want[key])
		}
	}
	for _, key := range sortedKeys(got) {
		if _, ok := want[key]; !ok {
			t.Errorf("%s was loaded but is not in the expected set (%d rows)", key, got[key])
		}
	}
}

func connect(ctx context.Context, t *testing.T, url string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(ctx)) })
	return conn
}

func scanInt(ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args ...any) int {
	t.Helper()
	return scanOne[int](ctx, t, conn, query, args...)
}

func scanBool(ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args ...any) bool {
	t.Helper()
	return scanOne[bool](ctx, t, conn, query, args...)
}

func scanString(ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args ...any) string {
	t.Helper()
	return scanOne[string](ctx, t, conn, query, args...)
}

func scanOne[T any](ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args ...any) T {
	t.Helper()

	var v T
	if err := conn.QueryRow(ctx, query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", firstLine(query), err)
	}
	return v
}

func scanStrings(ctx context.Context, t *testing.T, conn *pgx.Conn, query string, args ...any) []string {
	t.Helper()

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("%s: %v", firstLine(query), err)
	}
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("%s: %v", firstLine(query), err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", firstLine(query), err)
	}
	return out
}

func quote(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
