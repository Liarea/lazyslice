// SPDX-License-Identifier: Apache-2.0

// Package introspect reads the source catalog into a pipeline.Schema.
//
// Definition text comes from the catalog's own deparser (pg_get_expr,
// pg_get_constraintdef, pg_get_indexdef), never from a printer of ours, so the
// target receives the source's expression verbatim and a schema we cannot
// recreate is a refusal at plan rather than a subtly different table.
//
// Sampling happens here too: up to 200 rows per table by TABLESAMPLE SYSTEM
// with a fixed seed. LIMIT is never the sampling method, because it would take
// the front of the table; a LIMIT after TABLESAMPLE is a stop on what the
// server reads, and the statement carries one (sample.go). TABLESAMPLE is not
// accepted on a partitioned table, so a partitioned root is sampled through its
// largest leaf and Table.SampledFrom names it. A table the source role cannot
// SELECT has no samples and does not end the run: ARCHITECTURE.md §3.6 answers
// an unreadable table at plan.
//
// Table.Samples holds production values and is never serialised.
package introspect

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

type introspector struct{}

// New returns the catalog introspector.
func New() pipeline.Introspector { return introspector{} }

var _ pipeline.Introspector = introspector{}

// catalog is one run of Introspect. It holds the tables by reference while the
// per-object statements are read into them, because every statement after the
// first names its table by (schema, name) and nothing here iterates a Go map
// for output: Schema.Tables is built at the end, in the order the catalog
// returned, which is (schema, name).
type catalog struct {
	schema *pipeline.Schema
	order  []ref.TableRef
	tables map[ref.TableRef]*pipeline.Table
	// partitionOf is the immediate parent of every partition and children is
	// its inverse; linkPartitions turns the pair into Table.Partitions and
	// Table.Parent, which the sampler then reads.
	partitionOf map[ref.TableRef]ref.TableRef
	children    map[ref.TableRef][]ref.TableRef
	// pages is pg_class.relpages per table. It is the sampler's input and no
	// field of pipeline.Table holds it: §2 carries reltuples as ApproxRows and
	// nothing else of the relation's size.
	pages map[ref.TableRef]int64
	// unreadable holds the tables has_table_privilege says the source role
	// cannot SELECT. It is the sampler's input and no field of pipeline.Table
	// holds it either: §3.6 answers an unreadable table at plan, from
	// RolePrivileges, and this package only needs to know which relations not to
	// send a sample to. It is keyed the other way round from "readable" so that
	// the zero value attempts the sample rather than skipping it.
	unreadable map[ref.TableRef]bool
}

// Introspect reads the whole catalog through one Reader, which is one
// transaction on one snapshot: every statement below sees the same database
// (ARCHITECTURE.md §2).
func (introspector) Introspect(ctx context.Context, r pipeline.Reader) (*pipeline.Schema, error) {
	c := &catalog{
		schema:      &pipeline.Schema{Enums: map[string][]string{}},
		tables:      map[ref.TableRef]*pipeline.Table{},
		partitionOf: map[ref.TableRef]ref.TableRef{},
		children:    map[ref.TableRef][]ref.TableRef{},
		pages:       map[ref.TableRef]int64{},
		unreadable:  map[ref.TableRef]bool{},
	}

	steps := []struct {
		what string
		read func(context.Context, pipeline.Reader) error
	}{
		{"the server version", c.readVersion},
		{"the schema list", c.readSchemas},
		{"the extension list", c.readExtensions},
		{"the enum types", c.readEnums},
		{"the domains", c.readDomains},
		{"the composite types", c.readComposites},
		{"the table list", c.readTables},
		{"the columns", c.readColumns},
		{"the column checks", c.readColumnChecks},
		{"the indexes", c.readIndexes},
		{"the constraints", c.readConstraints},
		{"the primary keys", c.readPrimaryKeys},
		{"the sequences", c.readSequences},
		{"the foreign keys", c.readForeignKeys},
		{"the partitions", c.readPartitions},
		{"the objects v1 does not recreate", c.readNotRecreated},
	}
	for _, s := range steps {
		if err := s.read(ctx, r); err != nil {
			return nil, fmt.Errorf("introspect: reading %s: %w", s.what, err)
		}
	}

	// The sampler reads Table.Partitions, so the inheritance tree is resolved
	// before it runs and after every statement that describes a table. The
	// edges are moved off the leaves at the same point, because that too needs
	// Table.Parent.
	c.linkPartitions()
	c.repointPartitionForeignKeys()
	if err := c.readSamples(ctx, r); err != nil {
		return nil, fmt.Errorf("introspect: reading the samples: %w", err)
	}
	c.markIndexedForeignKeys()
	c.collect()
	c.schema.Fingerprint = schemaFingerprint(c.schema)
	return c.schema, nil
}

