// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/ref"
)

// Every statement the planner sends to the source is built here, and every one
// of them is a read of user tables under the run's snapshot. There is no
// pushdown: nothing here creates a temp table, and no statement carries a key
// value as literal text — keys travel as bound array parameters through unnest
// (ARCHITECTURE.md §3).
//
// Identifiers are always quoted, because testdata/nasty.sql trap 9 exists to
// turn an unquoted concatenation into a loud 42P01 rather than a quiet read of
// something else.

// chunkSize is the number of key tuples in one unnest argument list. It is the
// 2,000 of §3's `new.Chunks(2000)`.
const chunkSize = 2000

// countProbeLimit is the bound on the lookup-table count probe: §3's
// `SELECT count(*) FROM (SELECT 1 FROM t LIMIT 1001) x`, one more than the
// 1,000-row lookup ceiling so that "1001" means "more than a lookup".
const countProbeLimit = 1001

// probeSampleRows is how many rows a sampling probe aims to read, and
// probeSampleSeed is the seed it samples with. Both probes that sample take
// them: §3.4's pseudo-key uniqueness probe and §3.2's distinct `_type` read.
// They are one pair of constants rather than two because they are one bound —
// the most rows a probe may read inside the holder transaction (THREAT_MODEL.md
// T9) — and the seed is fixed so that two runs probe the same pages, which is
// what makes a sampled answer reproducible (ARCHITECTURE.md §3, determinism).
const (
	probeSampleRows = 2000
	probeSampleSeed = 1
)

// explicitKeyProbeRows bounds the --key uniqueness probe: a table whose
// reltuples is at or below it is probed whole, and every other table — one that
// is larger, and one nothing has analysed — is probed over a prefix of that
// many rows. The two are the same number because they are the same bound: the
// most rows a probe may aggregate inside the holder transaction.
const explicitKeyProbeRows = 100_000

// quoteIdent quotes one identifier. Every identifier lazyslice writes into a
// statement goes through here.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteTable quotes a schema-qualified table name.
func quoteTable(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// outExpr is the select-list expression for one key column: the column itself,
// except that a kindOther column travels as its text form, which is what
// §2 "Chunk" sends back through unnest($i::text[]), and a kindBpchar column
// does the same because a character(n) read back whole is blank-padded and the
// comparison the chunk join makes is not (keyset.go, typeOf).
func outExpr(alias, col string, ty keyType, n int) string {
	e := alias + "." + quoteIdent(col)
	if ty.textOut() {
		e += "::text"
	}
	return e + " AS " + outName(n)
}

func outName(n int) string { return "o" + strconv.Itoa(n+1) }

// outList renders the select list for a set of key columns.
func outList(alias string, cols []string, types []keyType) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = outExpr(alias, c, types[i], i)
	}
	return strings.Join(parts, ", ")
}

// outOrder renders "ORDER BY o1, o2", which is the only ordering a SELECT
// DISTINCT may carry.
func outOrder(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = outName(i)
	}
	return " ORDER BY " + strings.Join(parts, ", ")
}

// unnestFrom renders the chunk side of a join: one array parameter per column,
// each with the cast §2 gives it.
func unnestFrom(types []keyType) string {
	args := make([]string, len(types))
	names := make([]string, len(types))
	for i, ty := range types {
		args[i] = "$" + strconv.Itoa(i+1) + ty.arrayCast
		names[i] = "k" + strconv.Itoa(i+1)
	}
	return "unnest(" + strings.Join(args, ", ") + ") AS k(" + strings.Join(names, ", ") + ")"
}

// joinOn renders the join condition between a table's columns and the chunk. A
// kindOther column is compared against the text form cast back to the column's
// own type, so the comparison is the type's own equality and never a text one.
func joinOn(alias string, cols []string, types []keyType) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		k := "k." + "k" + strconv.Itoa(i+1)
		if types[i].kind == kindOther {
			k += "::" + types[i].typeName
		}
		parts[i] = alias + "." + quoteIdent(c) + " = " + k
	}
	return strings.Join(parts, " AND ")
}

