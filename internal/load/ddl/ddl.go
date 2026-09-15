// SPDX-License-Identifier: Apache-2.0

// Package ddl generates the CREATE statements for the object classes v1
// recreates (ARCHITECTURE.md section 11.1). It is a deliberately narrow
// reimplementation of pg_dump --schema-only, and section 11.1 is its whole
// specification.
//
// v1 recreates, in this order: schemas, extensions, enum/domain/composite types,
// unowned sequences, tables with their columns and non-foreign constraints and,
// after the data, indexes, foreign keys, setval and ANALYZE. A partitioned
// source table becomes one plain table, which removes the masked-partition-key
// trap as a side effect.
//
// v1 does not recreate views, materialised views, functions, procedures,
// triggers, policies, rules, comments, privileges, publications, foreign tables,
// operators, operator classes, collations or partitions. None of those is a
// refusal on its own; each is counted and printed under not_recreated.
//
// A recreated object that depends on one we do not recreate is a refusal at
// plan, exit 13, before the snapshot is used for keys and before anything in the
// target is dropped. There is no flag that drops the offending default silently,
// because the application's first INSERT is the point of the tool.
//
// Every definition this package emits that describes an expression is the
// catalog's own text — pg_get_expr, pg_get_constraintdef, pg_get_indexdef,
// rendered at introspect — so the target receives the source's expression
// verbatim and no printer of ours can paraphrase it. Only the keywords between
// them are written here.
package ddl

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// markerTable is the bookkeeping table internal/load writes in the target
// (ARCHITECTURE.md section 11.2). It is not an object class section 11.1
// recreates, and recreated below leaves it out on both sides of the fingerprint:
// introspect reads it back from a target we wrote as an ordinary user table, and
// a fingerprint that counted it could never equal the source's, so section
// 11.2's marker could never bind and the gate would refuse every non-empty
// target lazyslice itself wrote.
//
// The name is internal/pg's, imported rather than repeated, because the loader
// and the gate must agree with this package on exactly which table is
// bookkeeping. It is matched on the name alone and in any schema, which is how
// internal/load already skips it in the drop list.
const markerTable = pg.MarkerTable

// TableDrop is one DROP TABLE and the table it names.
//
// It is a structure rather than a string because ARCHITECTURE.md section 11.1
// requires each drop to be printed before it happens, and the loader cannot
// name a table it can only see as SQL text.
type TableDrop struct {
	Table ref.TableRef
	SQL   string
}

// DropTables returns one DROP TABLE per table section 11.1 recreates, in the
// reverse of the order PreData creates them, plus one for every table in extra
// that PreData does not create.
//
// CASCADE is deliberate: lazyslice owns the target schema (ADR-005), the tables
// reference each other, and a view somebody left behind is not a reason to stop
// half way through a drop list the operator has already been shown. IF EXISTS is
// deliberate too: the gate admits an empty target that never held these tables
// as readily as a marked one that holds all of them.
//
// extra is the target's own user tables, which this package cannot read:
// pipeline.Writer has no way to query (ARCHITECTURE.md section 2), so a table
// the target holds under a name the source does not use is dropped only if the
// caller names it. A table this
// function is not given is left alone rather than dropped silently.
func DropTables(schema *pipeline.Schema, extra []ref.TableRef) []TableDrop {
	if schema == nil {
		return nil
	}
	tables := recreated(schema)
	seen := make(map[ref.TableRef]bool, len(tables)+len(extra))
	out := make([]TableDrop, 0, len(tables)+len(extra))
	add := func(t ref.TableRef) {
		if seen[t] {
			return
		}
		seen[t] = true
		out = append(out, TableDrop{Table: t, SQL: "DROP TABLE IF EXISTS " + tableName(t) + " CASCADE"})
	}
	for i := len(tables) - 1; i >= 0; i-- {
		add(tables[i].Ref)
	}
	for _, t := range extra {
		add(t)
	}
	return out
}