// each runs one statement and calls fn per row. Rows.Err is checked on every
// path, because a source that refuses a statement returns its refusal there
// (internal/pg/source.go) and a caller that only ranged would read a refusal as
// an empty catalog.
func each(ctx context.Context, r pipeline.Reader, sql string, fn func(pipeline.Rows) error) error {
	rows, err := r.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// exec runs one statement that returns no rows. Reader has only Query (§2), and
// in pgx.QueryExecModeExec a server error surfaces on Rows.Err rather than on
// Query, so both have to be consulted or a failed SAVEPOINT would look like
// success (internal/pg/source.go).
func exec(ctx context.Context, r pipeline.Reader, sql string) error {
	rows, err := r.Query(ctx, sql)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	rows.Close()
	return rows.Err()
}

func (c *catalog) readVersion(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlServerVersion, func(rows pipeline.Rows) error {
		return rows.Scan(&c.schema.ServerVersion)
	})
}

func (c *catalog) readSchemas(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlSchemas, func(rows pipeline.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		c.schema.Schemas = append(c.schema.Schemas, name)
		return nil
	})
}

func (c *catalog) readExtensions(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlExtensions, func(rows pipeline.Rows) error {
		var e pipeline.Extension
		if err := rows.Scan(&e.Name, &e.Schema); err != nil {
			return err
		}
		c.schema.Extensions = append(c.schema.Extensions, e)
		return nil
	})
}

func (c *catalog) readEnums(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlEnums, func(rows pipeline.Rows) error {
		var name, label string
		if err := rows.Scan(&name, &label); err != nil {
			return err
		}
		c.schema.Enums[name] = append(c.schema.Enums[name], label)
		return nil
	})
}

func (c *catalog) readDomains(ctx context.Context, r pipeline.Reader) error {
	defs, err := readNamedDefs(ctx, r, sqlDomains)
	c.schema.Domains = defs
	return err
}

func (c *catalog) readComposites(ctx context.Context, r pipeline.Reader) error {
	defs, err := readNamedDefs(ctx, r, sqlComposites)
	c.schema.Composites = defs
	return err
}

func readNamedDefs(ctx context.Context, r pipeline.Reader, sql string) ([]pipeline.NamedDef, error) {
	var out []pipeline.NamedDef
	err := each(ctx, r, sql, func(rows pipeline.Rows) error {
		var d pipeline.NamedDef
		if err := rows.Scan(&d.Name, &d.Def); err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	return out, err
}

func (c *catalog) readTables(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlTables, func(rows pipeline.Rows) error {
		var (
			t           ref.TableRef
			relkind     string
			isPartition bool
			rls, force  bool
			approx      int64
			pages       int64
			partKeyDef  string
			triggers    int
			readable    bool
		)
		if err := rows.Scan(&t.Schema, &t.Name, &relkind, &isPartition,
			&rls, &force, &approx, &pages, &partKeyDef, &triggers, &readable); err != nil {
			return err
		}
		// reltuples is -1 on a relation nothing has analysed. It is a planning
		// hint and never a gate (ARCHITECTURE.md §2), so "unknown" is carried as
		// zero rather than as a negative count downstream code would have to
		// special-case; the sampler reads the same zero alongside relpages.
		if approx < 0 {
			approx = 0
		}
		if pages < 0 {
			pages = 0
		}
		c.pages[t] = pages
		if !readable {
			c.unreadable[t] = true
		}
		if isPartition {
			// Filled in by linkPartitions, which needs the whole inheritance
			// map to find the root rather than the immediate parent.
			c.partitionOf[t] = ref.TableRef{}
		}
		c.order = append(c.order, t)
		c.tables[t] = &pipeline.Table{
			Ref:          t,
			Partitioned:  relkind == "p",
			PartitionKey: partitionKeyColumns(partKeyDef),
			RLS:          rls,
			ForceRLS:     force,
			Triggers:     triggers,
			ApproxRows:   approx,
		}
		return nil
	})
}

func (c *catalog) readColumns(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlColumns, func(rows pipeline.Rows) error {
		var (
			t         ref.TableRef
			col       pipeline.Column
			def       string
			generated string
		)
		if err := rows.Scan(&t.Schema, &t.Name, &col.Name,
			&col.TypeName, &col.TypeOID, &col.TypMod, &col.Collation, &col.Nullable,
			&def, &generated, &col.Identity, &col.Domain); err != nil {
			return err
		}
		// A generated column's expression is stored in pg_attrdef, so one
		// column reads it as its default; attgenerated says which it is.
		if generated == "" {
			col.Default = def
		} else {
			col.Generated = def
		}
		col.Fingerprint = columnFingerprint(col)
		tbl, ok := c.tables[t]
		if !ok {
			return nil
		}
		tbl.Columns = append(tbl.Columns, col)
		return nil
	})
}

