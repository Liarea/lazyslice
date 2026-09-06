// SPDX-License-Identifier: Apache-2.0

// Package plan computes the subset: which rows of which tables the snapshot
// will hold, and why each one is there (ARCHITECTURE.md section 3).
//
// It is a client-side monotone worklist with provenance tags, never SQL
// pushdown. Pushdown would be faster and would cost the two things that matter:
// the plan could not say why a row is present, and the source would have to
// accept a temp table from us.
//
// Determinism is a property, not an accident. The queue is FIFO; tables iterate
// in (schema, name) order; edges iterate in constraint-name order; every key
// set, chunk and SQL result is ordered by the identity columns; nothing iterates
// a Go map. Two runs over one snapshot therefore produce byte-identical
// selected sets, which is what makes lazyslice.yml a record of what happened.
//
// A row's mode is decided once, when it is first popped. A table reached only as
// a parent does not have its own children pulled through, which is both the size
// control and the privacy control.
package plan

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The defaults ARCHITECTURE.md §3 states. A PlanRequest field left at zero
// takes the default, and a caller that means an explicit zero — no children, no
// root rows, no rows per parent key — passes a negative Take, Cap or Depth,
// which clamps to zero. Zero cannot mean both "unset" and "none" in an int
// field, so the negative is the escape until PlanRequest can carry the
// difference; core owes `--take 0`, `--cap 0` and `--depth 0` that translation,
// because a zero the operator typed must mean zero. Nothing here reads a flag.
const (
	DefaultTake         = 500
	DefaultCap          = 100
	DefaultDepth        = 3
	DefaultRowBudget    = 1_000_000
	DefaultMemoryBudget = 256 << 20 // 256 MiB
)

// followsAsParent says whether the walk enters a table through this edge in the
// parent direction, and followsAsChild the same in the child direction. They
// are the only statement of the rule: walk (through parents and children) and
// staticReach both call them, so an edge filter cannot be changed in the row
// walk and missed in the table-level over-approximation §3.6 depends on. What
// the two do differ in is scope, and only scope: staticReach runs before
// --skip-table, the unreadable drops and findLookups, because §3.6 needs its
// answer before a key is fetched.
//
// A virtual edge is followed in neither direction. §3.2's mapping half is not
// implemented in v1 (polymorphic.go), so Plan.Virtual is empty; following a
// Virtual edge introspect supplied would pull parent rows into the slice
// through an inferred edge the plan does not print, which is the one thing
// §3.5 says a plan must never do.
func followsAsParent(fk pipeline.ForeignKey) bool { return fk.Validated && !fk.Virtual }

func followsAsChild(fk pipeline.ForeignKey) bool { return !fk.Virtual }

// lookupRowCeiling is §3's bound on a lookup table: more rows than this and the
// table is walked rather than copied whole.
const lookupRowCeiling = 1000

// lookupApproxCeiling is the reltuples above which the bounded count probe is
// not even issued. ApproxRows may skip a probe, never decide one.
const lookupApproxCeiling = 10_000

// residualBitsPerCell is the residual Bloom filter's cost per masked cell at a
// 10^-6 false-positive rate (ARCHITECTURE.md §6). The planner counts it against
// the memory budget and prints it in the estimate; internal/transform builds
// the filter itself.
const residualBitsPerCell = 29

type planner struct{}

// New returns the subset planner.
func New() pipeline.Planner { return planner{} }

var _ pipeline.Planner = planner{}

// Plan computes the subset (ARCHITECTURE.md §3).
func (planner) Plan(
	ctx context.Context,
	r pipeline.Reader,
	schema *pipeline.Schema,
	cls *pipeline.Classification,
	req pipeline.PlanRequest,
) (*pipeline.Plan, error) {
	if schema == nil {
		return nil, errors.New("plan: no schema")
	}
	if r == nil {
		return nil, errors.New("plan: no reader")
	}
	p := &run{r: r, schema: schema, cls: cls, req: withDefaults(req), started: time.Now()}
	return p.plan(ctx)
}

