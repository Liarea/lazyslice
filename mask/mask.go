// SPDX-License-Identifier: Apache-2.0

// Package mask is the deterministic masking module of lazyslice.
//
// It is a nested Go module, github.com/Liarea/lazyslice/mask (ADR-006), so a
// program that has never heard of lazyslice can import it and its tests run
// without a database. It depends only on the standard library,
// golang.org/x/text and github.com/nyaruka/phonenumbers.
//
// The scheme is ARCHITECTURE.md section 5 and it is part of this module's
// public contract; changing it is a major version of the module:
//
//	K     = 32 random bytes                 // the run key
//	K_cat = HKDF-SHA256(K, nil, "lazyslice/v1/"+category, 32)
//	h     = HMAC-SHA256(K_cat, Encode(typeTag, canonical(value)))
//	fake  = registry[id].Mask(h, value, constraints)
//
// Encode is length-prefixed so that ("email", "a@b.com") and ("emai",
// "la@b.com") hash differently.
//
// Scaffold status: declarations only. This file is the type surface
// internal/pipeline compiles against; the key schedule above, the
// canonicalisation, the registry and every generator arrive with the masking
// task, each with the section 5 test vectors that make them a contract rather
// than a guess.
package mask

// ID names a masker in the registry, as it is written in lazyslice.yml:
// "email", "phone", "person_name", "null", "fixed:$lazyslice$invalid".
type ID string

// KeyLen is the length of a run key in bytes.
const KeyLen = 32

// Key is the run key K. It is read from ./lazyslice.secret or
// $LAZYSLICE_SECRET, or generated per run and discarded. It never leaves the
// process and it is never written to lazyslice.yml, an event or the marker
// table; only its fingerprint is.
type Key [KeyLen]byte

// Value is one column value on its way through a masker. Exactly one of Null,
// Text and Bytes carries the value: Null for SQL NULL, Bytes for bytea, Text
// for everything else in its canonical form.
//
// A Value holds production data. Nothing in this package serialises one, and
// pipeline.Config, event.Event, pipeline.Plan and pipeline.Report may not
// reach one (ARCHITECTURE.md section 2 "Value-free types").
type Value struct {
	Null  bool
	Text  string
	Bytes []byte
}

// Constraints is what the column will accept, gathered at introspect. A masker
// reads it to stay inside the shape the application checks: varchar(n) length,
// enum membership, parseable CHECK shapes.
type Constraints struct {
	// TypeTag names the canonical form, so that a bigint and a text copy of one
	// identifier hash alike.
	TypeTag string
	// MaxLen is atttypmod less the header, 0 when the type is unbounded.
	MaxLen int
	// EnumLabels is the type's labels when the column is an enum; a masked enum
	// must emit one of them.
	EnumLabels []string
	// Checks holds the CHECK expressions naming the column, verbatim from
	// pg_get_constraintdef.
	Checks []string
	// Nullable reports whether the column accepts NULL.
	Nullable bool
	// Unique reports that the column sits under a unique index, so the chosen
	// generator's Domain must reach d_required (ARCHITECTURE.md section 5).
	Unique bool
	// Rows is the planned row count of the table, the n in d_required.
	Rows int64
	// Region is the libphonenumber region hint for phone canonicalisation, ""
	// when unknown.
	Region string
}

// Masker is one generator. Mask is pure: every choice it makes comes from h,
// never from a global seed, the clock or the input's length.
type Masker interface {
	// Mask returns the replacement for in. A Null input returns a Null output.
	Mask(h [32]byte, in Value, c Constraints) (Value, error)
	// Domain is the number of distinct outputs the generator can emit under c.
	// The planner compares it against the column's own admissible domain and
	// refuses a unique column that cannot carry the row count.
	Domain(c Constraints) int64
}
