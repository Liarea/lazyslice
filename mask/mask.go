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
// Scaffold status: the key schedule, the encoding and the registry are real;
// no generator is registered yet, so Lookup finds nothing and Registered
// returns an empty slice. Generators arrive with the masking task.
package mask

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
)

// ErrNotImplemented is returned by every placeholder in the scaffold. No
// caller should ever see it once the masking task has landed.
var ErrNotImplemented = errors.New("mask: not implemented")

// ID names a masker in the registry, as it is written in lazyslice.yml:
// "email", "phone", "person_name", "null", "fixed:$lazyslice$invalid".
type ID string

// KeyLen is the length of a run key in bytes.
const KeyLen = 32

// Key is the run key K. It is read from ./lazyslice.secret or
// $LAZYSLICE_SECRET, or generated per run and discarded. It never leaves the
// process and it is never written to lazyslice.yml, an event or the marker
// table; only Fingerprint is.
type Key [KeyLen]byte

// NewKey returns a random run key.
func NewKey() (*Key, error) {
	var k Key
	if _, err := rand.Read(k[:]); err != nil {
		return nil, fmt.Errorf("mask: generating run key: %w", err)
	}
	return &k, nil
}

// Fingerprint is sha256(K)[:8] in hex, the value lazyslice.yml records as
// secret_fingerprint and lazyslice_meta as secret_fingerprint. It identifies a
// key without revealing it.
func (k *Key) Fingerprint() string {
	sum := sha256.Sum256(k[:])
	return hex.EncodeToString(sum[:4])
}

// Derive returns K_cat for a classification category: HKDF-SHA256 over the run
// key with info "lazyslice/v1/<category>". Deriving per category is what makes
// a column's mapping change when its category changes, which
// ARCHITECTURE.md section 5 "Determinism scope" requires the tool to announce.
func (k *Key) Derive(category string) ([KeyLen]byte, error) {
	var out [KeyLen]byte
	b, err := hkdf.Key(sha256.New, k[:], nil, "lazyslice/v1/"+category, KeyLen)
	if err != nil {
		return out, fmt.Errorf("mask: deriving category key: %w", err)
	}
	copy(out[:], b)
	return out, nil
}

// Sum is h: HMAC-SHA256 of the length-prefixed fields under a derived category
// key. Every generator choice is a function of h and nothing else, so there is
// no global seed to reset between runs.
func Sum(catKey [KeyLen]byte, fields ...[]byte) [32]byte {
	m := hmac.New(sha256.New, catKey[:])
	m.Write(Encode(fields...))
	var out [32]byte
	copy(out[:], m.Sum(nil))
	return out
}

// Encode is the length-prefixed encoding used everywhere a tuple is hashed,
// here and in the residual filter:
//
//	Encode(f1, ..., fn) = u32be(len(f1)) || f1 || ... || u32be(len(fn)) || fn
//
// It is unambiguous, so no two distinct tuples share an encoding and a column
// "b.c" in schema "a" never aliases column "c" in schema "a.b".
func Encode(fields ...[]byte) []byte {
	n := 0
	for _, f := range fields {
		n += 4 + len(f)
	}
	buf := make([]byte, 0, n)
	var l [4]byte
	for _, f := range fields {
		// A field longer than the u32 prefix can hold cannot be encoded
		// unambiguously, and truncating the prefix would let two distinct tuples
		// share an encoding. Postgres caps a field at 1 GB, so this is
		// unreachable; it fails loudly rather than quietly.
		if uint64(len(f)) > math.MaxUint32 {
			panic("mask: field too long to length-prefix")
		}
		binary.BigEndian.PutUint32(l[:], uint32(len(f))) //nolint:gosec // G115: bounded on the line above
		buf = append(buf, l[:]...)
		buf = append(buf, f...)
	}
	return buf
}

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

var registry sync.Map // ID -> Masker

// Register adds a masker to the build-time registry. Nothing is loaded at
// runtime (ADR-006): registration happens in an init function of this module or
// of a program that imports it. Registering an ID twice panics, because a
// silently replaced masker changes the mapping.
func Register(id ID, m Masker) {
	if m == nil {
		panic("mask: Register called with a nil masker for " + string(id))
	}
	if _, loaded := registry.LoadOrStore(id, m); loaded {
		panic("mask: masker registered twice: " + string(id))
	}
}

// Lookup returns the registered masker for id.
func Lookup(id ID) (Masker, bool) {
	v, ok := registry.Load(id)
	if !ok {
		return nil, false
	}
	m, ok := v.(Masker)
	return m, ok
}

// Registered lists every registered ID in sorted order. The classifier rule
// pack is checked against this list, so a category with no masker cannot exist.
func Registered() []ID {
	var ids []ID
	registry.Range(func(k, _ any) bool {
		if id, ok := k.(ID); ok {
			ids = append(ids, id)
		}
		return true
	})
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