// withDefaults fills the zero fields of a request with §3's defaults.
func withDefaults(req pipeline.PlanRequest) pipeline.PlanRequest {
	if req.Take == 0 {
		req.Take = DefaultTake
	}
	if req.Take < 0 {
		req.Take = 0
	}
	if req.Cap == 0 {
		req.Cap = DefaultCap
	}
	if req.Cap < 0 {
		req.Cap = 0
	}
	if req.Depth == 0 {
		req.Depth = DefaultDepth
	}
	if req.Depth < 0 {
		req.Depth = 0
	}
	if req.RowBudget == 0 {
		req.RowBudget = DefaultRowBudget
	}
	if req.MemoryBudget == 0 {
		req.MemoryBudget = DefaultMemoryBudget
	}
	return req
}

// run is one planning run. Every map in it is read through a sorted key list or
// by a single lookup; nothing iterates one where the order would show.
type run struct {
	r       pipeline.Reader
	schema  *pipeline.Schema
	cls     *pipeline.Classification
	req     pipeline.PlanRequest
	started time.Time

	tables   []pipeline.Table                       // planned tables, (schema, name) order, no partitions
	byRef    map[ref.TableRef]*pipeline.Table       // every non-partition table, planned or not
	inScope  map[ref.TableRef]bool                  // the planned set
	fks      []pipeline.ForeignKey                  // non-partition edges, constraint-name order
	outgoing map[ref.TableRef][]pipeline.ForeignKey // t references these
	incoming map[ref.TableRef][]pipeline.ForeignKey // these reference t

	rootOf      map[ref.TableRef]ref.TableRef // a partition leaf to the table that stands for it
	ids         map[ref.TableRef]identity
	lookups     map[ref.TableRef]bool
	lookupRows  map[ref.TableRef]int64
	cellsPerRow map[ref.TableRef]int
	selected    map[ref.TableRef]*keys
	label       map[ref.TableRef]pipeline.Mode
	why         map[ref.TableRef]string
	depth       map[ref.TableRef]int
	capOf       map[ref.TableRef]int
	skipped     []ref.TableRef
	unreadable  []ref.TableRef
	selectedRow int64
}

// item is one entry of the FIFO worklist.
type item struct {
	table ref.TableRef
	keys  *keys
	mode  pipeline.Mode
	depth int
}

func (p *run) plan(ctx context.Context) (*pipeline.Plan, error) {
	// The request is checked before the schema is: --where is text from outside
	// this program, and a predicate the source allowlist would refuse is a usage
	// error named here rather than an allowlist violation named nowhere
	// (where.go, THREAT_MODEL.md T9). Nothing has been built or read yet.
	if err := checkWhere(p.req.Where); err != nil {
		return nil, err
	}
	// The schema refusal comes next: it is raised before the snapshot is used
	// for keys and before anything in the target is dropped (§11.1).
	if err := p.checkRecreatable(); err != nil {
		return nil, err
	}
	p.build()

	root, rootReason, err := p.chooseRoot()
	if err != nil {
		return nil, err
	}
	if err := p.applySkipAndPrivileges(ctx, root); err != nil {
		return nil, err
	}
	if err := p.resolveIdentities(ctx); err != nil {
		return nil, err
	}
	if err := p.findLookups(ctx, root); err != nil {
		return nil, err
	}
	if err := p.walk(ctx, root); err != nil {
		return nil, err
	}
	return p.assemble(root, rootReason), nil
}

// checkRecreatable refuses a foreign key the target's schema cannot carry.
func (p *run) checkRecreatable() error {
	fks := append([]pipeline.ForeignKey(nil), p.schema.FKs...)
	sort.Slice(fks, func(a, b int) bool { return fks[a].Name < fks[b].Name })
	for _, fk := range fks {
		if !fk.NotRecreatable {
			continue
		}
		cols := joinCols(fk.ChildCols)
		return refuse(CodeNotRecreatable, exitSchema, fk.Child,
			fmt.Sprintf("the foreign key %s on %s (%s) into %s cannot be recreated in the target",
				fk.Name, fk.Child, cols, fk.Parent),
			event.Args{
				event.ArgTable:  fk.Child.String(),
				event.ArgColumn: cols,
				event.ArgReason: fk.Parent.String(),
			})
	}
	return nil
}

