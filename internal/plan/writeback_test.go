// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The write-back refusal, held without a database.
//
// writeback_integration_test.go asserts the thing that matters over a real
// fixture — that the real classifier and the real rule pack produce nothing
// this check refuses — but it asserts it by re-deriving the answer from
// mask.Writable rather than by calling checkWriteBack, so it stays green if the
// refusal is lost. This file is the other half: a schema and a classification
// built by hand, one column whose category its type cannot hold, and the exact
// refusal the operator gets. Replacing checkWriteBack's body with `return nil`
// fails TestUnwritableColumnIsRefusedAtPlan.

// emptyRows is a result set with no rows. The privilege pass is the only query
// that runs before the write-back check, and it treats an empty result as "the
// role can read everything".
type emptyRows struct{}

func (emptyRows) Next() bool        { return false }
func (emptyRows) Scan(...any) error { return errors.New("plan test: no row to scan") }
func (emptyRows) Err() error        { return nil }
func (emptyRows) Close()            {}

// countingReader answers every statement with no rows and counts them, so a
// test can say how much of the run happened before the refusal.
type countingReader struct{ queries int }

func (r *countingReader) Query(context.Context, string, ...any) (pipeline.Rows, error) {
	r.queries++
	return emptyRows{}, nil
}

func (*countingReader) Close(context.Context) error { return nil }

// actorTable is pagila's shape, reduced to the three columns that matter: a
// surrogate key, a column every text category can be written into, and the
// timestamptz the classifier called `credential` on every pagila run before
// T-0054.
func actorTable() (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "billing", Name: "actor"}
	return t, &pipeline.Schema{
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "actor_id", TypeName: "bigint", TypeOID: 20},
				{Name: "first_name", TypeName: "character varying(45)", TypMod: 49},
				{Name: "last_update", TypeName: "timestamp with time zone"},
			},
			PK: []string{"actor_id"},
		}},
	}
}

func masking(t ref.TableRef, column string, cat pipeline.Category, id mask.ID) *pipeline.Classification {
	return &pipeline.Classification{
		Decisions: map[ref.ColumnRef]pipeline.Decision{
			{Table: t, Column: column}: {
				Col:      ref.ColumnRef{Table: t, Column: column},
				Category: cat,
				Masker:   id,
				Masked:   true,
			},
		},
	}
}

func TestUnwritableColumnIsRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := actorTable()
	cls := masking(tbl, "last_update", pipeline.CatCredential, mask.CredentialMasker)

	r := &countingReader{}
	_, err := New().Plan(context.Background(), r, schema, cls, pipeline.PlanRequest{Root: &tbl})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeUnwritable {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeUnwritable)
	}
	if refusal.Exit != 12 {
		t.Errorf("Exit = %d, want 12", refusal.Exit)
	}
	if refusal.Table != tbl {
		t.Errorf("Table = %s, want %s", refusal.Table, tbl)
	}
	if refusal.Column != "last_update" {
		t.Errorf("Column = %q, want %q", refusal.Column, "last_update")
	}
	// The message has to name the column, the category and the family, or the
	// operator cannot tell which of an --unmask, a yml category or a rule
	// change is their fix.
	for _, want := range []string{"billing.actor.last_update", "credential", "timestamp"} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("message %q does not name %q", refusal.Error(), want)
		}
	}
	if got := refusal.Args[event.ArgTable]; got != tbl.String() {
		t.Errorf("args[table] = %q, want %q", got, tbl)
	}
	if got := refusal.Args[event.ArgColumn]; got != "last_update" {
		t.Errorf("args[column] = %q, want %q", got, "last_update")
	}
	if got := refusal.Args[event.ArgReason]; !strings.Contains(got, "credential") {
		t.Errorf("args[reason] = %q, want the category named", got)
	}
	// The whole point of moving the check here is that it happens before the
	// run reads rows. The privilege pass is the only statement that precedes
	// it.
	if r.queries > 1 {
		t.Errorf("the planner sent %d statements before refusing; only the privilege pass should run", r.queries)
	}
}

// TestWritableColumnIsNotRefusedAtPlan is the other side of the predicate: a
// column whose type its category's masker can write into plans normally. Without
// it, a check that refused everything would pass the test above.
func TestWritableColumnIsNotRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := actorTable()
	cls := masking(tbl, "first_name", pipeline.CatPersonName, mask.MaskerPersonName)

	_, err := New().Plan(context.Background(), &countingReader{}, schema, cls, pipeline.PlanRequest{Root: &tbl})
	var refusal *Refusal
	if errors.As(err, &refusal) {
		t.Fatalf("Plan refused a person_name on a varchar: %v", refusal)
	}
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
}

// compositeTable is testdata/nasty.sql trap 27's shape: a table with a column
// of a user-defined composite type, and the CREATE TYPE line introspect reads
// out of the catalog. Schema.Composites is the only thing that tells a
// composite from an ltree or a PostGIS geometry, and this check refuses one and
// passes the others.
func compositeTable() (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "public", Name: "settlements"}
	return t, &pipeline.Schema{
		Composites: []pipeline.NamedDef{{
			Name: "public.postal_address",
			Def:  "CREATE TYPE public.postal_address AS (line1 text, city text, email text)",
		}},
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "settlement_id", TypeName: "bigint", TypeOID: 20},
				{Name: "billing", TypeName: "public.postal_address"},
				{Name: "route", TypeName: "public.ltree"},
			},
			PK: []string{"settlement_id"},
		}},
	}
}