// notNullAll renders §3's MATCH SIMPLE predicate: every referencing column is
// NOT NULL, because a foreign key with any NULL component references nothing.
// MATCH FULL selects the same rows under it, since a half-NULL row cannot
// exist.
func notNullAll(alias string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c) + " IS NOT NULL"
	}
	return strings.Join(parts, " AND ")
}

// seedSQL is the root's key read: ORDER BY identity LIMIT take, with --where
// applied when it is set.
//
// The LIMIT is applied whether or not --where is set. §3 writes the two cases
// as alternatives; keeping the bound in both is the reading THREAT_MODEL.md
// T11 asks for, and a predicate that matches the whole table is exactly the
// case where dropping it would hurt.
func seedSQL(root ref.TableRef, cols []string, types []keyType, where string, take int) string {
	var b strings.Builder
	b.WriteString("SELECT " + outList("t", cols, types))
	b.WriteString(" FROM " + quoteTable(root) + " t")
	if where != "" {
		b.WriteString(" WHERE (" + where + ")")
	}
	b.WriteString(orderByColumns("t", cols))
	b.WriteString(" LIMIT " + strconv.Itoa(take))
	return b.String()
}

// orderByColumns orders by the table's own columns, which is what a LIMIT has
// to be taken over.
func orderByColumns(alias string, cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = alias + "." + quoteIdent(c)
	}
	return " ORDER BY " + strings.Join(parts, ", ")
}

// mapKeysSQL reads one set of columns out of a table for the rows a chunk of
// another set of columns names. It is §3's parent step (with notNull set, so
// that a NULL foreign-key component references nothing) and, when a foreign key
// references something other than the parent's identity, the translation
// between the two key spaces.
func mapKeysSQL(table ref.TableRef, inCols []string, inTypes []keyType, outCols []string, outTypes []keyType, notNull []string) string {
	var b strings.Builder
	b.WriteString("SELECT DISTINCT " + outList("t", outCols, outTypes))
	b.WriteString(" FROM " + quoteTable(table) + " t")
	b.WriteString(" JOIN " + unnestFrom(inTypes) + " ON " + joinOn("t", inCols, inTypes))
	if len(notNull) > 0 {
		b.WriteString(" WHERE " + notNullAll("t", notNull))
	}
	b.WriteString(outOrder(len(outCols)))
	return b.String()
}

// childKeysSQL is §3's child step: the child rows a chunk of parent keys
// references, capped per parent key per edge by row_number over the edge's own
// columns.
func childKeysSQL(child ref.TableRef, childCols []string, parentTypes []keyType, identity []string, idTypes []keyType, perKey int) string {
	inner := make([]string, 0, len(identity)+1)
	for i, c := range identity {
		inner = append(inner, "t."+quoteIdent(c)+" AS c"+strconv.Itoa(i+1))
	}
	partition := make([]string, len(childCols))
	for i, c := range childCols {
		partition[i] = "t." + quoteIdent(c)
	}
	inner = append(inner, "row_number() OVER (PARTITION BY "+strings.Join(partition, ", ")+
		orderByColumns("t", identity)+") AS rn")

	outs := make([]string, len(identity))
	orders := make([]string, len(identity))
	for i, ty := range idTypes {
		e := "c" + strconv.Itoa(i+1)
		orders[i] = e
		if ty.textOut() {
			e += "::text"
		}
		outs[i] = e + " AS " + outName(i)
	}

	var b strings.Builder
	b.WriteString("SELECT " + strings.Join(outs, ", ") + " FROM (")
	b.WriteString("SELECT " + strings.Join(inner, ", "))
	b.WriteString(" FROM " + quoteTable(child) + " t")
	b.WriteString(" JOIN " + unnestFrom(parentTypes) + " ON " + joinOn("t", childCols, parentTypes))
	b.WriteString(") s WHERE s.rn <= " + strconv.Itoa(perKey))
	b.WriteString(" ORDER BY " + strings.Join(orders, ", "))
	return b.String()
}