// DropObjects returns the DROP statements for the sequences and types PreData
// creates, in the reverse of the order it creates them.
//
// They exist because CREATE TYPE has no IF NOT EXISTS and a sequence a previous
// run left behind is not owned by any table, so without this a second run
// against a marked target — the case ARCHITECTURE.md section 11.2 exists to
// allow — would fail at 42710 with the target's tables already dropped.
//
// There is no CASCADE here, unlike DropTables. A type or sequence something
// still depends on after every table this run knows about has been dropped is
// something the run has not been told about, and a loud 2BP01 naming it is
// better than dropping a stranger's column.
func DropObjects(schema *pipeline.Schema) []string {
	drops := ObjectDrops(schema)
	out := make([]string, 0, len(drops))
	for _, d := range drops {
		out = append(out, d.SQL)
	}
	return out
}

// ObjectDrop is one non-table object PreData creates, with the name to print
// before it is dropped. It is DropTables' TableDrop for the object classes that
// are not tables, and it exists for the same reason: ARCHITECTURE.md section
// 11.1 requires each drop to be printed before it happens, and a caller cannot
// name an object it can only see as SQL text.
type ObjectDrop struct {
	Name string
	SQL  string
}

// ObjectDrops returns the sequences and types PreData creates, in the reverse
// of the order it creates them, each with its name.
//
// load.DropLoaded reads this (the 2026-09-15 red team's A07). THREAT_MODEL.md
// T8 claimed that after a content-class verify failure the target ends "either
// empty or holding nothing this run wrote", and that was false for every
// non-table object the loader creates: the quarantine dropped ddl.DropTables'
// list and nothing else, so a domain whose CHECK carried an address stayed in
// the target after the exit-9 refusal that found it, and stayed again on every
// rerun.
func ObjectDrops(schema *pipeline.Schema) []ObjectDrop {
	if schema == nil {
		return nil
	}
	var out []ObjectDrop
	for _, s := range sequences(schema) {
		out = append(out, ObjectDrop{Name: s.Name, SQL: "DROP SEQUENCE IF EXISTS " + qualified(s.Name)})
	}
	types := typeOrder(schema)
	for i := len(types) - 1; i >= 0; i-- {
		t := types[i]
		if t.kind == typeDomain {
			out = append(out, ObjectDrop{Name: t.name, SQL: "DROP DOMAIN IF EXISTS " + qualified(t.name)})
			continue
		}
		out = append(out, ObjectDrop{Name: t.name, SQL: "DROP TYPE IF EXISTS " + qualified(t.name)})
	}
	return out
}

// PreData returns the statements that must run before any row is copied:
// schemas, extensions, types, sequences, tables and their non-foreign
// constraints — items 1 to 5 of ARCHITECTURE.md section 11.1, in that order.
//
// The plan is not read. Every table of the source is recreated, including one
// the plan dropped to SchemaOnly: section 11.1 recreates the schema and the plan
// decides only which rows travel. The parameter is section 2's signature and is
// kept so that a later object class which does depend on the plan has somewhere
// to read it.
func PreData(schema *pipeline.Schema, _ *pipeline.Plan) ([]string, error) {
	if schema == nil {
		return nil, errors.New("ddl: no schema")
	}
	tables := recreated(schema)
	exts := extensions(schema)
	types := typeOrder(schema)
	seqs := sequences(schema)

	var out []string
	// 1. Schemas, for every schema an object below is created in — not only
	// every schema a recreated table lives in. An extension, a sequence or a
	// type may name a schema that holds no table at all (CREATE SCHEMA
	// extensions; CREATE EXTENSION pg_trgm SCHEMA extensions is the common
	// convention), and CREATE EXTENSION ... SCHEMA into a schema that does not
	// exist is a 3F000 at the first pre-data statement — after the drop has
	// already emptied the target.
	for _, name := range schemaNames(tables, exts, types, seqs) {
		out = append(out, "CREATE SCHEMA IF NOT EXISTS "+quoteIdent(name))
	}
	// 2. Extensions, version unpinned.
	for _, x := range exts {
		out = append(out, "CREATE EXTENSION IF NOT EXISTS "+quoteIdent(x.Name)+" SCHEMA "+quoteIdent(x.Schema))
	}
	// 3. Enum types, domains and composite types, in dependency order.
	for _, t := range types {
		out = append(out, t.def)
	}
	// 4. Sequences that are not owned by an identity column, with their
	// parameters. An identity column's sequence is created by the column.
	for _, s := range seqs {
		out = append(out, createSequence(s))
	}
	// 5. Tables.
	for _, t := range tables {
		out = append(out, createTable(t))
	}
	// A sequence that a column owns is dropped with the table, which is what
	// makes the drop path above enough on the second run. The ALTER is here,
	// after the tables, because the table it names has to exist.
	for _, t := range tables {
		for _, s := range t.Sequences {
			col := sequenceColumn(t, s)
			if col == "" {
				continue
			}
			if identityColumn(t, col) {
				continue
			}
			out = append(out, "ALTER SEQUENCE "+qualified(s.Name)+" OWNED BY "+
				tableName(t.Ref)+"."+quoteIdent(col))
		}
	}
	return out, nil
}