func (c *catalog) readColumnChecks(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlColumnChecks, func(rows pipeline.Rows) error {
		var t ref.TableRef
		var column, def string
		if err := rows.Scan(&t.Schema, &t.Name, &column, &def); err != nil {
			return err
		}
		tbl, ok := c.tables[t]
		if !ok {
			return nil
		}
		for i := range tbl.Columns {
			if tbl.Columns[i].Name == column {
				tbl.Columns[i].Checks = append(tbl.Columns[i].Checks, def)
				// Checks are not part of Column.Fingerprint (ARCHITECTURE.md
				// §2 names four fields), so nothing is recomputed here.
				break
			}
		}
		return nil
	})
}

func (c *catalog) readIndexes(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlIndexes, func(rows pipeline.Rows) error {
		var t ref.TableRef
		var idx pipeline.Index
		var cols []string
		if err := rows.Scan(&t.Schema, &t.Name, &idx.Name,
			&idx.Unique, &idx.Partial, &idx.Expression, &idx.Immediate, &idx.Def, &cols); err != nil {
			return err
		}
		// An expression index has no column list (ARCHITECTURE.md §2); indkey
		// carries a zero for each expression member, which the catalog query
		// cannot resolve to a name, so a partial list would read as a plain
		// index on fewer columns and the planner would accept it as identity.
		if !idx.Expression {
			idx.Columns = cols
		}
		if tbl, ok := c.tables[t]; ok {
			tbl.Indexes = append(tbl.Indexes, idx)
		}
		return nil
	})
}

func (c *catalog) readConstraints(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlConstraints, func(rows pipeline.Rows) error {
		var t ref.TableRef
		var con pipeline.Constraint
		var kind string
		if err := rows.Scan(&t.Schema, &t.Name, &con.Name, &kind, &con.Def); err != nil {
			return err
		}
		if kind == "" {
			return fmt.Errorf("constraint %s on %s has no contype", con.Name, t)
		}
		con.Kind = kind[0]
		if tbl, ok := c.tables[t]; ok {
			tbl.Constraints = append(tbl.Constraints, con)
		}
		return nil
	})
}

func (c *catalog) readPrimaryKeys(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlPrimaryKeys, func(rows pipeline.Rows) error {
		var t ref.TableRef
		var column string
		if err := rows.Scan(&t.Schema, &t.Name, &column); err != nil {
			return err
		}
		if tbl, ok := c.tables[t]; ok {
			tbl.PK = append(tbl.PK, column)
		}
		return nil
	})
}

func (c *catalog) readSequences(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlSequences, func(rows pipeline.Rows) error {
		var t ref.TableRef
		var seq pipeline.SequenceDef
		if err := rows.Scan(&t.Schema, &t.Name, &seq.Column, &seq.Name,
			&seq.Start, &seq.Increment, &seq.Min, &seq.Max, &seq.Cache, &seq.Cycle); err != nil {
			return err
		}
		tbl, ok := c.tables[t]
		if !ok {
			return nil
		}
		tbl.Sequences = append(tbl.Sequences, seq)
		for i := range tbl.Columns {
			if tbl.Columns[i].Name == seq.Column && tbl.Columns[i].Identity != "" {
				owned := seq
				tbl.Columns[i].IdentitySeq = &owned
				break
			}
		}
		return nil
	})
}

