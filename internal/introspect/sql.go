// SPDX-License-Identifier: Apache-2.0

package introspect

import "strings"

// Every statement this package sends to the source is a constant below, and
// every one of them is registered on the source allowlist under the name
// Shapes gives it (ARCHITECTURE.md §2 "Source", THREAT_MODEL.md T9). The
// statement text and the registered template are the same string wherever the
// statement is fixed, so the two cannot drift; the one statement that is not
// fixed — the sample — has a template beside it and sampleSQL is the only
// place that builds it.
//
// Identifier filter: a user schema is one whose name does not begin "pg_" and
// is not information_schema. It is spelled with left(nspname, 3) rather than
// NOT LIKE 'pg\_%' so that no statement lazyslice sends carries a backslash
// (internal/pg/tracer.go's elideLiterals treats one as a statement whose
// literals cannot be shown to be elided).
const userSchemas = `left(n.nspname, 3) <> 'pg_' AND n.nspname <> 'information_schema'`

// notFromExtension excludes an object an extension owns: CREATE EXTENSION
// recreates those in the target, so they are not ours to describe
// (ARCHITECTURE.md §11.1 item 2).
const notFromExtension = `NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                WHERE d.classid = %s AND d.objid = %s AND d.deptype = 'e')`

func extensionFilter(classid, objid string) string {
	return strings.Replace(strings.Replace(notFromExtension, "%s", classid, 1), "%s", objid, 1)
}

const sqlServerVersion = `SELECT current_setting('server_version_num')::int`

const sqlSchemas = `SELECT n.nspname
FROM pg_catalog.pg_namespace n
WHERE ` + userSchemas + `
ORDER BY n.nspname`