// build indexes the schema: the planned tables in (schema, name) order and the
// edges in constraint-name order. A partition is never a step, so it is not a
// table here; §3.3 collapses one into its root at introspect.
func (p *run) build() {
	p.byRef = map[ref.TableRef]*pipeline.Table{}
	p.rootOf = map[ref.TableRef]ref.TableRef{}
	for i := range p.schema.Tables {
		t := &p.schema.Tables[i]
		if t.Parent != nil {
			// introspect points a leaf at the top of its tree, not at its
			// immediate parent, so one hop is the whole answer.
			p.rootOf[t.Ref] = *t.Parent
			continue
		}
		p.byRef[t.Ref] = t
		p.tables = append(p.tables, *t)
	}
	sort.Slice(p.tables, func(a, b int) bool { return tableRefLess(p.tables[a].Ref, p.tables[b].Ref) })

	p.outgoing = map[ref.TableRef][]pipeline.ForeignKey{}
	p.incoming = map[ref.TableRef][]pipeline.ForeignKey{}
	fks := append([]pipeline.ForeignKey(nil), p.schema.FKs...)
	sort.Slice(fks, func(a, b int) bool { return fks[a].Name < fks[b].Name })
	for _, fk := range fks {
		if p.byRef[fk.Child] == nil || p.byRef[fk.Parent] == nil {
			continue
		}
		p.fks = append(p.fks, fk)
		p.outgoing[fk.Child] = append(p.outgoing[fk.Child], fk)
		p.incoming[fk.Parent] = append(p.incoming[fk.Parent], fk)
	}

	p.inScope = map[ref.TableRef]bool{}
	for _, t := range p.tables {
		p.inScope[t.Ref] = true
	}
	p.ids = map[ref.TableRef]identity{}
	p.lookups = map[ref.TableRef]bool{}
	p.lookupRows = map[ref.TableRef]int64{}
	p.cellsPerRow = map[ref.TableRef]int{}
	p.selected = map[ref.TableRef]*keys{}
	p.label = map[ref.TableRef]pipeline.Mode{}
	p.why = map[ref.TableRef]string{}
	p.depth = map[ref.TableRef]int{}
	p.capOf = map[ref.TableRef]int{}
	for _, t := range p.tables {
		p.cellsPerRow[t.Ref] = p.maskedCellsPerRow(t)
	}
}

// maskedCellsPerRow is how many residual-filter entries one row of a table
// costs: one for each masked scalar column, and one for each scalar leaf of a
// masked json, jsonb or hstore document, because §4 replaces every leaf of a
// masked document and §6 keys the filter by path. ARCHITECTURE.md §3's estimate
// is maskedCells(selected, cls) x 29 / 8 with "JSON leaves counted
// individually", so counting a document as one cell would undercount both the
// printed estimate and the memory budget by the document's leaf count.
//
// The leaf count is measured from the samples introspect already read, averaged
// over the documents that parsed and rounded up.
func (p *run) maskedCellsPerRow(t pipeline.Table) int {
	if p.cls == nil {
		return 0
	}
	n := 0
	for i, c := range t.Columns {
		if d, ok := p.cls.Decisions[ref.ColumnRef{Table: t.Ref, Column: c.Name}]; ok && d.Masked {
			n += leavesPerDocument(t, i, c)
		}
	}
	return n
}

// chooseRoot takes --root when it is given and §3.1's default otherwise.
func (p *run) chooseRoot() (ref.TableRef, string, error) {
	if p.req.Root != nil {
		root := *p.req.Root
		if !p.inScope[root] {
			return ref.TableRef{}, "", refuse(CodeNoRoot, exitUsage, root,
				fmt.Sprintf("--root %s names no table in the source", root),
				event.Args{event.ArgFlag: "--root " + root.String()})
		}
		return root, "named by --root", nil
	}
	root, reason, ok := defaultRoot(p.tables, p.fks)
	if !ok {
		return ref.TableRef{}, "", refuse(CodeNoRoot, exitUsage, ref.TableRef{},
			"the source has no table to slice from", event.Args{event.ArgFlag: "--root"})
	}
	return root, reason, nil
}

