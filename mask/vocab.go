// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"crypto/sha256"
	"sort"
	"strings"
)

// The vocabulary gate of ADR-015 (proposed, 2026-09-22).
//
// A residual hit is a masked value in the target that equals a value of the
// same column in the source. For a masker that draws from a closed list of
// real words — a name list — such a hit is expected on a correct run: a masked
// "Mary" is some other customer's real "Mary". The residual scan
// (internal/verify, ARCHITECTURE.md section 6 item 3) can explain that hit
// instead of refusing it only when three things hold, and this file is the
// first: the value is one the masker could have produced for this column.
// The other two — transform emitted it at least as often as the target holds
// it, and no row kept its own — are internal/transform's and internal/verify's.
//
// The answer comes from an unexported method, so only a masker inside this
// package can give it: a registered custom masker, however it is built, cannot
// claim a vocabulary, and a hit in its column keeps the column probe
// (mask/CLAUDE.md, "The vocabulary gate").
//
// **The fold-faithful criterion.** A masker may implement vocabulary only if,
// for every value o it can emit and every input x, canonical equality of o and
// x implies letters-and-digits fold equality of o with x's canonical form
// (looksLikeItsInput's comparison). That is what makes maskCell's redraw —
// which fires on the fold — cover every value the residual scan would call
// equal under the canonical form, so that a masked value never equals its own
// source value under either comparison. person_name qualifies: its words are
// lower-case ASCII letters, and fold (NFKC, case fold, collapsed whitespace)
// changes nothing but case and spacing over them. phone, date, national
// identifiers and email do not: their canonical forms parse, reorder or drop
// characters, so two values can be canonical-equal and fold-different.
// TestVocabularyIsFoldFaithful walks every entry.

// vocabularyMasker is a Masker that can say whether a value is one it could
// have produced under a column's constraints.
type vocabularyMasker interface {
	Masker
	// vocabulary reports whether v, a value as the target holds it, is one
	// this masker could have emitted under c. It tests membership after the
	// category's canonical fold. It is never asked about a closed column
	// (Emits answers false there before it asks).
	vocabulary(v Value, c Constraints) bool
}

// Emitting reports whether Emits can answer true for any value under c: the
// masker registered under id implements the vocabulary method and the column
// is not closed by an enum or a CHECK value list. It is the value-free half of
// Emits, for a caller that decides once per column — internal/transform, which
// counts what an emitting column emitted, and internal/verify's statement
// allowlist, which is built before any value is read.
//
// It is false for an unknown id, a "fixed:" id, a masker without the method —
// every custom masker registered from outside this package — and a column
// whose constraints carry labels.
func Emitting(id ID, c Constraints) bool {
	_, ok := vocabularyOf(id, c)
	return ok
}

// Emits reports whether v could have been produced by the masker registered
// under id, for a column with constraints c. It is ADR-015's vocabulary gate:
// internal/verify explains a residual hit only when this is true, and a hit
// it rejects keeps the column probe.
//
// It is false wherever Emitting is, and for a NULL, an empty or a bytea value,
// none of which a vocabulary masker emits.
func Emits(id ID, v Value, c Constraints) bool {
	vm, ok := vocabularyOf(id, c)
	if !ok {
		return false
	}
	if v.Null || v.Empty() || len(v.Bytes) > 0 {
		return false
	}
	return vm.vocabulary(v, c)
}

// FoldEqual reports whether a and b carry the same letters and digits in the
// same order, ignoring case and everything that is neither. It is the
// comparison maskCell's post-condition and redraw fire on (looksLikeItsInput),
// exported so that a caller checking "did a row keep its own value" asks the
// question the redraw guarantees against, and not a second spelling of it.
func FoldEqual(a, b string) bool { return foldEqual([]byte(a), []byte(b)) }

func vocabularyOf(id ID, c Constraints) (vocabularyMasker, bool) {
	if strings.HasPrefix(string(id), FixedPrefix) {
		return nil, false
	}
	if len(labels(c)) > 0 {
		// A closed column emits one of its own labels, whatever the category
		// (labelValue), and a label is not a vocabulary word.
		return nil, false
	}
	m, ok := Get(id)
	if !ok {
		return nil, false
	}
	vm, ok := m.(vocabularyMasker)
	return vm, ok
}

// redraws is how many times maskCell redraws a vocabulary masker whose output
// reads as its input (ADR-015 decision 3).
const redraws = 8

// redrawInfo separates a redraw's digest from every other use of h.
const redrawInfo = "lazyslice/redraw"

// redrawDigest is h_i = SHA-256(h || "lazyslice/redraw" || i), i one byte. It
// is a function of h alone, so a redrawn value is still a pure function of the
// key, the category, the canonical value and the constraints: determinism,
// foreign-key equality groups and byte-identical reruns (invariant I3) hold.
func redrawDigest(h [32]byte, i byte) [32]byte {
	buf := make([]byte, 0, len(h)+len(redrawInfo)+1)
	buf = append(buf, h[:]...)
	buf = append(buf, redrawInfo...)
	buf = append(buf, i)
	return sha256.Sum256(buf)
}

// holds reports whether s is a word of the list that fits budget bytes. The
// list is ordered by length and then bytewise (newWordList), so this is a
// binary search and allocates nothing.
func (w *wordList) holds(s string, budget int) bool {
	if budget > 0 && len(s) > budget {
		return false
	}
	i := sort.Search(len(w.words), func(i int) bool {
		x := w.words[i]
		if len(x) != len(s) {
			return len(x) > len(s)
		}
		return x >= s
	})
	return i < len(w.words) && w.words[i] == s
}

// vocabulary is personNameMasker's answer, over exactly the lists Mask draws
// from: roleGivenNames for RoleGiven, roleFamilyNames for RoleFamily, and for
// RoleFull a "given surname" pair off givenNames and surnames or a single
// given name — both of RoleFull's forms, whichever the column's width would
// have chosen, because the count check behind this gate (internal/verify)
// refuses a value transform never emitted whatever this answers.
//
// The test is on the canonical form (Canonical's fold), so "MARY", "mary" and
// " Mary " are all the word "mary". A full name in a given-name column, a
// given name in a family-name column and a word off the shared lists in a role
// column are all outside it.
func (personNameMasker) vocabulary(v Value, c Constraints) bool {
	s := fold(v.Text)
	if s == "" {
		return false
	}
	budget := room(c)
	switch c.Role {
	case RoleGiven:
		return roleGivenNames.holds(s, budget)
	case RoleFamily:
		return roleFamilyNames.holds(s, budget)
	default:
		if givenNames.holds(s, budget) {
			return true
		}
		if budget > 0 && len(s) > budget {
			return false
		}
		g, sn, ok := strings.Cut(s, " ")
		if !ok {
			return false
		}
		return givenNames.holds(g, 0) && surnames.holds(sn, 0)
	}
}

var _ vocabularyMasker = personNameMasker{}