// PostData returns the statements that run after every row is copied: indexes,
// foreign keys NOT VALID then VALIDATE CONSTRAINT, setval and ANALYZE — item 6
// of ARCHITECTURE.md section 11.1, in that order.
//
// Foreign keys come after the data, so no edge is deferred and the load order
// inside a strongly connected component does not matter (ADR-005). They are
// added NOT VALID and validated in a second statement because that is two
// failures the operator can tell apart: a constraint the target will not accept
// at all, and a constraint the slice does not satisfy.
func PostData(schema *pipeline.Schema, plan *pipeline.Plan) ([]string, error) {
	steps, err := PostDataSteps(schema, plan)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.SQL)
	}
	return out, nil
}

// StepKind is what a post-data statement does. The loader reads it to tell a
// foreign key the target will not accept from one the slice does not satisfy:
// ADR-005's exit table gives foreign-key verification its own code (8) and puts
// everything else in the load under 7, and a statement list of bare strings
// cannot be told apart afterwards without parsing SQL back.
type StepKind int

// The post-data statement kinds, in the order PostDataSteps emits them.
const (
	StepIndex StepKind = iota
	StepForeignKey
	StepValidate
	StepSetval
	StepAnalyze
)

// Step is one post-data statement and what it acts on. Object is an index name,
// a constraint name or a sequence name, and is empty for ANALYZE.
type Step struct {
	Kind   StepKind
	Table  ref.TableRef
	Object string
	SQL    string
}

// PostDataSteps is PostData with each statement's kind and object attached.
func PostDataSteps(schema *pipeline.Schema, _ *pipeline.Plan) ([]Step, error) {
	if schema == nil {
		return nil, errors.New("ddl: no schema")
	}
	tables := recreated(schema)

	var out []Step
	for _, t := range tables {
		for _, idx := range indexStatements(t) {
			out = append(out, Step{Kind: StepIndex, Table: t.Ref, Object: idx.name, SQL: idx.sql})
		}
	}
	fks := foreignKeys(schema, tables)
	for _, fk := range fks {
		out = append(out, Step{Kind: StepForeignKey, Table: fk.table, Object: fk.name, SQL: fk.add})
	}
	for _, fk := range fks {
		if fk.validate != "" {
			out = append(out, Step{Kind: StepValidate, Table: fk.table, Object: fk.name, SQL: fk.validate})
		}
	}
	for _, t := range tables {
		for _, s := range Setvals(t) {
			out = append(out, Step{Kind: StepSetval, Table: t.Ref, Object: s.Sequence, SQL: s.SQL})
		}
	}
	for _, t := range tables {
		out = append(out, Step{Kind: StepAnalyze, Table: t.Ref, SQL: "ANALYZE " + tableName(t.Ref)})
	}
	return out, nil
}

// Setval is one sequence reset and the sequence it names, so that the loader can
// record it in pipeline.LoadResult.Sequences without parsing SQL back.
type Setval struct {
	Sequence string
	SQL      string
}