// applySkipAndPrivileges resolves --skip-table and the unreadable tables of
// §3.6, before the snapshot is used for a key.
func (p *run) applySkipAndPrivileges(ctx context.Context, root ref.TableRef) error {
	asParent := p.staticReach(root)

	for _, t := range sortedRefs(p.req.Skip) {
		if !p.inScope[t] {
			continue
		}
		if t == root {
			return refuse(CodeSkipParent, exitPlan, t,
				fmt.Sprintf("--skip-table %s names the root of the slice", t),
				event.Args{event.ArgTable: t.String(), event.ArgFlag: "--skip-table"})
		}
		if asParent[t] {
			return refuse(CodeSkipParent, exitPlan, t,
				fmt.Sprintf("--skip-table %s cannot skip a table the slice needs as a parent: "+
					"the slice would not be referentially complete", t),
				event.Args{event.ArgTable: t.String(), event.ArgFlag: "--skip-table"})
		}
		p.drop(t)
		p.skipped = append(p.skipped, t)
		p.why[t] = "skipped"
	}

	unreadable, err := p.readUnreadable(ctx)
	if err != nil {
		return err
	}
	if len(unreadable) == 0 {
		return nil
	}
	role, err := p.readRole(ctx)
	if err != nil {
		return err
	}
	seen := map[ref.TableRef]bool{}
	for _, u := range unreadable {
		if !p.inScope[u.table] || seen[u.table] {
			continue
		}
		if u.table == root || asParent[u.table] {
			what := u.table.String()
			if u.relation != u.table {
				what = u.table.String() + " (its partition " + u.relation.String() + ")"
			}
			return refuse(CodeUnreadable, exitPlan, u.table,
				fmt.Sprintf("%s cannot read %s, which the slice needs: %s",
					role, what, grantStatement(u.relation, role)),
				event.Args{
					event.ArgTable:     u.table.String(),
					event.ArgRole:      role,
					event.ArgStatement: grantStatement(u.relation, role),
				})
		}
		// Unreachable or reachable only as a child: dropped to SchemaOnly, listed,
		// and the run continues (§3.6).
		seen[u.table] = true
		p.drop(u.table)
		p.unreadable = append(p.unreadable, u.table)
		p.why[u.table] = "unreadable"
	}
	return nil
}

// drop removes a table from the planned set. Its steps become SchemaOnly and
// no edge into or out of it is followed.
func (p *run) drop(t ref.TableRef) {
	delete(p.inScope, t)
}

// staticReach is the table-level over-approximation of the walk: which tables
// can be reached from the root at all, and which of them can be entered through
// a parent edge. §3.6 needs the second question answered before a key is
// fetched, so it is answered on the graph rather than on the rows — a table the
// rows would never reach is treated as one the slice needs, which fails towards
// a refusal naming a GRANT rather than towards a mid-extract permission error.
func (p *run) staticReach(root ref.TableRef) map[ref.TableRef]bool {
	asParent := map[ref.TableRef]bool{}

	type node struct {
		t    ref.TableRef
		mode pipeline.Mode
	}
	best := map[node]int{}
	queue := []struct {
		node
		d int
	}{{node{root, pipeline.ChildOK}, 0}}
	best[node{root, pipeline.ChildOK}] = 0

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if d, ok := best[cur.node]; ok && d < cur.d {
			continue
		}
		push := func(t ref.TableRef, mode pipeline.Mode, d int) {
			n := node{t, mode}
			if old, ok := best[n]; ok && old <= d {
				return
			}
			best[n] = d
			queue = append(queue, struct {
				node
				d int
			}{n, d})
		}
		for _, fk := range p.outgoing[cur.t] {
			if !followsAsParent(fk) {
				continue
			}
			asParent[fk.Parent] = true
			push(fk.Parent, pipeline.ParentOnly, cur.d)
		}
		if cur.mode == pipeline.ChildOK && cur.d < p.req.Depth {
			for _, fk := range p.incoming[cur.t] {
				if !followsAsChild(fk) {
					continue
				}
				push(fk.Child, pipeline.ChildOK, cur.d+1)
			}
		}
	}
	return asParent
}

