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
// Apply is the one right way to mask a cell: it holds the NULL and empty
// rules, canonicalises, derives h and calls the generator, and it hands back
// the canonical source bytes the caller needs for the residual filter
// (ARCHITECTURE.md section 6). Nothing here reads a database, a file, the
// clock or the network.
package mask

import (
	"errors"
	"fmt"
)

// ID names a masker in the registry, as it is written in lazyslice.yml:
// "email", "phone", "person_name", "null", "fixed:$lazyslice$invalid".
type ID string

// Category is the classification a column was masked under. It is the info
// string of the HKDF derivation, so two columns in different categories never
// share a mapping, and changing a category changes every masked value in it.
//
// The values are the same strings as pipeline.Category in the parent module;
// mask cannot import that package (it is a separate module and internal/), so
// the two lists are kept in step by the rule pack, which names both.
type Category string

// The categories ARCHITECTURE.md section 4 classifies a column under.
const (
	CatNone       Category = "none"
	CatPersonName Category = "person_name"
	CatEmail      Category = "email"
	CatPhone      Category = "phone"
	CatAddress    Category = "address"
	CatGeo        Category = "geo"
	CatPersonDate Category = "person_date"
	CatNationalID Category = "national_id"
	CatFinancial  Category = "financial_account"
	CatNetworkID  Category = "network_id"
	CatOnlineID   Category = "online_id"
	CatCredential Category = "credential"
	CatFreeText   Category = "free_text"
	CatSpecial    Category = "special_category"
	CatBinary     Category = "binary_personal"
	CatSemiStruct Category = "semi_structured"
)

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

// Empty reports whether v carries a value with no bytes in it. An empty value
// is passed through unchanged, which with NULL is one of the two stated
// exceptions in ARCHITECTURE.md section 6 item 6. The test is on the value as
// it arrived, never on its canonical form: a canonicaliser that trims a value
// to nothing must not drop it into this rule (THREAT_MODEL.md T12).
func (v Value) Empty() bool { return !v.Null && v.Text == "" && len(v.Bytes) == 0 }

// bytes returns the value's bytes for hashing. Null has no bytes; a Null value
// never reaches a digest, because Apply returns before that.
func (v Value) bytes() []byte {
	if len(v.Bytes) > 0 {
		return v.Bytes
	}
	return []byte(v.Text)
}

