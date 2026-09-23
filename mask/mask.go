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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// ID names a masker in the registry, as it is written in lazyslice.yml:
// "email", "phone", "person_name", "null", "fixed:$lazyslice$invalid".
type ID string

// Role is a person_name column's sub-category: given, family or full
// (T-0287). It is decided at classify time from the column's name, carried on
// pipeline.Decision beside Category, written to lazyslice.yml and read back
// from it, the same route pipeline.Config.PhoneRegion follows to reach a
// masker's Constraints. Every other category ignores it; Get and the registry
// carry no per-category notion of a role, so a masker that wants one reads
// Constraints.Role itself.
type Role string

const (
	// RoleFull is the zero value and personNameMasker's original behaviour:
	// a given name and a surname, "Given Family". A caller built before this
	// field existed leaves it unset, so RoleFull has to be what an unset
	// Constraints.Role already meant.
	RoleFull Role = ""
	// RoleGiven emits a given name only.
	RoleGiven Role = "given"
	// RoleFamily emits a surname only.
	RoleFamily Role = "family"
)

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
	// CatDerivedText is a column the database derives from text that may
	// itself be masked -- a tsvector maintained by a trigger or a generated
	// expression over a name, an address or a note. It is never copied: the
	// derivation carries the words of the source column, so copying it would
	// ship in cleartext exactly what the column it was derived from is being
	// masked for. Its masker emits the type's empty value.
	CatDerivedText Category = "derived_text"
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
	// Role is a person_name column's sub-category (T-0287): RoleGiven emits a
	// given name only, RoleFamily a surname only, and the zero value RoleFull
	// keeps personNameMasker's original "Given Family". No other generator
	// reads it.
	Role Role
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
	// ErrPassthrough is Apply's post-condition: the generator's output follows
	// the value it was handed instead of the digest, so the cells it masks are
	// not masked at all. It is a hard stop and never a warning — a
	// passed-through cell is the cleartext THREAT_MODEL.md T12 exists to keep
	// out of a target, and Apply used to return it with Masked true, which put
	// a digest for an unmasked cell into the residual filter and left the
	// residual scan as the only thing between a passthrough generator and a
	// shipped cleartext column.
	//
	// It is not returned for a single cell whose masked value happened to equal
	// its source: every generator here is a function of h, so at a domain of d
	// that is expected once in d distinct values, and refusing it would abort a
	// run over a short-id or birthdate column for a bug that is not there.
	// Apply calls the generator a second time under the same h to tell the two
	// apart (see maskCell).
	//
	// The error names the masker id and the category. It never carries the
	// value, and neither does anything else this module returns.
	ErrPassthrough = errors.New("the masker's output follows the value it was given, so the cell was not masked")
	// ErrMaskerPanic is a generator that panicked. Apply recovers it and
	// converts it into this error, naming the masker id, the category and the
	// *type* of the panic value — never the value, which is a free-form string
	// written by whoever panicked and is where a masker that panics with the
	// offending row in its message used to reach stderr (THREAT_MODEL.md T4,
	// T7). This module is importable on its own (ADR-006), so its contract does
	// not depend on the parent binary's redaction.
	ErrMaskerPanic = errors.New("the masker panicked")
	// ErrMaskerFailed is a generator that returned an error instead of a
	// value. maskCell wraps it the way it wraps a panic (T-0223, round-3
	// replay R2-13): a sentinel naming the masker id, the category and the
	// *type* of the returned error — never the error's own message, which is
	// free-form text a masker can write however it likes and is where a
	// masker that quotes the offending value in its error ("cannot mask
	// %q") used to cross this module's public boundary verbatim. This module
	// is importable on its own (ADR-006), so its contract does not depend on
	// the parent binary's redaction.
	ErrMaskerFailed = errors.New("the masker returned an error")
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

// maskerErrorSentinels lists this module's sentinel errors in the order
// wrapMaskerError checks them, so a masker's error that wraps more than one
// keeps naming the first — the same order isModuleError used to check them
// in before T-0238.
var maskerErrorSentinels = []error{
	ErrNoRoom, ErrUnknownMasker, ErrNoCategory, ErrRowCountUnknown,
	ErrPassthrough, ErrMaskerPanic, ErrMaskerFailed,
}