// sqlExtensions is the extensions ARCHITECTURE.md §11.1 item 2 recreates:
// "every extension a recreated column type, default, index or operator depends
// on ... and any other found through pg_depend", not every extension installed.
// The difference matters twice. A target that cannot create pg_cron, pgaudit or
// postgis fails at DDL for an extension nothing lazyslice recreates needs; and
// Schema.Fingerprint hashes this list, so an extension the source has and the
// target does not would stop §11.2's marker from ever binding.
//
// The walk is one hop of pg_depend from the objects §11.1 recreates — the
// tables, their defaults, their constraints, their indexes, and the enums,
// domains and composites of a user schema — onto whatever those reference, then
// one further hop through pg_type for an array's element type and a domain's
// base type, because pg_depend records a citext[] column's dependency on _citext
// and extension membership is recorded on citext.
//
// A user type reaches the walk three ways, because Postgres files a type's own
// dependencies under three different objects. The pg_type row carries a domain's
// base type. A composite's *fields* are attributes of its pg_class relation, so
// the dependency is recorded as (pg_class, typrelid, attnum) and a walk that
// only knew the pg_type row would miss them: verified on postgres:16, a
// `CREATE TYPE public.addr AS (email citext, note text)` used by a table
// returned no extension at all, while Schema.Composites still carried the CREATE
// TYPE — so §11.1 item 2 would create no extension and item 3 would then fail
// with "type citext does not exist", against a target whose tables item 1 had
// already dropped. A domain's CHECK constraints are pg_constraint rows with
// conrelid = 0 and contypid = the type, so the conrelid branch above cannot see
// them either.
//
// The composite branch carries relkind = 'c', exactly as sqlComposites does,
// because a view's and a materialised view's row types are typtype 'c' too and
// their typrelid is the view itself. Without it, (pg_class, view oid) entered
// the walk, and a view's columns are attributes of that relation, so every
// extension a view alone used was collected: verified on postgres:16, a table
// of (int, text) with `CREATE VIEW v AS SELECT id, addr::citext FROM t`
// returned citext before this filter and returns nothing after it, while a
// citext *column* and a composite with a citext field still return it. §11.1
// does not recreate views, so such an extension is one the target may not have
// — and, because Schema.Fingerprint hashes this list, a target lazyslice wrote
// (which holds no views) could never fingerprint equal to its source, the
// marker would never bind and every second run would be refused with exit 4.
var sqlExtensions = `WITH recreated AS (
  SELECT c.oid
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE c.relkind IN ('r', 'p') AND ` + userSchemas + `
     AND ` + extensionFilter(`'pg_class'::regclass`, `c.oid`) + `
), types AS (
  SELECT t.oid, t.typtype, t.typrelid
    FROM pg_catalog.pg_type t
    JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
   WHERE t.typtype IN ('e', 'd', 'c') AND ` + userSchemas + `
     AND ` + extensionFilter(`'pg_type'::regclass`, `t.oid`) + `
), ours AS (
  SELECT 'pg_class'::regclass AS classid, oid AS objid FROM recreated
  UNION ALL
  SELECT 'pg_attrdef'::regclass, ad.oid FROM pg_catalog.pg_attrdef ad
   WHERE ad.adrelid IN (SELECT oid FROM recreated)
  UNION ALL
  SELECT 'pg_constraint'::regclass, con.oid FROM pg_catalog.pg_constraint con
   WHERE con.conrelid IN (SELECT oid FROM recreated)
  UNION ALL
  SELECT 'pg_class'::regclass, i.indexrelid FROM pg_catalog.pg_index i
   WHERE i.indrelid IN (SELECT oid FROM recreated)
  UNION ALL
  SELECT 'pg_type'::regclass, oid FROM types
  UNION ALL
  SELECT 'pg_class'::regclass, t.typrelid FROM types t
    JOIN pg_catalog.pg_class rc ON rc.oid = t.typrelid
   WHERE t.typtype = 'c' AND rc.relkind = 'c'
  UNION ALL
  SELECT 'pg_constraint'::regclass, con.oid FROM pg_catalog.pg_constraint con
   WHERE con.contypid IN (SELECT oid FROM types)
), referenced AS (
  SELECT DISTINCT d.refclassid, d.refobjid
    FROM pg_catalog.pg_depend d
    JOIN ours o ON o.classid = d.classid AND o.objid = d.objid
   WHERE d.deptype <> 'e'
), needed AS (
  SELECT refclassid, refobjid FROM referenced
  UNION
  SELECT 'pg_type'::regclass, t.typelem
    FROM pg_catalog.pg_type t
    JOIN referenced ON referenced.refclassid = 'pg_type'::regclass
                   AND referenced.refobjid = t.oid
   WHERE t.typelem <> 0
  UNION
  SELECT 'pg_type'::regclass, t.typbasetype
    FROM pg_catalog.pg_type t
    JOIN referenced ON referenced.refclassid = 'pg_type'::regclass
                   AND referenced.refobjid = t.oid
   WHERE t.typbasetype <> 0
)
SELECT e.extname, n.nspname
FROM pg_catalog.pg_extension e
JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname <> 'plpgsql'
  AND EXISTS (SELECT 1 FROM pg_catalog.pg_depend x
                JOIN needed ON x.classid = needed.refclassid AND x.objid = needed.refobjid
               WHERE x.deptype = 'e'
                 AND x.refclassid = 'pg_extension'::regclass AND x.refobjid = e.oid)
ORDER BY e.extname`

// sqlEnums, sqlDomains and sqlComposites exclude a type an extension owns, the
// same way sqlTables excludes its tables: §11.1 item 2 creates the extension
// before item 3 creates the types, so recreating one of its types is a 42710 on
// a target whose tables have already been dropped (dblink ships
// public.dblink_pkey_results, hstore ships an operator class, PostGIS ships
// several types).
var sqlEnums = `SELECT n.nspname || '.' || t.typname, e.enumlabel
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
JOIN pg_catalog.pg_enum e ON e.enumtypid = t.oid
WHERE t.typtype = 'e' AND ` + userSchemas + `
  AND ` + extensionFilter(`'pg_type'::regclass`, `t.oid`) + `
ORDER BY n.nspname, t.typname, e.enumsortorder`

