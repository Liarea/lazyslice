// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The root default of ARCHITECTURE.md §3.1: score(t) = inbound − outbound,
// lookup-shaped tables discarded, ties broken by a name a person would call
// their customer table and then by row count.

// lookupNamePattern is §3.1's name test for a lookup-shaped table.
var lookupNamePattern = regexp.MustCompile(`_(types?|statuses|kinds|codes)$`)

// lookupNames are the table names §3.1 names outright. The migration
// bookkeeping names are ADR-005's list, which is the same set the target gate
// exempts, so "..." in §3.1 is read as that list rather than as an open
// invitation.
var lookupNames = map[string]bool{
	"countries": true, "currencies": true,
	"migrations": true, "schema_migrations": true, "_prisma_migrations": true,
	"alembic_version": true, "__diesel_schema_migrations": true,
	"flyway_schema_history": true, "goose_db_version": true,
	"atlas_schema_revisions": true, "knex_migrations": true,
	"django_migrations": true,
}

// preferredRootNames is §3.1's tie-break: the name a person would give the
// table their data hangs off.
var preferredRootNames = map[string]bool{
	"customers": true, "users": true, "accounts": true, "organizations": true,
	"organisations": true, "tenants": true, "companies": true, "clients": true,
}

// rootScore is one candidate and the components of its score, which is what the
// reason line prints.
type rootScore struct {
	table     ref.TableRef
	inbound   int
	outbound  int
	rows      int64
	preferred bool
}

func (s rootScore) score() int { return s.inbound - s.outbound }

// defaultRoot picks the root when --root is absent. It returns the table and
// the reason line §3.1 prints beside it.
func defaultRoot(tables []pipeline.Table, fks []pipeline.ForeignKey) (ref.TableRef, string, bool) {
	inbound := map[ref.TableRef]int{}
	outbound := map[ref.TableRef]int{}
	inboundFrom := map[ref.TableRef]map[ref.TableRef]bool{}
	for _, fk := range fks {
		inbound[fk.Parent]++
		outbound[fk.Child]++
		if inboundFrom[fk.Parent] == nil {
			inboundFrom[fk.Parent] = map[ref.TableRef]bool{}
		}
		inboundFrom[fk.Parent][fk.Child] = true
	}

	var all, kept []rootScore
	for _, t := range tables {
		s := rootScore{
			table:     t.Ref,
			inbound:   inbound[t.Ref],
			outbound:  outbound[t.Ref],
			rows:      t.ApproxRows,
			preferred: preferredRootNames[t.Ref.Name],
		}
		all = append(all, s)
		if lookupShaped(t, len(inboundFrom[t.Ref])) {
			continue
		}
		kept = append(kept, s)
	}
	// Every table being lookup-shaped is a schema of nothing but lookups; the
	// best of them still beats refusing to name a root at all.
	if len(kept) == 0 {
		kept = all
	}
	if len(kept) == 0 {
		return ref.TableRef{}, "", false
	}

	sort.SliceStable(kept, func(a, b int) bool {
		x, y := kept[a], kept[b]
		if x.score() != y.score() {
			return x.score() > y.score()
		}
		if x.preferred != y.preferred {
			return x.preferred
		}
		if x.rows != y.rows {
			return x.rows > y.rows
		}
		return tableRefLess(x.table, y.table)
	})

	best := kept[0]
	reason := strconv.Itoa(best.inbound) + " inbound - " + strconv.Itoa(best.outbound) + " outbound FKs"
	if best.preferred && len(kept) > 1 && kept[1].score() == best.score() {
		reason += "; name preference"
	}
	return best.table, reason, true
}

// lookupShaped is §3.1's discard rule: small and referenced from at most two
// tables, or named like a lookup.
func lookupShaped(t pipeline.Table, inboundTables int) bool {
	if t.ApproxRows >= 0 && t.ApproxRows < 1000 && inboundTables <= 2 {
		return true
	}
	return lookupNamePattern.MatchString(t.Ref.Name) || lookupNames[t.Ref.Name]
}
