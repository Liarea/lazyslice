// SPDX-License-Identifier: Apache-2.0

//go:build integration

package plan

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/mask"
)

// Which rows a plan holds is a statement about a real Postgres, so these run
// against the two fixtures in testdata/. testdata/README.md is the
// specification: each trap the planner is responsible for has a subtest named
// for it.
//
// Every run over nasty.sql carries --skip-table public.click_stream, because
// trap 12's table has no row identity at all and §3.4 ends in exit 12 for it.
// The refusal itself is asserted in Trap12.

// The reader every test here plans through is a real internal/pg.Source with
// this package's shapes and introspect's registered on its allowlist, so the
// suite also proves that every statement the walk sends matches a registered
// shape: Source.Violation is checked when the test ends (THREAT_MODEL.md T9).
// A statement no shape covers does not merely get logged — the tracer hands the
// call a cancelled context and the plan fails.

// allowlist is the shapes the planner and the introspection it runs first send.
func allowlist(t *testing.T) []pg.Shape {
	t.Helper()
	shapes := make([]pg.Shape, 0, len(introspect.Shapes())+len(Shapes()))
	for _, s := range introspect.Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	return shapes
}

// fixture loads a fixture, opens one snapshot over it and introspects it.
func fixture(ctx context.Context, t *testing.T, load func(context.Context, string) error) (pipeline.Reader, *pipeline.Schema) {
	t.Helper()
	r, schema, _ := fixtureURL(ctx, t, load)
	return r, schema
}

// fixtureURL is fixture, and also the connection URL of the container it
// started. Only the write-back suite needs it: it reads a batch of real rows
// back through a connection of its own, because the reader above answers only
// statements on the source's allowlist and "one batch of whatever this table
// holds" is not a shape the planner sends (writeback_integration_test.go).
func fixtureURL(ctx context.Context, t *testing.T, load func(context.Context, string) error) (pipeline.Reader, *pipeline.Schema, string) {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	if err := load(ctx, url); err != nil {
		t.Fatalf("loading the fixture: %v", err)
	}
	analyse(ctx, t, url)

	src, err := pg.OpenSource(ctx, dsn.DSN(url), allowlist(t)...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)

	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader on the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(context.WithoutCancel(ctx)) })
	// Cleanups run last-registered first, so this one runs while the reader and
	// the snapshot are still open, which is where a refusal would have happened.
	t.Cleanup(func() {
		if refused := src.Violation(); refused != nil {
			t.Errorf("the source allowlist refused a statement the planner sent: %v", refused)
		}
	})

	schema, err := introspect.New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	return r, schema, url
}

// analyse fills pg_class.reltuples, which is -1 until something analyses: both
// the root default and the lookup probe read it. It runs on a connection of its
// own, because ANALYZE is not a statement the source's allowlist carries and
// not one a READ ONLY transaction would take.
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

