// SPDX-License-Identifier: Apache-2.0

package pipeline

import "context"

// RowBatch is the extract to transform to load contract.
//
// Tables are strictly sequential on the channel: every batch of table T is sent
// before the first batch of the next table in plan order, and Last marks T's
// final batch. A table with no rows sends exactly one empty batch with Last set,
// so the loader always sees a boundary. The loader opens T's transaction on Seq
// 0 and commits it on Last; it never infers a boundary from a change of Table.
//
// A future parallel extract must change this contract, not work around it.
//
// RowBatch holds production values. It is never serialised
// (THREAT_MODEL.md T4).
type RowBatch struct {
	Table TableRef
	Cols  []string
	Rows  [][]any
	Seq   int  // per table, from 0
	Last  bool // final batch for Table
}

// Extractor streams the planned rows out of the source snapshot.
type Extractor interface {
	// Extract streams every step in plan order, one table at a time, into out
	// and closes it.
	Extract(ctx context.Context, r Reader, plan *Plan, out chan<- RowBatch) error
}