// sqlDomains renders CREATE DOMAIN from the catalog's own deparsers: the base
// type through format_type, the default through pg_get_expr, each constraint
// through pg_get_constraintdef. Only the keywords between them are ours
// (internal/introspect/CLAUDE.md).
var sqlDomains = `SELECT n.nspname || '.' || t.typname,
       'CREATE DOMAIN ' || quote_ident(n.nspname) || '.' || quote_ident(t.typname)
       || ' AS ' || pg_catalog.format_type(t.typbasetype, t.typtypmod)
       || coalesce(' COLLATE ' || quote_ident(cn.nspname) || '.' || quote_ident(co.collname), '')
       || coalesce(' DEFAULT ' || pg_catalog.pg_get_expr(t.typdefaultbin, 0), '')
       || CASE WHEN t.typnotnull THEN ' NOT NULL' ELSE '' END
       || coalesce((SELECT ' ' || string_agg('CONSTRAINT ' || quote_ident(c.conname) || ' '
                                             || pg_catalog.pg_get_constraintdef(c.oid), ' '
                                             ORDER BY c.conname)
                    FROM pg_catalog.pg_constraint c WHERE c.contypid = t.oid), '')
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
LEFT JOIN pg_catalog.pg_collation co ON co.oid = t.typcollation AND co.collname <> 'default'
LEFT JOIN pg_catalog.pg_namespace cn ON cn.oid = co.collnamespace
WHERE t.typtype = 'd' AND ` + userSchemas + `
  AND ` + extensionFilter(`'pg_type'::regclass`, `t.oid`) + `
ORDER BY n.nspname, t.typname`

var sqlComposites = `SELECT n.nspname || '.' || t.typname,
       'CREATE TYPE ' || quote_ident(n.nspname) || '.' || quote_ident(t.typname) || ' AS ('
       || coalesce((SELECT string_agg(quote_ident(a.attname) || ' '
                                      || pg_catalog.format_type(a.atttypid, a.atttypmod), ', '
                                      ORDER BY a.attnum)
                    FROM pg_catalog.pg_attribute a
                    WHERE a.attrelid = t.typrelid AND a.attnum > 0 AND NOT a.attisdropped), '')
       || ')'
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
JOIN pg_catalog.pg_class c ON c.oid = t.typrelid
WHERE t.typtype = 'c' AND c.relkind = 'c' AND ` + userSchemas + `
  AND ` + extensionFilter(`'pg_type'::regclass`, `t.oid`) + `
ORDER BY n.nspname, t.typname`

// sqlTables is every user table, partitions included: a leaf partition is a
// table of the source and the planner names it in Table.Partitions, even
// though §11.1 recreates only the root.
// relpages is read beside reltuples because the sample fraction is a fraction
// of pages, which is the unit TABLESAMPLE SYSTEM works in (sample.go).
// has_table_privilege is read for the sampler: it is the same predicate
// RolePrivileges.Unreadable is built from (ARCHITECTURE.md §2 "Source"), and it
// is what stops a sample being sent to a relation the source role cannot read
// (sample.go).
var sqlTables = `SELECT n.nspname, c.relname, c.relkind::text, c.relispartition,
       c.relrowsecurity, c.relforcerowsecurity, c.reltuples::int8, c.relpages::int8,
       coalesce(pg_catalog.pg_get_partkeydef(c.oid), ''),
       (SELECT count(*) FROM pg_catalog.pg_trigger tg
         WHERE tg.tgrelid = c.oid AND NOT tg.tgisinternal),
       pg_catalog.has_table_privilege(c.oid, 'SELECT')
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND ` + userSchemas + `
  AND ` + extensionFilter(`'pg_class'::regclass`, `c.oid`) + `
ORDER BY n.nspname, c.relname`

const sqlColumns = `SELECT n.nspname, c.relname, a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod), a.atttypid::int8, a.atttypmod,
       coalesce(co.collname, ''), NOT a.attnotnull,
       coalesce(pg_catalog.pg_get_expr(ad.adbin, ad.adrelid), ''),
       a.attgenerated::text, a.attidentity::text,
       CASE WHEN t.typtype = 'd' THEN tn.nspname || '.' || t.typname ELSE '' END
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
JOIN pg_catalog.pg_namespace tn ON tn.oid = t.typnamespace
LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN pg_catalog.pg_collation co ON co.oid = a.attcollation AND co.collname <> 'default'
WHERE c.relkind IN ('r', 'p') AND a.attnum > 0 AND NOT a.attisdropped AND ` + userSchemas + `
ORDER BY n.nspname, c.relname, a.attnum`

