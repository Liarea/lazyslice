// SPDX-License-Identifier: Apache-2.0

//go:build integration

package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/mask"
)

// The test T-CORE relies on: introspect, classify, plan and transform over both
// fixtures, with the real classifier and the real rule pack rather than a
// classification written by hand.
//
// Every other suite in this package hands the planner a classification it built
// itself, and internal/transform's own tests build both the schema and the
// batch. Neither can see the defect T-0054 was opened for, because the defect is
// a disagreement *between* the stages: internal/classify decided `credential`
// on pagila's every `last_update timestamptz` and `address` on `film.fulltext`,
// and internal/transform was the first thing to notice, refusing at exit 7 in
// the middle of a run that had already moved rows. It took every whole-pipeline
// run over pagila down.
//
// So this suite runs the real thing end to end as far as transform: zero plan
// refusals, and every planned column masking a batch of the fixture's own rows
// without a refusal. What it does not do is load — internal/verify's suite owns
// the target half — so the assertion here is "the masker produced a value this
// package's coercion can write into the column it came from", which is the
// refusal that was killing the run.

// recordedSampler is pipeline.Sampler over the rows introspect already took,
// which is what internal/core hands the classifier.
type recordedSampler struct{ schema *pipeline.Schema }

func (s recordedSampler) Samples(c ref.ColumnRef) []any {
	for i := range s.schema.Tables {
		t := &s.schema.Tables[i]
		if t.Ref != c.Table {
			continue
		}
		idx := -1
		for j, col := range t.Columns {
			if col.Name == c.Column {
				idx = j
				break
			}
		}
		if idx < 0 {
			return nil
		}
		out := make([]any, 0, len(t.Samples))
		for _, row := range t.Samples {
			if idx < len(row) {
				out = append(out, row[idx])
			}
		}
		return out
	}
	return nil
}

// sampleBatchRows is one batch of a table's own rows, decoded the way extract
// decodes them: the same driver, the same codecs, the same Go kinds. It reads
// on a connection of its own rather than through the snapshot reader, because
// "everything in this table, twenty rows of it" is not a statement shape the
// planner sends and the source's allowlist would refuse it (THREAT_MODEL.md
// T9).
const sampleBatchRows = 20

func batchOf(ctx context.Context, t *testing.T, conn *pgx.Conn, table ref.TableRef) (pipeline.RowBatch, bool) {
	t.Helper()
	sql := fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoteTable(table), sampleBatchRows)
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		t.Fatalf("reading a batch of %s: %v", table, err)
	}
	defer rows.Close()

	b := pipeline.RowBatch{Table: table, Last: true}
	for _, f := range rows.FieldDescriptions() {
		b.Cols = append(b.Cols, f.Name)
	}
	for rows.Next() {
		vals, valErr := rows.Values()
		if valErr != nil {
			t.Fatalf("decoding a row of %s: %v", table, valErr)
		}
		b.Rows = append(b.Rows, vals)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading a batch of %s: %v", table, err)
	}
	return b, len(b.Rows) > 0
}

// checkPlanAndTransform is the whole assertion, run over one fixture.
func checkPlanAndTransform(
	ctx context.Context,
	t *testing.T,
	load func(context.Context, string) error,
	req func(ref.TableRef) pipeline.PlanRequest,
	root ref.TableRef,
	take int,
) {
	t.Helper()
	r, schema, url := fixtureURL(ctx, t, load)

	cls, err := classify.New().Classify(schema, recordedSampler{schema: schema}, nil)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}

	pr := req(root)
	pr.Take = take
	p, err := New().Plan(ctx, r, schema, cls, pr)
	if err != nil {
		// A *Refusal here is the thing this test exists to catch. Its message
		// names the table, the column, the category and the type.
		t.Fatalf("the plan refused: %v", err)
	}

	// Every planned column the classification masks passes the write-back
	// check. Plan returning no error already says so; this says which column
	// would have been first if it had not, and it is the assertion T-CORE reads.
	// constraintsOf is the planner's own reduction, and it needs nothing but
	// the schema: no walk, no keys, no build().
	check := &run{schema: schema}
	byRef := map[ref.TableRef]*pipeline.Table{}
	for i := range schema.Tables {
		byRef[schema.Tables[i].Ref] = &schema.Tables[i]
	}
	masked := 0
	for _, step := range p.Steps {
		table, ok := byRef[step.Table]
		if !ok {
			continue
		}
		for _, col := range table.Columns {
			d, has := cls.Decisions[ref.ColumnRef{Table: step.Table, Column: col.Name}]
			if !has || !d.Masked || d.Category == pipeline.CatNone {
				continue
			}
			masked++
			c, judged := check.constraintsOf(col)
			if !judged {
				continue
			}
			if !mask.Writable(mask.Category(d.Category), d.Masker, c) {
				t.Errorf("%s.%s is masked as %s on a %s column and %s cannot write one there: %s",
					step.Table, col.Name, d.Category, c.TypeTag, d.Masker, d.Reason)
			}
		}
	}
	if masked == 0 {
		t.Fatal("the plan masks no column at all; this fixture is full of personal data and the test is no longer testing anything")
	}
	t.Logf("%d masked columns over %d steps", masked, len(p.Steps))

	// And now the other half: every planned table's own rows through the real
	// transformer. A refusal here is exit 7 mid-run, which is what the plan
	// check above is supposed to have made unreachable.
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to read a batch: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	key, err := mask.NewKey()
	if err != nil {
		t.Fatalf("making a masking key: %v", err)
	}
	res := transform.NewResidual(int64(sampleBatchRows) * int64(len(cls.Decisions)))
	tr := transform.New(schema)
	for _, step := range p.Steps {
		if step.Mode == pipeline.SchemaOnly {
			// No row is copied out of a SchemaOnly step, so no value of it ever
			// reaches a masker.
			continue
		}
		b, nonEmpty := batchOf(ctx, t, conn, step.Table)
		if !nonEmpty {
			continue
		}
		if _, err := tr.Transform(b, cls, &key, res); err != nil {
			t.Errorf("transform refused a batch of %s, which is exit 7 in the middle of a run: %v", step.Table, err)
		}
	}
}

// TestPlanAndTransformPagila is the run that used to die: pagila from customer
// with --take 50, real classifier, real rule pack.
func TestPlanAndTransformPagila(t *testing.T) {
	ctx := context.Background()
	checkPlanAndTransform(ctx, t, testutil.LoadPagila,
		func(root ref.TableRef) pipeline.PlanRequest {
			r := root
			return pipeline.PlanRequest{Root: &r}
		},
		tref("public", "customer"), 50)
}

// TestPlanAndTransformNasty is the same over testdata/nasty.sql from
// tenant_users with --take 3: a composite-key root, quoted identifiers, arrays,
// jsonb, an enum, a domain, a macaddr and every other trap the fixture carries.
func TestPlanAndTransformNasty(t *testing.T) {
	ctx := context.Background()
	checkPlanAndTransform(ctx, t,
		func(ctx context.Context, url string) error { return testutil.LoadNasty(ctx, url, false) },
		nastyRequest, tref("public", "tenant_users"), 3)
}
