// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0097. ARCHITECTURE.md §11.1: the not-recreatable refusal is raised "at plan
// — before the snapshot is used for keys and before anything in the target is
// dropped". It used to be raised by load.Load as its first statement, because a
// stage package may not import another stage package and the caller §11.1
// describes is this one. One of the ten schemas in testdata/torture/ reaches it
// as the fixtures stand -- Mastodon's timestamp_id on nine primary keys, carried
// unedited for that reason -- and GitLab reaches the same refusal upstream on
// two objects its 43-table subset removes (docs/TORTURE.md). An operator with
// either schema paid for a whole extract, holding the source snapshot
// throughout, before being told the target could not be built.
//
// The run here has no reader and no target at all. That is the assertion, not a
// convenience: planStage's next statement after this check builds the plan
// request and calls Plan with r.reader, so a run that reached either would fail
// on a nil interface rather than return the refusal below.
func TestTheNotRecreatableRefusalIsRaisedAtPlanBeforeAnyRead(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "accounts"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{{
			Ref: tbl,
			Columns: []pipeline.Column{
				// Mastodon's shape: a primary key defaulting to a plpgsql
				// function the application installs.
				{Name: "id", TypeName: "bigint", TypeOID: 20, Default: "public.timestamp_id('accounts'::text)"},
				{Name: "username", TypeName: "text"},
			},
			PK: []string{"id"},
		}},
		NotRecreated: []pipeline.Object{{Kind: "function", Name: "public.timestamp_id"}},
	}

	sink := &collector{}
	r := &run{req: normalise(Request{}), sink: sink, schema: schema}

	err := r.planStage(context.Background())
	if err == nil {
		t.Fatal("a schema whose primary key defaults to a function lazyslice does not recreate was planned")
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("planStage returned %v, want a *Stop", err)
	}
	if stop.Code != ddl.CodeNotRecreatableFunction {
		t.Errorf("Code = %q, want %q", stop.Code, ddl.CodeNotRecreatableFunction)
	}
	if stop.Exit != ddl.ExitNotRecreatable {
		t.Errorf("Exit = %d, want %d (ADR-005: target schema not recreatable)",
			stop.Exit, ddl.ExitNotRecreatable)
	}
	if stop.Table != tbl || stop.Column != "id" {
		t.Errorf("the refusal names %v.%s; §11.1 requires the table, the column and the dependency",
			stop.Table, stop.Column)
	}
}