func tref(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

// containsKey says whether a step's integer key set holds one key.
func containsKey(keys []int64, want int64) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

// stepsByTable indexes a plan's steps.
func stepsByTable(p *pipeline.Plan) map[ref.TableRef]pipeline.Step {
	out := make(map[ref.TableRef]pipeline.Step, len(p.Steps))
	for _, s := range p.Steps {
		out[s.Table] = s
	}
	return out
}

// intKeysOf reads a step's key set back as sorted integers, for a single-column
// integer identity.
func intKeysOf(t *testing.T, s pipeline.Step) []int64 {
	t.Helper()
	var out []int64
	if s.Keys == nil {
		return out
	}
	for _, c := range s.Keys.Chunks(chunkSize) {
		col, ok := c.Column(0).([]int64)
		if !ok {
			t.Fatalf("%s: key column 0 is %T, want []int64", s.Table, c.Column(0))
		}
		out = append(out, col...)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// stringKeysOf reads a step's key set back as sorted strings, for a
// single-column text, varchar, bpchar or citext identity.
func stringKeysOf(t *testing.T, s pipeline.Step) []string {
	t.Helper()
	var out []string
	if s.Keys == nil {
		return out
	}
	for _, c := range s.Keys.Chunks(chunkSize) {
		col, ok := c.Column(0).([]string)
		if !ok {
			t.Fatalf("%s: key column 0 is %T, want []string", s.Table, c.Column(0))
		}
		out = append(out, col...)
	}
	sort.Strings(out)
	return out
}

// uuidKeysOf reads a step's key set back as sorted uuid text, for a
// single-column uuid identity.
func uuidKeysOf(t *testing.T, s pipeline.Step) []string {
	t.Helper()
	var out []string
	if s.Keys == nil {
		return out
	}
	for _, c := range s.Keys.Chunks(chunkSize) {
		col, ok := c.Column(0).([]pgtype.UUID)
		if !ok {
			t.Fatalf("%s: key column 0 is %T, want []pgtype.UUID", s.Table, c.Column(0))
		}
		for _, u := range col {
			text, err := u.MarshalJSON()
			if err != nil {
				t.Fatalf("%s: rendering a uuid key: %v", s.Table, err)
			}
			out = append(out, strings.Trim(string(text), `"`))
		}
	}
	sort.Strings(out)
	return out
}

// digest renders a plan as text, so that "two runs produce byte-identical
// plans" is a comparison of everything the plan says and not of a pointer.
func digest(p *pipeline.Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "root %s (%s) take %d\n", p.Root, p.RootReason, p.Take)
	for _, s := range p.Steps {
		fmt.Fprintf(&b, "step %s mode=%d identity=%d%v cap=%d depth=%d why=%q\n",
			s.Table, s.Mode, s.Identity.Kind, s.Identity.Columns, s.Cap, s.Depth, s.Why)
		if s.Keys == nil {
			continue
		}
		fmt.Fprintf(&b, "  keys=%d bytes=%d\n", s.Keys.Len(), s.Keys.Bytes())
		for i, c := range s.Keys.Chunks(chunkSize) {
			for col := 0; col < len(s.Identity.Columns); col++ {
				fmt.Fprintf(&b, "  chunk %d col %d %s %v\n", i, col, c.Cast(col), c.Column(col))
			}
		}
	}
	for _, c := range p.SCCs {
		fmt.Fprintf(&b, "scc %v\n", c)
	}
	for _, u := range p.Polymorphic {
		fmt.Fprintf(&b, "polymorphic %s\n", u)
	}
	for _, u := range p.Unmapped {
		fmt.Fprintf(&b, "unmapped %s\n", u)
	}
	for _, u := range p.Unindexed {
		fmt.Fprintf(&b, "unindexed %s\n", u.Name)
	}
	fmt.Fprintf(&b, "skipped %v unreadable %v\n", p.Skipped, p.Unreadable)
	fmt.Fprintf(&b, "estimate rows=%d bytes=%d key=%d filter=%d\n",
		p.Estimate.Rows, p.Estimate.Bytes, p.Estimate.KeyMemory, p.Estimate.FilterMemory)
	return b.String()
}

// ---------- nasty.sql ----------

func nastyRequest(root ref.TableRef) pipeline.PlanRequest {
	r := root
	return pipeline.PlanRequest{
		Root: &r,
		// Trap 12: click_stream has no row identity at all, and every run over
		// this fixture carries the flag that drops it (testdata/README.md).
		Skip: []ref.TableRef{tref("public", "click_stream")},
	}
}

// loadNasty is testutil.LoadNasty(ctx, url, false): trap 25's
// price_list_notes.list_id -> price_lists_eu (list_id) edge is
// ForeignKey.NotRecreatable by design, and checkRecreatable refuses any Plan
// call over a schema carrying one — unconditionally, before a root is even
// chosen (ARCHITECTURE.md §11.1, exit 13) — so testutil.LoadNasty leaves that
// edge out of every load (testdata/nasty.sql's `notrecreatable` gate).
// TestPlanNastyNotRecreatable is the one test that wants the edge present and
// loads through testutil.LoadNastyNotRecreatable instead; every other
// nasty.sql test here uses this loader.
func loadNasty(ctx context.Context, url string) error {
	return testutil.LoadNasty(ctx, url, false)
}

func TestPlanNasty(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNasty)
	people := tref("public", "people")

	req := nastyRequest(people)
	req.Take = 500
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	// Trap 2: a three-table foreign-key cycle. The walk terminates, all three
	// tables are in the slice, and the component is reported.
	t.Run("Trap2_ThreeTableCycleTerminates", func(t *testing.T) {
		for _, name := range []string{"organisations", "teams", "projects"} {
			s, ok := steps[tref("public", name)]
			if !ok {
				t.Fatalf("public.%s has no step", name)
			}
			if s.Mode == pipeline.SchemaOnly {
				t.Errorf("public.%s is SchemaOnly; the cycle is reached from the root through organisations.founded_by", name)
			}
			if s.Keys == nil || s.Keys.Len() == 0 {
				t.Errorf("public.%s selected no row", name)
			}
		}
		want := []ref.TableRef{tref("public", "organisations"), tref("public", "projects"), tref("public", "teams")}
		found := false
		for _, c := range p.SCCs {
			if fmt.Sprint(c) == fmt.Sprint(want) {
				found = true
			}
		}
		if !found {
			t.Errorf("SCCs = %v, want one of them to be %v", p.SCCs, want)
		}
	})

	// Trap 3: the two-table cycle is a component of its own, and people's own
	// self-reference puts it in one.
	t.Run("Trap3_TwoTableCycleIsReported", func(t *testing.T) {
		want := []ref.TableRef{tref("public", "orders"), tref("public", "people")}
		found := false
		for _, c := range p.SCCs {
			if fmt.Sprint(c) == fmt.Sprint(want) {
				found = true
			}
		}
		if !found {
			t.Errorf("SCCs = %v, want one of them to be %v", p.SCCs, want)
		}
	})

	// Trap 4: a composite foreign key, and the MATCH SIMPLE rule under it.
	// Session 5044 has a tenant and no user, so it references no tenant_users
	// row and its parent edge selects nothing.
	t.Run("Trap4_CompositeForeignKey", func(t *testing.T) {
		tu := steps[tref("public", "tenant_users")]
		if tu.Keys == nil || tu.Keys.Len() != 4 {
			t.Fatalf("tenant_users selected %v rows, want all 4", keyLen(tu))
		}
		if len(tu.Identity.Columns) != 2 {
			t.Fatalf("tenant_users identity is %v, want the composite primary key", tu.Identity.Columns)
		}
		chunks := tu.Keys.Chunks(chunkSize)
		if len(chunks) != 1 {
			t.Fatalf("tenant_users keys came back in %d chunks, want 1", len(chunks))
		}
		if got := chunks[0].Cast(0); got != "::int8[]" {
			t.Errorf("tenant_users key column 0 casts %q, want ::int8[]", got)
		}
		sessions := steps[tref("public", "tenant_user_sessions")]
		got := intKeysOf(t, sessions)
		want := []int64{5000, 5011, 5022, 5033}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("tenant_user_sessions = %v, want %v: session 5044 has a NULL user_id and references no tenant_users row", got, want)
		}
	})

	// Trap 6: the polymorphic pair is detected, both of its `_type` values are
	// mapped, and each mapping is one parent-direction virtual edge (§3.2).
	// attachments.owner_type holds 'people' and 'projects', which name
	// public.people and public.projects, so nothing about this pair is left
	// unresolved and neither the "not followed" line nor the unmapped list has
	// anything to say.
	t.Run("Trap6_PolymorphicPairIsInferredAndFollowed", func(t *testing.T) {
		// The name is one identifier — the discriminator column the inference
		// read — and never the `_type` value it resolved from, because
		// internal/emit copies this list into lazyslice.yml and §10 admits an
		// identifier, a count, a fingerprint or a flag value into that file and
		// nothing else. It stops at the discriminator because emit renders the
		// name in front of the reconstructed edge, so a name that also carried
		// the parent printed the arrow clause twice; the parent is read off the
		// edge, which is where the rendered line states it.
		want := []string{
			`public.attachments.owner_type -> public.people`,
			`public.attachments.owner_type -> public.projects`,
		}
		var got []string
		for _, fk := range p.Virtual {
			if !fk.Virtual {
				t.Errorf("Virtual carries %s with Virtual unset", fk.Name)
			}
			if fk.Validated {
				t.Errorf("%s is Validated; there is no constraint behind an inferred edge", fk.Name)
			}
			if fk.Name != `public.attachments.owner_type` {
				t.Errorf("Virtual carries the name %q, want the discriminator column alone", fk.Name)
			}
			if fk.Child != tref("public", "attachments") || fmt.Sprint(fk.ChildCols) != "[owner_id]" {
				t.Errorf("%s is on %s %v, want public.attachments [owner_id]", fk.Name, fk.Child, fk.ChildCols)
			}
			got = append(got, fk.Name+" -> "+fk.Parent.String())
		}
		sort.Strings(got)
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("Virtual = %v, want %v", got, want)
		}
		if len(p.Polymorphic) != 0 {
			t.Errorf("Polymorphic = %v, want none: both _type values resolved", p.Polymorphic)
		}
		if len(p.Unmapped) != 0 {
			t.Errorf("Unmapped = %v, want none: both _type values name a table", p.Unmapped)
		}

		// The edge is followed, and the row it reaches is in the slice.
		// attachment 826 is `owner_type = 'projects'`, `owner_id = 700`, so
		// public.projects holds project 700 through an edge PostgreSQL knows
		// nothing about (research/COMPLAINTS.md FK-10 is the silent version of
		// this, and the whole point of the trap).
		projects := steps[tref("public", "projects")]
		if !containsKey(intKeysOf(t, projects), 700) {
			t.Errorf("public.projects = %v, want it to hold 700: attachment 826 points at it through the pair",
				intKeysOf(t, projects))
		}

		// And it is followed in the parent direction only. attachment 839 is
		// the dangling owner — `owner_type = 'people'`, `owner_id = 99999`,
		// which is no person — and its uploaded_by_person_id is NULL, so no
		// declared edge reaches it either. A virtual edge walked as a child
		// would put it in the slice.
		//
		// What this cannot say is what happens to a dangling id that *is* read:
		// 839 is outside the slice, so its pair never reaches the parent step
		// at all. That case has a fixture of its own
		// (TestPlanVirtualEdgeToADanglingRow); asserting it here would be an
		// assertion that cannot fail.
		attachments := steps[tref("public", "attachments")]
		if attachments.Mode != pipeline.ChildOK {
			t.Errorf("public.attachments mode = %v, want ChildOK", attachments.Mode)
		}
		if containsKey(intKeysOf(t, attachments), 839) {
			t.Errorf("public.attachments = %v, want 839 left out: a virtual edge is never followed as a child",
				intKeysOf(t, attachments))
		}

		// attachment 845 is `owner_type = 'Person'`, the Rails-spelled class
		// name, resolved through the first form tried — underscore and
		// pluralise, 'Person' -> 'people' — rather than through the
		// raw-table-name fallback 'people'/'projects' resolve through. It is
		// selected through its own declared uploaded_by_person_id edge, so
		// its presence alone does not prove the resolution; what proves it is
		// the absence of a third finding: both Polymorphic and Unmapped are
		// asserted empty above, and a 'Person' value neither form resolved
		// would have added one.
		if !containsKey(intKeysOf(t, attachments), 845) {
			t.Errorf("public.attachments = %v, want 845 included: uploaded_by_person_id reaches it",
				intKeysOf(t, attachments))
		}
	})

	// The trap testdata/nasty.sql calls "keys that are not integers": one table
	// per branch of the chunk encoding, each with a child to follow the edge
	// into. Getting a cast wrong here is silent — unnest($1::text[]) joined
	// against a uuid column matches nothing — so the assertion is the selected
	// rows themselves and the cast each key column travels under, not merely
	// that the tables have steps.
	t.Run("KeyKindsText", func(t *testing.T) {
		s := steps[tref("public", "sites")]
		if s.Identity.Kind != pipeline.IdentityPK || fmt.Sprint(s.Identity.Columns) != "[site_code]" {
			t.Fatalf("sites identity = %d %v, want the text primary key", s.Identity.Kind, s.Identity.Columns)
		}
		if got := fmt.Sprint(stringKeysOf(t, s)); got != "[SITE-LDN SITE-NYC]" {
			t.Errorf("sites = %s, want both rows: a text key that casts wrong selects none", got)
		}
		if got := s.Keys.Chunks(chunkSize)[0].Cast(0); got != "::text[]" {
			t.Errorf("sites key column 0 casts %q, want ::text[]", got)
		}
	})

	t.Run("KeyKindsUUID", func(t *testing.T) {
		s := steps[tref("public", "devices")]
		want := "[11111111-2222-4333-8444-555555555551 11111111-2222-4333-8444-555555555552 11111111-2222-4333-8444-555555555553]"
		if got := fmt.Sprint(uuidKeysOf(t, s)); got != want {
			t.Errorf("devices = %s, want %s", got, want)
		}
		if got := s.Keys.Chunks(chunkSize)[0].Cast(0); got != "::uuid[]" {
			t.Errorf("devices key column 0 casts %q, want ::uuid[]: a uuid key sent as text matches nothing", got)
		}
	})

	t.Run("KeyKindsCompositeUUIDAndTimestamp", func(t *testing.T) {
		s := steps[tref("public", "device_readings")]
		if fmt.Sprint(s.Identity.Columns) != "[device_id taken_at]" {
			t.Fatalf("device_readings identity = %v, want (device_id, taken_at)", s.Identity.Columns)
		}
		if keyLen(s) != 4 {
			t.Fatalf("device_readings selected %d rows, want all 4", keyLen(s))
		}
		ch := s.Keys.Chunks(chunkSize)[0]
		if got := ch.Cast(0); got != "::uuid[]" {
			t.Errorf("device_readings key column 0 casts %q, want ::uuid[]", got)
		}
		// timestamptz has no chunk type of its own: it travels as its text form
		// and the join casts it back to the column's type.
		if got := ch.Cast(1); got != "::text[]" {
			t.Errorf("device_readings key column 1 casts %q, want ::text[]", got)
		}
		if _, ok := ch.Column(1).([]string); !ok {
			t.Errorf("device_readings key column 1 is %T, want []string", ch.Column(1))
		}
	})

	// Trap 11: audit_log has no primary key and three unique indexes, only one
	// of which is a legal identity.
	t.Run("Trap11_UniqueIndexIdentity", func(t *testing.T) {
		s := steps[tref("public", "audit_log")]
		if s.Identity.Kind != pipeline.IdentityUnique {
			t.Errorf("audit_log identity kind = %d, want IdentityUnique", s.Identity.Kind)
		}
		if fmt.Sprint(s.Identity.Columns) != "[entry_uid]" {
			t.Errorf("audit_log identity = %v, want [entry_uid]", s.Identity.Columns)
		}
	})

	// Trap 12: without --skip-table, the table with no row identity stops the
	// run at plan with exit 12.
	t.Run("Trap12_NoIdentityRefuses", func(t *testing.T) {
		bare := pipeline.PlanRequest{Root: &people, Take: 500}
		_, err := New().Plan(ctx, r, schema, nil, bare)
		var refusal *Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("Plan returned %v, want a refusal for public.click_stream", err)
		}
		if refusal.Code != CodeNoIdentity || refusal.Exit != 12 {
			t.Fatalf("refusal = %s exit %d, want %s exit 12", refusal.Code, refusal.Exit, CodeNoIdentity)
		}
		if refusal.Table != tref("public", "click_stream") {
			t.Errorf("refusal names %s, want public.click_stream", refusal.Table)
		}
	})

	// Trap 12's documented bypass, which must not be one: an explicit key that
	// does not identify a row is refused like any other candidate (T-0032).
	t.Run("Trap12_ExplicitKeyIsProbed", func(t *testing.T) {
		req := pipeline.PlanRequest{
			Root: &people,
			Take: 500,
			Keys: map[ref.TableRef][]string{
				tref("public", "click_stream"): {"person_id", "url", "clicked_at"},
			},
		}
		_, err := New().Plan(ctx, r, schema, nil, req)
		var refusal *Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("Plan returned %v, want a refusal: the first two click_stream rows are identical", err)
		}
		if refusal.Code != CodeKeyNotUnique {
			t.Errorf("refusal = %s, want %s", refusal.Code, CodeKeyNotUnique)
		}
	})

	// There is no ctid rung (ADR-005).
	t.Run("NoCtidRung", func(t *testing.T) {
		req := nastyRequest(people)
		req.Keys = map[ref.TableRef][]string{tref("public", "audit_log"): {"ctid"}}
		_, err := New().Plan(ctx, r, schema, nil, req)
		var refusal *Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("Plan returned %v, want a refusal for --key audit_log=ctid", err)
		}
		if refusal.Code != CodeKeyColumn {
			t.Errorf("refusal = %s, want %s", refusal.Code, CodeKeyColumn)
		}
	})

	// The skipped table is a step, and it is schema only.
	t.Run("SkippedTableIsSchemaOnly", func(t *testing.T) {
		s := steps[tref("public", "click_stream")]
		if s.Mode != pipeline.SchemaOnly {
			t.Errorf("click_stream mode = %d, want SchemaOnly", s.Mode)
		}
		if s.Why != "skipped" {
			t.Errorf("click_stream why = %q, want %q", s.Why, "skipped")
		}
		if len(p.Skipped) != 1 || p.Skipped[0] != tref("public", "click_stream") {
			t.Errorf("Skipped = %v, want [public.click_stream]", p.Skipped)
		}
	})

	// A partition is never a step; its root is (§3.3).
	t.Run("PartitionsAreNotSteps", func(t *testing.T) {
		for _, name := range []string{"events_2024", "events_2025"} {
			if _, ok := steps[tref("public", name)]; ok {
				t.Errorf("public.%s is a step; a partition is never one", name)
			}
		}
		if _, ok := steps[tref("public", "events")]; !ok {
			t.Error("public.events has no step; the partitioned root is the step")
		}
	})

	// Load order: a parent comes before its child wherever no cycle forces
	// otherwise, and every table appears exactly once.
	t.Run("LoadOrderCoversEveryTableOnce", func(t *testing.T) {
		seen := map[ref.TableRef]int{}
		for _, s := range p.Steps {
			seen[s.Table]++
		}
		for tbl, n := range seen {
			if n != 1 {
				t.Errorf("%s appears %d times in the load order", tbl, n)
			}
		}
		if len(p.Steps) != len(seen) {
			t.Errorf("%d steps over %d tables", len(p.Steps), len(seen))
		}
	})

	// The estimate is filled in, and the key memory is the sum of the key sets'
	// own Bytes (§2: KeySet.Bytes is the single source of truth).
	t.Run("EstimateIsFilled", func(t *testing.T) {
		var want int64
		var rows int64
		for _, s := range p.Steps {
			if s.Keys != nil {
				want += s.Keys.Bytes()
				rows += int64(s.Keys.Len())
			}
		}
		if p.Estimate.KeyMemory != want {
			t.Errorf("Estimate.KeyMemory = %d, want the sum of Bytes() = %d", p.Estimate.KeyMemory, want)
		}
		if p.Estimate.Rows < rows {
			t.Errorf("Estimate.Rows = %d, want at least the %d selected rows", p.Estimate.Rows, rows)
		}
		if p.Estimate.Bytes <= 0 || p.Estimate.HoldSeconds <= 0 {
			t.Errorf("Estimate = %+v, want a byte and hold estimate", p.Estimate)
		}
	})

	// Two runs over one snapshot produce byte-identical plans (§3
	// "Determinism"). This is what makes lazyslice.yml a record of what
	// happened (ADR-004).
	t.Run("TwoRunsAreIdentical", func(t *testing.T) {
		req := nastyRequest(people)
		req.Take = 500
		first, err := New().Plan(ctx, r, schema, nil, req)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		second, err := New().Plan(ctx, r, schema, nil, req)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if digest(first) != digest(second) {
			t.Errorf("two runs over one snapshot differ:\n--- first\n%s\n--- second\n%s", digest(first), digest(second))
		}
	})
}