func (c *catalog) readForeignKeys(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlForeignKeys, func(rows pipeline.Rows) error {
		var fk pipeline.ForeignKey
		if err := rows.Scan(&fk.Name,
			&fk.Child.Schema, &fk.Child.Name, &fk.Parent.Schema, &fk.Parent.Name,
			&fk.MatchFull, &fk.Validated, &fk.ChildCols, &fk.ParentCols); err != nil {
			return err
		}
		c.schema.FKs = append(c.schema.FKs, fk)
		return nil
	})
}

func (c *catalog) readPartitions(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlPartitions, func(rows pipeline.Rows) error {
		var parent, child ref.TableRef
		var relkind string
		if err := rows.Scan(&parent.Schema, &parent.Name,
			&child.Schema, &child.Name, &relkind); err != nil {
			return err
		}
		c.children[parent] = append(c.children[parent], child)
		c.partitionOf[child] = parent
		return nil
	})
}

func (c *catalog) readNotRecreated(ctx context.Context, r pipeline.Reader) error {
	return each(ctx, r, sqlNotRecreated, func(rows pipeline.Rows) error {
		var o pipeline.Object
		if err := rows.Scan(&o.Kind, &o.Name); err != nil {
			return err
		}
		c.schema.NotRecreated = append(c.schema.NotRecreated, o)
		return nil
	})
}

// linkPartitions fills Table.Partitions with the leaves under each partitioned
// table and Table.Parent with each partition's root, which is the table the
// planner and the loader address (testdata/README.md trap 7). A partition of a
// partition therefore names the top of the tree, not its immediate parent.
func (c *catalog) linkPartitions() {
	for _, t := range c.order {
		tbl := c.tables[t]
		if tbl.Partitioned {
			leaves := c.leavesOf(t, map[ref.TableRef]bool{})
			sort.Slice(leaves, func(i, j int) bool { return leaves[i].Less(leaves[j]) })
			tbl.Partitions = leaves
		}
		if root, ok := c.rootOf(t); ok {
			r := root
			tbl.Parent = &r
		}
	}
}

// leavesOf returns every non-partitioned descendant of t, which are the tables
// that actually hold rows. seen guards a catalog that somehow describes a cycle;
// pg_inherits cannot, and an unbounded recursion here would hang the run rather
// than fail it.
func (c *catalog) leavesOf(t ref.TableRef, seen map[ref.TableRef]bool) []ref.TableRef {
	if seen[t] {
		return nil
	}
	seen[t] = true
	var out []ref.TableRef
	for _, child := range c.children[t] {
		if tbl, ok := c.tables[child]; ok && tbl.Partitioned {
			out = append(out, c.leavesOf(child, seen)...)
			continue
		}
		out = append(out, child)
	}
	return out
}

func (c *catalog) rootOf(t ref.TableRef) (ref.TableRef, bool) {
	parent, ok := c.partitionOf[t]
	if !ok || parent == (ref.TableRef{}) {
		return ref.TableRef{}, false
	}
	for i := 0; i < len(c.tables)+1; i++ {
		next, ok := c.partitionOf[parent]
		if !ok || next == (ref.TableRef{}) {
			return parent, true
		}
		parent = next
	}
	return parent, true
}