// sqlColumnChecks is one row per (column, CHECK constraint naming it). A check
// over three columns appears three times, which is what Column.Checks means.
const sqlColumnChecks = `SELECT n.nspname, c.relname, a.attname,
       pg_catalog.pg_get_constraintdef(con.oid)
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN LATERAL unnest(con.conkey) AS k(attnum)
JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
WHERE con.contype = 'c' AND c.relkind IN ('r', 'p') AND ` + userSchemas + `
ORDER BY n.nspname, c.relname, a.attname, con.conname`

// sqlIndexes reports only an index that is live, valid and ready. A failed
// CREATE UNIQUE INDEX CONCURRENTLY leaves indislive true with indisvalid and
// indisready false, and pg_get_indexdef still prints CREATE UNIQUE INDEX: the
// index enforces nothing and the table may hold duplicates, so §3.4's identity
// ladder must not take it for a key (testdata/README.md trap 12) and §11.1 item
// 6 must not replay it as a valid one.
//
// indimmediate is read because it is the only catalog field that separates the
// index backing a DEFERRABLE primary key or unique constraint from a usable
// key: it is unique, non-partial, non-expression, valid, live and ready like
// any other, and Postgres still refuses it as the referenced side of a foreign
// key (verified on postgres:16). hasKeyOver needs it.
const sqlIndexes = `SELECT n.nspname, c.relname, ic.relname,
       i.indisunique, i.indpred IS NOT NULL, i.indexprs IS NOT NULL,
       i.indimmediate,
       pg_catalog.pg_get_indexdef(i.indexrelid),
       (SELECT coalesce(array_agg(a.attname ORDER BY k.ord), '{}'::text[])
          FROM unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_catalog.pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
         WHERE k.ord <= i.indnkeyatts)
FROM pg_catalog.pg_index i
JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
JOIN pg_catalog.pg_class c ON c.oid = i.indrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND i.indislive AND i.indisvalid AND i.indisready
  AND ` + userSchemas + `
ORDER BY n.nspname, c.relname, ic.relname`

// sqlConstraints is ordered by oid, which is creation order (ARCHITECTURE.md
// §2 "Constraints ... in creation order").
//
// contype is restricted to the five kinds §2 enumerates. PostgreSQL 18 stores a
// column's NOT NULL in pg_constraint as contype 'n', which is not one of them:
// unfiltered, nasty.sql yields 43 constraints on 14 and 16 and 122 on 18, so the
// same logical schema would fingerprint differently per major (§11.2's marker
// could never bind across them) and §11.1 item 5 would emit PG18-only
// `ADD CONSTRAINT ... NOT NULL a` DDL duplicating the column's own NOT NULL.
const sqlConstraints = `SELECT n.nspname, c.relname, con.conname, con.contype::text,
       pg_catalog.pg_get_constraintdef(con.oid)
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND con.contype IN ('p', 'u', 'c', 'f', 'x')
  AND ` + userSchemas + `
ORDER BY n.nspname, c.relname, con.oid`

const sqlPrimaryKeys = `SELECT n.nspname, c.relname, a.attname
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN LATERAL unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
WHERE con.contype = 'p' AND c.relkind IN ('r', 'p') AND ` + userSchemas + `
ORDER BY n.nspname, c.relname, k.ord`

