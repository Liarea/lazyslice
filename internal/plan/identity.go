// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The row-identity ladder of ARCHITECTURE.md §3.4:
//
//	--key (or the yml's keys: block) -> primary key -> non-partial,
//	non-expression unique index over NOT NULL columns -> pseudo-key from the
//	table's NOT NULL foreign-key columns plus its NOT NULL discriminators,
//	probed for uniqueness over a TABLESAMPLE -> refuse with exit 12.
//
// There is no ctid rung and there is no guess: a guessed identity produces a
// slice whose rows are silently the wrong ones (testdata/README.md trap 12), so
// the ladder ends in a refusal naming the table and the two flags that clear it.

// systemColumns are the names the ladder refuses outright. ctid is the one that
// matters: ADR-005 cuts it because it is not reproducible across VACUUM FULL,
// which breaks invariants I3 and I5, and a --key naming it has to land
// somewhere loud rather than quietly becoming an identity.
var systemColumns = map[string]bool{
	"ctid": true, "xmin": true, "xmax": true, "cmin": true, "cmax": true,
	"tableoid": true, "oid": true,
}

// identity is a resolved row identity together with the key encoding its
// columns travel under.
type identity struct {
	pipeline.Identity
	types []keyType
}

// resolveIdentity walks the ladder for one table.
func (p *run) resolveIdentity(ctx context.Context, tbl *pipeline.Table) (identity, error) {
	if cols, ok := p.req.Keys[tbl.Ref]; ok && len(cols) > 0 {
		return p.explicitIdentity(ctx, tbl, cols)
	}
	if len(tbl.PK) > 0 {
		id, err := p.identityOver(tbl, tbl.PK, pipeline.IdentityPK)
		if err == nil {
			return id, nil
		}
	}
	if cols, ok := uniqueIndexColumns(tbl); ok {
		id, err := p.identityOver(tbl, cols, pipeline.IdentityUnique)
		if err == nil {
			return id, nil
		}
	}
	if cols := p.pseudoKeyColumns(tbl); len(cols) > 0 {
		ok, err := p.probePseudoKey(ctx, tbl, cols)
		if err != nil {
			return identity{}, err
		}
		if ok {
			return p.identityOver(tbl, cols, pipeline.IdentityPseudo)
		}
	}
	return identity{}, refuse(CodeNoIdentity, exitPlan, tbl.Ref,
		fmt.Sprintf("%s has no row identity: pass --key %s=col,col or --skip-table %s",
			tbl.Ref, tbl.Ref, tbl.Ref),
		event.Args{
			event.ArgTable: tbl.Ref.String(),
			event.ArgFlag:  "--key " + tbl.Ref.String() + "=col,col or --skip-table " + tbl.Ref.String(),
		})
}

// explicitIdentity takes the first rung: a key the caller named. It is a
// candidate like any other and is probed for uniqueness before it is trusted
// (T-0032), because the flag's whole purpose is to identify a row.
func (p *run) explicitIdentity(ctx context.Context, tbl *pipeline.Table, cols []string) (identity, error) {
	byName := columnsByName(tbl)
	for _, c := range cols {
		if systemColumns[c] {
			return identity{}, refuse(CodeKeyColumn, exitUsage, tbl.Ref,
				fmt.Sprintf("--key %s=%s names the system column %q: there is no ctid rung on the identity ladder",
					tbl.Ref, joinCols(cols), c),
				event.Args{
					event.ArgTable:  tbl.Ref.String(),
					event.ArgColumn: c,
					event.ArgFlag:   "--key",
				})
		}
		if _, ok := byName[c]; !ok {
			return identity{}, refuse(CodeKeyColumn, exitUsage, tbl.Ref,
				fmt.Sprintf("--key %s=%s names %q, which %s does not have", tbl.Ref, joinCols(cols), c, tbl.Ref),
				event.Args{
					event.ArgTable:  tbl.Ref.String(),
					event.ArgColumn: c,
					event.ArgFlag:   "--key",
				})
		}
	}
	dup, sampled, err := p.probeExplicitKey(ctx, tbl, cols)
	if err != nil {
		return identity{}, err
	}
	if dup {
		return identity{}, refuse(CodeKeyNotUnique, exitPlan, tbl.Ref,
			fmt.Sprintf("--key %s=%s does not identify a row: at least one key appears more than once",
				tbl.Ref, joinCols(cols)),
			event.Args{
				event.ArgTable: tbl.Ref.String(),
				event.ArgFlag:  "--key " + tbl.Ref.String() + "=" + joinCols(cols),
			})
	}
	// An explicit key that survived a whole-table probe is a unique key,
	// whatever the catalog calls it. One that survived a bounded probe is
	// IdentityPseudo, which is what that rung means: only ever probed on a
	// sample. The probe is bounded on a table too large to aggregate inside the
	// holder transaction (THREAT_MODEL.md T9), so the plan says which of the two
	// happened rather than claiming the stronger one.
	kind := pipeline.IdentityUnique
	if sampled {
		kind = pipeline.IdentityPseudo
	}
	return p.identityOver(tbl, cols, kind)
}