// repointPartitionForeignKeys moves an edge declared on a leaf partition, or
// referencing one, onto the leaf's root.
//
// A foreign key declared on a leaf — `ALTER TABLE public.ev_2024 ADD CONSTRAINT
// ev24_dev_fk FOREIGN KEY (dev_id) REFERENCES public.dev(id)` — has
// conparentid = 0, so it is not one of the copies Postgres clones onto every
// partition; it is a constraint somebody wrote, on a column that lives on the
// root, and it is a real dependency of the root's data. The root is the table
// §11.1 recreates as one plain table and the only one the planner addresses
// (§3.3), so the edge is moved there: the planner then walks it and pulls the
// parent rows, and §11.1 item 6 recreates it against a table the target has. An
// edge *referencing* a leaf is moved the same way and for the same reason; the
// leaf's columns are the root's columns, so the column lists carry over
// unchanged. Both directions widen what the edge covers — the root holds every
// partition's rows — which pulls a superset of the parents the source needed and
// never a subset, so the slice stays referentially complete.
//
// The parent end moves only onto a key the root actually carries. A foreign key
// needs a unique or primary-key constraint over the columns it references, and
// a leaf can carry one the root cannot: a partitioned table's unique constraint
// must include every partition key column and a leaf's need not, so
// `ev_2024 UNIQUE (id)` is legal under a root partitioned by `at` whose own
// unique constraint can only be `(id, at)`. §11.1 recreates none of a leaf's
// constraints or indexes, so re-pointing such an edge would emit an ADD
// CONSTRAINT that fails with "there is no unique constraint matching given keys
// for referenced table" — at item 6, after item 1 has dropped every user table
// in the target. The parent end is therefore re-pointed only when the root
// carries a unique or primary-key constraint whose columns are exactly the
// edge's ParentCols; otherwise that end stays on the leaf and the edge is
// marked ForeignKey.NotRecreatable, which is the planner's cue to refuse under
// §11.1 (exit 13) before anything is dropped. Such an edge is the one place
// Schema.FKs may still name a partition. Its child end still moves, because
// that half is a constraint declared on a leaf over the root's own columns and
// is right however far the run gets.
//
// A re-pointed edge that lands on an edge the source already declares on the
// root, or on another leaf, is dropped: same child, same parent, same columns,
// same match type and the same validity is the same constraint twice — the
// shape pagila v3.1.0 is written in, where six payment partitions each declare
// customer, rental and staff and the root declares none — and recreating it
// once per leaf would be six names for one check. Validity is part of that
// sameness, so a copy the source has not validated never absorbs one it has:
// an unvalidated edge is a hint the planner does not follow (§3.1), and
// collapsing a validated copy into it would drop a parent nobody pulls. Only a
// re-pointed edge is ever dropped this way, so an ordinary table that genuinely
// carries two identical foreign keys still carries both, and a target lazyslice
// wrote reads back exactly the set that was hashed (§11.2).
func (c *catalog) repointPartitionForeignKeys() {
	// The key is what makes two edges the same constraint. The column lists are
	// joined on a NUL, which no identifier can contain, so ("a", "b,c") and
	// ("a,b", "c") do not collide.
	type edge struct {
		child, parent         ref.TableRef
		childCols, parentCols string
		matchFull, validated  bool
	}
	key := func(fk pipeline.ForeignKey) edge {
		return edge{
			child: fk.Child, parent: fk.Parent,
			childCols:  strings.Join(fk.ChildCols, "\x00"),
			parentCols: strings.Join(fk.ParentCols, "\x00"),
			matchFull:  fk.MatchFull,
			validated:  fk.Validated,
		}
	}
	root := func(t ref.TableRef) (ref.TableRef, bool) {
		tbl, ok := c.tables[t]
		if !ok || tbl.Parent == nil {
			return t, false
		}
		return *tbl.Parent, true
	}

	out := make([]pipeline.ForeignKey, 0, len(c.schema.FKs))
	seen := map[edge]bool{}
	var moved []pipeline.ForeignKey
	for _, fk := range c.schema.FKs {
		child, movedChild := root(fk.Child)
		parent, movedParent := root(fk.Parent)
		// The root may not carry the key the leaf carries. Leaving the end on
		// the leaf and marking the edge is a refusal the planner makes at plan
		// time; re-pointing it anyway is an ADD CONSTRAINT that fails after the
		// target's tables have been dropped.
		if movedParent && !c.hasKeyOver(parent, fk.ParentCols) {
			fk.NotRecreatable = true
			parent, movedParent = fk.Parent, false
		}
		if !movedChild && !movedParent {
			out = append(out, fk)
			seen[key(fk)] = true
			continue
		}
		fk.Child, fk.Parent = child, parent
		moved = append(moved, fk)
	}
	for _, fk := range moved {
		k := key(fk)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, fk)
	}
	// Back into the order the statement returned — by constraint name, then by
	// child — which is a no-op for every edge that did not move.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Child != out[j].Child {
			return out[i].Child.Less(out[j].Child)
		}
		return out[i].Parent.Less(out[j].Parent)
	})
	c.schema.FKs = out
}