// resolveIdentities walks the identity ladder for every planned table, which is
// where a table with no identity stops the run (§3.4). It runs after
// --skip-table has been applied, so that the flag clears the refusal for a
// child-only table (T-0032).
func (p *run) resolveIdentities(ctx context.Context) error {
	for i := range p.tables {
		t := &p.tables[i]
		if !p.inScope[t.Ref] {
			continue
		}
		id, err := p.resolveIdentity(ctx, t)
		if err != nil {
			return err
		}
		p.ids[t.Ref] = id
	}
	return nil
}

// findLookups is §3's lookup rule: a table with no outgoing edge, at least one
// incoming edge, at most 1,000 rows and no column the classifier masks is
// copied whole rather than walked. A lookup that carries personal data is
// walked, because copying it whole would copy every person in it.
func (p *run) findLookups(ctx context.Context, root ref.TableRef) error {
	for _, t := range p.tables {
		if !p.inScope[t.Ref] || t.Ref == root {
			continue
		}
		if len(p.outgoing[t.Ref]) > 0 || len(p.incoming[t.Ref]) == 0 {
			continue
		}
		if p.hasMaskedColumn(t) {
			continue
		}
		if t.ApproxRows > lookupApproxCeiling {
			continue
		}
		n, err := p.boundedCount(ctx, t.Ref)
		if err != nil {
			return err
		}
		if n > lookupRowCeiling {
			continue
		}
		p.lookups[t.Ref] = true
		p.lookupRows[t.Ref] = n
		p.why[t.Ref] = "lookup"
	}
	return nil
}

// hasMaskedColumn says whether any column of the table reaches the mask
// threshold. It is the "∀c∈t: cls[c].Confidence < Possible" half of §3's
// lookup rule.
//
// A nil classification answers yes for every table, which is what stops the
// rule from failing open. "No column reaches Possible" is trivially true when
// nothing was classified, and a lookup-shaped table — no outgoing edge, at
// least one incoming edge, under a thousand rows — is the shape of staff,
// admins, api_keys and contacts. Copying one whole hands over every production
// row of it, surrogate keys included, because §6 item 6 keeps those verbatim.
// An unclassified run therefore walks rather than copies (root CLAUDE.md: when
// in doubt, mask it), and `lazyslice plan` over a schema alone produces no
// Lookup step.
func (p *run) hasMaskedColumn(t pipeline.Table) bool {
	if p.cls == nil {
		return true
	}
	for _, c := range t.Columns {
		if d, ok := p.cls.Decisions[ref.ColumnRef{Table: t.Ref, Column: c.Name}]; ok {
			if d.Confidence >= pipeline.ConfPossible {
				return true
			}
		}
	}
	return false
}

// walk is the FIFO worklist of §3.
func (p *run) walk(ctx context.Context, root ref.TableRef) error {
	seed, err := p.seedKeys(ctx, root)
	if err != nil {
		return err
	}
	p.why[root] = "root"
	queue := []item{{table: root, keys: seed, mode: pipeline.ChildOK, depth: 0}}

	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]

		fresh := p.subtract(it.table, it.keys)
		if fresh.Len() == 0 {
			continue
		}
		p.absorb(it.table, fresh)
		if it.mode == pipeline.ChildOK {
			p.label[it.table] = pipeline.ChildOK
		} else if _, ok := p.label[it.table]; !ok {
			p.label[it.table] = pipeline.ParentOnly
		}
		if _, ok := p.depth[it.table]; !ok {
			p.depth[it.table] = it.depth
		}
		if err := p.checkBudgets(it.table); err != nil {
			return err
		}

		pushed, err := p.parents(ctx, it, fresh)
		if err != nil {
			return err
		}
		queue = append(queue, pushed...)

		if it.mode == pipeline.ChildOK && it.depth < p.req.Depth {
			pushed, err = p.children(ctx, it, fresh)
			if err != nil {
				return err
			}
			queue = append(queue, pushed...)
		}
	}
	return nil
}

