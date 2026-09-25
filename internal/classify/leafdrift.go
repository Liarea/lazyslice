// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"sort"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// A committed lazyslice.yml pins which document keys are copied (T-0404, the
// 2026-09-25 JSON red team's round 1, entry 28).
//
// The per-leaf map (T-0272) follows the keys the samples show, and it is not
// in Classification.Fingerprint (ARCHITECTURE.md §5 "Determinism scope"), so
// before this pass a re-run from a committed file whose source documents had
// grown a key -- `guest`, `emergency` -- copied every leaf beneath it with no
// line saying so: the file named the column and nothing about its keys, the
// run reported "0 drift", and --strict-schema passed. CONCEPT.md's promise is
// that a committed file never excuses what it has not seen.
//
// leafKeys does two things for every column whose decision carries a per-leaf
// map. It sets Decision.RecordedLeafKeys, which internal/emit writes under
// `leaf_keys:`. And, for a column the prior carries an entry for, it takes out
// of the map every key the map calls none that the entry's LeafKeys does not
// list, so internal/transform masks every leaf beneath it exactly as it masks
// a key the samples never showed and internal/verify, reading the same map,
// reads it the same way; the key goes on Classification.LeafDrift for
// internal/core to report and for --strict-schema to refuse.
//
// What is recorded is what a run from the written file may copy, and a
// drifted key is not in it (T-0404 review round, finding 1): the first draft
// recorded the drifted keys too, and since every non-strict run rewrites
// --config in place, the next run from the same, uncommitted file copied
// them with no drift line, so the pin lasted one run. Now:
//
//   - a column the file does not carry (or no file at all) is decided fresh,
//     keys included, as a column the file has never seen always is: its
//     copied keys are recorded;
//   - a column whose entry carries `leaf_keys:` records that list as it
//     stands, so a key joins it only by an operator's hand edit and a listed
//     key the samples happened not to show this run keeps its place;
//   - a column whose entry carries no `leaf_keys:` at all -- a file written
//     before T-0404, or a column that had no per-leaf map when the file was
//     written -- has every copied key masked and reported as drift this run,
//     and records them, so the next run from the written file copies them.
//     That is the one route by which a run adds keys to an existing entry,
//     and the first-draft behaviour it keeps is still more masking than
//     before T-0404, when those keys were copied on every run.
//
// A key the entry lists keeps whatever this run decided for it, which is copy
// or, when new evidence gave it a category, a mask: listing a key never
// copies one.
//
// It runs after finalise, because LeafMap reads the decision's final Category
// and Source, and a pass before it (FK propagation's second sweep, a yml
// raise) can still move either. It only ever removes keys from a map, so it
// only ever masks more.
func (st *state) leafKeys(prior *pipeline.Config, cls *pipeline.Classification) {
	for _, c := range st.order {
		w := st.dec[c]
		m := w.d.LeafMap()
		if m == nil {
			continue
		}
		many := len(m) > leafKeyNameLimit
		keys := make([]string, 0, len(m))
		for k, cat := range m {
			if cat == pipeline.CatNone {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		var cc pipeline.ColumnConfig
		carried := false
		if prior != nil {
			cc, carried = prior.Columns[c]
		}
		if carried && cc.LeafKeys != nil {
			w.d.RecordedLeafKeys = sortedUnique(cc.LeafKeys)
		} else {
			recorded := make([]string, 0, len(keys))
			for _, k := range keys {
				recorded = append(recorded, leafKeySpelling(k, many))
			}
			w.d.RecordedLeafKeys = sortedUnique(recorded)
		}
		if !carried {
			// Not in the file at all: Classification.Drift already names it,
			// and it is classified fresh, keys and all.
			continue
		}

		listed := make(map[string]bool, len(cc.LeafKeys))
		for _, k := range cc.LeafKeys {
			listed[k] = true
		}
		for _, k := range keys {
			if leafKeyListed(listed, k) {
				continue
			}
			delete(w.d.LeafKeys, k)
			cls.LeafDrift = append(cls.LeafDrift, pipeline.LeafDrift{Col: c, Key: leafKeySpelling(k, many)})
		}
	}
	sort.Slice(cls.LeafDrift, func(i, j int) bool {
		a, b := cls.LeafDrift[i], cls.LeafDrift[j]
		if a.Col != b.Col {
			return a.Col.Less(b.Col)
		}
		return a.Key < b.Key
	})
}

// leafKeyListed reports whether a `leaf_keys:` list names key in either of the
// spellings leafKeySpelling can give it: its fingerprint always, and the key
// itself when it is identifier-shaped. A column whose key count crosses
// leafKeyNameLimit between two runs switches spelling, and a listed key must
// not turn into drift for that alone.
func leafKeyListed(listed map[string]bool, key string) bool {
	if listed[leafKeyFingerprint(key)] {
		return true
	}
	return identifierShaped(key) && listed[key]
}

func sortedUnique(in []string) []string {
	out := append(make([]string, 0, len(in)), in...)
	sort.Strings(out)
	return slices.Compact(out)
}

// leafKeyMaxLen is the longest key leafKeySpelling writes as itself. A name
// someone chose for a document field is short; a longer run of identifier
// characters is as likely a token or a slug.
const leafKeyMaxLen = 32

// leafKeyNameLimit is how many distinct keys a column's per-leaf map may hold
// before leafKeySpelling fingerprints every one of them. Documents an
// application writes by field name show a few dozen keys across a sample; a
// map keyed by data -- one key per customer, per username, per slug -- shows
// one per row, and there a key that looks like a word (`jsmith`) is a value
// all the same (T-0404 review round, finding 2). The keys themselves are
// still listed, and still compared, as fingerprints; how many is bounded by
// jsonKeyLimit.
const leafKeyNameLimit = 64

// leafKeySpelling is how a document key is written into lazyslice.yml's
// `leaf_keys:` and named in a classify.column.drift line (T-0404): the key
// itself when it has an identifier's shape (identifierShaped) and its column
// holds no more than leafKeyNameLimit keys (many is false), and otherwise
// leafKeyFingerprint's "sha256:" followed by the first 16 hex characters of
// the key's SHA-256.
//
// A key is a name, and the yml records names; but a document keyed by
// whatever the application stored (a person's name with a space in it, a
// UUID, a slug with a colon, a username) makes the key a value, and the file
// never holds a row value (THREAT_MODEL.md T5). A fingerprint still tells a
// re-run which keys it has seen, which is all the comparison needs; it is the
// footing where_fingerprint stands on for a withheld --where. The two forms
// cannot collide: ':' never appears in the first.
//
// It is this package's alone: internal/emit writes the spellings this package
// put on Decision.RecordedLeafKeys and Classification.LeafDrift, and compares
// nothing, so there is no second copy to disagree with.
func leafKeySpelling(key string, many bool) string {
	if !many && identifierShaped(key) {
		return key
	}
	return leafKeyFingerprint(key)
}

func leafKeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

// identifierShaped is the shape of a field name someone chose: an ASCII
// letter or underscore, then ASCII letters, underscores and hyphens, at most
// leafKeyMaxLen bytes, with digits only as a trailing run of one or two
// (`line1`, `address_2`). Everything else is fingerprinted (T-0404 review
// round, finding 2): a digit anywhere else is the mark of a generated value --
// every UUID carries its version digit mid-string, and so do most tokens,
// order numbers and customer ids -- a dot is the mark of a dotted handle or a
// domain (`alice.smith`), and a key whose letters are all hexadecimal, eight
// or more of them, is a hash or an id however it is punctuated. A word-shaped
// key that is personal (`jsmith`) passes this test; leafKeyNameLimit is what
// catches a column full of them.
func identifierShaped(s string) bool {
	if s == "" || len(s) > leafKeyMaxLen {
		return false
	}
	digits, letters, hexLetters := 0, 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			if digits > 0 {
				return false
			}
			letters++
			if c|0x20 >= 'a' && c|0x20 <= 'f' {
				hexLetters++
			}
		case c == '_' || (i > 0 && c == '-'):
			if digits > 0 {
				return false
			}
		case i > 0 && c >= '0' && c <= '9':
			digits++
		default:
			return false
		}
	}
	if digits > 2 {
		return false
	}
	return letters < 8 || hexLetters != letters
}
