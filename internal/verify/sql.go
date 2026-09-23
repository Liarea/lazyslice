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

// scanRowsSQL reads several columns of one target table, whole: the identity
// columns and the masked column beside them, for a column whose residual hits
// ADR-015 may explain by row identity (explain.go). It is a target read like
// scanSQL, unordered for the same reason.
func scanRowsSQL(t ref.TableRef, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = quoteIdent(c)
	}
	return "SELECT " + strings.Join(parts, ", ") + " FROM " + quoteTable(t)
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

// sequenceNameSQL asks the *target* which sequence backs one column, and answers
// with the empty string when neither the target's own answer nor the source's
// name names a relation the target has.
//
// It exists for the same reason the identity branch of ddl.Setvals does: an
// identity column's sequence is created and named by the target, and the two
// names part company the moment the source's table has been renamed, because
// Postgres does not rename an owned sequence with its table. Metabase's
// `sandboxes` still owns `group_table_access_policy_id_seq`; the target's is
// `sandboxes_id_seq`; reading the source's name there is 42P01 against a target
// that is correct (testdata/regressions/006-identity-sequence-renamed-table.sql).
//
// The source's name is the fallback and not the first choice, because
// pg_get_serial_sequence answers only where ownership is recorded: pagila
// declares none — every one of its sequences is reached through a column default
// calling nextval — and for those the target's name is the source's, since that
// is the name §11.1's CREATE SEQUENCE gave it.
func sequenceNameSQL(t ref.TableRef, column, fallback string) string {
	return "SELECT coalesce((SELECT n.nspname || '.' || c.relname" +
		" FROM pg_catalog.pg_class c" +
		" JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace" +
		" WHERE c.oid = coalesce(" +
		"pg_catalog.to_regclass(pg_catalog.pg_get_serial_sequence(" +
		quoteStringLiteral(quoteTable(t)) + ", " + quoteStringLiteral(column) + "))," +
		" pg_catalog.to_regclass(" + quoteStringLiteral(qualifiedName(fallback)) + "))), '')"
}

// quoteStringLiteral renders a Go string as a SQL string literal. Every value it
// is given here is an identifier this package built, never a row.
func quoteStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
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

