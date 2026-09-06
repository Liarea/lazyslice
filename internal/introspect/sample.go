// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

const (
	// SampleRows is how many rows of a table reach Table.Samples. The
	// classifier's validators run over them (ARCHITECTURE.md §4) and every one
	// of them is a production value, so this is also the number of production
	// values the process holds per table.
	SampleRows = 200

	// sampleOversample is the factor the requested fraction is multiplied by.
	// TABLESAMPLE SYSTEM picks whole pages, so asking for exactly SampleRows
	// rows returns fewer than that as often as not; asking for three times as
	// many and keeping the first SampleRows costs one extra page read per two
	// and makes an empty sample mean "the table is nearly empty".
	sampleOversample = 3

	// sampleSeed is the REPEATABLE seed. It is a constant, not a random value
	// and not derived from the masking key: two runs over one snapshot must
	// classify identically, and the seed is half of what makes the sample the
	// same one twice (ADR-004).
	sampleSeed = 4242

	// sampleWidenFactor is how much more of a relation the one retry may read
	// when the first sample came back empty, as a multiple of the pages the
	// first fraction asked for. It is a page budget rather than a jump to 100%
	// because TABLESAMPLE's cost is in pages and the LIMIT bounds only rows: on
	// stale-high statistics — the case the retry exists for — a 100% retry is a
	// sequential scan that stops only when it has found its rows, or never
	// (sampleTable).
	sampleWidenFactor = 10

	// sampleLimit is the LIMIT on the sample statement: the server-side bound
	// on what a sample may cost, where SampleRows is the client-side one. It is
	// the oversampled target, so it truncates nothing the client would have
	// kept, and it is what stops TABLESAMPLE SYSTEM (100) — the fraction an
	// unanalysed relation gets — from reading a whole table (sql.go). It is a
	// bound on rows: the pages stop with them only where the pages hold rows,
	// which is why the retry has a page budget of its own (sampleWidenFactor).
	sampleLimit = SampleRows * sampleOversample
)

// errPermissionDenied is SQLSTATE 42501. A source role that lacks SELECT on one
// relation is a condition of that table, not of the run: ARCHITECTURE.md §3.6
// answers it at plan, where an unreadable child-only table is dropped to
// SchemaOnly and an unreadable parent or root stops with exit 12 naming the
// GRANT to run — "There is no mid-extract permission failure by design".
// Introspect runs before plan and has no privileges input, so failing here is
// what would make both of those outcomes unreachable.
const errPermissionDenied = "42501"

// deniedRelation reports whether err is the source refusing one relation, which
// readSamples tolerates. Every other error stays fatal: swallowing them all
// would turn a systematically broken sample statement into a silent
// name-and-type-only classification of the whole database (THREAT_MODEL.md T1).
func deniedRelation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == errPermissionDenied
}

// readSamples fills Table.Samples for every table, in catalog order.
//
// TABLESAMPLE is accepted on regular tables and materialised views only, so a
// partitioned root is sampled through its largest leaf by reltuples (ties by
// name), the rows are attributed to the root and Table.SampledFrom names the
// leaf (ARCHITECTURE.md §4). A partitioned table with no leaves has no samples
// and classifies on its name and type signals alone.
//
// A table the source role cannot SELECT is left in that same state — no samples,
// no SampledFrom — rather than failing the run, because §3.6 answers an
// unreadable table at plan and cannot do so if introspect never returns. That
// is decided twice over. has_table_privilege, read with the table list, keeps
// the statement from being sent at all, so a production source logs no
// permission error and the trace shows no doomed statement; and a 42501 that
// arrives anyway — the privilege was revoked between the two reads — is rolled
// back to a savepoint and the loop goes on. The savepoint is not optional:
// Introspect reads the whole catalog in one transaction, so without it the
// first refusal aborts the transaction and every table after it fails with
// 25P02 (verified on postgres:18).
func (c *catalog) readSamples(ctx context.Context, r pipeline.Reader) error {
	if err := exec(ctx, r, sqlSampleSavepoint); err != nil {
		return err
	}
	for _, t := range c.order {
		tbl := c.tables[t]
		if len(tbl.Columns) == 0 {
			continue
		}
		from, ok := c.sampleSource(t)
		if !ok || c.unreadable[from] {
			continue
		}
		src := c.tables[from]
		rows, err := sampleTable(ctx, r, tbl, from, src.ApproxRows, c.pages[from])
		if err != nil {
			if deniedRelation(err) {
				if rollback := exec(ctx, r, sqlSampleRollback); rollback != nil {
					// The transaction is aborted and nothing after this can
					// read anything, so both errors are the answer: the
					// refusal, and the rollback that could not undo it.
					return fmt.Errorf("%s: %w, and rolling back to the sample savepoint failed: %w", t, err, rollback)
				}
				continue
			}
			return fmt.Errorf("%s: %w", t, err)
		}
		tbl.Samples = rows
		if from != t {
			leaf := from
			tbl.SampledFrom = &leaf
		}
	}
	return nil
}