// identityOver builds the identity for a set of columns.
func (p *run) identityOver(tbl *pipeline.Table, cols []string, kind pipeline.IdentityKind) (identity, error) {
	byName := columnsByName(tbl)
	types := make([]keyType, len(cols))
	oids := make([]uint32, len(cols))
	for i, c := range cols {
		col, ok := byName[c]
		if !ok {
			return identity{}, fmt.Errorf("plan: %s has no column %q for its row identity", tbl.Ref, c)
		}
		types[i] = typeOf(col)
		oids[i] = col.TypeOID
	}
	return identity{
		Identity: pipeline.Identity{Kind: kind, Columns: append([]string(nil), cols...), Types: oids},
		types:    types,
	}, nil
}

// uniqueIndexColumns is the third rung: a unique index that is not partial, not
// an expression index, and whose columns are all NOT NULL. A nullable column
// under a unique index does not identify a row, because Postgres allows any
// number of NULLs in one.
//
// Ties are broken by the fewest columns and then the index name, so that two
// runs over one schema pick the same index.
func uniqueIndexColumns(tbl *pipeline.Table) ([]string, bool) {
	byName := columnsByName(tbl)
	var candidates [][]string
	var names []string
	for _, idx := range tbl.Indexes {
		if !idx.Unique || idx.Partial || idx.Expression || len(idx.Columns) == 0 {
			continue
		}
		usable := true
		for _, c := range idx.Columns {
			col, ok := byName[c]
			if !ok || col.Nullable {
				usable = false
				break
			}
		}
		if usable {
			candidates = append(candidates, idx.Columns)
			names = append(names, idx.Name)
		}
	}
	if len(candidates) == 0 {
		return nil, false
	}
	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := order[a], order[b]
		if len(candidates[x]) != len(candidates[y]) {
			return len(candidates[x]) < len(candidates[y])
		}
		return names[x] < names[y]
	})
	return candidates[order[0]], true
}

// pseudoKeyColumns is the fourth rung's candidate, and it is exactly what §3.4
// names: "the table's FK columns plus NOT NULL discriminators", in table column
// order. Generated columns are left out — the target recomputes them, so they
// are never copied and never an identity.
//
// The candidate is narrow on purpose, and both halves of that matter.
//
// A nullable column is never a candidate, foreign key or not. The probe cannot
// catch one: count(DISTINCT (a, b)) counts a tuple with a NULL in it as
// distinct, so (NULL, 1) and (NULL, 2) pass a uniqueness probe that (3, 4) and
// (3, 4) would fail (verified on postgres:16). readKeys then drops every tuple
// with a NULL in a key column, so the rows would leave the slice with no
// refusal, no warning and no line in the plan — the silent omission of
// testdata/README.md trap 12 without the exit 12. The rung fails closed
// instead.
//
// Nor is every other NOT NULL column a candidate. Widening it to the whole row
// makes exit 12 nearly unreachable — any table without two byte-identical rows
// acquires an "identity" — and applies §3.4's sampled-uniqueness risk, which it
// accepts for a narrow candidate, to almost every table without a primary key.
// It also admits columns the key encoding cannot compare at all: a NOT NULL
// json, xml or geometric column has no default btree opclass, so the probe's
// count(DISTINCT (a, b, c)) and childKeysSQL's ORDER BY die inside pgx with
// "could not identify a comparison function for type json" rather than at
// §3.4's refusal. Every column admitted here is comparable by construction —
// the two admission rules below each carry their own reason — so the rung ends
// at CodeNoIdentity naming --key and --skip-table, which is the one ending §3.4
// gives it.
func (p *run) pseudoKeyColumns(tbl *pipeline.Table) []string {
	fkCols := map[string]bool{}
	for _, fk := range p.outgoing[tbl.Ref] {
		for _, c := range fk.ChildCols {
			fkCols[c] = true
		}
	}
	enums := enumTypeNames(p.schema)
	var cols []string
	for _, col := range tbl.Columns {
		if col.Generated != "" || col.Nullable {
			continue
		}
		// A foreign-key column is comparable because the edge exists: its
		// referenced side carries a unique index, which in PostgreSQL is a
		// btree, and the two ends must share an equality operator in a btree
		// operator family before the constraint can be created at all.
		if fkCols[col.Name] || isDiscriminator(col, enums) {
			cols = append(cols, col.Name)
		}
	}
	return cols
}

// oidBool is the boolean type's catalog OID. It is not one of keyset.go's key
// kinds — a boolean key travels as text like any other kindOther column — so it
// is named here, where the question is "is this a discriminator", not "how does
// this travel".
const oidBool uint32 = 16

