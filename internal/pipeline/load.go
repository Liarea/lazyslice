// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"time"
)

// LoadResult is what the loader wrote.
type LoadResult struct {
	Rows      map[TableRef]int64
	Sequences map[TableRef][]string
	Elapsed   time.Duration
}

// Loader recreates the schema and copies the rows in.
type Loader interface {
	// Load recreates the object classes in ARCHITECTURE.md section 11.1
	// (pre-data), copies every table under one transaction opened at Seq 0 and
	// committed at Last, then creates indexes, foreign keys NOT VALID, resets
	// sequences and runs ANALYZE.
	//
	// Foreign keys are created after the data, so no edge is deferred and the
	// load order inside a strongly connected component does not matter.
	Load(ctx context.Context, w Writer, plan *Plan, schema *Schema, in <-chan RowBatch) (*LoadResult, error)
}