// Trap 25: a partitioned root's own key is DEFERRABLE, a leaf carries a key
// the root cannot hold, and public.price_list_notes.list_id references the
// leaf. Introspect leaves that edge un-re-pointed with
// ForeignKey.NotRecreatable set (asserted in
// introspect_integration_test.go's TestIntrospectNasty/Trap25); this is the
// planner's half, over the same catalog rather than a hand-built schema
// (TestNotRecreatableForeignKeyIsRefusedBeforeAnyRead in plan_test.go is the
// seam that has no fixture).
//
// The refusal must come before any statement reaches the source: schema is
// already introspected, so the Plan call here is given refusingReader, which
// fails the test if the walk sends anything at all (ARCHITECTURE.md §11.1:
// "before the snapshot is used for keys and before anything in the target is
// dropped").
func TestPlanNastyNotRecreatable(t *testing.T) {
	ctx := context.Background()
	// LoadNastyNotRecreatable, not loadNasty: this is the one test that wants
	// trap 25's edge present.
	_, schema := fixture(ctx, t, testutil.LoadNastyNotRecreatable)

	notes := tref("public", "price_list_notes")
	leaf := tref("public", "price_lists_eu")
	var fk *pipeline.ForeignKey
	for i := range schema.FKs {
		// price_list_notes also carries person_id -> public.people, an
		// ordinary edge that connects it to the root independently of this
		// trap (testdata/README.md trap 25); the trap edge is the one over
		// list_id.
		if schema.FKs[i].Child == notes && len(schema.FKs[i].ChildCols) == 1 && schema.FKs[i].ChildCols[0] == "list_id" {
			fk = &schema.FKs[i]
		}
	}
	if fk == nil {
		t.Fatal("no edge from public.price_list_notes.list_id in the introspected schema")
	}
	if !fk.NotRecreatable {
		t.Fatalf("%s.NotRecreatable = false, want true: price_lists' own key is DEFERRABLE and cannot back this edge", fk.Name)
	}
	if fk.Parent != leaf {
		t.Errorf("%s parent = %s, want %s: an edge introspect cannot re-point stays on the leaf", fk.Name, fk.Parent, leaf)
	}

	req := nastyRequest(tref("public", "people"))
	req.Take = 500
	_, err := New().Plan(ctx, refusingReader{t}, schema, nil, req)
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v (%T), want a *Refusal", err, err)
	}
	if refusal.Code != CodeNotRecreatable {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeNotRecreatable)
	}
	if refusal.Exit != 13 {
		t.Errorf("Exit = %d, want 13", refusal.Exit)
	}
	if refusal.Table != notes {
		t.Errorf("refusal names %s, want %s", refusal.Table, notes)
	}
	if !strings.Contains(refusal.Message, fk.Name) {
		t.Errorf("refusal message = %q, want it to name %s", refusal.Message, fk.Name)
	}
}