// wrapMaskerError turns whatever a generator's Mask returned into one of this
// module's own error objects. maskCell never returns err itself, whatever it
// is: a masker controls its message freely, and it also controls every field
// of a *NoRoomError or *DomainError it constructs or wraps — including the
// string fields, which is what let a masker declare
// fmt.Errorf("cannot fit %q: %w", in.Text, mask.ErrNoRoom) and have the round-3
// fix's isModuleError wave the whole object, canary and all, straight through
// the module boundary because it recognised ErrNoRoom inside it (T-0223
// round-4 replay R3-1, new variant against T-0223 itself). So a match no
// longer returns the masker's object: it returns a fresh error naming the
// matched sentinel (%w), the category, the masker id and the returned value's
// *type* — never its text — and, for a *NoRoomError or *DomainError, a
// same-typed error this call rebuilds itself, from cat/id (maskCell's own,
// trusted arguments) and, for NoRoomError, the TypeTag and MaxLen off the
// Constraints maskCell was already given — never off the fields the masker's
// error carried, which a masker is free to set to anything, canary included.
// No field of a masker's error is trusted, numeric or not: DomainError's
// four fields are recomputed from c alone — Domain is ColumnDomain(c), never
// Admissible(id, c), because Admissible calls back into the registered
// masker's own Domain method, and that masker is the same object whose Mask
// just ran on this cell. A masker that stashes the cell's value in Mask and
// hands it back from Domain on the very next call would otherwise smuggle it
// out through this error path with a real, registered id and no forged
// field at all (T-0238 round 2).
func wrapMaskerError(cat Category, id ID, c Constraints, err error) error {
	var noRoom *NoRoomError
	if errors.As(err, &noRoom) {
		return &NoRoomError{Category: cat, ID: id, TypeTag: c.TypeTag, MaxLen: c.MaxLen}
	}
	var domain *DomainError
	if errors.As(err, &domain) {
		d := ColumnDomain(c)
		return &DomainError{
			Category: cat,
			ID:       id,
			Domain:   d,
			Required: Required(c.Rows),
			Rows:     c.Rows,
			MaxRows:  MaxRows(d),
		}
	}
	for _, sentinel := range maskerErrorSentinels {
		if errors.Is(err, sentinel) {
			return fmt.Errorf("%w: category %s: masker %s returned %s",
				sentinel, cat, id, panicKind(err))
		}
	}
	return fmt.Errorf("%w: category %s: masker %s returned %s",
		ErrMaskerFailed, cat, id, panicKind(err))
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
	out, err := maskCell(m, cat, id, h, in, canon, c)
	if err != nil {
		return Result{}, err
	}
	return Result{Out: out, Canonical: canon.bytes(), TypeTag: tag, Masked: true}, nil
}

// maskCell calls the generator behind the guards the module owes a caller
// that cannot see inside it: a recover, so a panicking masker becomes an error
// naming the masker and nothing else; the same treatment for a *generator's*
// own returned error, whatever it is — its free-form message, and every field
// of a *NoRoomError or *DomainError it built or wrapped, never crosses this
// module's public boundary (T-0223; T-0238 closed the escape hatch a masker
// got by declaring one of this module's own sentinels); and the
// post-condition, so a masker that tracks its input is a refusal rather than
// a cell the caller records as masked (THREAT_MODEL.md T7, T12). This
// module's own sentinels and typed errors still answer errors.Is/As —
// wrapMaskerError rebuilds them from maskCell's own trusted arguments rather
// than returning the masker's object.
//
// Both guards are in one place because the post-condition calls the generator a
// second time, with a sentinel input, and that call is the generator's code too
// and may panic for the same reasons the first one can.
func maskCell(m Masker, cat Category, id ID, h [32]byte, in Value, canon Value, c Constraints) (out Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = Value{}, fmt.Errorf("%w: category %s: masker %s panicked with %s",
				ErrMaskerPanic, cat, id, panicKind(r))
		}
	}()
	out, err = m.Mask(h, in, c)
	if err != nil {
		return Value{}, wrapMaskerError(cat, id, c, err)
	}
	if !looksLikeItsInput(out, in, canon) {
		return out, nil
	}
	if nothingToMask(in) {
		// A document whose leaves are all empty containers or nulls — {"a":{}},
		// {"tags":[]} — has no scalar for any generator here to replace, so the
		// only output it can have is itself, and that is not evidence about the
		// generator. Refusing it would stop a run over an ordinary jsonb column
		// deterministically, and the cell carries nothing a mask would have
		// removed: key names survive a mask by design (ARCHITECTURE.md section
		// 6 item 6).
		return out, nil
	}
	if !tracksItsInput(m, h, in, c) {
		// The generator ignored its input and still landed on it: at a domain
		// of d that happens to one distinct source value in d, and refusing it
		// would abort a whole run over an ordinary short-id or birthdate
		// column with a message about a masker bug that is not there. The
		// residual scan is the check on a coincidence; this guard is the check
		// on a generator that is not a function of h alone.
		//
		// Except for a vocabulary masker (vocab.go, ADR-015): the residual scan
		// explains a hit inside its vocabulary instead of probing it, so it can
		// no longer be the check on this coincidence, and the masker is
		// redrawn until its output no longer reads as its input. The redraw is
		// here, after tracksItsInput has answered, and never before it: a
		// masker that follows its input — wholly, or only for some h — has
		// already been refused above, and a redraw ahead of that question
		// would launder it instead.
		if vm, ok := m.(vocabularyMasker); ok {
			return redraw(vm, cat, id, h, in, canon, c, out)
		}
		return out, nil
	}
	return Value{}, fmt.Errorf("%w: category %s: masker %s", ErrPassthrough, cat, id)
}