// Constraints is what the column will accept, gathered at introspect. A masker
// reads it to stay inside the shape the application checks: varchar(n) length,
// enum membership, parseable CHECK shapes.
type Constraints struct {
	// TypeTag is the column's PostgreSQL type family, in the names
	// internal/classify/types.go uses: text, varchar, bpchar, citext, boolean,
	// integer, bigint, numeric, float, date, timestamp, time, interval, uuid,
	// inet, cidr, macaddr, bytea, json, jsonb, hstore, enum, other. It shapes
	// the output — a phone in a bigint column emits digits, one in a text
	// column emits E.164 — and it is deliberately *not* the type tag that goes
	// into the digest. That tag names the canonical form and Canonical returns
	// it, which is what makes a bigint and a text copy of one identifier hash
	// alike (ARCHITECTURE.md section 5; scaffold review note on mask.Canonical,
	// tracker T-0020).
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
	// Distinct is the number of distinct non-null values among the column's
	// samples, 0 when unknown. It is the other half of section 5's small-domain
	// rule: a masked column whose admissible domain is below 2 × Distinct is a
	// substitution recoverable by frequency, which Small reports and which the
	// special_category masker collapses rather than pretends to mask.
	Distinct int64
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

// Errors this module returns. Every one of them is a refusal: none of them is
// a reason to copy the value through.
var (
	// ErrUnknownMasker is returned for an ID no build registered.
	ErrUnknownMasker = errors.New("no masker is registered under that id")
	// ErrNoCategory is returned when a category has no registered masker at
	// all, which ADR-006 makes impossible for a category in the rule pack.
	ErrNoCategory = errors.New("no masker is registered for that category")
	// ErrNoRoom is returned when the column cannot hold any value the
	// generator can produce — a varchar too short for a masked email, say. It
	// is the Mask side of a Domain of 0.
	ErrNoRoom = errors.New("the column is too short to hold a masked value")
	// ErrKeyLength is returned by ParseKey for anything but 32 bytes of hex.
	ErrKeyLength = errors.New("a masking key is 32 bytes, 64 hex characters")
	// ErrRowCountUnknown is Pick's refusal for a column under a unique index
	// whose table row count it was not given. n is the whole of d_required, so
	// without it there is no comparison to make, and accepting the column would
	// move the unique violation to load — where its PgError.Detail is dropped
	// (THREAT_MODEL.md T4) — instead of refusing it at plan.
	ErrRowCountUnknown = errors.New("a unique column needs the table's row count before a generator can be chosen")
)

// NoRoomError is the plan-time half of ErrNoRoom: the column cannot hold any
// value the generator can produce, whatever the row count. It is a separate
// type from DomainError because there is no d_required to print — no --take
// makes a varchar(8) hold a masked email — so the planner renders it as "no
// masked value fits this column" and names the type, not a row count. It wraps
// ErrNoRoom, so a caller that only wants to know "refused for want of room"
// can ask errors.Is.
type NoRoomError struct {
	Category Category
	ID       ID
	TypeTag  string
	MaxLen   int
}

func (e *NoRoomError) Error() string {
	if e.MaxLen > 0 {
		return fmt.Sprintf(
			"category %s: masker %s has no value that fits %s(%d)",
			e.Category, e.ID, e.TypeTag, e.MaxLen)
	}
	return fmt.Sprintf(
		"category %s: masker %s has no value that fits a column of type %s",
		e.Category, e.ID, e.TypeTag)
}

func (e *NoRoomError) Unwrap() error { return ErrNoRoom }

// DomainError is the unique-index collision refusal of ARCHITECTURE.md section
// 5: no registered generator for the category can emit d_required = n²/2ε
// distinct values at ε = 10⁻⁶. The planner renders it as exit 12, naming the
// column, d, d_required and the three escapes; MaxRows is the largest --take
// or --cap the column can carry at that ε.
type DomainError struct {
	Category Category
	ID       ID
	Domain   int64
	Required int64
	Rows     int64
	MaxRows  int64
}

func (e *DomainError) Error() string {
	return fmt.Sprintf(
		"category %s: masker %s can emit %d distinct values, %d rows need %d",
		e.Category, e.ID, e.Domain, e.Rows, e.Required)
}

// Result is one masked cell.
type Result struct {
	// Out is the value to load.
	Out Value
	// Canonical is the canonical form of the *source* value, the bytes that
	// went into the digest. The caller adds it to the residual filter
	// (ARCHITECTURE.md section 6). It is production data: it never reaches an
	// event, the yml, a log line or an error string.
	Canonical []byte
	// TypeTag names the canonical form Canonical is in.
	TypeTag string
	// Masked is false when the value was passed through under the NULL or the
	// empty rule, in which case there is nothing to add to the filter.
	Masked bool
}

// Apply masks one cell: the NULL and empty rules, canonicalisation, the key
// schedule and the generator, in that order.
//
// It is the only entry point that holds all four; a caller that reaches for
// Canonical, Digest and Masker.Mask by hand owns the NULL and empty rules
// itself, and getting them wrong is how a value ships in cleartext under a
// column the report calls masked (THREAT_MODEL.md T12).
func Apply(k Key, cat Category, id ID, in Value, c Constraints) (Result, error) {
	if in.Null {
		return Result{Out: in}, nil
	}
	if in.Empty() {
		return Result{Out: in}, nil
	}
	m, ok := Get(id)
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownMasker, id)
	}
	canon, tag, err := Canonical(cat, in, c)
	if err != nil {
		return Result{}, err
	}
	h, err := Digest(k, cat, tag, canon.bytes())
	if err != nil {
		return Result{}, err
	}
	out, err := m.Mask(h, in, c)
	if err != nil {
		return Result{}, err
	}
	return Result{Out: out, Canonical: canon.bytes(), TypeTag: tag, Masked: true}, nil
}