// Trap 1: the self-referencing foreign key. A walk with no visited set never
// terminates here; lazyslice expands the manager chain once and stops.
func TestPlanNastySelfReference(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNasty)

	people := tref("public", "people")
	req := nastyRequest(people)
	req.Take = 1
	req.Where = "person_id = 90021"
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	got := intKeysOf(t, steps[people])
	// Katherine (90021), her manager Grace (90007), and Grace's manager Ada
	// (90000). The chain closes; nothing else pulls a fourth person in.
	want := []int64{90000, 90007, 90021}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("people = %v, want %v", got, want)
	}
	if steps[people].Mode != pipeline.ChildOK {
		t.Errorf("people mode = %d, want ChildOK: it is the root", steps[people].Mode)
	}
}

// Trap 4 from the other side: rooting at the sessions makes the parent step's
// MATCH SIMPLE rule observable. Session 5044 has a tenant and no user, so it
// must not pull tenant 2's other users in.
func TestPlanNastyCompositeParentStep(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNasty)

	sessions := tref("public", "tenant_user_sessions")
	req := nastyRequest(sessions)
	req.Take = 500
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	if got := len(intKeysOf(t, steps[sessions])); got != 5 {
		t.Fatalf("tenant_user_sessions selected %d rows, want all 5", got)
	}
	tu := steps[tref("public", "tenant_users")]
	if tu.Mode != pipeline.ParentOnly {
		t.Errorf("tenant_users mode = %d, want ParentOnly", tu.Mode)
	}
	pairs := compositeKeys(t, tu)
	want := "[[1 1] [1 2] [2 1] [2 7]]"
	if fmt.Sprint(pairs) != want {
		t.Errorf("tenant_users = %v, want %s: session 5044's NULL user_id references nothing", pairs, want)
	}
	// tenant_users is PARENT_ONLY, so its own children are not pulled through:
	// tenant_user_flags is reachable only as its child.
	if s := steps[tref("public", "tenant_user_flags")]; s.Mode != pipeline.SchemaOnly {
		t.Errorf("tenant_user_flags mode = %d, want SchemaOnly: its parent is PARENT_ONLY", s.Mode)
	}
}