// parents is the parent step: mandatory, uncapped, at any depth, PARENT_ONLY.
func (p *run) parents(ctx context.Context, it item, fresh *keys) ([]item, error) {
	var out []item
	id := p.ids[it.table]
	for _, fk := range p.outgoing[it.table] {
		if !followsAsParent(fk) {
			continue
		}
		if !p.inScope[fk.Parent] || p.lookups[fk.Parent] {
			continue
		}
		parent := p.byRef[fk.Parent]
		refTypes, err := typesFor(parent, fk.ParentCols)
		if err != nil {
			return nil, err
		}
		// The referenced value space is the parent's, not the child's: a
		// widening edge (integer child, bigint parent) has to travel as the
		// parent's type or the join back into the parent would compare two
		// different things.
		for _, ch := range fresh.Chunks(chunkSize) {
			sql := mapKeysSQL(it.table, id.Columns, id.types, fk.ChildCols, refTypes, fk.ChildCols)
			refs, err := p.readKeys(ctx, sql, refTypes, chunkArgs(ch, len(id.types))...)
			if err != nil {
				return nil, fmt.Errorf("plan: reading %s from %s: %w", fk.Name, it.table, err)
			}
			if refs.Len() == 0 {
				continue
			}
			pk, err := p.identityKeys(ctx, fk.Parent, fk.ParentCols, refTypes, refs)
			if err != nil {
				return nil, err
			}
			pending := p.subtract(fk.Parent, pk)
			if pending.Len() == 0 {
				continue
			}
			p.noteWhy(fk.Parent, "parent of "+it.table.String()+" via "+it.table.String()+"."+joinCols(fk.ChildCols))
			out = append(out, item{table: fk.Parent, keys: pending, mode: pipeline.ParentOnly, depth: it.depth})
		}
	}
	return out, nil
}

// children is the child step: only from CHILD_OK rows, capped per parent key
// per edge, depth-limited.
func (p *run) children(ctx context.Context, it item, fresh *keys) ([]item, error) {
	var out []item
	for _, fk := range p.incoming[it.table] {
		if !followsAsChild(fk) || !p.inScope[fk.Child] || p.lookups[fk.Child] {
			continue
		}
		perKey := p.req.Cap
		if c, ok := p.req.TableCaps[fk.Child]; ok {
			perKey = c
		}
		parent := p.byRef[it.table]
		refTypes, err := typesFor(parent, fk.ParentCols)
		if err != nil {
			return nil, err
		}
		refKeys, err := p.referencedKeys(ctx, it.table, fk.ParentCols, refTypes, fresh)
		if err != nil {
			return nil, err
		}
		childID := p.ids[fk.Child]
		for _, ch := range refKeys.Chunks(chunkSize) {
			sql := childKeysSQL(fk.Child, fk.ChildCols, refTypes, childID.Columns, childID.types, perKey)
			ck, err := p.readKeys(ctx, sql, childID.types, chunkArgs(ch, len(refTypes))...)
			if err != nil {
				return nil, fmt.Errorf("plan: reading children of %s over %s: %w", it.table, fk.Name, err)
			}
			pending := p.subtract(fk.Child, ck)
			if pending.Len() == 0 {
				continue
			}
			p.capOf[fk.Child] = perKey
			p.noteWhy(fk.Child, "child of "+it.table.String()+" via "+fk.Child.String()+"."+joinCols(fk.ChildCols))
			out = append(out, item{table: fk.Child, keys: pending, mode: pipeline.ChildOK, depth: it.depth + 1})
		}
	}
	return out, nil
}

// referencedKeys turns a set of a table's identity keys into the values of the
// columns a foreign key references. It is the identity itself in every schema
// where a foreign key points at the primary key; when it points somewhere else,
// the two key spaces are translated with one read.
func (p *run) referencedKeys(ctx context.Context, table ref.TableRef, cols []string, types []keyType, from *keys) (*keys, error) {
	id := p.ids[table]
	if sameColumns(id.Columns, cols) {
		return from, nil
	}
	out := newKeys(types)
	for _, ch := range from.Chunks(chunkSize) {
		sql := mapKeysSQL(table, id.Columns, id.types, cols, types, nil)
		got, err := p.readKeys(ctx, sql, types, chunkArgs(ch, len(id.types))...)
		if err != nil {
			return nil, fmt.Errorf("plan: translating %s keys: %w", table, err)
		}
		got.forEach(out.add)
	}
	return out, nil
}