// redraw calls a vocabulary masker again under h_i = SHA-256(h ||
// "lazyslice/redraw" || i), i = 1..8 (redrawDigest), until its output no
// longer reads as its input under looksLikeItsInput. It is reached only from
// maskCell, after the post-condition has cleared the masker of following its
// input, so every call here is the generator's own code under maskCell's
// recover.
//
// After eight redraws the last output is returned as it stands, which is what
// maskCell returned before the redraw existed: a column so narrow that one or
// two words fit it has no other value to give, and the residual scan's count
// and row checks still stand behind it. At a list of a few hundred words the
// chance of reaching the ninth draw is about one in 10^21.
func redraw(m vocabularyMasker, cat Category, id ID, h [32]byte, in, canon Value, c Constraints, out Value) (Value, error) {
	for i := byte(1); i <= redraws; i++ {
		next, err := m.Mask(redrawDigest(h, i), in, c)
		if err != nil {
			return Value{}, wrapMaskerError(cat, id, c, err)
		}
		out = next
		if !looksLikeItsInput(out, in, canon) {
			return out, nil
		}
	}
	return out, nil
}

// panicKind names the type of a recovered panic value, or of a masker's
// returned error, and never the value or the error's message — either can
// quote the input it failed on. It is core.PanicSummary's rule in the module
// that cannot import it — mask depends on the standard library and two
// third-party packages, and nothing under internal/ (ADR-006) — and it is
// deliberately blunter: there is no flag here to offer, because this module
// has no flags.
func panicKind(v any) string {
	if v == nil {
		return "a nil value"
	}
	if _, ok := v.(error); ok {
		return fmt.Sprintf("an error of type %T (its message is withheld because it may quote the value)", v)
	}
	return fmt.Sprintf("a value of type %T (it is withheld because it may quote the value)", v)
}

// looksLikeItsInput reports whether out is, or reads as, the value that went
// in. It is the cheap trigger for the post-condition, not its verdict: every
// masked cell is tested here, so nothing in it parses, allocates or
// canonicalises. A NULL or an empty output is not a passthrough — Apply has
// already returned for a NULL or an empty input, so neither can be the input —
// and the comparison is made against the raw input and against the input's
// canonical form, first byte for byte and then over letters and digits only, so
// that a masker that folds the case of an address, re-spaces it or re-punctuates
// it and hands it back still trips the trigger.
//
// The fold is not a canonicalisation: a generator that returns the input in a
// form whose letters and digits differ from the source's — a bare local number
// handed back in E.164, a date handed back in another layout — is not caught
// here, and the residual scan is what sees it. Canonicalising the output
// instead would put the phonenumbers parser and the date-layout loop on every
// masked cell, which measured at roughly 85% of the cost of masking a phone.
func looksLikeItsInput(out, in, canon Value) bool {
	if out.Null || out.Empty() {
		return false
	}
	o := out.bytes()
	return bytes.Equal(o, in.bytes()) || bytes.Equal(o, canon.bytes()) ||
		foldEqual(o, in.bytes()) || foldEqual(o, canon.bytes())
}

