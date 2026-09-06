// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Every statement this package sends — to the target, which it reads directly,
// and to the source, which it reads through the registered shapes in shapes.go
// — is built here.
//
// No value is ever written into the text of a statement. The residual
// confirmation binds its candidate as a parameter and the sample comparison
// binds its keys as typed arrays through unnest, exactly as internal/extract
// does (ARCHITECTURE.md section 2 "Chunk"). Identifiers are always quoted:
// testdata/nasty.sql trap 9 exists to turn an unquoted concatenation into a
// loud 42P01 rather than a quiet read of something else.
//
// The four builders below duplicate internal/extract/sql.go, which is another
// stage package and therefore not importable from here (internal/CLAUDE.md).
// internal/verify/CLAUDE.md records that, and the fix is a shared home for the
// identifier quoting and the chunk join, not a third copy.

// quoteIdent quotes one identifier.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteTable quotes a schema-qualified table name.
func quoteTable(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// qualifiedName quotes a name that may already carry its schema, as
// pipeline.SequenceDef.Name does.
func qualifiedName(name string) string {
	if i := strings.Index(name, "."); i >= 0 {
		return quoteIdent(name[:i]) + "." + quoteIdent(name[i+1:])
	}
	return quoteIdent(name)
}

// columnList renders columns as a select list under an alias.
func columnList(alias string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c)
	}
	return strings.Join(parts, ", ")
}

// countSQL counts the rows of one target table.
func countSQL(t ref.TableRef) string {
	return "SELECT count(*) FROM " + quoteTable(t)
}

// scanSQL reads one column of one target table, whole. The target is small by
// construction (ARCHITECTURE.md section 6 item 4), so this is a scan and not a
// sample, and it is unordered because neither the residual scan nor the second
// net cares which row a value came from — only that the value is there.
func scanSQL(t ref.TableRef, column string) string {
	return "SELECT " + quoteIdent(column) + " FROM " + quoteTable(t)
}

// maxSQL is the maximum of one column of one target table, which is the value a
// sequence must have been reset to. The cast is to the type a sequence counts
// in: a sequence's column is an integer of some width, and reading them all
// back as bigint is one scan destination rather than four.
func maxSQL(t ref.TableRef, column string) string {
	return "SELECT max(" + quoteIdent(column) + ")::bigint FROM " + quoteTable(t)
}

// sequenceSQL reads a sequence's position. last_value and is_called are the two
// halves of the strict-NULL form load writes with setval (ARCHITECTURE.md
// section 11.1, THREAT_MODEL.md T8).
func sequenceSQL(name string) string {
	return "SELECT last_value, is_called FROM " + qualifiedName(name)
}

// orphansSQL counts the child rows of one foreign key that reference a parent
// row the target does not hold.
//
// The NULL handling is the constraint's own: under MATCH SIMPLE a row with any
// NULL key column references nothing and satisfies the edge, and under MATCH
// FULL only an all-NULL row does, so a partially NULL row is counted here.
func orphansSQL(fk pipeline.ForeignKey) string {
	var b strings.Builder
	b.WriteString("SELECT count(*) FROM " + quoteTable(fk.Child) + " c WHERE ")
	b.WriteString("(" + nullFilter(fk) + ") AND NOT EXISTS (SELECT 1 FROM " +
		quoteTable(fk.Parent) + " p WHERE ")
	terms := make([]string, len(fk.ChildCols))
	for i := range fk.ChildCols {
		terms[i] = "p." + quoteIdent(fk.ParentCols[i]) + " = c." + quoteIdent(fk.ChildCols[i])
	}
	b.WriteString(strings.Join(terms, " AND ") + ")")
	return b.String()
}

// nullFilter is the set of child rows the edge applies to.
func nullFilter(fk pipeline.ForeignKey) string {
	terms := make([]string, len(fk.ChildCols))
	for i, c := range fk.ChildCols {
		terms[i] = "c." + quoteIdent(c) + " IS NOT NULL"
	}
	if fk.MatchFull {
		// MATCH FULL: the row is exempt only when every key column is NULL, so
		// a row with one NULL among several is a violation of the constraint
		// itself and is counted.
		return strings.Join(terms, " OR ")
	}
	return strings.Join(terms, " AND ")
}

// sampleSQL is the chunked typed unnest join, the same statement shape
// internal/extract reads a keyed step with (ARCHITECTURE.md section 2 "Chunk").
// It is sent to both sides: to the source through the registered shape in
// shapes.go, and to the target, which has no allowlist.
//
// The keys travel as bound array parameters. Nothing about a row is written
// into the text.
func sampleSQL(t ref.TableRef, cols, idCols, casts []string, ch pipeline.Chunk) string {
	var b strings.Builder
	b.WriteString("SELECT " + columnList("t", cols))
	b.WriteString(" FROM " + quoteTable(t) + " t")
	b.WriteString(" JOIN " + unnestFrom(ch, len(idCols)) + " ON " + joinOn("t", idCols, casts))
	b.WriteString(" ORDER BY " + columnList("t", idCols))
	return b.String()
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

// joinOn compares the table's identity columns with the chunk, casting the
// chunk value back to the column's own type where it travelled as text.
func joinOn(alias string, cols, casts []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c) + " = k.k" + strconv.Itoa(i+1) + casts[i]
	}
	return strings.Join(parts, " AND ")
}

// probeSQL is section 6 item 3's indexable confirmation probe: does the source
// still hold this exact value in this column? The candidate is bound, never
// written into the statement.
func probeSQL(t ref.TableRef, column string) string {
	return "SELECT EXISTS (SELECT 1 FROM " + quoteTable(t) + " t WHERE t." +
		quoteIdent(column) + " = $1)"
}

// foldedProbeSQL is the case-folded probe, run only when the indexable one is
// false. It can be a sequential scan on production, which is half the reason
// the probe cap exists (THREAT_MODEL.md T4).
func foldedProbeSQL(t ref.TableRef, column string) string {
	return "SELECT EXISTS (SELECT 1 FROM " + quoteTable(t) + " t WHERE lower(t." +
		quoteIdent(column) + "::text) = lower($1::text))"
}