// Setvals returns the sequence resets for one table.
//
// The strict-NULL form is the whole point (THREAT_MODEL.md T8,
// research/HARD_PROBLEMS.md section 4.1): setval(seq, coalesce(max(id), 1),
// max(id) IS NOT NULL) leaves an empty table's sequence unadvanced and about to
// hand out 1, where a literal 0 is out of range for a sequence whose MINVALUE is
// 1 and where setval(seq, max(id)) would fail on a table the slice left empty.
//
// A sequence no column of the table refers to is skipped: there is no column to
// take a maximum of, and advancing it to a guess would be worse than leaving it
// where the source's parameters put it.
func Setvals(t pipeline.Table) []Setval {
	var out []Setval
	for _, s := range t.Sequences {
		col := sequenceColumn(t, s)
		if col == "" {
			continue
		}
		c := quoteIdent(col)
		// setval's first argument is a regclass, and text is cast to regclass
		// by the same rules that parse an identifier: an unquoted name is
		// folded to lower case. The literal therefore carries the *quoted*
		// name — setval('"public"."LegacyCustomer_CustomerID_seq"', ...) — or a
		// sequence whose name needs quoting resolves to a relation that does
		// not exist, and the whole run dies at 42P01 after every table has been
		// copied, with the sequences unreset. That is the T8 outcome the
		// strict-NULL form below exists to prevent.
		//
		// **An identity column's sequence is named by the target and not by the
		// source.** PreData creates such a sequence through the column's
		// `GENERATED ... AS IDENTITY` (sequences() skips it deliberately), and
		// the server names it `<table>_<column>_seq` — which is not the source's
		// name whenever the source's table has been renamed since, because
		// Postgres does not rename the sequence with it. Metabase's is the live
		// case: `group_table_access_policy` became `sandboxes` and its identity
		// sequence is still `group_table_access_policy_id_seq`, so this
		// statement named a relation the target has never had and the run died
		// at 42P01 with every table already copied
		// (testdata/regressions/006-identity-sequence-renamed-table.sql).
		//
		// So for an identity column the name is asked of the target, through
		// pg_get_serial_sequence, which answers with whatever the target's own
		// identity column created. A non-identity sequence keeps the source's
		// name, because that is the name PreData's CREATE SEQUENCE gave it.
		seq := quoteLiteral(qualified(s.Name))
		if identityColumn(t, col) {
			seq = "coalesce(pg_catalog.pg_get_serial_sequence(" + quoteLiteral(tableName(t.Ref)) + ", " +
				quoteLiteral(col) + "), " + seq + ")"
		}
		out = append(out, Setval{
			Sequence: s.Name,
			SQL: "SELECT pg_catalog.setval(" + seq + ", coalesce(max(" + c +
				"), 1), max(" + c + ") IS NOT NULL) FROM " + tableName(t.Ref),
		})
	}
	return out
}

