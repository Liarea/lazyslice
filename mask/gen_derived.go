// SPDX-License-Identifier: Apache-2.0

package mask

// derivedTextMasker empties a column the database derives from text that may
// itself be masked.
//
// A tsvector is the case it exists for: `film.fulltext` in pagila is maintained
// by a trigger over `title` and `description`, and a tsvector holds the lexemes
// of the text it was built from. Copying one through would ship the words of a
// masked column in cleartext beside it (THREAT_MODEL.md T12), and there is no
// fake worth generating: a tsvector of invented lexemes is a search index that
// matches nothing, which is exactly what the empty one is, honestly.
//
// So the answer is the type's empty value, always: the empty tsvector literal
// is a valid tsvector with no lexemes in it, every row gets it, and the domain
// is 1 and says so, which keeps the generator out of a unique column by the
// ordinary route.
type derivedTextMasker struct{}

// Domain is 1 for the type this masker empties and 0 for anything else. The
// rule pack accepts only tsvector for the category, so the zero is a refusal
// nothing should be able to reach: it is the mask side of the plan's write-back
// check, and it fails as a *NoRoomError at Pick rather than as a value some
// other type could not hold.
func (derivedTextMasker) Domain(c Constraints) int64 {
	if c.TypeTag != famTSVector {
		return 0
	}
	return 1
}

func (derivedTextMasker) Mask(_ [32]byte, _ Value, c Constraints) (Value, error) {
	if c.TypeTag != famTSVector {
		return Value{}, ErrNoRoom
	}
	// The empty tsvector. It travels as text, which is what the loader encodes
	// back into the column: '' is a tsvector with no lexemes in it, not a NULL
	// and not a string of one space.
	return Value{Text: ""}, nil
}