// identityKeys turns referenced values into the parent's own identity keys,
// which is what selected is keyed by.
func (p *run) identityKeys(ctx context.Context, parent ref.TableRef, cols []string, types []keyType, refs *keys) (*keys, error) {
	id := p.ids[parent]
	if sameColumns(id.Columns, cols) {
		return refs, nil
	}
	out := newKeys(id.types)
	for _, ch := range refs.Chunks(chunkSize) {
		sql := mapKeysSQL(parent, cols, types, id.Columns, id.types, nil)
		got, err := p.readKeys(ctx, sql, id.types, chunkArgs(ch, len(types))...)
		if err != nil {
			return nil, fmt.Errorf("plan: translating %s keys: %w", parent, err)
		}
		got.forEach(out.add)
	}
	return out, nil
}

// seedKeys reads the root's keys: --where when it is set, ordered by the
// identity, limited to --take.
func (p *run) seedKeys(ctx context.Context, root ref.TableRef) (*keys, error) {
	id := p.ids[root]
	sql := seedSQL(root, id.Columns, id.types, p.req.Where, p.req.Take)
	ks, err := p.readKeys(ctx, sql, id.types)
	if err != nil {
		return nil, fmt.Errorf("plan: reading the root's keys from %s: %w", root, err)
	}
	return ks, nil
}

// subtract returns the tuples of ks that the table has not already selected.
func (p *run) subtract(t ref.TableRef, ks *keys) *keys {
	have := p.selected[t]
	if have == nil {
		return ks
	}
	out := newKeys(ks.types)
	ks.forEach(func(tuple []keyValue) {
		if !have.has(tuple) {
			out.add(tuple)
		}
	})
	return out
}

// absorb adds a batch to the table's selected set. The row count is the growth
// of the set itself, because the set is what dedups: a tuple already there adds
// nothing to it and nothing to the budget.
func (p *run) absorb(t ref.TableRef, ks *keys) {
	have := p.selected[t]
	if have == nil {
		have = newKeys(ks.types)
		p.selected[t] = have
	}
	before := have.Len()
	ks.forEach(have.add)
	p.selectedRow += int64(have.Len() - before)
}

// noteWhy records the first reason a table entered the slice. The reason is
// built from identifiers, never from a value.
func (p *run) noteWhy(t ref.TableRef, why string) {
	if _, ok := p.why[t]; !ok {
		p.why[t] = why
	}
}

// checkBudgets is §3's checkBudgets: the row budget and the memory budget are
// checked during the walk, not after it, so a runaway slice stops at the table
// that caused it (THREAT_MODEL.md T11).
func (p *run) checkBudgets(t ref.TableRef) error {
	if p.selectedRow > p.req.RowBudget {
		return refuse(CodeRowBudget, exitBudget, t,
			fmt.Sprintf("%s takes the slice past the row budget of %d rows: raise --row-budget, "+
				"or lower --take, --cap or --depth", t, p.req.RowBudget),
			event.Args{
				event.ArgTable: t.String(),
				event.ArgCount: fmt.Sprint(p.req.RowBudget),
				event.ArgFlag:  "--row-budget",
			})
	}
	mem := p.keyMemory() + p.filterMemory()
	if mem > p.req.MemoryBudget {
		return refuse(CodeMemoryBudget, exitBudget, t,
			fmt.Sprintf("%s takes the estimated key and filter memory past the budget of %d bytes: "+
				"raise --memory-budget, or lower --take, --cap or --depth", t, p.req.MemoryBudget),
			event.Args{
				event.ArgTable: t.String(),
				event.ArgCount: fmt.Sprint(p.req.MemoryBudget),
				event.ArgFlag:  "--memory-budget",
			})
	}
	return nil
}