// compositeKeys reads a two-column integer key set back as pairs.
func compositeKeys(t *testing.T, s pipeline.Step) [][2]int64 {
	t.Helper()
	var out [][2]int64
	if s.Keys == nil {
		return out
	}
	for _, c := range s.Keys.Chunks(chunkSize) {
		a, ok := c.Column(0).([]int64)
		if !ok {
			t.Fatalf("%s: key column 0 is %T, want []int64", s.Table, c.Column(0))
		}
		b, ok := c.Column(1).([]int64)
		if !ok {
			t.Fatalf("%s: key column 1 is %T, want []int64", s.Table, c.Column(1))
		}
		for i := range a {
			out = append(out, [2]int64{a[i], b[i]})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// ---------- pagila ----------

// pagilaPersonal is the columns lazyslice masks in pagila, from the hand
// labelling in internal/classify/pagila_test.go. The planner reads a
// classification for one thing only — a lookup table with personal data in it
// is walked rather than copied whole (§3) — so the test states the labelling
// rather than depending on the classifier's own precision.
var pagilaPersonal = []struct {
	table  string
	column string
}{
	{"actor", "first_name"}, {"actor", "last_name"},
	{"address", "address"}, {"address", "address2"}, {"address", "district"},
	{"address", "postal_code"}, {"address", "phone"},
	{"customer", "first_name"}, {"customer", "last_name"}, {"customer", "email"},
	{"staff", "first_name"}, {"staff", "last_name"}, {"staff", "email"},
	{"staff", "username"}, {"staff", "password"}, {"staff", "picture"},
}

func pagilaClassification() *pipeline.Classification {
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}
	for _, c := range pagilaPersonal {
		col := ref.ColumnRef{Table: tref("public", c.table), Column: c.column}
		cls.Decisions[col] = pipeline.Decision{
			Col:        col,
			Confidence: pipeline.ConfCertain,
			Masked:     true,
		}
	}
	return cls
}

// Pagila from customer with take 50: the expected table set, mode by mode.
func TestPlanPagilaFromCustomer(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, testutil.LoadPagila)

	customer := tref("public", "customer")
	p, err := New().Plan(ctx, r, schema, pagilaClassification(), pipeline.PlanRequest{
		Root: &customer,
		Take: 50,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	want := map[string]pipeline.Mode{
		// The root and what hangs off it.
		"customer": pipeline.ChildOK,
		"payment":  pipeline.ChildOK,
		"rental":   pipeline.ChildOK,
		// Reached as parents. Their own children are never pulled through,
		// which is the size control and the privacy control.
		"address":   pipeline.ParentOnly,
		"city":      pipeline.ParentOnly,
		"film":      pipeline.ParentOnly,
		"inventory": pipeline.ParentOnly,
		"staff":     pipeline.ParentOnly,
		"store":     pipeline.ParentOnly,
		// No outgoing edge, few rows, no personal data: copied whole.
		"category": pipeline.Lookup,
		"country":  pipeline.Lookup,
		"language": pipeline.Lookup,
		// actor holds names, so it is walked rather than copied whole — and
		// nothing walks to it, because film is PARENT_ONLY.
		"actor":         pipeline.SchemaOnly,
		"film_actor":    pipeline.SchemaOnly,
		"film_category": pipeline.SchemaOnly,
	}
	if len(steps) != len(want) {
		t.Errorf("the plan has %d steps over %d tables; pagila has 15 base tables and 7 payment partitions that are never steps",
			len(steps), len(want))
	}
	for name, mode := range want {
		s, ok := steps[tref("public", name)]
		if !ok {
			t.Errorf("public.%s has no step", name)
			continue
		}
		if s.Mode != mode {
			t.Errorf("public.%s mode = %d, want %d (%s)", name, s.Mode, mode, s.Why)
		}
	}

	// --take 50 seeds customers 1 to 50. Customer 182 joins them because a
	// parent edge is mandatory and uncapped: one of those 50 customers has a
	// payment attached to a rental of customer 182, so the rental is pulled as
	// a parent of the payment and the customer as a parent of the rental. That
	// is the referential-completeness half of §3, and pagila's own data is what
	// makes it visible here.
	gotCustomers := intKeysOf(t, steps[customer])
	if len(gotCustomers) != 51 || gotCustomers[50] != 182 {
		t.Fatalf("customer selected %d rows (%v...), want the 50 --take asked for plus customer 182 as a parent",
			len(gotCustomers), gotCustomers[:min(len(gotCustomers), 5)])
	}
	for i := 0; i < 50; i++ {
		if gotCustomers[i] != int64(i+1) {
			t.Fatalf("customer keys = %v, want 1 to 50 in order then 182", gotCustomers)
		}
	}
	// payment's primary key is (payment_date, payment_id): a composite key whose
	// first column is a timestamptz, which travels as text and is cast back.
	pay := steps[tref("public", "payment")]
	if fmt.Sprint(pay.Identity.Columns) != "[payment_date payment_id]" {
		t.Fatalf("payment identity = %v, want the partitioned root's composite key", pay.Identity.Columns)
	}
	if pay.Keys == nil || pay.Keys.Len() == 0 {
		t.Fatal("payment selected no row")
	}
	if got := pay.Keys.Chunks(chunkSize)[0].Cast(0); got != "::text[]" {
		t.Errorf("payment key column 0 casts %q, want ::text[] with a cast back to the column's type", got)
	}

	// Two runs over one snapshot produce byte-identical plans.
	second, err := New().Plan(ctx, r, schema, pagilaClassification(), pipeline.PlanRequest{
		Root: &customer,
		Take: 50,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if digest(p) != digest(second) {
		t.Error("two runs over one snapshot produced different plans")
	}
}

// TestPlanPagilaLookupCountryCarriesItsRowCount is T-0346: the Orchestrator's
// evaluation of --tui against Pagila found public.country — a parent reached
// only as a lookup (no outgoing edge, an incoming one from city, few rows, no
// masked column) — printed "0 rows, lookup; lookup" on its plan line and its
// plan screen row, while the estimate's total already added country's 109 rows
// in. The 0 came from stepRows (internal/core/names.go) looking at nothing but
// Step.Keys, which is nil for a Lookup step by design (ARCHITECTURE.md §2 — it
// is copied whole, not walked); this pins that a Lookup step's Why instead
// carries its row count (internal/plan's lookupWhyWithRows/ParseLookupRows)
// and says "copied whole" rather than the bare "lookup" T-0346 found.
func TestPlanPagilaLookupCountryCarriesItsRowCount(t *testing.T) {
	ctx := context.Background()
	r, schema, url := fixtureURL(ctx, t, testutil.LoadPagila)

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to count public.country directly: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	var want int64
	if scanErr := conn.QueryRow(ctx, "SELECT count(*) FROM public.country").Scan(&want); scanErr != nil {
		t.Fatalf("counting public.country: %v", scanErr)
	}

	customer := tref("public", "customer")
	p, err := New().Plan(ctx, r, schema, pagilaClassification(), pipeline.PlanRequest{
		Root: &customer,
		Take: 50,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)
	country, ok := steps[tref("public", "country")]
	if !ok {
		t.Fatal("public.country has no step")
	}
	if country.Mode != pipeline.Lookup {
		t.Fatalf("public.country mode = %d, want Lookup (T-0346's shape: no outgoing edge, reached only "+
			"as a parent of city, few rows, no masked column)", country.Mode)
	}
	if country.Keys != nil {
		t.Fatalf("public.country has Keys, want nil: a Lookup step is copied whole, not walked (ARCHITECTURE.md §2)")
	}

	plain, gotRows, ok := ParseLookupRows(country.Why)
	if !ok {
		t.Fatalf("public.country Why = %q carries no row count ParseLookupRows can read back off it, "+
			"which is what stepRows (internal/core/names.go) needs since Keys is nil (T-0346)", country.Why)
	}
	if gotRows != want {
		t.Errorf("public.country Why carries %d rows, want the table's actual %d (T-0346: the plan's own "+
			"estimate already adds this table's rows in, and the step must say the same number)", gotRows, want)
	}
	if plain != "copied whole" {
		t.Errorf("public.country Why (with its row count stripped) = %q, want \"copied whole\": T-0346 found "+
			"the bare \"lookup\", which does not say a lookup actually copied its rows rather than finding none",
			plain)
	}
}

// keyLen is a nil-safe key count for a failure message.
func keyLen(s pipeline.Step) int {
	if s.Keys == nil {
		return -1
	}
	return s.Keys.Len()
}

// ---------- two shapes nasty.sql does not carry ----------

// extraSchema adds four schema shapes to nasty.sql for the length of one test.
// All three are shapes the planner has code for and testdata/ has no table for,
// and each fails silently or obscurely rather than loudly when that code is
// wrong, which is the reason testdata/README.md gives for the key-kinds trap
// existing at all:
//
//   - character(n) as a key. The column is blank-padded on disk and its
//     comparison against a text array is not, so a key read back padded matches
//     no row and the child table comes back empty.
//   - a foreign key that references a unique column that is not the parent's
//     primary key, which is the only thing referencedKeys and identityKeys
//     exist for. Every foreign key in testdata/ references a primary key.
//   - a table with no primary key, no unique index and a NOT NULL column of a
//     type with no default btree opclass (json). §3.4's fourth rung must not
//     put that column in a candidate: the probe's count(DISTINCT (...)) and the
//     child walk's ORDER BY would both die inside pgx with "could not identify
//     a comparison function for type json", which is a raw driver error with no
//     exit code and no remedy where §3.4 promises exit 12 naming --key and
//     --skip-table. testdata/nasty.sql's click_stream is the same rung with
//     only comparable columns, so it cannot catch this.
//   - a polymorphic pair whose dangling `_type`/`_id` pair is on a row the
//     slice actually holds. testdata/nasty.sql has a dangling owner
//     (attachment 839) and it is on the one attachment no declared edge
//     reaches, so the pair is never read and the dangling id never reaches the
//     parent step: the assertion that the parent holds no phantom key cannot
//     fail there, and it did not, while pushVirtual was pushing the referenced
//     value straight into the parent's key set as if the row existed.
//
// The tables live here rather than in testdata/nasty.sql because that file is
// shared with introspect, classify and the gate, and each of them counts its
// tables; adding two shapes for the planner's sake would move numbers in suites
// this task does not own. A fixture table is still owed — see the return value.
const extraSchema = `
CREATE TABLE public.ledgers (
    ledger_code character(12) PRIMARY KEY,
    title       text NOT NULL
);

CREATE TABLE public.ledger_entries (
    entry_id    bigint PRIMARY KEY,
    ledger_code character(12) NOT NULL REFERENCES public.ledgers (ledger_code),
    amount      numeric(12,2) NOT NULL
);

INSERT INTO public.ledgers (ledger_code, title) VALUES
    ('LDG-0001', 'Main'), ('LDG-0002', 'Petty');

INSERT INTO public.ledger_entries (entry_id, ledger_code, amount) VALUES
    (1, 'LDG-0001', 10.00), (2, 'LDG-0001', 20.00), (3, 'LDG-0002', 30.00);

CREATE TABLE public.accounts (
    account_id      bigint PRIMARY KEY,
    account_ref     text NOT NULL UNIQUE,
    owner_person_id bigint NOT NULL REFERENCES public.people (person_id)
);

CREATE TABLE public.invoices (
    invoice_id  bigint PRIMARY KEY,
    account_ref text NOT NULL REFERENCES public.accounts (account_ref),
    total       numeric(12,2) NOT NULL
);

INSERT INTO public.accounts (account_id, account_ref, owner_person_id) VALUES
    (7001, 'ACC-A', 90000), (7002, 'ACC-B', 90007);

INSERT INTO public.invoices (invoice_id, account_ref, total) VALUES
    (9001, 'ACC-A', 100.00), (9002, 'ACC-A', 200.00), (9003, 'ACC-B', 300.00);

CREATE TABLE public.page_views (
    person_id bigint NOT NULL REFERENCES public.people (person_id),
    payload   json NOT NULL,
    viewed_at timestamp with time zone NOT NULL
);

-- The two rows differ in the json column and in nothing else, so every
-- candidate the ladder may legally build is non-unique and the run must reach
-- exit 12. A candidate that reached the json column would not: it would fail
-- inside the driver instead.
INSERT INTO public.page_views (person_id, payload, viewed_at) VALUES
    (90000, '{"path": "/pricing"}', '2025-04-01 10:00:00+00'),
    (90000, '{"path": "/docs"}',    '2025-04-01 10:00:00+00');

-- A polymorphic pair whose dangling owner is on a selected row. target_type
-- holds the Rails class name, so §3.2's own rule -- underscore and pluralise --
-- maps 'Note' to public.notes, and link 2 names a note that does not exist.
--
-- Link 3's 'Ghost' names no table in any spelling, so the sample reports it as
-- an unmapped value. The walk then meets it again on a selected row, which is
-- the case that must not be reported a second time as a value the sample never
-- produced.
CREATE TABLE public.notes (
    note_id bigint PRIMARY KEY,
    body    text NOT NULL
);

CREATE TABLE public.note_links (
    link_id     bigint PRIMARY KEY,
    target_type text NOT NULL,
    target_id   bigint NOT NULL
);

INSERT INTO public.notes (note_id, body) VALUES (1, 'kept');

INSERT INTO public.note_links (link_id, target_type, target_id) VALUES
    (1, 'Note', 1),
    (2, 'Note', 999999),
    (3, 'Ghost', 5);

-- T-0314's own fixture: two framework metadata tables reached by no foreign
-- key at all, which is the ordinary shape migration bookkeeping has and the
-- one shape §3's own lookup rule ("at least one incoming edge") cannot
-- reach. TestPlanFrameworkMetadataTablesCopiedAsLookups is what pins this.
CREATE TABLE public.schema_migrations (
    version character varying NOT NULL PRIMARY KEY
);

INSERT INTO public.schema_migrations (version) VALUES
    ('20250101000000'), ('20250102000000'), ('20250103000000');

CREATE TABLE public.ar_internal_metadata (
    key        character varying NOT NULL PRIMARY KEY,
    value      character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

INSERT INTO public.ar_internal_metadata (key, value, created_at, updated_at) VALUES
    ('environment', 'production', now(), now());
`

// nastyPlusRequest is nastyRequest with the second table extraSchema adds whose
// required behaviour is a refusal: public.page_views has no row identity, and
// like click_stream it is child-only, so every run over this fixture that is
// not asserting the refusal carries the flag that drops it.
func nastyPlusRequest(root ref.TableRef) pipeline.PlanRequest {
	r := nastyRequest(root)
	r.Skip = append(r.Skip, tref("public", "page_views"))
	return r
}

// loadNastyPlus loads nasty.sql and then extraSchema, before the snapshot the
// planner reads is opened.
func loadNastyPlus(ctx context.Context, url string) error {
	if err := loadNasty(ctx, url); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connecting to add the planner's own tables: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, extraSchema); err != nil {
		return fmt.Errorf("adding the planner's own tables: %w", err)
	}
	return nil
}

// A character(n) key: the child rows are selected, and the keys are the trimmed
// form the comparison produces rather than the padded form the column holds.
func TestPlanBlankPaddedKey(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNastyPlus)

	ledgers := tref("public", "ledgers")
	req := nastyPlusRequest(ledgers)
	req.Take = 500
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	s := steps[ledgers]
	if got := fmt.Sprint(stringKeysOf(t, s)); got != "[LDG-0001 LDG-0002]" {
		t.Errorf("ledgers = %q, want the trimmed keys: character(12) is stored blank-padded, "+
			"and `t.col = k.k1` against a text array compares the trimmed form", got)
	}
	if got := s.Keys.Chunks(chunkSize)[0].Cast(0); got != "::text[]" {
		t.Errorf("ledgers key column 0 casts %q, want ::text[]", got)
	}

	entries := steps[tref("public", "ledger_entries")]
	if keyLen(entries) != 3 {
		t.Fatalf("ledger_entries selected %d rows, want all 3: a padded key joins nothing", keyLen(entries))
	}
	if got := fmt.Sprint(intKeysOf(t, entries)); got != "[1 2 3]" {
		t.Errorf("ledger_entries = %s, want [1 2 3]", got)
	}
}

// A foreign key that references a unique column that is not the parent's
// primary key, walked in both directions: the child step translates the
// parent's identity into the referenced values, and the parent step translates
// the referenced values back.
func TestPlanForeignKeyToNonPrimaryUniqueColumn(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNastyPlus)

	accounts := tref("public", "accounts")
	invoices := tref("public", "invoices")

	t.Run("ChildStepTranslatesIdentityToReferencedColumns", func(t *testing.T) {
		req := nastyPlusRequest(accounts)
		req.Take = 500
		p, err := New().Plan(ctx, r, schema, nil, req)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		steps := stepsByTable(p)
		if got := fmt.Sprint(intKeysOf(t, steps[accounts])); got != "[7001 7002]" {
			t.Fatalf("accounts = %s, want both rows", got)
		}
		inv := steps[invoices]
		if got := fmt.Sprint(intKeysOf(t, inv)); got != "[9001 9002 9003]" {
			t.Errorf("invoices = %s, want all 3: the edge references account_ref, not the primary key", got)
		}
		if inv.Mode != pipeline.ChildOK {
			t.Errorf("invoices mode = %d, want ChildOK", inv.Mode)
		}
	})

	t.Run("ParentStepTranslatesReferencedColumnsToIdentity", func(t *testing.T) {
		req := nastyPlusRequest(invoices)
		req.Take = 500
		p, err := New().Plan(ctx, r, schema, nil, req)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		steps := stepsByTable(p)
		acc := steps[accounts]
		if acc.Mode != pipeline.ParentOnly {
			t.Errorf("accounts mode = %d, want ParentOnly", acc.Mode)
		}
		// The selected set is keyed by the parent's own identity, not by the
		// values the edge references.
		if fmt.Sprint(acc.Identity.Columns) != "[account_id]" {
			t.Fatalf("accounts identity = %v, want the primary key", acc.Identity.Columns)
		}
		if got := fmt.Sprint(intKeysOf(t, acc)); got != "[7001 7002]" {
			t.Errorf("accounts = %s, want [7001 7002] translated from account_ref", got)
		}
		// And the referential completeness carries on: both account owners are
		// pulled in as people.
		people := steps[tref("public", "people")]
		if got := fmt.Sprint(intKeysOf(t, people)); got != "[90000 90007]" {
			t.Errorf("people = %s, want the two account owners", got)
		}
	})
}

// T-0314: schema_migrations and ar_internal_metadata are reached by no
// foreign key at all — extraSchema's own comment explains why that is the
// ordinary shape of migration bookkeeping — so §3's ordinary lookup rule
// ("no outgoing edge, at least one incoming edge") would leave both
// SchemaOnly, and dogfood session 1 found exactly that: zero migration rows,
// and ar_internal_metadata's source environment carried into the target
// verbatim. Both must plan as Lookup, with every row, regardless of root.
func TestPlanFrameworkMetadataTablesCopiedAsLookups(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNastyPlus)

	// The root is ordinary and unrelated to either table, which is the point:
	// neither is reached by the walk from anywhere, and both must still be
	// planned.
	req := nastyPlusRequest(tref("public", "accounts"))
	req.Take = 500
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	migrations := tref("public", "schema_migrations")
	s, ok := steps[migrations]
	if !ok {
		t.Fatalf("public.schema_migrations has no step")
	}
	if s.Mode != pipeline.Lookup {
		t.Errorf("public.schema_migrations mode = %v, want Lookup: it has no incoming foreign key, "+
			"which is the ordinary shape of a migration bookkeeping table and not a reason to leave it "+
			"SchemaOnly", s.Mode)
	}
	if !strings.Contains(s.Why, "framework metadata table") {
		t.Errorf("public.schema_migrations Why = %q, want it to name the reason a lookup with no "+
			"incoming edge exists at all", s.Why)
	}

	metadata := tref("public", "ar_internal_metadata")
	m, ok := steps[metadata]
	if !ok {
		t.Fatalf("public.ar_internal_metadata has no step")
	}
	if m.Mode != pipeline.Lookup {
		t.Errorf("public.ar_internal_metadata mode = %v, want Lookup", m.Mode)
	}
	if !strings.Contains(m.Why, "framework metadata table") {
		t.Errorf("public.ar_internal_metadata Why = %q, want it to name the reason", m.Why)
	}
	if !strings.Contains(m.Why, "environment rewritten to development") {
		t.Errorf("public.ar_internal_metadata Why = %q, want it to say the environment row will be "+
			"rewritten (§3.5: the plan says so before any row moves)", m.Why)
	}
}

// truncatedFrameworkMetadataSchema is a standalone, minimal schema for
// TestPlanFrameworkMetadataTableOverTheLookupCeilingSaysSo: one framework
// metadata table with more rows than lookupRowCeiling, reached by no foreign
// key at all (the ordinary shape), and one ordinary, unrelated table to plan
// from as root. It does not reuse nasty.sql or extraSchema, both of which are
// read by suites elsewhere that count their tables (this file's own
// extraSchema comment says the same).
const truncatedFrameworkMetadataSchema = `
CREATE TABLE public.reg_widgets (
    widget_id bigint PRIMARY KEY,
    name      text NOT NULL
);

INSERT INTO public.reg_widgets (widget_id, name) VALUES (1, 'Widget One');

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL PRIMARY KEY
);

INSERT INTO public.schema_migrations (version)
    SELECT lpad(gs::text, 14, '0') FROM generate_series(1, 1002) AS gs;
`

func loadTruncatedFrameworkMetadata(ctx context.Context, url string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connecting to load the truncated framework metadata fixture: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, truncatedFrameworkMetadataSchema); err != nil {
		return fmt.Errorf("loading the truncated framework metadata fixture: %w", err)
	}
	return nil
}

// TestPlanFrameworkMetadataTableOverTheLookupCeilingSaysSo is the T-0314
// review round's finding 1: internal/extract's own Lookup read carries the
// identical 1,001-row bound findLookups' own boundedCount probe does
// (lookupLimit, internal/extract/sql.go; countProbeLimit, sql.go in this
// package; T-0347), so a framework metadata table with more than
// lookupRowCeiling (1,000) rows is truncated regardless of what the plan
// says. Before this fix, frameworkMetadataWhy said "copied whole" for such a
// table anyway, because it never looked at boundedCount's own answer — a
// mature Rails app's schema_migrations (easily past 1,000 migrations) would
// plan with a line claiming completeness the run cannot back up, and an
// operator would discover the shortfall only by counting rows in the target.
func TestPlanFrameworkMetadataTableOverTheLookupCeilingSaysSo(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadTruncatedFrameworkMetadata)

	root := tref("public", "reg_widgets")
	req := pipeline.PlanRequest{Root: &root, Take: 10}
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	migrations := tref("public", "schema_migrations")
	s, ok := steps[migrations]
	if !ok {
		t.Fatalf("public.schema_migrations has no step")
	}
	if s.Mode != pipeline.Lookup {
		t.Errorf("public.schema_migrations mode = %v, want Lookup", s.Mode)
	}
	if strings.Contains(s.Why, "copied whole regardless of reachability") {
		t.Errorf("public.schema_migrations Why = %q, want it not to claim the table was copied whole: "+
			"it holds 1,002 rows and internal/extract's own Lookup read is bounded at 1,001 (T-0347)", s.Why)
	}
	if !strings.Contains(s.Why, "1000") {
		t.Errorf("public.schema_migrations Why = %q, want it to name the row ceiling actually applied", s.Why)
	}

	// T-0346 review round, finding 1: the count packed into Why (and so the
	// plan line's own row count) must agree with what the sentence just
	// above claims reaches the target — lookupRowCeiling (1,000) — and not
	// leak findLookups' internal probe cap (countProbeLimit, 1,001), which
	// contradicts "only the first 1000 ... are copied" and matches neither
	// the source's 1,002 rows nor what the sentence says is copied.
	_, n, ok := ParseLookupRows(s.Why)
	if !ok {
		t.Fatalf("ParseLookupRows(%q): ok = false, want a Lookup step's Why to carry a row count", s.Why)
	}
	if n != lookupRowCeiling {
		t.Errorf("public.schema_migrations row count = %d, want %d (the ceiling the Why sentence says was "+
			"applied, not countProbeLimit's 1,001 probe cap)", n, lookupRowCeiling)
	}
}