// sampleSource is the table TABLESAMPLE is issued against for t: t itself when
// it is a regular table, and its largest leaf partition when it is partitioned.
//
// A leaf the source role cannot read is not a candidate, so a partitioned root
// whose largest leaf is unreadable is sampled through the largest one that is
// readable rather than not at all.
func (c *catalog) sampleSource(t ref.TableRef) (ref.TableRef, bool) {
	tbl := c.tables[t]
	if !tbl.Partitioned {
		return t, true
	}
	var (
		best  ref.TableRef
		found bool
	)
	// Partitions is already in (schema, name) order, so taking a strictly
	// larger candidate leaves the first name standing on a tie.
	for _, leaf := range tbl.Partitions {
		lt, ok := c.tables[leaf]
		if !ok || c.unreadable[leaf] {
			continue
		}
		if !found || lt.ApproxRows > c.tables[best].ApproxRows {
			best, found = leaf, true
		}
	}
	return best, found
}

// sampleTable reads up to SampleRows rows of from, projected onto tbl's
// columns in tbl's column order, so that Samples[i][j] is column j of the table
// the samples are attributed to.
//
// A fraction below 100 that returns nothing is widened once, by
// sampleWidenFactor. The statistics can say a table is large when it is not —
// analysed at ten million rows, since deleted down to a few thousand, not
// re-analysed — and the fraction then lands on pages that hold nothing. Without
// the retry the classifier sees no values for that table and every values-only
// signal is lost, which is testdata/README.md trap 20 (a column whose name says
// nothing and whose values are all email addresses) classifying at none and
// being copied in cleartext.
//
// The retry is bounded in *pages*, not only in rows. Widening to 100% would be
// bounded by the LIMIT on the wire and on nothing else: the LIMIT stops when it
// has accumulated its rows, so on the very case the retry exists for — ten
// million rows of statistics over a million pages holding a few thousand live
// rows — the server would read hundreds of thousands of pages to find them, and
// on a bloated relation holding none it would read the whole thing, all while
// the run holds the source snapshot and pins the xmin horizon (THREAT_MODEL.md
// T7; a standby cancels the query after max_standby_streaming_delay). Ten times
// the pages the first sample asked for is a second chance at a bounded price;
// 100% is asked for only when the relation is small enough that the whole of it
// fits in that budget. A table whose live rows are hidden in a large bloated
// relation therefore still comes back with no samples, and classifies on its
// name and type alone, which is the outcome an unreadable table already has.
func sampleTable(ctx context.Context, r pipeline.Reader, tbl *pipeline.Table, from ref.TableRef, approx, pages int64) ([][]any, error) {
	num, den := samplePercent(approx, pages)
	out, err := sampleOnce(ctx, r, tbl, from, num, den)
	if err != nil || len(out) > 0 {
		return out, err
	}
	wide, wideDen := sampleRetryPercent(approx, pages)
	if wide == num && wideDen == den {
		return out, nil
	}
	return sampleOnce(ctx, r, tbl, from, wide, wideDen)
}