// The distinct-value sample, §3.2's `_type` read and the django_content_type
// read. Both forms are bounded in the template as well as here — a bare
// `SELECT DISTINCT col FROM t` is not stopped by a LIMIT above it, since both
// HashAggregate and Sort+GroupAggregate consume their whole input before they
// emit a row, which is the unbounded read inside the holder transaction
// THREAT_MODEL.md T9 forbids — and both are **reproducible**, which the prefix
// alone was not: an unordered `LIMIT n` takes whichever rows the scan reaches
// first, and synchronize_seqscans (on by default) starts a second scan of a
// large table where a recent one left off. §3.2's sample decides which virtual
// edges exist, hence which rows are selected, so a sample that moves between
// two runs over one snapshot breaks §3's determinism rule and I3 with it.
//
// distinctSampleSQL is the form to prefer: TABLESAMPLE SYSTEM at a fraction
// that reads about `limit` rows, REPEATABLE at the fixed seed, so two runs
// visit the same pages — the same reproducibility §3.4's pseudo-key probe takes
// it for. The outer LIMIT is the bound for a stale reltuples, and the sample
// scan it truncates is a serial, ordered page walk, so what it cuts is
// reproducible too.
func distinctSampleSQL(t ref.TableRef, cols []string, types []keyType, num, den int64, limit int) string {
	var b strings.Builder
	b.WriteString("SELECT DISTINCT " + outList("t", cols, types))
	b.WriteString(" FROM (SELECT " + quotedList(cols) + " FROM " + quoteTable(t) +
		" TABLESAMPLE SYSTEM (" + strconv.FormatInt(num, 10) + "::float8 / " + strconv.FormatInt(den, 10) + ")" +
		" REPEATABLE (" + strconv.Itoa(probeSampleSeed) + ")" +
		" LIMIT " + strconv.Itoa(limit) + ") t")
	b.WriteString(outOrder(len(cols)))
	return b.String()
}

// distinctPrefixSQL is the fallback for the two relations TABLESAMPLE cannot be
// taken on — a partitioned table, and one nothing has analysed, where a
// fraction of an unknown row count is a full scan wearing a probe's name. The
// prefix is ordered by the table's own identity columns, because that is what
// makes *which* rows it reads a property of the data rather than of the plan
// the server happened to choose.
func distinctPrefixSQL(t ref.TableRef, cols []string, types []keyType, order []string, limit int) string {
	var b strings.Builder
	b.WriteString("SELECT DISTINCT " + outList("t", cols, types))
	b.WriteString(" FROM (SELECT " + quotedList(cols) + " FROM " + quoteTable(t) +
		" ORDER BY " + quotedList(order) +
		" LIMIT " + strconv.Itoa(limit) + ") t")
	b.WriteString(outOrder(len(cols)))
	return b.String()
}

// quotedList renders a bare, quoted column list.
func quotedList(cols []string) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	return strings.Join(quoted, ", ")
}

// boundedCountSQL is §3's bounded lookup count: it reads at most 1,001 rows and
// can therefore never scan a large table by accident (THREAT_MODEL.md T9).
func boundedCountSQL(t ref.TableRef) string {
	return "SELECT count(*) FROM (SELECT 1 FROM " + quoteTable(t) +
		" LIMIT " + strconv.Itoa(countProbeLimit) + ") s"
}

// explicitKeyProbeSQL asks whether an explicit --key identifies a row.
//
// The aggregate is bounded, never open. A HAVING over the whole table is not
// stopped by the LIMIT above it — both HashAggregate and Sort+GroupAggregate
// consume their whole input before they emit a row, and a --key is passed
// precisely when no index covers the columns — so the shape THREAT_MODEL.md T9
// forbids is exactly the shape this used to have: an unbounded aggregate inside
// the holder transaction, repeated on every run once the key is recorded in
// lazyslice.yml. When limit is set the probe reads that many rows and no more,
// and the identity it yields is IdentityPseudo, because a bounded probe is a
// sampled one.
func explicitKeyProbeSQL(t ref.TableRef, cols []string, limit int) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	list := strings.Join(quoted, ", ")
	src := quoteTable(t)
	if limit > 0 {
		src = "(SELECT " + list + " FROM " + quoteTable(t) + " LIMIT " + strconv.Itoa(limit) + ")"
	}
	return "SELECT count(*) FROM (SELECT 1 FROM " + src + " p" +
		" GROUP BY " + list + " HAVING count(*) > 1 LIMIT 2) s"
}

