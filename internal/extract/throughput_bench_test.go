// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// BenchmarkExtractThroughput is `make bench`'s subject and the CI regression
// gate's (docs/PERF.md, .github/workflows/bench.yml): this package's own
// row-processing throughput — statement building, scanning into []any and
// batching — with no database and no network, so the number is about this
// package's code and not about a container's disk or a laptop's Docker
// daemon on the day it ran.
//
// It is not a substitute for docs/PERF.md's real-Postgres numbers (extract,
// transform and load against a 2,000,000-row table): those need Docker and
// take tens of seconds, which is the wrong cost for every push. This is the
// number that can run on every pull request.
const benchRows = 200_000

func benchTable() pipeline.Table {
	return pipeline.Table{
		Ref: tbl("bench"),
		Columns: []pipeline.Column{
			intCol("id"), textCol("email"), textCol("body"),
		},
		PK: []string{"id"},
	}
}

func BenchmarkExtractThroughput(b *testing.B) {
	table := benchTable()
	p := &pipeline.Plan{Steps: []pipeline.Step{{
		Table: table.Ref, Mode: pipeline.ChildOK,
		Identity: pipeline.Identity{Columns: []string{"id"}},
		Keys:     fakeKeys{n: benchRows},
	}}}
	r := &fakeReader{rowsFor: func(_ string, args []any) [][]any {
		ids := args[0].([]int64)
		out := make([][]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, []any{id, "user@example.test", "a short message body"})
		}
		return out
	}}
	e := New(schemaFor(table))
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		out := make(chan pipeline.RowBatch, 8)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for batch := range out {
				_ = batch
			}
		}()
		if err := e.Extract(ctx, r, p, out); err != nil {
			b.Fatalf("Extract: %v", err)
		}
		<-done
	}
	b.StopTimer()

	rowsPerSec := float64(b.N) * float64(benchRows) / b.Elapsed().Seconds()
	b.ReportMetric(rowsPerSec, "rows/sec")
}
