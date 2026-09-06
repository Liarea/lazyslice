// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What the planner does is a statement about a real Postgres, so the suite that
// matters is plan_integration_test.go. The one case that cannot be a fixture is
// here: ForeignKey.NotRecreatable is set by introspect for a schema shape
// nothing in testdata/ has, and the refusal it raises has to happen before the
// planner reads anything at all.

// refusingReader fails every statement. A test that uses it is asserting that
// no statement was sent.
type refusingReader struct{ t *testing.T }

func (r refusingReader) Query(context.Context, string, ...any) (pipeline.Rows, error) {
	r.t.Helper()
	r.t.Fatal("the planner queried the source before refusing a schema it cannot recreate")
	return nil, errors.New("unreachable")
}

func (refusingReader) Close(context.Context) error { return nil }

func TestNotRecreatableForeignKeyIsRefusedBeforeAnyRead(t *testing.T) {
	child := ref.TableRef{Schema: "public", Name: "device_readings"}
	parent := ref.TableRef{Schema: "public", Name: "devices_2024"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: child, Columns: []pipeline.Column{{Name: "device_id", TypeOID: 20}}, PK: []string{"device_id"}},
			{Ref: parent, Columns: []pipeline.Column{{Name: "device_id", TypeOID: 20}}, PK: []string{"device_id"}},
		},
		FKs: []pipeline.ForeignKey{{
			Name:           "device_readings_device_id_fkey",
			Child:          child,
			ChildCols:      []string{"device_id"},
			Parent:         parent,
			ParentCols:     []string{"device_id"},
			Validated:      true,
			NotRecreatable: true,
		}},
	}

	_, err := New().Plan(context.Background(), refusingReader{t}, schema, nil, pipeline.PlanRequest{})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeNotRecreatable {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeNotRecreatable)
	}
	if refusal.Exit != 13 {
		t.Errorf("Exit = %d, want 13 (ADR-005: target schema not recreatable)", refusal.Exit)
	}
	// The refusal has to name both ends of the edge and the columns, or it says
	// nothing a user can act on.
	if got := refusal.Args[event.ArgTable]; got != child.String() {
		t.Errorf("args[table] = %q, want %q", got, child)
	}
	if got := refusal.Args[event.ArgColumn]; got != "device_id" {
		t.Errorf("args[column] = %q, want %q", got, "device_id")
	}
	if got := refusal.Args[event.ArgReason]; got != parent.String() {
		t.Errorf("args[reason] = %q, want the parent %q", got, parent)
	}
}