// Fingerprint is sha256 over the recreated object classes only, as the DDL text
// PreData and PostData emit, in that order (ARCHITECTURE.md section 11.1).
//
// It is the same hash whether it is computed on the source or on a target
// lazyslice wrote, which is what the marker's binding needs (section 11.2);
// views and functions do not enter it, because no statement above creates one,
// and neither does the marker table itself, which lazyslice creates in the
// target and the source does not have (see recreated).
//
// It is the only definition of section 11.2's schema fingerprint (ADR-009).
// internal/introspect used to fill Schema.Fingerprint by hashing the catalog
// fields this text is rendered from; that hash counted the marker table and
// every non-virtual foreign key rather than only the edges foreignKeys
// recreates, so it never round-tripped, and ADR-009 deleted it. Schema.
// Fingerprint is now filled by the caller from this function, through
// load.SchemaFingerprint.
//
// Both ends of section 11.2's binding go through internal/load, so they cannot
// be wired apart: load.Load computes the marker's value itself, and
// load.GateFingerprint is the gate's CatalogFingerprinter. That is what stops a
// marker being written with one definition and recomputed with another, which
// binds nothing and leaves exit 4 on a target lazyslice itself wrote.
//
// The encoding is length-prefixed for the reason section 5 gives: without it two
// different statement lists whose concatenations coincide would hash alike.
func Fingerprint(schema *pipeline.Schema) (string, error) {
	pre, err := PreData(schema, nil)
	if err != nil {
		return "", err
	}
	post, err := PostData(schema, nil)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var n [4]byte
	for _, s := range append(pre, post...) {
		//nolint:gosec // G115: a statement built from catalog identifiers is
		// never four gigabytes long, and len is never negative.
		binary.BigEndian.PutUint32(n[:], uint32(len(s)))
		_, _ = h.Write(n[:])
		_, _ = h.Write([]byte(s))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// recreated is the tables section 11.1 recreates: every user table that is not a
// partition and is not the marker table, in (schema, name) order. A partitioned
// source table is one of them and becomes one plain table; its leaves are not
// recreated at all.
//
// The marker is left out because this list is what PreData, PostData and
// Fingerprint are computed over, and section 11.2's binding requires the
// fingerprint of a target lazyslice wrote to equal the fingerprint of its
// source. lazyslice creates lazyslice_meta in the target itself, so a target
// re-introspected after a load carries one table the source does not; counting
// it would make every marker unbound. A source table of that name is therefore
// not recreated either — refusing such a source outright belongs to the gate and
// is recorded as owed in internal/load/CLAUDE.md.
func recreated(s *pipeline.Schema) []pipeline.Table {
	out := make([]pipeline.Table, 0, len(s.Tables))
	for _, t := range s.Tables {
		if t.Parent != nil {
			continue
		}
		if t.Ref.Name == markerTable {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ref.Schema != out[j].Ref.Schema {
			return out[i].Ref.Schema < out[j].Ref.Schema
		}
		return out[i].Ref.Name < out[j].Ref.Name
	})
	return out
}

// schemaNames is every schema PreData creates something in: a recreated table's
// own schema, the schema an extension is installed into, and the schema half of
// each sequence and named type. A schema that holds none of them is not created,
// and one that holds only an extension is.
func schemaNames(
	tables []pipeline.Table,
	exts []pipeline.Extension,
	types []namedType,
	seqs []pipeline.SequenceDef,
) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, t := range tables {
		add(t.Ref.Schema)
	}
	for _, x := range exts {
		add(x.Schema)
	}
	for _, t := range types {
		add(schemaOf(t.name))
	}
	for _, s := range seqs {
		add(schemaOf(s.Name))
	}
	sort.Strings(out)
	return out
}

// schemaOf is the schema half of a name the catalog wrote as "schema.name", or
// "" for a bare name. The split is qualified's, so the two agree about where the
// schema ends.
func schemaOf(name string) string {
	if i := strings.Index(name, "."); i >= 0 {
		return name[:i]
	}
	return ""
}

func extensions(s *pipeline.Schema) []pipeline.Extension {
	out := append([]pipeline.Extension(nil), s.Extensions...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// sequences is every sequence a recreated table needs that is not an identity
// column's, in name order.
//
// A sequence reaches Table.Sequences either owned (pg_depend deptype 'a' or 'i')
// or merely referenced by a column default that calls nextval on it
// (internal/introspect/sql.go). Both are created here, because a default calling
// a sequence that does not exist is a table the application cannot insert into,
// which is the point of the tool.
func sequences(s *pipeline.Schema) []pipeline.SequenceDef {
	seen := map[string]bool{}
	var out []pipeline.SequenceDef
	for _, t := range recreated(s) {
		for _, q := range t.Sequences {
			if seen[q.Name] {
				continue
			}
			if q.Column != "" && identityColumn(t, q.Column) {
				continue
			}
			seen[q.Name] = true
			out = append(out, q)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// sequenceColumn is the column a sequence belongs to.
//
// SequenceDef.Column is filled only when the catalog records ownership.
// pagila 3.1.0 declares no ownership at all — every one of its sequences is
// reached through a column default calling nextval — so a setval that read
// SequenceDef.Column alone would reset nothing in the project's own friendly
// fixture, which is exactly the failure THREAT_MODEL.md T8 names. The default
// text is the catalog's own (nextval('public.actor_actor_id_seq'::regclass)), so
// the sequence's name appears in it as a literal — qualified or bare, because
// pg_get_expr writes nextval('customer_customer_id_seq'::regclass) for a
// sequence in the search path and nextval('other.s'::regclass) for one outside
// it, and pagila's are all in public. Both spellings are looked for; the search
// is over the table's own columns and its own sequences, so a bare name cannot
// match another table's.
func sequenceColumn(t pipeline.Table, s pipeline.SequenceDef) string {
	if s.Column != "" {
		return s.Column
	}
	needles := []string{quoteLiteral(s.Name)}
	if i := strings.Index(s.Name, "."); i >= 0 {
		needles = append(needles, quoteLiteral(s.Name[i+1:]))
	}
	for _, c := range t.Columns {
		if c.Default == "" {
			continue
		}
		for _, needle := range needles {
			if strings.Contains(c.Default, needle) {
				return c.Name
			}
		}
	}
	return ""
}

func identityColumn(t pipeline.Table, name string) bool {
	for _, c := range t.Columns {
		if c.Name == name {
			return c.Identity != ""
		}
	}
	return false
}

func createSequence(s pipeline.SequenceDef) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE ")
	b.WriteString(qualified(s.Name))
	b.WriteString(" INCREMENT BY ")
	b.WriteString(strconv.FormatInt(s.Increment, 10))
	b.WriteString(" MINVALUE ")
	b.WriteString(strconv.FormatInt(s.Min, 10))
	b.WriteString(" MAXVALUE ")
	b.WriteString(strconv.FormatInt(s.Max, 10))
	b.WriteString(" START WITH ")
	b.WriteString(strconv.FormatInt(s.Start, 10))
	b.WriteString(" CACHE ")
	b.WriteString(strconv.FormatInt(s.Cache, 10))
	if s.Cycle {
		b.WriteString(" CYCLE")
	} else {
		b.WriteString(" NO CYCLE")
	}
	return b.String()
}

// createTable is item 5: every column with its type, collation, nullability,
// default, generation expression and identity, then the table-level constraints
// except foreign keys.
//
// A partitioned source table becomes one plain table holding the root's columns
// and constraints (section 11.1), so PartitionKey is not written and the leaves
// are not created.
func createTable(t pipeline.Table) string {
	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(tableName(t.Ref))
	b.WriteString(" (\n")
	first := true
	for _, c := range t.Columns {
		if !first {
			b.WriteString(",\n")
		}
		first = false
		b.WriteString("  ")
		b.WriteString(columnDef(c))
	}
	for _, con := range t.Constraints {
		if con.Kind == 'f' {
			// Foreign keys are item 6, after the data.
			continue
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		b.WriteString("  CONSTRAINT ")
		b.WriteString(quoteIdent(con.Name))
		b.WriteString(" ")
		b.WriteString(con.Def)
	}
	b.WriteString("\n)")
	return b.String()
}

func columnDef(c pipeline.Column) string {
	var b strings.Builder
	b.WriteString(quoteIdent(c.Name))
	b.WriteString(" ")
	b.WriteString(c.TypeName)
	if c.Collation != "" {
		b.WriteString(" COLLATE ")
		b.WriteString(quoteIdent(c.Collation))
	}
	switch {
	case c.Generated != "":
		b.WriteString(" GENERATED ALWAYS AS (")
		b.WriteString(c.Generated)
		b.WriteString(") STORED")
	case c.Identity != "":
		if c.Identity == "a" {
			b.WriteString(" GENERATED ALWAYS AS IDENTITY")
		} else {
			b.WriteString(" GENERATED BY DEFAULT AS IDENTITY")
		}
		if q := c.IdentitySeq; q != nil {
			b.WriteString(" (INCREMENT BY " + strconv.FormatInt(q.Increment, 10) +
				" MINVALUE " + strconv.FormatInt(q.Min, 10) +
				" MAXVALUE " + strconv.FormatInt(q.Max, 10) +
				" START WITH " + strconv.FormatInt(q.Start, 10) +
				" CACHE " + strconv.FormatInt(q.Cache, 10))
			if q.Cycle {
				b.WriteString(" CYCLE)")
			} else {
				b.WriteString(" NO CYCLE)")
			}
		}
	case c.Default != "":
		b.WriteString(" DEFAULT ")
		b.WriteString(c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

// indexStatements is every index of one table that a constraint did not already
// create.
//
// pg_get_indexdef prints CREATE UNIQUE INDEX for the index behind a primary key
// or a unique constraint, and createTable has already created that constraint,
// so replaying it would be a 42P07 on the constraint's own index name. The
// filter is by name, which is the name both the constraint and its index carry.
func indexStatements(t pipeline.Table) []namedStatement {
	byConstraint := map[string]bool{}
	for _, con := range t.Constraints {
		switch con.Kind {
		case 'p', 'u', 'x':
			byConstraint[con.Name] = true
		}
	}
	idxs := append([]pipeline.Index(nil), t.Indexes...)
	sort.Slice(idxs, func(i, j int) bool { return idxs[i].Name < idxs[j].Name })
	out := make([]namedStatement, 0, len(idxs))
	for _, idx := range idxs {
		if byConstraint[idx.Name] {
			continue
		}
		def := idx.Def
		if t.Partitioned {
			def = plainIndexDef(def)
		}
		out = append(out, namedStatement{name: idx.Name, sql: def})
	}
	return out
}

type namedStatement struct{ name, sql string }

// plainIndexDef is an index definition as section 11.1 recreates it on a plain
// table: pg_get_indexdef's ON ONLY, which it prints for an index on a
// partitioned table's own relation, becomes ON.
//
// It is the only rewrite of its kind in the tree (ADR-009 deleted the second
// copy, in internal/introspect/fingerprint.go, along with the catalog hash it
// served). It exists because section 11.1 recreates a partitioned source table
// as one plain table, which cannot be indexed ON ONLY. On postgres:16,
// pg_get_indexdef prints CREATE UNIQUE INDEX ev_pkey ON ONLY public.ev ... for
// an index on a partitioned table's own relation and prints the same index on a
// plain table without ONLY; ON ONLY is accepted at creation but is not
// round-tripped, so the raw text of a source's index could never equal the text
// of the target lazyslice wrote from it. Since Fingerprint is over this DDL,
// that is not only a failed CREATE INDEX but a marker that never binds: any
// source holding a partitioned table with an index or a primary key would be
// refused with exit 4 on its second run against a target lazyslice itself wrote
// (section 11.2) — testdata/nasty.sql's public.events has
// PRIMARY KEY (event_id, occurred_at), so the project's own fixture hits it.
// TestAPartitionedTableBecomesOnePlainTable covers the behaviour.
//
// The scan skips quoted identifiers, so an index actually named `x ON ONLY y` is
// not rewritten inside its own name.
func plainIndexDef(def string) string {
	const onOnly = " ON ONLY "
	quoted := false
	for i := 0; i+len(onOnly) <= len(def); i++ {
		if def[i] == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if def[i:i+len(onOnly)] == onOnly {
			return def[:i] + " ON " + def[i+len(onOnly):]
		}
	}
	return def
}

type fkStatements struct {
	table         ref.TableRef
	name          string
	add, validate string
}

// foreignKeys is item 6's foreign keys: every edge whose two ends are both
// tables this package recreates.
//
// An edge onto or from a leaf partition is skipped, because the leaf is not
// recreated and the root carries the same edge; a virtual edge is skipped
// because it is an inference and not a constraint (ARCHITECTURE.md section 2);
// an edge introspect marked NotRecreatable is skipped because the planner has
// already refused the run for it with exit 13, and reaching here at all would
// mean that refusal did not run.
//
// The definition is the child's own pg_get_constraintdef, so ON DELETE, ON
// UPDATE, MATCH and the deferrability come across unaltered. An edge the source
// itself has not validated stays NOT VALID and is not validated here: it holds
// in the target exactly as much as it holds in the source.
func foreignKeys(s *pipeline.Schema, tables []pipeline.Table) []fkStatements {
	recreatedRef := make(map[ref.TableRef]bool, len(tables))
	defs := map[ref.TableRef]map[string]string{}
	for _, t := range tables {
		recreatedRef[t.Ref] = true
		m := map[string]string{}
		for _, con := range t.Constraints {
			if con.Kind == 'f' {
				m[con.Name] = con.Def
			}
		}
		defs[t.Ref] = m
	}

	fks := append([]pipeline.ForeignKey(nil), s.FKs...)
	sort.Slice(fks, func(i, j int) bool { return fks[i].Name < fks[j].Name })

	out := make([]fkStatements, 0, len(fks))
	for _, fk := range fks {
		if fk.Virtual || fk.NotRecreatable {
			continue
		}
		if !recreatedRef[fk.Child] || !recreatedRef[fk.Parent] {
			continue
		}
		def, ok := defs[fk.Child][fk.Name]
		if !ok {
			continue
		}
		st := fkStatements{
			table: fk.Child,
			name:  fk.Name,
			add:   "ALTER TABLE " + tableName(fk.Child) + " ADD CONSTRAINT " + quoteIdent(fk.Name) + " " + def,
		}
		if fk.Validated {
			st.add += " NOT VALID"
			st.validate = "ALTER TABLE " + tableName(fk.Child) + " VALIDATE CONSTRAINT " + quoteIdent(fk.Name)
		}
		out = append(out, st)
	}
	return out
}

const (
	typeEnum = iota
	typeDomain
	typeComposite
)

type namedType struct {
	kind int
	name string
	def  string
}

// typeOrder is item 3: the enums, domains and composite types of the source, in
// dependency order.
//
// Enums first, because an enum's definition is its labels and refers to nothing.
// Domains and composites are then ordered by whether one's definition names
// another: a domain can be over a composite and a composite can have a field of
// a domain, so the two kinds are sorted together rather than one after the
// other. The ordering is by name reference over the catalog's own CREATE text,
// which is not a parse; a definition that names a type it does not depend on
// only makes the order more conservative than it has to be, and PostgreSQL types
// cannot be mutually recursive, so this terminates.
func typeOrder(s *pipeline.Schema) []namedType {
	names := make([]string, 0, len(s.Enums))
	for name := range s.Enums {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]namedType, 0, len(names)+len(s.Domains)+len(s.Composites))
	for _, name := range names {
		labels := make([]string, 0, len(s.Enums[name]))
		for _, l := range s.Enums[name] {
			labels = append(labels, quoteLiteral(l))
		}
		out = append(out, namedType{
			kind: typeEnum,
			name: name,
			def:  "CREATE TYPE " + qualified(name) + " AS ENUM (" + strings.Join(labels, ", ") + ")",
		})
	}

	rest := make([]namedType, 0, len(s.Domains)+len(s.Composites))
	for _, d := range s.Domains {
		rest = append(rest, namedType{kind: typeDomain, name: d.Name, def: d.Def})
	}
	for _, d := range s.Composites {
		rest = append(rest, namedType{kind: typeComposite, name: d.Name, def: d.Def})
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].name < rest[j].name })

	// A stable insertion sort over "names the other": a type whose definition
	// mentions another's name is emitted after it.
	placed := make([]namedType, 0, len(rest))
	pending := rest
	for len(pending) > 0 {
		progressed := false
		next := pending[:0:0]
		for _, t := range pending {
			blocked := false
			for _, other := range pending {
				if other.name == t.name {
					continue
				}
				if mentionsType(t.def, other.name) {
					blocked = true
					break
				}
			}
			if blocked {
				next = append(next, t)
				continue
			}
			placed = append(placed, t)
			progressed = true
		}
		if !progressed {
			// Not reachable through the catalog — PostgreSQL types cannot be
			// mutually recursive — but a textual test is not a dependency
			// graph, so the remainder is emitted in name order rather than
			// looping.
			placed = append(placed, next...)
			break
		}
		pending = next
	}
	return append(out, placed...)
}

// mentionsType reports whether a CREATE statement names a type, qualified or
// bare, outside a quoted identifier or a string literal.
func mentionsType(def, name string) bool {
	bare := name
	if i := strings.Index(name, "."); i >= 0 {
		bare = name[i+1:]
	}
	for _, id := range identifiers(def) {
		if id == name || id == bare {
			return true
		}
	}
	return false
}

// TableName is the qualified, quoted name of a table, spelled the way every
// statement this package builds spells it.
//
// It is exported for internal/load's lock-and-recheck (ARCHITECTURE.md §11.2,
// T-0130): the LOCK TABLE that precedes a DROP, and the recheck between them,
// have to name the same relation the DROP names, and two spellings of a
// quoted identifier are two things to keep in step.
func TableName(t ref.TableRef) string { return tableName(t) }

func tableName(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// qualified quotes a name the catalog wrote as "schema.name".
//
// The split is at the first dot, which is where introspect's own concatenation
// put it (nspname || '.' || relname). A schema whose name contains a dot would
// be split in the wrong place; it is not worth a second catalog column to a
// package that would then have to be given one for every such name, and the
// failure is a loud 3F000 rather than a wrong object.
func qualified(name string) string {
	if i := strings.Index(name, "."); i >= 0 {
		return quoteIdent(name[:i]) + "." + quoteIdent(name[i+1:])
	}
	return quoteIdent(name)
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