// A polymorphic id that names no row, on a row the slice holds.
//
// There is no constraint behind an inferred edge, so nothing guarantees the
// referenced row exists — which is exactly the assumption the parent step's
// short-circuit makes for a declared one, and taking it here put a key for a
// row that does not exist into the parent's selected set. What follows from a
// phantom key is not a wrong number in a report: it inflates Estimate.Rows and
// the printed step count, extract returns one row fewer than the plan promised,
// and internal/verify refuses the finished run at exit 7 with the target
// already dropped and loaded.
func TestPlanVirtualEdgeToADanglingRow(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNastyPlus)

	links := tref("public", "note_links")
	notes := tref("public", "notes")

	req := nastyPlusRequest(links)
	req.Take = 500
	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	if got := fmt.Sprint(intKeysOf(t, steps[links])); got != "[1 2 3]" {
		t.Fatalf("note_links = %s, want every row: the root is the table the pair is on", got)
	}
	want := `public.note_links.target_type -> public.notes`
	var names []string
	found := false
	for _, fk := range p.Virtual {
		names = append(names, fk.Name+" -> "+fk.Parent.String())
		found = found || (fk.Name == `public.note_links.target_type` && fk.Parent == notes)
	}
	if !found {
		t.Fatalf("Virtual = %v, want it to carry %q: 'Note' underscores and pluralises to notes", names, want)
	}

	// 'Ghost' names no table, so the sample reports it once, under Unmapped.
	// The walk meets it again on link 3 — a selected row — and that must not
	// become a second finding saying the sample never produced it: the sample
	// did, the reason would be false, and the remedy it points at (widen the
	// sample) would change nothing.
	// 'Ghost' itself never appears: T-0131 replaced the quoted value, and then
	// the digest that stood in for it, with a count — this only checks the
	// shape and that the sampled value is not in it anywhere.
	wantUnmapped := []string{
		"public.note_links.target_type: 1 distinct value mapping to no table; " +
			"inspect the distinct values of target_type on public.note_links in the source to see what they are",
	}
	if fmt.Sprint(p.Unmapped) != fmt.Sprint(wantUnmapped) {
		t.Errorf("Unmapped = %v, want %v", p.Unmapped, wantUnmapped)
	}
	if strings.Contains(fmt.Sprint(p.Unmapped), "Ghost") {
		t.Errorf("Unmapped = %v, carries the raw sampled value %q", p.Unmapped, "Ghost")
	}
	for _, f := range p.Polymorphic {
		if strings.Contains(f, "Ghost") {
			t.Errorf("Polymorphic carries %q; 'Ghost' is already reported as an unmapped value, and the "+
				"sample did produce it", f)
		}
	}

	// Link 1 names note 1, which exists. Link 2 names note 999999, which does
	// not, and the key set is what the target will be checked against.
	n := steps[notes]
	if n.Mode != pipeline.ParentOnly {
		t.Errorf("notes mode = %d, want ParentOnly: a virtual edge is followed as a parent only", n.Mode)
	}
	if got := fmt.Sprint(intKeysOf(t, n)); got != "[1]" {
		t.Errorf("notes = %s, want [1]: 999999 is no note, and a key set that holds it promises a row "+
			"the extract cannot produce", got)
	}
}