// rowCheckSQL is ADR-015's row check (explain.go): the same chunked typed
// unnest join sampleSQL is, over identity tuples the target holds verbatim,
// selecting the identity columns and the one masked column, ordered by the
// identity. It is sent to the source only, through Source.Short, and each
// statement counts as one probe against --residual-probe-cap.
//
// The table's alias is `r` and not sampleSQL's `t`, so that the two statements
// match two different registered shapes (shapes.go) and the source's trace
// names a row check as one.
//
// Only identifiers the target already holds travel: the identity values are
// bound as typed arrays, and the candidate value itself is never bound.
func rowCheckSQL(t ref.TableRef, cols, idCols, casts []string, ch pipeline.Chunk) string {
	var b strings.Builder
	b.WriteString("SELECT " + columnList("r", cols))
	b.WriteString(" FROM " + quoteTable(t) + " r")
	b.WriteString(" JOIN " + unnestFrom(ch, len(idCols)) + " ON " + joinOn("r", idCols, casts))
	b.WriteString(" ORDER BY " + columnList("r", idCols))
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

// The two catalog reads of ARCHITECTURE.md section 6's catalog pass
// (catalog.go). They are the target's own pg_attrdef and pg_constraint, read
// back after the load, and they carry no parameter: the whole of what a literal
// rule has to look at is every expression the target's schema holds.
//
// pg_attrdef holds a generated column's expression as well as an ordinary
// default -- pg_attribute.attgenerated is what tells them apart -- so one read
// covers two of the three object classes section 11.1 recreates that can carry
// a literal. The system schemas are excluded by name and by the pg_ prefix,
// because lazyslice never writes into one and pg_catalog's own defaults are
// thousands of rows of noise.
//
// Both are ordered, so two runs over one target report the same object first.
const catalogDefaultsSQL = `
SELECT n.nspname,
       c.relname,
       a.attname,
       CASE WHEN a.attgenerated <> '' THEN 'generated expression' ELSE 'default' END,
       pg_get_expr(d.adbin, d.adrelid)
  FROM pg_attrdef d
  JOIN pg_class c ON c.oid = d.adrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum
 WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
 ORDER BY n.nspname, c.relname, a.attname`

// catalogIndexesSQL is the third catalog read: a partial index's predicate and
// an expression index's key expressions. Neither has a pg_constraint row —
// pg_index.indpred and pg_index.indexprs are where they live — and
// internal/load/ddl replays an index through pg_get_indexdef, so
// `CREATE UNIQUE INDEX ... WHERE email = 'x@y.test'` carries its literal into
// the target by a route internal/plan's own pass does not walk at all
// (tracker T-0163). This is the only control over it, which is why the two
// halves are read here rather than left to the comment above.
const catalogIndexesSQL = `
SELECT n.nspname,
       c.relname,
       ic.relname,
       'index predicate',
       pg_get_expr(i.indpred, i.indrelid)
  FROM pg_index i
  JOIN pg_class ic ON ic.oid = i.indexrelid
  JOIN pg_class c ON c.oid = i.indrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE i.indpred IS NOT NULL
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
UNION ALL
SELECT n.nspname,
       c.relname,
       ic.relname,
       'index expression',
       pg_get_expr(i.indexprs, i.indrelid)
  FROM pg_index i
  JOIN pg_class ic ON ic.oid = i.indexrelid
  JOIN pg_class c ON c.oid = i.indrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE i.indexprs IS NOT NULL
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
 ORDER BY 1, 2, 3, 4`

// A domain's CHECK is in here too (conrelid is 0 rather than absent), and it
// names its *type* rather than a relation: pg_constraint.contypid is the
// domain. The second column carries that name and the kind says which of the
// two a row is, because internal/plan's --allow-type-literal opt-out is per type and
// this pass has to be able to attribute a domain's CHECK to the type the
// operator named (the T-REDFIX review's fourth finding). Before that the row
// carried an empty relation name and nothing said which domain it belonged to.
const catalogConstraintsSQL = `
SELECT n.nspname,
       coalesce(c.relname, dt.typname, ''),
       t.conname,
       CASE WHEN t.contypid <> 0 THEN 'domain constraint' ELSE 'constraint' END,
       pg_get_constraintdef(t.oid)
  FROM pg_constraint t
  JOIN pg_namespace n ON n.oid = t.connamespace
  LEFT JOIN pg_class c ON c.oid = t.conrelid
  LEFT JOIN pg_type dt ON dt.oid = t.contypid
 WHERE t.contype IN ('c', 'x')
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
 ORDER BY n.nspname, coalesce(c.relname, dt.typname, ''), t.conname`

// catalogEnumLabelsSQL is the fourth catalog read: the labels of every enum
// type in a user schema (the 2026-09-15 red team's A4b and A11).
//
// internal/load/ddl recreates an enum with `CREATE TYPE ... AS ENUM (...)`,
// spelling every label as a string literal, so a label is a DDL string literal
// that crosses into the target verbatim exactly as a DEFAULT does — and
// neither this pass nor internal/plan's read pg_enum, so
// `CREATE TYPE assignee AS ENUM ('unassigned','enum.canary@bigcorp.com',
// '+1-415-555-0199')` put an address and a phone number into the target under
// a green tick, with nothing in the yml.
//
// The label's *ordinal* is what the refusal names, never the label text
// (THREAT_MODEL.md T4), which is why enumsortorder is selected rather than the
// label being used as the object name.
const catalogEnumLabelsSQL = `
SELECT n.nspname,
       '',
       t.typname || ' label ' || e.enumsortorder::text,
       'enum label',
       e.enumlabel
  FROM pg_enum e
  JOIN pg_type t ON t.oid = e.enumtypid
  JOIN pg_namespace n ON n.oid = t.typnamespace
 WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
 ORDER BY n.nspname, t.typname, e.enumsortorder`

// catalogDomainDefaultsSQL is the fifth catalog read: a domain's DEFAULT,
// which lives in pg_type.typdefault and not in pg_attrdef (the 2026-09-15 red
// team's A12).
//
// It is one catalog table to the left of where T-0134 looked, and it is the
// same mechanism as the 2026-09-09 review's finding 5: the value sits in the
// target's catalog and the application's next INSERT that omits the column
// materialises it into a row. A domain's CHECK is already covered, because
// pg_constraint carries it with conrelid 0; its DEFAULT was covered by
// nothing.
const catalogDomainDefaultsSQL = `
SELECT n.nspname,
       '',
       t.typname,
       'domain default',
       t.typdefault
  FROM pg_type t
  JOIN pg_namespace n ON n.oid = t.typnamespace
 WHERE t.typtype = 'd'
   AND t.typdefault IS NOT NULL
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND left(n.nspname, 3) <> 'pg_'
 ORDER BY n.nspname, t.typname`
