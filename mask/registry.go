// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// FixedPrefix introduces the one parametric masker id: "fixed:LITERAL" writes
// that literal into every cell. It is how a credential becomes
// $lazyslice$invalid and how a log-shaped JSON document becomes {} without a
// registry entry per literal.
const FixedPrefix = "fixed:"

type entry struct {
	cat Category
	m   Masker
}

// The registry is populated by init in this package and, in a build that adds
// one, by a call to Register. Nothing is loaded at runtime: no .so, no
// subprocess, no expression language, no file path accepted as a masker
// (ADR-006). The mutex is for a Register from another package's init, not for
// a runtime edit; there is no Unregister.
var (
	mu         sync.RWMutex
	registry   = map[ID]entry{}
	categories = map[Category][]ID{}
)

// Register adds a generator under an id and a category. It panics on a
// duplicate id, because two maskers under one name is a build that cannot say
// what a lazyslice.yml means. Registration order within a category matters:
// the first is the category's default, and the rest are the alternates Pick
// considers for a unique column.
func Register(id ID, cat Category, m Masker) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[id]; dup {
		panic("mask: duplicate masker id " + string(id))
	}
	registry[id] = entry{cat: cat, m: m}
	categories[cat] = append(categories[cat], id)
}

// Get returns the masker registered under id. It understands the "fixed:"
// prefix, whose literal is part of the id.
func Get(id ID) (Masker, bool) {
	if lit, ok := strings.CutPrefix(string(id), FixedPrefix); ok {
		return fixedMasker{literal: lit}, true
	}
	mu.RLock()
	defer mu.RUnlock()
	e, ok := registry[id]
	return e.m, ok
}

// IDs lists every registered masker, ordered, for the test that walks the rule
// pack against the registry (ADR-006: a category with no shipped masker cannot
// exist).
func IDs() []ID {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]ID, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Candidates lists the maskers registered for a category, the default first.
func Candidates(cat Category) []ID {
	mu.RLock()
	defer mu.RUnlock()
	return append([]ID(nil), categories[cat]...)
}

// Admissible is d for one generator on one column: the smaller of the column's
// own domain and the generator's (ARCHITECTURE.md section 5).
func Admissible(id ID, c Constraints) int64 {
	m, ok := Get(id)
	if !ok {
		return 0
	}
	return min(ColumnDomain(c), m.Domain(c))
}

// Pick applies the domain rule of ARCHITECTURE.md section 5 to one column.
//
// A column not under a unique index gets its category's default generator; the
// only thing that can refuse it is a column too small to hold any masked value
// at all. A column under a unique index gets the registered generator for its
// category with the largest admissible domain, and is refused with a
// *DomainError when even that one cannot emit d_required = n²/2ε distinct
// values at ε = 10⁻⁶. There is no retry counter and no fallback to a smaller
// generator: the refusal happens at plan, before any row moves.
func Pick(cat Category, c Constraints) (ID, error) {
	cands := Candidates(cat)
	if len(cands) == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoCategory, cat)
	}
	if !c.Unique {
		id := cands[0]
		if Admissible(id, c) <= 0 {
			return "", &NoRoomError{Category: cat, ID: id, TypeTag: c.TypeTag, MaxLen: c.MaxLen}
		}
		return id, nil
	}
	best, bestD := cands[0], Admissible(cands[0], c)
	for _, id := range cands[1:] {
		if d := Admissible(id, c); d > bestD {
			best, bestD = id, d
		}
	}
	if bestD <= 0 {
		return "", &NoRoomError{Category: cat, ID: best, TypeTag: c.TypeTag, MaxLen: c.MaxLen}
	}
	// n is the whole of d_required, so a unique column with no row count is a
	// refusal and not a free pass: Required(0) is 0, and comparing against it
	// would accept any generator for any unique column, which is the collision
	// at load that section 5 moves to plan time.
	if c.Rows <= 0 {
		return "", fmt.Errorf("%w: category %s, masker %s", ErrRowCountUnknown, cat, best)
	}
	if req := Required(c.Rows); bestD < req {
		return "", &DomainError{
			Category: cat, ID: best, Domain: bestD,
			Required: req, Rows: c.Rows, MaxRows: MaxRows(bestD),
		}
	}
	return best, nil
}

// Small reports ARCHITECTURE.md section 5's other domain rule, the one that
// runs on every masked column and not only the unique ones: an admissible
// domain below twice the number of distinct sampled values makes the masking a
// stable substitution over a small alphabet, which frequency against public
// prevalence data recovers. The run lists every such column under
// small_domain: in the yml and under "what the green tick does not prove"; it
// does not refuse.
//
// With no samples it falls back to smallDomain's absolute ceiling rather than
// reporting nothing: a column whose domain is three labels is a small domain
// whether or not anybody counted its values, and a silent one is a column the
// report calls masked and says nothing else about.
func Small(id ID, c Constraints) bool {
	return smallDomain(Admissible(id, c), c)
}