// discriminatorNamePattern is the name shape §3.4 calls a discriminator: the
// `_type` half of §3.2's polymorphic pair, and the type/status/kind/code family
// §3.1 already recognises by name, either as the whole column name or as its
// last underscore-separated word.
var discriminatorNamePattern = regexp.MustCompile(`(^|_)(types?|status|statuses|kinds?|states?|codes?)$`)

// isDiscriminator says whether a NOT NULL column is one of §3.4's
// discriminators: a column whose declared domain is a small fixed set
// (boolean, an enum), or one named like the type/status/kind/code family.
//
// A name is not a licence to key on anything: a name-matched column is admitted
// only when the key encoding already knows how to carry it (keyset.go's int,
// text, character(n) and uuid kinds). That is what keeps a `payload_type json`
// column out of a candidate the probe would then die on, and it is why this
// rung needs no deny-list of uncomparable types — boolean, enum and those four
// kinds all have a default btree opclass, and nothing else is admitted.
func isDiscriminator(col pipeline.Column, enums map[string]bool) bool {
	if col.TypeOID == oidBool || enums[col.TypeName] {
		return true
	}
	if !discriminatorNamePattern.MatchString(col.Name) {
		return false
	}
	switch typeOf(col).kind {
	case kindInt, kindText, kindBpchar, kindUUID:
		return true
	default:
		return false
	}
}

// enumTypeNames is the schema's enum type names, both as the catalog spells
// them (schema-qualified) and bare, because pg_catalog.format_type — which is
// where Column.TypeName comes from — qualifies a name only when the type is not
// visible in the search path.
func enumTypeNames(schema *pipeline.Schema) map[string]bool {
	out := make(map[string]bool, 2*len(schema.Enums))
	for name := range schema.Enums {
		out[name] = true
		if i := strings.LastIndex(name, "."); i >= 0 {
			out[name[i+1:]] = true
		}
	}
	return out
}

// probeExplicitKey says whether the named columns hold a duplicate, and whether
// the probe that answered was a bounded one. A table nothing has analysed
// (reltuples -1) is bounded like a large one: an unknown row count is not a
// licence to aggregate the whole table inside the holder transaction.
func (p *run) probeExplicitKey(ctx context.Context, tbl *pipeline.Table, cols []string) (dup, sampled bool, err error) {
	limit := 0
	if tbl.ApproxRows < 0 || tbl.ApproxRows > explicitKeyProbeRows {
		limit = explicitKeyProbeRows
	}
	var duplicates int64
	if err := p.scalar(ctx, explicitKeyProbeSQL(tbl.Ref, cols, limit), &duplicates); err != nil {
		return false, false, fmt.Errorf("plan: probing --key on %s: %w", tbl.Ref, err)
	}
	return duplicates > 0, limit > 0, nil
}

// probePseudoKey says whether the candidate identified every row of a sample.
// An empty sample is not a pass: a candidate nothing was read for has not been
// probed, and §3.4 trusts a pseudo-key only after the probe.
func (p *run) probePseudoKey(ctx context.Context, tbl *pipeline.Table, cols []string) (bool, error) {
	// TABLESAMPLE is not accepted on a partitioned table, and a fraction of an
	// unknown row count is a full scan rather than a probe, so both take the
	// bounded prefix.
	bounded := tbl.Partitioned || tbl.ApproxRows <= 0
	num, den := samplePercent(tbl.ApproxRows)
	var rows, distinct int64
	if err := p.scalar(ctx, pseudoKeyProbeSQL(tbl.Ref, cols, bounded, num, den), &rows, &distinct); err != nil {
		return false, fmt.Errorf("plan: probing a pseudo-key on %s: %w", tbl.Ref, err)
	}
	return rows > 0 && rows == distinct, nil
}

// columnsByName indexes a table's columns.
func columnsByName(tbl *pipeline.Table) map[string]pipeline.Column {
	out := make(map[string]pipeline.Column, len(tbl.Columns))
	for _, c := range tbl.Columns {
		out[c.Name] = c
	}
	return out
}

// joinCols renders a column list for a message. It carries identifiers only.
func joinCols(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ","
		}
		out += c
	}
	return out
}

// typesFor is the key encoding of an arbitrary column list of a table, used for
// the referenced side of a foreign key when it is not the parent's identity.
func typesFor(tbl *pipeline.Table, cols []string) ([]keyType, error) {
	byName := columnsByName(tbl)
	types := make([]keyType, len(cols))
	for i, c := range cols {
		col, ok := byName[c]
		if !ok {
			return nil, fmt.Errorf("plan: %s has no column %q", tbl.Ref, c)
		}
		types[i] = typeOf(col)
	}
	return types, nil
}

// sameColumns says whether two column lists are equal in order.
func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// tableRefLess orders tables by (schema, name), which is the order every
// iteration in this package uses.
func tableRefLess(a, b ref.TableRef) bool { return a.Less(b) }
