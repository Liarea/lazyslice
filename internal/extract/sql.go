// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Every statement extract sends to the source is built here, and every one of
// them is a read of one planned table under the run's snapshot. There is no
// pushdown: nothing here creates a temp table, and no key value is ever written
// into the text of a statement — keys travel as bound array parameters through
// unnest (ARCHITECTURE.md §2 "Chunk", §12).
//
// Identifiers are always quoted, because testdata/nasty.sql trap 9 exists to
// turn an unquoted concatenation into a loud 42P01 rather than a quiet read of
// something else.

// chunkSize is the number of key tuples in one unnest argument list, and
// therefore the largest number of rows one read can return: it is the 2,000 of
// ARCHITECTURE.md §2 ("chunks of 2,000 keys"), the same constant internal/plan
// cuts its own chunks with.
const chunkSize = 2000

// batchRows is the number of rows in one pipeline.RowBatch, the 2,000 of §1's
// "chan RowBatch (2,000 rows × 8, Last per table)". It is the same number as
// chunkSize, so a keyed read produces exactly one batch per chunk; a Lookup
// step has no chunks and is cut into batches of this size directly.
const batchRows = 2000

// batchBytes is the second cap a batch is cut at: a batcher flushes on
// whichever of batchRows or batchBytes it reaches first. 2,000 rows was
// never a memory bound by itself — docs/reviews/2026-09-09/REVIEW.md finding
// 9: "extraction batches 2,000 rows regardless of byte size. Even one batch
// of 2,000 one-MiB values can require roughly two GiB before transformation
// overhead." 8 MiB is a few times rowEstimate's per-value assumptions for an
// ordinary row and small enough that a table of wide (near-1-MiB) values now
// cuts a batch every few rows instead of holding 2,000 of them at once; see
// docs/PERF.md for the measurement.
const batchBytes = 8 << 20

// lookupLimit is the bound the Lookup read carries. A table is a Lookup step
// only after the planner's bounded count proved it under §3's 1,000-row lookup
// ceiling in this same snapshot (internal/plan's countProbeLimit is the same
// 1001, one more than the ceiling so that "1001" means "more than a lookup").
//
// It is in the statement and in the registered shape, not only in the planner's
// reasoning. The allowlist is one per-Source union that every stage registers
// into additively, so a table-agnostic `SELECT ... FROM t ORDER BY ...` here
// would be internal/plan's seed read with its bound removed, for every relation
// (THREAT_MODEL.md T9). The planner proving a table small is not the second
// layer; the allowlist is what has to hold when the planner is the thing that
// is wrong.
const lookupLimit = 1001

// quoteIdent quotes one identifier. Every identifier lazyslice writes into a
// statement goes through here.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteTable quotes a schema-qualified table name.
func quoteTable(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// columnList renders the copied columns as a select list, read as themselves:
// extract does not cast, reformat or rename a value on the way out. Masking is
// internal/transform's and nothing here touches a value.
func columnList(alias string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c)
	}
	return strings.Join(parts, ", ")
}

// orderBy renders "ORDER BY t.\"a\", t.\"b\"". Every read is ordered, because
// two runs over one snapshot have to produce byte-identical output (invariant
// I3) and an unordered read is only reproducible by accident.
func orderBy(alias string, cols []string) string {
	return " ORDER BY " + columnList(alias, cols)
}

// unnestFrom renders the chunk side of the join: one array parameter per
// identity column, each carrying the cast the Chunk itself declares.
func unnestFrom(ch pipeline.Chunk, n int) string {
	args := make([]string, n)
	names := make([]string, n)
	for i := range n {
		args[i] = "$" + strconv.Itoa(i+1) + ch.Cast(i)
		names[i] = "k" + strconv.Itoa(i+1)
	}
	return "unnest(" + strings.Join(args, ", ") + ") AS k(" + strings.Join(names, ", ") + ")"
}

// joinOn renders the comparison between the table's identity columns and the
// chunk, casting the chunk value back to the column's own type where it
// travelled as text (keys.go).
func joinOn(alias string, cols []string, casts []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c) + " = k.k" + strconv.Itoa(i+1) + casts[i]
	}
	return strings.Join(parts, " AND ")
}

// rowsSQL is §12's "chunked typed unnest join over the run snapshot": one
// table's copied columns for one chunk of that step's identity keys. A step's
// rows are read by key and never by predicate, which is what makes the rows in
// the target exactly the rows the plan says are there.
func rowsSQL(t ref.TableRef, cols []string, idCols []string, casts []string, ch pipeline.Chunk) string {
	var b strings.Builder
	b.WriteString("SELECT " + columnList("t", cols))
	b.WriteString(" FROM " + quoteTable(t) + " t")
	b.WriteString(" JOIN " + unnestFrom(ch, len(idCols)) + " ON " + joinOn("t", idCols, casts))
	b.WriteString(orderBy("t", idCols))
	return b.String()
}

// lookupSQL is the read for a Lookup step, which has no key set
// (ARCHITECTURE.md §2 "Step": Keys is nil for Lookup and SchemaOnly) and is
// copied whole, bounded by lookupLimit.
func lookupSQL(t ref.TableRef, cols []string, order []string) string {
	return "SELECT " + columnList("t", cols) +
		" FROM " + quoteTable(t) + " t" +
		orderBy("t", order) +
		" LIMIT " + strconv.Itoa(lookupLimit)
}
