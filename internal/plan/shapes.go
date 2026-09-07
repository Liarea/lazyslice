// SPDX-License-Identifier: Apache-2.0

package plan

// The statement shapes the planner sends to the source. Every statement built
// in sql.go matches one of the templates here, and a statement that matches
// none of them never reaches the server: the source's tracer refuses it with a
// cancelled context and records a violation that fails the run
// (ARCHITECTURE.md §2 "Source", THREAT_MODEL.md T9).
//
// The templates are internal/pg's grammar, not free text
// (internal/pg/tracer.go): {ident}, {idents}, {int}, {selectlist}, {casts},
// {keypred} and {where}. The three the planner needed and the grammar did not
// have — the select-list item, the variable-arity cast list, and the operator's
// own predicate — were added there, where they are reviewed once, rather than
// registered here as a looser template. shapes_test.go builds a statement of
// every shape below with the real builders and runs it through a real
// pg.Tracer, so a change to sql.go that this file does not follow fails that
// test rather than the first run against a production source.

// Statement is one statement this package sends to the source, with the name
// the allowlist and the trace record it under.
//
// It is not internal/pg's Shape, for the reason internal/introspect's Statement
// is not either: ARCHITECTURE.md §2's import graph has the stage packages
// importing pipeline and nothing else of the tree, so the wiring that owns both
// — internal/core — converts these into pg.Shape and registers them before
// Plan is called.
type Statement struct{ Name, SQL string }

// The key reads. seedSQL, mapKeysSQL and childKeysSQL differ in whether a
// --where predicate and a MATCH SIMPLE NOT NULL filter are present, and an
// absent clause is not something a template can say, so each combination the
// planner builds is its own shape. That is the direction that fails safe: a
// shape too narrow refuses a statement of ours and the test above catches it,
// where a shape wide enough to make a clause optional would be a shape that
// admits a statement with the clause missing.
const (
	// seedShape is §3's root read: identity columns, ordered, bounded by --take.
	seedShape = `SELECT {selectlist} FROM {ident} t ORDER BY {idents} LIMIT {int}`

	// seedWhereShape is the same read under --where. The LIMIT is in the
	// template, so a predicate that tried to comment the LIMIT away would not
	// match this shape at all ({where} in internal/pg/tracer.go).
	seedWhereShape = `SELECT {selectlist} FROM {ident} t WHERE ({where}) ORDER BY {idents} LIMIT {int}`

	// mapKeysShape reads one set of columns for the rows a chunk of another set
	// names: §3's parent step, and the translation between key spaces when a
	// foreign key references something other than the parent's identity.
	mapKeysShape = `SELECT DISTINCT {selectlist} FROM {ident} t ` +
		`JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`

	// mapKeysNotNullShape is mapKeysShape with §3's MATCH SIMPLE filter: a
	// foreign key with any NULL component references nothing.
	mapKeysNotNullShape = `SELECT DISTINCT {selectlist} FROM {ident} t ` +
		`JOIN unnest({casts}) AS k({idents}) ON {keypred} WHERE {keypred} ORDER BY {idents}`

	// childKeysShape is §3's child step, capped per parent key per edge by
	// row_number over the edge's own columns.
	childKeysShape = `SELECT {selectlist} FROM (` +
		`SELECT {selectlist}, row_number() OVER (PARTITION BY {idents} ORDER BY {idents}) AS rn ` +
		`FROM {ident} t JOIN unnest({casts}) AS k({idents}) ON {keypred}` +
		`) s WHERE s.rn <= {int} ORDER BY {idents}`
)

// The probes. Every one of them is bounded in the template as well as in the
// builder, so a bound dropped in sql.go is a statement no shape carries
// (THREAT_MODEL.md T9).
//
// That second layer is a property of the whole allowlist, not of this file. A
// Source carries one tracer and every stage registers into it additively
// (pg.Tracer.Register), so a shape registered by another stage that admits an
// unbounded read admits it for the planner's statements too — which is what a
// table-agnostic `SELECT ... FROM t ORDER BY ...` in pg.ExtractShapes() did
// until it was given a LIMIT of its own. TestStatementsOutsideTheGrammarAreStillRefused
// measures this file alone and cannot see that;
// TestTheComposedAllowlistStillRefusesAnUnboundedRead measures the union, and
// is the test that says what will hold at run time.
const (
	// boundedCountShape is §3's lookup count, stopped at countProbeLimit rows.
	boundedCountShape = `SELECT count(*) FROM (SELECT 1 FROM {ident} LIMIT {int}) s`

	// explicitKeyProbeShape is the --key uniqueness probe over a table small
	// enough to aggregate whole.
	explicitKeyProbeShape = `SELECT count(*) FROM (SELECT 1 FROM {ident} p ` +
		`GROUP BY {idents} HAVING count(*) > 1 LIMIT {int}) s`

	// explicitKeyProbeBoundedShape is the same probe over a prefix, which is
	// what every larger table and every unanalysed one gets.
	explicitKeyProbeBoundedShape = `SELECT count(*) FROM (SELECT 1 FROM ` +
		`(SELECT {idents} FROM {ident} LIMIT {int}) p ` +
		`GROUP BY {idents} HAVING count(*) > 1 LIMIT {int}) s`

	// pseudoKeyProbeShape is §3.4's uniqueness probe over a TABLESAMPLE.
	pseudoKeyProbeShape = `SELECT count(*), count(DISTINCT ({idents})) FROM {ident} ` +
		`TABLESAMPLE SYSTEM ({int}::float8 / {int}) REPEATABLE ({int})`

	// pseudoKeyProbeBoundedShape is the same probe over a bounded prefix, for a
	// partitioned table and for a relation whose row count is unknown.
	pseudoKeyProbeBoundedShape = `SELECT count(*), count(DISTINCT ({idents})) FROM ` +
		`(SELECT {idents} FROM {ident} LIMIT {int}) s`
)

// Shapes is every statement shape internal/plan sends to the source. A caller
// registers them on the source allowlist before calling Plan; a statement whose
// shape is not registered never reaches the server (THREAT_MODEL.md T9).
func Shapes() []Statement {
	return []Statement{
		{Name: "plan.seed", SQL: seedShape},
		{Name: "plan.seed_where", SQL: seedWhereShape},
		{Name: "plan.map_keys", SQL: mapKeysShape},
		{Name: "plan.map_keys_not_null", SQL: mapKeysNotNullShape},
		{Name: "plan.child_keys", SQL: childKeysShape},
		{Name: "plan.bounded_count", SQL: boundedCountShape},
		{Name: "plan.explicit_key_probe", SQL: explicitKeyProbeShape},
		{Name: "plan.explicit_key_probe_bounded", SQL: explicitKeyProbeBoundedShape},
		{Name: "plan.pseudo_key_probe", SQL: pseudoKeyProbeShape},
		{Name: "plan.pseudo_key_probe_bounded", SQL: pseudoKeyProbeBoundedShape},
		{Name: "plan.unreadable_partition_leaves", SQL: sqlUnreadablePartitionLeaves},
	}
}