// pseudoKeyProbeSQL is §3.4's uniqueness probe over a TABLESAMPLE: the
// candidate is trusted only when the sample holds rows and every one of them is
// distinct. A sample that comes back empty is not a pass.
//
// TABLESAMPLE is not accepted on a partitioned table, and a fraction of an
// unknown row count is not a probe, so both cases are probed over a bounded
// prefix instead. That is a weaker probe and it is only ever a fallback: it
// still fails closed on a duplicate it sees, and it never turns the rung that
// large unkeyed tables reach into a full scan inside the holder transaction
// (THREAT_MODEL.md T9).
func pseudoKeyProbeSQL(t ref.TableRef, cols []string, bounded bool, num, den int64) string {
	quoted := make([]string, len(cols))
	for i, c := range cols {
		quoted[i] = quoteIdent(c)
	}
	list := strings.Join(quoted, ", ")
	counts := "count(*), count(DISTINCT (" + list + "))"
	if bounded {
		return "SELECT " + counts + " FROM (SELECT " + list + " FROM " + quoteTable(t) +
			" LIMIT " + strconv.Itoa(probeSampleRows) + ") s"
	}
	return "SELECT " + counts + " FROM " + quoteTable(t) +
		" TABLESAMPLE SYSTEM (" + strconv.FormatInt(num, 10) + "::float8 / " + strconv.FormatInt(den, 10) + ")" +
		" REPEATABLE (" + strconv.Itoa(probeSampleSeed) + ")"
}

// samplePercent is the TABLESAMPLE fraction, as an integer numerator over an
// integer denominator, that reads about probeSampleRows rows. It is asked only
// for a relation whose row count is known: a relation nothing has analysed
// (reltuples -1) takes the bounded prefix instead, because sampling 100% of an
// unknown row count is a full scan wearing a probe's name.
func samplePercent(approx int64) (num, den int64) {
	const den100 = 100 // hundredths of a percent, so the fraction has two decimals
	if approx <= probeSampleRows {
		return 100 * den100, den100
	}
	num = (probeSampleRows * 100 * den100) / approx
	if num < 1 {
		num = 1
	}
	return num, den100
}

// sqlUnreadablePartitionLeaves is the partition leaves the source role cannot
// read.
//
// §3.6 consults RolePrivileges.Unreadable, which now arrives on
// PlanRequest.Priv from one Source.Privileges call — and that query excludes
// partition leaves (`NOT c.relispartition`, internal/pg/source.go), because a
// leaf is not a table anything else in the tree plans, loads or counts. §3.3
// makes the leaves a privilege question all the same: a partitioned table's keys
// are fetched from the *root* and Postgres routes the query to the leaves, so a
// leaf the role cannot SELECT fails the root's own read. Without this query that
// surfaces as a raw 42501 from pgx partway through extract — with the target
// already dropped and partly loaded — where §3.6 says "there is no mid-extract
// permission failure by design" and promises exit 12 with a GRANT naming the
// leaf.
//
// It is deliberately leaves *only*: everything else comes from the one
// privileges read, so the two queries cannot disagree about the same relation.
// Merging it into internal/pg's read is the better shape and is owed there;
// internal/plan/CLAUDE.md records it.
const sqlUnreadablePartitionLeaves = `SELECT n.nspname, c.relname
FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND c.relispartition
  AND left(n.nspname, 3) <> 'pg_' AND n.nspname <> 'information_schema'
  AND NOT has_table_privilege(c.oid, 'SELECT')
ORDER BY n.nspname, c.relname`

// grantStatement is the remedy §3.6 prints. It is rendered here, from
// identifiers, and never carries a statement the run executed.
func grantStatement(t ref.TableRef, role string) string {
	return "GRANT SELECT ON " + quoteTable(t) + " TO " + quoteIdent(role) + ";"
}