// sqlSequences is every sequence a user table needs, reached two ways.
//
// A sequence is *owned* by a column when pg_depend records deptype 'i' (an
// identity column) or 'a' (ALTER SEQUENCE ... OWNED BY); SequenceDef.Column
// names that column, which is how §11.1 tells an identity sequence from one it
// must create in its own right. A sequence is merely *referenced* when a column
// default calls nextval on it and nobody ever declared ownership — which is how
// pagila v3.1.0 is written, so a query that read ownership alone would drop
// every sequence in that fixture and leave twenty-two defaults in the target
// calling a sequence that does not exist. Such a sequence has Column "",
// exactly as ARCHITECTURE.md §2 says of one pg_get_serial_sequence does not
// resolve.
const sqlSequences = `SELECT n.nspname, t.relname,
       coalesce(CASE WHEN u.owned THEN a.attname END, ''),
       sn.nspname || '.' || s.relname,
       q.seqstart, q.seqincrement, q.seqmin, q.seqmax, q.seqcache, q.seqcycle
FROM pg_catalog.pg_class s
JOIN pg_catalog.pg_namespace sn ON sn.oid = s.relnamespace
JOIN pg_catalog.pg_sequence q ON q.seqrelid = s.oid
CROSS JOIN LATERAL (
  SELECT reloid, attnum, bool_or(owned) AS owned FROM (
    SELECT d.refobjid AS reloid, d.refobjsubid AS attnum, true AS owned
      FROM pg_catalog.pg_depend d
     WHERE d.classid = 'pg_class'::regclass AND d.objid = s.oid
       AND d.refclassid = 'pg_class'::regclass AND d.deptype IN ('a', 'i')
    UNION ALL
    SELECT ad.adrelid, ad.adnum, false
      FROM pg_catalog.pg_depend d
      JOIN pg_catalog.pg_attrdef ad ON ad.oid = d.objid
     WHERE d.classid = 'pg_attrdef'::regclass
       AND d.refclassid = 'pg_class'::regclass AND d.refobjid = s.oid
  ) r GROUP BY reloid, attnum
) u
JOIN pg_catalog.pg_class t ON t.oid = u.reloid
JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = t.oid AND a.attnum = u.attnum
WHERE s.relkind = 'S' AND t.relkind IN ('r', 'p') AND ` + userSchemas + `
ORDER BY n.nspname, t.relname, sn.nspname, s.relname`

// sqlForeignKeys keys edges by constraint name, never by (child, parent):
// pagila's film.language_id and film.original_language_id both point at
// language, and a set keyed by the pair silently loses one (testdata/README.md).
// conparentid = 0 drops the copies Postgres makes of a partitioned table's
// foreign key on each partition, so a partitioned root contributes one edge.
//
// Both ends are filtered exactly as sqlTables filters the table list, so
// Schema.FKs can only name a table Schema.Tables describes: relkind 'r' or 'p'
// (never a foreign table) and not owned by an extension (an extension whose
// tables live outside a pg_ schema — _timescaledb_catalog, pgq — is excluded
// from the table list and must be excluded from the edges too).
//
// A partition end is not only the clone case: `ALTER TABLE ev_2024 ADD
// CONSTRAINT ... FOREIGN KEY` on a leaf, and an edge referencing a leaf, both
// have conparentid = 0 (verified on postgres:16). Such an end is *not* filtered
// out here: the edge is read, and repointPartitionForeignKeys moves each end to
// its root, which is the table §11.1 recreates and the planner addresses. The
// alternative — dropping it here — lost a real dependency of the root's data
// with nothing printed: the parent rows were never pulled and, the constraint
// not being recreated either, the load succeeded on a referentially incomplete
// slice.
var sqlForeignKeys = `SELECT con.conname,
       cn.nspname, cc.relname, pn.nspname, pc.relname,
       con.confmatchtype::text = 'f', con.convalidated,
       (SELECT coalesce(array_agg(a.attname ORDER BY k.ord), '{}'::text[])
          FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
       (SELECT coalesce(array_agg(a.attname ORDER BY k.ord), '{}'::text[])
          FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_catalog.pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum)
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class cc ON cc.oid = con.conrelid
JOIN pg_catalog.pg_namespace cn ON cn.oid = cc.relnamespace
JOIN pg_catalog.pg_class pc ON pc.oid = con.confrelid
JOIN pg_catalog.pg_namespace pn ON pn.oid = pc.relnamespace
WHERE con.contype = 'f' AND con.conparentid = 0
  AND cc.relkind IN ('r', 'p')
  AND pc.relkind IN ('r', 'p')
  AND left(cn.nspname, 3) <> 'pg_' AND cn.nspname <> 'information_schema'
  AND left(pn.nspname, 3) <> 'pg_' AND pn.nspname <> 'information_schema'
  AND ` + extensionFilter(`'pg_class'::regclass`, `cc.oid`) + `
  AND ` + extensionFilter(`'pg_class'::regclass`, `pc.oid`) + `
ORDER BY con.conname, cn.nspname, cc.relname`