// §3.4's fourth rung over a table whose NOT NULL columns include one the key
// encoding cannot compare: the ladder ends at the refusal §3.4 promises, not at
// a driver error.
//
// public.page_views has no primary key and no unique index; its only comparable
// candidate column is the foreign key person_id, which repeats. A candidate
// that also took the json column would not reach this refusal at all — the
// probe would fail inside pgx with SQLSTATE 42883, which carries no exit code
// and no remedy — so a regression here shows up as a wrapped error rather than
// a *Refusal, and that is what the first assertion reads.
func TestPlanUncomparableColumnStillRefusesWithNoIdentity(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadNastyPlus)

	pageViews := tref("public", "page_views")
	req := nastyRequest(tref("public", "people")) // click_stream skipped, page_views not
	req.Take = 500

	_, err := New().Plan(ctx, r, schema, nil, req)
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v (%T), want a *Refusal for %s: a NOT NULL json column "+
			"has no default btree opclass and must never enter a pseudo-key candidate", err, err, pageViews)
	}
	if refusal.Code != CodeNoIdentity || refusal.Exit != 12 {
		t.Fatalf("refusal = %s exit %d, want %s exit 12", refusal.Code, refusal.Exit, CodeNoIdentity)
	}
	if refusal.Table != pageViews {
		t.Fatalf("refusal names %s, want %s", refusal.Table, pageViews)
	}
	if !strings.Contains(refusal.Message, "--key") || !strings.Contains(refusal.Message, "--skip-table") {
		t.Errorf("refusal message = %q, want both remedies §3.4 names", refusal.Message)
	}
}

// ---------- the root can hold more rows than --take named (T-0288) ----------

// rootClosureSchema is a two-table reduction of the shape the Pagila diagnosis
// for T-0288 found: `orders` is a child of `customers` via `customer_id` — the
// edge that reaches it in CHILD_OK mode — and carries a second foreign key,
// `referred_by`, back to the same table. A selected order can name, through
// that second edge, a customer the root's own `--take` seed never chose;
// referential completeness still pulls that customer in, as a PARENT_ONLY
// addition to the table the walk already labelled CHILD_OK for its seeded row.
// This is the two-table fixture ARCHITECTURE.md §3.7 describes: a child
// pointing back at an unchosen root.
const rootClosureSchema = `
CREATE TABLE public.customers (
    id   bigint PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE public.orders (
    id          bigint PRIMARY KEY,
    customer_id bigint NOT NULL REFERENCES public.customers (id),
    referred_by bigint REFERENCES public.customers (id)
);

INSERT INTO public.customers (id, name) VALUES (1, 'Ada'), (2, 'Grace');

INSERT INTO public.orders (id, customer_id, referred_by) VALUES (10, 1, 2);
`