// hasKeyOver reports whether t carries a key a foreign key referencing cols can
// be declared against: a primary key, a unique constraint, or a bare unique
// index that is neither partial, nor over an expression, nor deferrable.
// Postgres accepts all three as a referenced key, and §11.1 item 6 creates every
// index before it adds any foreign key, so an index the source has is on the
// target when the ADD CONSTRAINT runs.
//
// It is answered through t.Indexes rather than by parsing
// pg_get_constraintdef's text, which spells the same thing three ways once
// NULLS NOT DISTINCT and quoted identifiers are in it: a primary key and a
// unique constraint each have a backing index under the constraint's own name,
// so scanning the indexes covers the constraint-backed and the bare case at
// once. Partial and expression indexes are excluded because Postgres refuses
// them as a referenced key, and so is an index that is not immediate: a
// DEFERRABLE PRIMARY KEY or UNIQUE constraint is refused with "cannot use a
// deferrable unique constraint for referenced table" (verified on postgres:16),
// and indimmediate is the only field that tells one apart. An index that is not
// live, valid and ready never reaches Table.Indexes at all.
//
// The primary key is answered the same way rather than from t.PK, because
// t.PK carries the column names and not whether the constraint over them is
// deferrable; the index backing a primary key is always in t.Indexes.
//
// Order does not matter. Postgres matches a foreign key's referenced columns to
// a unique constraint's columns as a set, so `REFERENCES ev (at, id)` is
// satisfied by `UNIQUE (id, at)`.
func (c *catalog) hasKeyOver(t ref.TableRef, cols []string) bool {
	tbl, ok := c.tables[t]
	if !ok || len(cols) == 0 {
		return false
	}
	for _, idx := range tbl.Indexes {
		if !idx.Unique || idx.Partial || idx.Expression || !idx.Immediate {
			continue
		}
		if sameColumnSet(idx.Columns, cols) {
			return true
		}
	}
	return false
}

// sameColumnSet reports whether two column lists hold the same names. Neither a
// key nor a foreign key's column list can repeat a column, so equal lengths and
// containment one way is set equality.
func sameColumnSet(a, b []string) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	in := make(map[string]bool, len(a))
	for _, col := range a {
		in[col] = true
	}
	for _, col := range b {
		if !in[col] {
			return false
		}
	}
	return true
}

// markIndexedForeignKeys sets ForeignKey.Indexed: the child columns are covered
// when some index on the child has them as its leading key columns, in any
// order. That is the shape a key lookup uses, and it is what the plan reports
// as an unindexed edge when it is missing (ARCHITECTURE.md §2).
func (c *catalog) markIndexedForeignKeys() {
	for i := range c.schema.FKs {
		fk := &c.schema.FKs[i]
		child, ok := c.tables[fk.Child]
		if !ok || len(fk.ChildCols) == 0 {
			continue
		}
		for _, idx := range child.Indexes {
			if coversLeading(idx.Columns, fk.ChildCols) {
				fk.Indexed = true
				break
			}
		}
	}
}

func coversLeading(index, want []string) bool {
	if len(index) < len(want) {
		return false
	}
	leading := map[string]bool{}
	for _, col := range index[:len(want)] {
		leading[col] = true
	}
	for _, col := range want {
		if !leading[col] {
			return false
		}
	}
	return true
}

// collect copies the tables into the schema in catalog order, which is
// (schema, name) — the order everything downstream iterates (ARCHITECTURE.md §2).
func (c *catalog) collect() {
	c.schema.Tables = make([]pipeline.Table, 0, len(c.order))
	for _, t := range c.order {
		c.schema.Tables = append(c.schema.Tables, *c.tables[t])
	}
}

// partitionKeyColumns pulls the members out of pg_get_partkeydef's text:
// "RANGE (occurred_at)" is ["occurred_at"] and "LIST ((a + b), c)" is
// ["(a + b)", "c"]. A key member can be an expression, so this is a split of
// the catalog's own deparsed text rather than a read of partattrs, which
// carries a zero where the expression is and would silently drop it.
func partitionKeyColumns(def string) []string {
	open := strings.Index(def, "(")
	if open < 0 || !strings.HasSuffix(def, ")") {
		return nil
	}
	inner := def[open+1 : len(def)-1]

	var (
		out   []string
		depth int
		start int
		quote bool
	)
	for i := 0; i < len(inner); i++ {
		switch ch := inner[i]; {
		case ch == '"':
			quote = !quote
		case quote:
		case ch == '(':
			depth++
		case ch == ')':
			depth--
		case ch == ',' && depth == 0:
			out = append(out, strings.TrimSpace(inner[start:i]))
			start = i + 1
		}
	}
	if member := strings.TrimSpace(inner[start:]); member != "" {
		out = append(out, member)
	}
	return out
}
