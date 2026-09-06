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