func loadRootClosure(ctx context.Context, url string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connecting to load the root-closure fixture: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, rootClosureSchema); err != nil {
		return fmt.Errorf("loading the root-closure fixture: %w", err)
	}
	return nil
}

// TestPlanRootLineNamesRowsPulledInByReferences pins ARCHITECTURE.md §3.7's
// wording: with --take 1, the root's seed is {1}, and orders(10)'s
// referred_by pulls customer 2 into the same table as PARENT_ONLY. The root's
// count (2) exceeds the seed (1), so the plan's line for it must say why
// rather than leaving the extra row unexplained as "root" always used to.
func TestPlanRootLineNamesRowsPulledInByReferences(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadRootClosure)

	customers := tref("public", "customers")
	orders := tref("public", "orders")
	root := customers
	req := pipeline.PlanRequest{Root: &root, Take: 1}

	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	cust := steps[customers]
	if got := fmt.Sprint(intKeysOf(t, cust)); got != "[1 2]" {
		t.Fatalf("customers = %s, want [1 2]: --take 1 seeds customer 1, and orders(10)."+
			"referred_by pulls in customer 2", got)
	}
	if cust.Mode != pipeline.ChildOK {
		t.Errorf("customers mode = %d, want ChildOK: the seed's own mode, never overwritten "+
			"by the later PARENT_ONLY pop that pulls customer 2 in", cust.Mode)
	}
	if want := "root: 1 chosen, 1 pulled in by references"; cust.Why != want {
		t.Errorf("customers.Why = %q, want %q (ARCHITECTURE.md §3.7)", cust.Why, want)
	}

	ord := steps[orders]
	if got := fmt.Sprint(intKeysOf(t, ord)); got != "[10]" {
		t.Errorf("orders = %s, want [10]", got)
	}
	if ord.Why != "child of public.customers via public.orders.customer_id" {
		t.Errorf("orders.Why = %q, unaffected by this section", ord.Why)
	}
}

// TestPlanRootLineIsUnchangedWhenNothingIsPulledIn is the M == 0 side: the
// overwhelmingly common case, and every fixture elsewhere in this package,
// must keep the plain "root" Why the walk has always produced. --take 2 here
// seeds both customers, so referred_by names a customer already selected and
// pulls nothing in.
func TestPlanRootLineIsUnchangedWhenNothingIsPulledIn(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadRootClosure)

	customers := tref("public", "customers")
	root := customers
	req := pipeline.PlanRequest{Root: &root, Take: 2}

	p, err := New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	steps := stepsByTable(p)

	cust := steps[customers]
	if got := fmt.Sprint(intKeysOf(t, cust)); got != "[1 2]" {
		t.Fatalf("customers = %s, want [1 2]: --take 2 seeds both", got)
	}
	if cust.Why != "root" {
		t.Errorf(`customers.Why = %q, want "root": nothing was pulled in beyond the seed`, cust.Why)
	}
}

// ---------- collecting refusals across independent causes, end to end (T-0318) ----------

// collectedRefusalsSchema is a Plan()-level reduction of dogfood session 1's
// own shape: two Rails habtm join tables with no derivable identity at all
// (no primary key, no unique index, no foreign key declared on either, so
// resolveIdentity's pseudo-key rung has nothing to probe), and a root/child
// pair each carrying one masked column under a unique index too narrow for
// any registered generator to widen into -- the identical reduction
// unique_test.go's uniqueRun and refusals_test.go's narrowUniqueColumnTable
// both fake by hand. refusals_test.go's own fixture already proves the shape
// holds without a database, by calling resolveIdentities and checkUniqueDomain
// directly; this is the other half the T-0318 review round's third finding
// asked for -- a single New().Plan() call against a real Postgres, so a
// regression in plan()'s own sequencing (returning early once
// len(p.refusals) > 0 anywhere before the last check, say) fails this test
// and not only the one that calls the two checks by hand.
const collectedRefusalsSchema = `
CREATE TABLE public.customers (
    id     bigint PRIMARY KEY,
    handle character varying(18) NOT NULL
);

CREATE TABLE public.sessions (
    id          bigint PRIMARY KEY,
    customer_id bigint NOT NULL REFERENCES public.customers (id),
    token       character varying(18) NOT NULL
);

CREATE TABLE public.assemblies_kits (
    assembly_id bigint NOT NULL,
    kit_id      bigint NOT NULL
);

CREATE TABLE public.assemblies_parts (
    assembly_id bigint NOT NULL,
    part_id     bigint NOT NULL
);

INSERT INTO public.customers (id, handle) VALUES (1, 'cust-1');
INSERT INTO public.sessions (id, customer_id, token) VALUES (10, 1, 'sess-1');
`

func loadCollectedRefusals(ctx context.Context, url string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connecting to load the collected-refusals fixture: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, collectedRefusalsSchema); err != nil {
		return fmt.Errorf("loading the collected-refusals fixture: %w", err)
	}
	return nil
}

// narrowCredentialDecision is the same shape unique_test.go's uniqueRun and
// refusals_test.go's narrowUniqueColumnTable both fake: character varying(18)
// has no room for credential_unique's prefix and a distinguishing suffix, so
// the category's only masker left is the fixed literal, whose domain of 1
// clears no d_required at any row count -- including the single row each
// table plans here.
func narrowCredentialDecision(col ref.ColumnRef) pipeline.Decision {
	return pipeline.Decision{
		Col: col, Category: pipeline.CatCredential, Masker: mask.CredentialMasker,
		Masked: true, UniqueIndex: true,
	}
}

// TestPlanCollectsFourIndependentRefusalsInOneRun is the T-0318 review round's
// third finding, driven through Plan() end to end rather than through
// resolveIdentities and checkUniqueDomain directly
// (refusals_test.go's TestCollectsTwoNoIdentityAndTwoUniqueDomainRefusalsInOneRun
// takes the shortcut): dogfood session 1 took nine runs to reach a green
// verify, one cause at a time, and this is the same shape driven through a
// real Postgres and the same New().Plan() entry point an operator's own run
// takes.
func TestPlanCollectsFourIndependentRefusalsInOneRun(t *testing.T) {
	ctx := context.Background()
	r, schema := fixture(ctx, t, loadCollectedRefusals)

	customers := tref("public", "customers")
	sessions := tref("public", "sessions")
	kits := tref("public", "assemblies_kits")
	parts := tref("public", "assemblies_parts")

	handle := ref.ColumnRef{Table: customers, Column: "handle"}
	token := ref.ColumnRef{Table: sessions, Column: "token"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		handle: narrowCredentialDecision(handle),
		token:  narrowCredentialDecision(token),
	}}

	root := customers
	_, err := New().Plan(ctx, r, schema, cls, pipeline.PlanRequest{Root: &root})

	var refusals Refusals
	if !errors.As(err, &refusals) {
		t.Fatalf("Plan returned %v (%T), want plan.Refusals carrying all four causes", err, err)
	}
	if len(refusals) != 4 {
		t.Fatalf("collected %d refusals, want 4 (two no_identity, two unique_domain): %v", len(refusals), refusals)
	}

	// The order is deterministic and asserted exactly: resolveIdentities runs
	// before checkUniqueDomain (plan.go), each visits tables in (schema, name)
	// order, and neither reorders what it finds.
	if refusals[0].Code != CodeNoIdentity || refusals[0].Table != kits {
		t.Errorf("refusals[0] = %s %s, want %s %s", refusals[0].Code, refusals[0].Table, CodeNoIdentity, kits)
	}
	if refusals[1].Code != CodeNoIdentity || refusals[1].Table != parts {
		t.Errorf("refusals[1] = %s %s, want %s %s", refusals[1].Code, refusals[1].Table, CodeNoIdentity, parts)
	}
	if refusals[2].Code != CodeUniqueDomain || refusals[2].Table != customers {
		t.Errorf("refusals[2] = %s %s, want %s %s", refusals[2].Code, refusals[2].Table, CodeUniqueDomain, customers)
	}
	if refusals[3].Code != CodeUniqueDomain || refusals[3].Table != sessions {
		t.Errorf("refusals[3] = %s %s, want %s %s", refusals[3].Code, refusals[3].Table, CodeUniqueDomain, sessions)
	}
	for i, rf := range refusals {
		if rf.Exit != exitPlan {
			t.Errorf("refusals[%d].Exit = %d, want %d", i, rf.Exit, exitPlan)
		}
	}
}
