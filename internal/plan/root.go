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

// RootCandidate is one table's score for §3.1's default root, and for ADR-008
// Q2's "?" listing and its own answer.
//
// It is exported, with RankRoots below, so that Q2 (internal/core) and
// defaultRoot read the identical ranking rather than two copies of §3.1 that
// could drift apart — the question and the planner must not be able to
// disagree about which table is top.
type RootCandidate struct {
	Table     ref.TableRef
	Inbound   int
	Outbound  int
	Rows      int64
	Preferred bool
}

// Score is §3.1's score(t) = inbound(t) - outbound(t).
func (c RootCandidate) Score() int { return c.Inbound - c.Outbound }

// Reason is the score-components line printed beside a candidate: what
// defaultRoot's own reason string is built from for the top pick, and what
// Q2's "?" prints for each of the ranked top five (ADR-008 §3.1, §7).
func (c RootCandidate) Reason() string {
	return strconv.Itoa(c.Inbound) + " inbound - " + strconv.Itoa(c.Outbound) + " outbound FKs"
}

// CollapseForRanking returns the tables and foreign keys build() plans over:
// every partition leaf (Table.Parent != nil) dropped, and every foreign key
// whose child or parent is not one of the tables kept.
//
// It exists so that Q2 (internal/core's rootQuestion) and the planner's own
// chooseRoot rank — and validate a typed or flagged answer against —
// identical input. Exporting RankRoots alone did not do that: ranking the
// *raw* introspected schema can put a partition leaf at the top (it carries
// the table's own row count and FK count, and build() attributes both to the
// root instead), which is a table chooseRoot's own p.inScope then refuses by
// name. build() itself is unchanged; this is the filtering half of it,
// factored out so both callers share one statement of the rule rather than
// two that could drift (a review of T-0271 found exactly that drift).
func CollapseForRanking(tables []pipeline.Table, fks []pipeline.ForeignKey) ([]pipeline.Table, []pipeline.ForeignKey) {
	kept := make([]pipeline.Table, 0, len(tables))
	inScope := make(map[ref.TableRef]bool, len(tables))
	for _, t := range tables {
		if t.Parent != nil {
			continue
		}
		inScope[t.Ref] = true
		kept = append(kept, t)
	}
	var keptFKs []pipeline.ForeignKey
	for _, fk := range fks {
		if !inScope[fk.Child] || !inScope[fk.Parent] {
			continue
		}
		keptFKs = append(keptFKs, fk)
	}
	return kept, keptFKs
}

// RankRoots ranks every table by §3.1's rule: lookup-shaped tables discarded
// (unless the whole schema is lookup-shaped, in which case every table is
// kept rather than naming no root at all), highest score first, ties broken
// by a name a person would call their customer table, then by row count
// descending, then by name. The result is never empty unless tables is.
func RankRoots(tables []pipeline.Table, fks []pipeline.ForeignKey) []RootCandidate {
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

	var all, kept []RootCandidate
	for _, t := range tables {
		c := RootCandidate{
			Table:     t.Ref,
			Inbound:   inbound[t.Ref],
			Outbound:  outbound[t.Ref],
			Rows:      t.ApproxRows,
			Preferred: preferredRootNames[t.Ref.Name],
		}
		all = append(all, c)
		if lookupShaped(t, len(inboundFrom[t.Ref])) {
			continue
		}
		kept = append(kept, c)
	}
	// Every table being lookup-shaped is a schema of nothing but lookups; the
	// best of them still beats refusing to name a root at all.
	if len(kept) == 0 {
		kept = all
	}

	sort.SliceStable(kept, func(a, b int) bool {
		x, y := kept[a], kept[b]
		if x.Score() != y.Score() {
			return x.Score() > y.Score()
		}
		if x.Preferred != y.Preferred {
			return x.Preferred
		}
		if x.Rows != y.Rows {
			return x.Rows > y.Rows
		}
		return tableRefLess(x.Table, y.Table)
	})
	return kept
}

// defaultRoot picks the root when --root is absent. It returns the table and
// the reason line §3.1 prints beside it.
func defaultRoot(tables []pipeline.Table, fks []pipeline.ForeignKey) (ref.TableRef, string, bool) {
	kept := RankRoots(tables, fks)
	if len(kept) == 0 {
		return ref.TableRef{}, "", false
	}
	best := kept[0]
	reason := best.Reason()
	if best.Preferred && len(kept) > 1 && kept[1].Score() == best.Score() {
		reason += "; name preference"
	}
	return best.Table, reason, true
}

// lookupShaped is §3.1's discard rule: small and referenced from at most two
// tables, or named like a lookup.
func lookupShaped(t pipeline.Table, inboundTables int) bool {
	if t.ApproxRows >= 0 && t.ApproxRows < 1000 && inboundTables <= 2 {
		return true
	}
	return lookupNamePattern.MatchString(t.Ref.Name) || lookupNames[t.Ref.Name]
}