// keyMemory is Σ KeySet.Bytes() over every table with keys. KeySet.Bytes is the
// single source of truth for it (§2); nothing here computes a second estimate.
func (p *run) keyMemory() int64 {
	var total int64
	for _, t := range p.tables {
		if ks := p.selected[t.Ref]; ks != nil {
			total += ks.set.Bytes()
		}
	}
	return total
}

// filterMemory is the residual Bloom filter at 29 bits per masked cell.
func (p *run) filterMemory() int64 {
	var cells int64
	for _, t := range p.tables {
		ks := p.selected[t.Ref]
		if ks == nil {
			continue
		}
		cells += int64(ks.Len()) * int64(p.cellsPerRow[t.Ref])
	}
	return cells * residualBitsPerCell / 8
}

// assemble builds the Plan: one step per table, in load order, with the
// estimate §3.5 prints.
func (p *run) assemble(root ref.TableRef, rootReason string) *pipeline.Plan {
	steps := make(map[ref.TableRef]pipeline.Step, len(p.tables))
	var rows, estBytes int64
	for _, t := range p.tables {
		switch {
		case p.lookups[t.Ref]:
			steps[t.Ref] = pipeline.Step{Table: t.Ref, Mode: pipeline.Lookup, Why: "lookup"}
			rows += p.lookupRows[t.Ref]
			estBytes += p.lookupRows[t.Ref] * rowWidth(t)
		case p.selected[t.Ref] != nil:
			ks := p.selected[t.Ref]
			steps[t.Ref] = pipeline.Step{
				Table:    t.Ref,
				Mode:     p.label[t.Ref],
				Identity: p.ids[t.Ref].Identity,
				Keys:     ks.set,
				Cap:      p.capOf[t.Ref],
				Depth:    p.depth[t.Ref],
				Why:      p.why[t.Ref],
			}
			rows += int64(ks.Len())
			estBytes += int64(ks.Len()) * rowWidth(t)
		default:
			why := p.why[t.Ref]
			if why == "" {
				why = "unreachable"
			}
			steps[t.Ref] = pipeline.Step{Table: t.Ref, Mode: pipeline.SchemaOnly, Why: why}
		}
	}

	refs := make([]ref.TableRef, 0, len(p.tables))
	for _, t := range p.tables {
		refs = append(refs, t.Ref)
	}
	order, components := loadOrder(refs, p.fks)
	ordered := make([]pipeline.Step, 0, len(order))
	for _, t := range order {
		ordered = append(ordered, steps[t])
	}

	var unindexed []pipeline.ForeignKey
	for _, fk := range p.fks {
		if !fk.Virtual && !fk.Indexed {
			unindexed = append(unindexed, fk)
		}
	}
	var unmapped []string
	for _, pair := range polymorphicPairs(p.tables, p.outgoing) {
		unmapped = append(unmapped, pair.String())
	}

	return &pipeline.Plan{
		Root:       root,
		RootReason: rootReason,
		Take:       p.req.Take,
		Steps:      ordered,
		SCCs:       components,
		Unmapped:   unmapped,
		Unindexed:  unindexed,
		Skipped:    p.skipped,
		Unreadable: p.unreadable,
		Estimate: pipeline.Estimate{
			Rows:         rows,
			Bytes:        estBytes,
			KeyMemory:    p.keyMemory(),
			FilterMemory: p.filterMemory(),
			HoldSeconds:  time.Since(p.started).Seconds() + float64(rows)/float64(pipeline.AssumedRowsPerSec),
		},
	}
}

// sortedRefs copies a table list into (schema, name) order, because a request's
// own order is the order flags were typed in.
func sortedRefs(in []ref.TableRef) []ref.TableRef {
	out := append([]ref.TableRef(nil), in...)
	sort.Slice(out, func(a, b int) bool { return tableRefLess(out[a], out[b]) })
	return out
}

// chunkArgs is one chunk as query arguments: one typed array per identity
// column, never a []any, because pgx cannot infer an array OID for that.
func chunkArgs(ch pipeline.Chunk, n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = ch.Column(i)
	}
	return args
}