// sqlPartitions is the immediate parent of every partition. The recursion to
// leaves and to the root is done in Go, over one pass of this result, because a
// recursive CTE would be a second shape on the allowlist for the same answer.
const sqlPartitions = `SELECT pn.nspname, p.relname, cn.nspname, c.relname, c.relkind::text
FROM pg_catalog.pg_inherits h
JOIN pg_catalog.pg_class p ON p.oid = h.inhparent
JOIN pg_catalog.pg_namespace pn ON pn.oid = p.relnamespace
JOIN pg_catalog.pg_class c ON c.oid = h.inhrelid
JOIN pg_catalog.pg_namespace cn ON cn.oid = c.relnamespace
WHERE c.relispartition
  AND left(pn.nspname, 3) <> 'pg_' AND pn.nspname <> 'information_schema'
ORDER BY pn.nspname, p.relname, cn.nspname, c.relname`

// sqlNotRecreated counts what v1 leaves behind (ARCHITECTURE.md §11.1). The
// kinds are exactly the ones pipeline.Object names. It is one statement rather
// than thirteen because it is one answer: a list the plan prints.
var sqlNotRecreated = `SELECT kind, name FROM (
  SELECT CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'matview' ELSE 'foreign_table' END AS kind,
         n.nspname || '.' || c.relname AS name
    FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE c.relkind IN ('v', 'm', 'f') AND ` + userSchemas + `
     AND ` + extensionFilter(`'pg_class'::regclass`, `c.oid`) + `
  UNION ALL
  SELECT CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END,
         n.nspname || '.' || p.proname
    FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
   WHERE ` + userSchemas + `
     AND ` + extensionFilter(`'pg_proc'::regclass`, `p.oid`) + `
  UNION ALL
  SELECT 'trigger', n.nspname || '.' || c.relname || '.' || tg.tgname
    FROM pg_catalog.pg_trigger tg
    JOIN pg_catalog.pg_class c ON c.oid = tg.tgrelid
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE NOT tg.tgisinternal AND ` + userSchemas + `
  UNION ALL
  SELECT 'policy', n.nspname || '.' || c.relname || '.' || po.polname
    FROM pg_catalog.pg_policy po
    JOIN pg_catalog.pg_class c ON c.oid = po.polrelid
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE ` + userSchemas + `
  UNION ALL
  SELECT 'rule', n.nspname || '.' || c.relname || '.' || r.rulename
    FROM pg_catalog.pg_rewrite r
    JOIN pg_catalog.pg_class c ON c.oid = r.ev_class
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE r.rulename <> '_RETURN' AND ` + userSchemas + `
  UNION ALL
  SELECT 'comment', n.nspname || '.' || c.relname
    FROM pg_catalog.pg_description de
    JOIN pg_catalog.pg_class c ON c.oid = de.objoid
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE de.classoid = 'pg_class'::regclass AND ` + userSchemas + `
  UNION ALL
  SELECT 'privilege', n.nspname || '.' || c.relname
    FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
   WHERE c.relacl IS NOT NULL AND ` + userSchemas + `
  UNION ALL
  SELECT 'publication', pu.pubname FROM pg_catalog.pg_publication pu
  UNION ALL
  SELECT 'operator', n.nspname || '.' || o.oprname
    FROM pg_catalog.pg_operator o JOIN pg_catalog.pg_namespace n ON n.oid = o.oprnamespace
   WHERE ` + userSchemas + `
     AND ` + extensionFilter(`'pg_operator'::regclass`, `o.oid`) + `
  UNION ALL
  SELECT 'collation', n.nspname || '.' || co.collname
    FROM pg_catalog.pg_collation co JOIN pg_catalog.pg_namespace n ON n.oid = co.collnamespace
   WHERE ` + userSchemas + `
) o
ORDER BY kind, name`