// TestMaskedCompositeIsRefusedAtPlan is T-0094's decision, on the plan side: no
// masker can write a record, so a composite the classifier reached `possible`
// on is exit 12 here, naming the column and the two escapes, and never a
// verbatim copy of a record with personal data in it (THREAT_MODEL.md T1).
//
// Before it, `constraintsOf` declined to judge a composite — mask.TypeTag has
// no tag for one — and the column was handed to internal/transform, which
// masked the record as if it were a scalar and died in the middle of the load.
func TestMaskedCompositeIsRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := compositeTable()
	cls := masking(tbl, "billing", pipeline.CatEmail, mask.MaskerEmail)

	r := &countingReader{}
	_, err := New().Plan(context.Background(), r, schema, cls, pipeline.PlanRequest{Root: &tbl})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeUnwritable {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeUnwritable)
	}
	if refusal.Exit != 12 {
		t.Errorf("Exit = %d, want 12", refusal.Exit)
	}
	if refusal.Column != "billing" {
		t.Errorf("Column = %q, want %q", refusal.Column, "billing")
	}
	for _, want := range []string{"public.settlements.billing", "composite", "public.postal_address"} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("message %q does not name %q", refusal.Error(), want)
		}
	}
	// The refusal is only useful if it prints the way out. Both escapes are in
	// the rendered reason, because event.ArgKey has no key for a category or a
	// type and the message has to carry them (catalogue.yml, plan.refused.unwritable).
	for _, want := range []string{"--skip-table public.settlements", "--unmask public.settlements.billing=REASON"} {
		if !strings.Contains(refusal.Args[event.ArgReason], want) {
			t.Errorf("args[reason] = %q does not offer %q", refusal.Args[event.ArgReason], want)
		}
	}
	if r.queries > 1 {
		t.Errorf("the planner sent %d statements before refusing; only the privilege pass should run", r.queries)
	}
}

// TestUnmaskedCompositeIsNotRefusedAtPlan, and neither is a type that merely has
// no tag. The refusal is on the composite *and* on the decision to mask it: a
// composite the classifier found nothing in is copied, which is the other half
// of T-0094's decision, and an ltree is still the "nothing here knows enough to
// refuse" case constraintsOf describes.
func TestUnmaskedCompositeIsNotRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := compositeTable()

	t.Run("CompositeThatIsNotMasked", func(t *testing.T) {
		cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			{Table: tbl, Column: "billing"}: {
				Col:      ref.ColumnRef{Table: tbl, Column: "billing"},
				Category: pipeline.CatNone,
			},
		}}
		if _, err := New().Plan(context.Background(), &countingReader{}, schema, cls, pipeline.PlanRequest{Root: &tbl}); err != nil {
			t.Fatalf("Plan: %v", err)
		}
	})

	t.Run("UnknownTypeThatIsNotAComposite", func(t *testing.T) {
		cls := masking(tbl, "route", pipeline.CatEmail, mask.MaskerEmail)
		if _, err := New().Plan(context.Background(), &countingReader{}, schema, cls, pipeline.PlanRequest{Root: &tbl}); err != nil {
			t.Fatalf("Plan refused an ltree, which nothing here knows enough to refuse: %v", err)
		}
	})
}

// arrayTable is plausible's monthly_reports shape: the list of addresses a
// site's report is emailed to, held in a citext[]. The samples are what tell
// the two array columns apart — pgx decodes a text[] into a slice and hands an
// array of an extension type back as the server's own literal, because the
// source pool registers no user types (T-0076).
func arrayTable() (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "public", Name: "monthly_reports"}
	return t, &pipeline.Schema{
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "recipients", TypeName: "extensions.citext[]"},
				{Name: "tags", TypeName: "text[]"},
			},
			PK: []string{"id"},
			Samples: [][]any{
				{int64(1), `{bea.donnelly@example.test,cai.osei@example.test}`, []any{"weekly"}},
				{int64(2), `{dee.abara@example.test}`, []any{"monthly"}},
			},
		}},
	}
}

// TestArrayThatArrivesAsALiteralIsNotRefusedAtPlan is T-0127: internal/transform
// (T-0118) now masks such a column element-wise through the literal, so the
// plan-time stand-in that used to refuse it here is gone. internal/classify
// reads inside such a literal, so a citext[] of addresses is decided `email`
// instead of being copied verbatim, and the plan admits it the same way it
// admits a text[] the driver decodes as a slice.
func TestArrayThatArrivesAsALiteralIsNotRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := arrayTable()
	cls := masking(tbl, "recipients", pipeline.CatEmail, mask.MaskerEmail)

	if _, err := New().Plan(context.Background(), &countingReader{}, schema, cls, pipeline.PlanRequest{Root: &tbl}); err != nil {
		t.Fatalf("Plan refused a citext[] arriving as a literal: %v", err)
	}
}

// TestArrayTheDriverDecodesIsNotRefusedAtPlan: the refusal is on the driver's
// answer and not on the column being an array. A text[] comes back as a slice,
// internal/transform masks it element-wise as it always has, and refusing it
// would turn every masked array column in every schema into a plan refusal.
func TestArrayTheDriverDecodesIsNotRefusedAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := arrayTable()
	cls := masking(tbl, "tags", pipeline.CatFreeText, mask.MaskerFreeText)

	if _, err := New().Plan(context.Background(), &countingReader{}, schema, cls, pipeline.PlanRequest{Root: &tbl}); err != nil {
		t.Fatalf("Plan refused a text[] the driver decodes: %v", err)
	}
}