func sampleOnce(ctx context.Context, r pipeline.Reader, tbl *pipeline.Table, from ref.TableRef, num, den int64) ([][]any, error) {
	names := make([]string, len(tbl.Columns))
	for i, col := range tbl.Columns {
		names[i] = quoteIdent(col.Name)
	}
	sql := "SELECT " + strings.Join(names, ", ") +
		" FROM " + quoteTable(from) +
		" TABLESAMPLE SYSTEM (" + strconv.FormatInt(num, 10) + "::float8 / " + strconv.FormatInt(den, 10) + ")" +
		" REPEATABLE (" + strconv.Itoa(sampleSeed) + ")" +
		" LIMIT " + strconv.Itoa(sampleLimit)

	rows, err := r.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		out       [][]any
		truncated bool
	)
	for rows.Next() {
		if len(out) >= SampleRows {
			truncated = true
			break
		}
		row := make([]any, len(names))
		dest := make([]any, len(names))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if truncated {
		// Err is not consulted after a deliberate break: the statement did what
		// it was asked to and the remaining rows are none of our business.
		return out, nil
	}
	return out, rows.Err()
}

// samplePercent is the TABLESAMPLE fraction as an integer numerator over an
// integer denominator, in percent. It is a fraction rather than a float because
// the source allowlist's grammar has {int} and no float placeholder
// (internal/pg/tracer.go), and one shape has to cover every table.
//
// It is a fraction of *pages*, because that is what TABLESAMPLE SYSTEM selects:
// reltuples over relpages gives rows per page, which says how many pages hold
// the oversampled target, and that count over relpages is the fraction. Where
// the two estimates agree this is the row fraction; where they do not — a table
// analysed when it was ten times its current size — the page count is the one
// that decides how much is actually read.
//
// A relation nothing has analysed reports relpages 0 and reltuples -1 (carried
// as 0 by readTables), and a freshly restored dump is exactly that. There is
// nothing to derive a fraction from, so it asks for all of it and the LIMIT on
// the statement is the bound (sql.go); the alternative, a small fixed fraction,
// returns nothing on the small table it is meant to protect.
func samplePercent(approx, pages int64) (num, den int64) {
	return pageFraction(wantedPages(approx, pages), pages)
}

// sampleRetryPercent is the fraction the one retry uses when the first sample
// came back empty: sampleWidenFactor times the pages the first one asked for,
// which is 100% only when the whole relation fits in that budget. It equals
// samplePercent exactly when the first fraction was already the whole relation,
// and sampleTable reads that equality as "there is no retry to make".
func sampleRetryPercent(approx, pages int64) (num, den int64) {
	return pageFraction(sampleWidenFactor*wantedPages(approx, pages), pages)
}

// wantedPages is how many pages hold the oversampled target, from the two
// estimates. A relation nothing has analysed reports both as zero and there is
// nothing to derive from, which pageFraction answers with 100%.
func wantedPages(approx, pages int64) int64 {
	const want = SampleRows * sampleOversample
	if pages <= 0 {
		return 0
	}
	perPage := int64(1)
	if approx > 0 {
		perPage = (approx + pages - 1) / pages
	}
	return (want + perPage - 1) / perPage
}

// pageFraction is a page count as a percentage of the relation, as an integer
// numerator over an integer denominator.
func pageFraction(want, pages int64) (num, den int64) {
	if pages <= 0 || want >= pages {
		return 100, 1
	}
	return 100 * want, pages
}

// quoteIdent renders one identifier for a statement lazyslice builds.
// public."LegacyCustomer" exists only when quoted, and an identifier
// concatenated unquoted fails there with 42P01 rather than reading something
// else (testdata/README.md trap 9).
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteTable(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}