// sampleShape is the one statement here whose text is built rather than fixed.
// The fraction is written as an integer numerator over an integer denominator
// because the allowlist's grammar has {int} and no float placeholder
// (internal/pg/tracer.go); the seed is the constant in sample.go.
//
// The trailing LIMIT is a stop, not the sampling method: TABLESAMPLE has already
// chosen the pages, and §4's prohibition is on LIMIT choosing the rows. It is
// what bounds the server and the wire when the fraction is 100 — the fraction a
// relation nothing has analysed gets, where reltuples is -1 and relpages is 0.
// Without it pgx drains the whole result set on Close, so a multi-hundred-GB
// unanalysed table would be read in full while the run holds the source snapshot
// (THREAT_MODEL.md T7). Measured on postgres:16 over a 500,000-row table of
// (int, md5 text), EXPLAIN (ANALYZE, BUFFERS): 5 buffers and 600 rows with the
// LIMIT, 4,224 buffers and 500,000 rows without.
const sampleShape = `SELECT {idents} FROM {ident} TABLESAMPLE SYSTEM ({int}::float8 / {int}) REPEATABLE ({int}) LIMIT {int}`

// The sample savepoint. Introspect reads the whole catalog in one transaction,
// so a statement the server refuses aborts it and every statement after it
// fails with 25P02: without a savepoint to roll back to, tolerating one
// refusal would carry the run on into an aborted transaction and it would die
// naming an innocent table (verified on postgres:18). sqlTables' privilege
// column means the rollback is a backstop rather than the mechanism — a
// relation the role cannot read is not sampled in the first place — so this
// fires only when the privilege changed after the catalog was read.
//
// Neither statement reads or writes a row, and neither carries a placeholder,
// so the shapes are the text itself (THREAT_MODEL.md T9). There is no matching
// RELEASE: the savepoint costs nothing on a read-only transaction that is about
// to end, and the allowlist is shorter by one shape.
const (
	sqlSampleSavepoint = `SAVEPOINT lazyslice_sample`
	sqlSampleRollback  = `ROLLBACK TO SAVEPOINT lazyslice_sample`
)

// Statement is one statement this package sends to the source, with the name
// the allowlist and the trace record it under.
//
// It is not internal/pg's Shape: the import graph in ARCHITECTURE.md §2 has the
// stage packages importing pipeline and nothing else of the tree, so the wiring
// that owns both — internal/core — converts these into pg.Shape and registers
// them before Introspect is called.
type Statement struct{ Name, SQL string }

// Shapes is every statement shape internal/introspect sends to the source. A
// caller registers them on the source allowlist before calling Introspect; a
// statement whose shape is not registered never reaches the server
// (THREAT_MODEL.md T9).
func Shapes() []Statement {
	return []Statement{
		{Name: "introspect.server_version", SQL: sqlServerVersion},
		{Name: "introspect.schemas", SQL: sqlSchemas},
		{Name: "introspect.extensions", SQL: sqlExtensions},
		{Name: "introspect.enums", SQL: sqlEnums},
		{Name: "introspect.domains", SQL: sqlDomains},
		{Name: "introspect.composites", SQL: sqlComposites},
		{Name: "introspect.tables", SQL: sqlTables},
		{Name: "introspect.columns", SQL: sqlColumns},
		{Name: "introspect.column_checks", SQL: sqlColumnChecks},
		{Name: "introspect.indexes", SQL: sqlIndexes},
		{Name: "introspect.constraints", SQL: sqlConstraints},
		{Name: "introspect.primary_keys", SQL: sqlPrimaryKeys},
		{Name: "introspect.sequences", SQL: sqlSequences},
		{Name: "introspect.foreign_keys", SQL: sqlForeignKeys},
		{Name: "introspect.partitions", SQL: sqlPartitions},
		{Name: "introspect.not_recreated", SQL: sqlNotRecreated},
		{Name: "introspect.sample_savepoint", SQL: sqlSampleSavepoint},
		{Name: "introspect.sample_rollback", SQL: sqlSampleRollback},
		{Name: "introspect.sample", SQL: sampleShape},
	}
}