// tracksItsInput asks the question the post-condition actually wants answered:
// is this generator a function of h and the constraints, as the contract says,
// or does its output follow the value it was handed?
//
// Every generator in this module ignores in except to read its shape, so a
// second call under the same h with a sentinel input returns the same value it
// just returned. A generator that hands its input back returns the sentinel,
// and that — not a single cell whose masked value coincided with its source —
// is the contract breach ErrPassthrough names.
//
// It is reached only when looksLikeItsInput has already fired, so the extra
// call is not on the per-cell path. A generator that refuses the sentinel
// cannot answer the question, and an unanswered question is not evidence: the
// cell passes and the residual scan keeps its role as the pipeline's check.
func tracksItsInput(m Masker, h [32]byte, in Value, c Constraints) bool {
	probe := probeValue(in)
	out, err := m.Mask(h, probe, c)
	if err != nil {
		return false
	}
	return looksLikeItsInput(out, probe, probe)
}

// nothingToMask reports whether in is a JSON object or array with no scalar
// leaf in it: every leaf is an empty object, an empty array or a JSON null.
// Such a document has nothing a generator could replace — semi_structured
// keeps structure and key names and masks scalar leaves, and a null leaf stays
// null — so its masked form is the document itself, and so is the sentinel's.
// Both halves of the post-condition therefore fire on a cell that proves
// nothing, which is why this question is asked before the verdict rather than
// after it.
//
// It is computed here and never asked of the masker: a guard that let the
// object it contains declare itself exempt is not a guard (the same reason
// Domain() is not consulted). It runs only once looksLikeItsInput has fired,
// so no ordinary masked cell pays for the parse.
func nothingToMask(in Value) bool {
	b := in.bytes()
	if !json.Valid(b) {
		return false
	}
	doc, ok := decodeJSON(string(b))
	if !ok {
		return false
	}
	switch doc.(type) {
	case map[string]any, []any:
	default:
		// A bare scalar document — 42, "a string", true — is a leaf, and a
		// generator that hands one back is answering the question.
		return false
	}
	return !hasScalarLeaf(doc)
}

// hasScalarLeaf reports whether a decoded document holds a string, number or
// boolean anywhere in it. A JSON null is not one: maskJSON leaves it as null,
// so a document of nulls is returned unchanged by a generator that is working.
func hasScalarLeaf(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		for _, e := range t {
			if hasScalarLeaf(e) {
				return true
			}
		}
		return false
	case []any:
		for _, e := range t {
			if hasScalarLeaf(e) {
				return true
			}
		}
		return false
	case nil:
		return false
	default:
		return true
	}
}

// probeValue returns a value shaped like in but different from it: every ASCII
// letter and digit is stepped on by one, so an email stays an email, an IP
// stays four dotted groups and a JSON document keeps its structure, which is
// all any generator here reads an input for. A value with no letter or digit in
// it gets one appended.
func probeValue(in Value) Value {
	src := in.bytes()
	b := make([]byte, len(src))
	changed := false
	for i, ch := range src {
		b[i] = stepByte(ch)
		if b[i] != ch {
			changed = true
		}
	}
	if !changed {
		// A value with no letter or digit in it: the probe has to differ from
		// the input somehow, so it gets one.
		b = append(append(make([]byte, 0, len(b)+1), b...), '7')
	}
	if len(in.Bytes) > 0 {
		return Value{Bytes: b}
	}
	return Value{Text: string(b)}
}

// stepByte maps an ASCII letter or digit to the next one in its class and
// leaves everything else — punctuation, spaces, the bytes of a multi-byte rune
// — alone.
func stepByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return '0' + (c-'0'+1)%10
	case c >= 'a' && c <= 'z':
		return 'a' + (c-'a'+1)%26
	case c >= 'A' && c <= 'Z':
		return 'A' + (c-'A'+1)%26
	}
	return c
}

// foldEqual reports whether a and b carry the same letters and digits in the
// same order, ignoring case and everything that is neither. It decodes in place
// and stops at the first difference, so the common case — a masked value that
// is nothing like its source — costs one rune.
func foldEqual(a, b []byte) bool {
	for {
		ra, na := nextFoldRune(a)
		rb, nb := nextFoldRune(b)
		if ra != rb {
			return false
		}
		if ra < 0 {
			return true
		}
		a, b = a[na:], b[nb:]
	}
}

// nextFoldRune returns the next letter or digit in s, lowercased, and how many
// bytes of s it consumed. It returns -1 when s holds no more.
func nextFoldRune(s []byte) (rune, int) {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRune(s[i:])
		i += n
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r), i
		}
	}
	return -1, len(s)
}
