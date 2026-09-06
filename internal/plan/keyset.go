// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"bytes"
	"encoding/binary"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The key sets and chunks of ARCHITECTURE.md §2 "plan".
//
// A key set is a sorted set of key tuples in identity-column order. Iteration
// is over that order and never over a Go map, because the plan two runs produce
// has to be byte-identical (§3 "Determinism").
//
// The storage is the one §2 prescribes, and it is the whole of what a set
// holds: a single int8 key set is a []int64, and every other key set is one
// byte slab of encoded tuples with an index of spans into it. There is no
// parallel dedup map and no []keyValue backing array, because Bytes is the
// single source of truth for the memory budget (THREAT_MODEL.md T11) and a
// figure that ignores most of what the process is holding is a plan that lies.
// Duplicates are removed by sorting and scanning that same storage.
//
// The slab encoding is order-preserving: byte order over an encoded tuple is
// identity-column order over the tuple it encodes, so sorting the spans sorts
// the set and no comparison has to decode. An int8 is its big-endian form with
// the sign bit flipped, a uuid its 16 bytes, and a string its bytes with 0x00
// escaped as 0x00 0xff and terminated by 0x00 0x00.

// keyKind is how one identity column is carried in a chunk. It decides both the
// Go type of the array handed to pgx and the cast written into the statement
// (ARCHITECTURE.md §2 "Chunk").
type keyKind int

const (
	kindInt    keyKind = iota // int2, int4, int8         -> []int64,       ::int8[]
	kindText                  // text, varchar, citext     -> []string,      ::text[]
	kindBpchar                // bpchar                    -> []string,      ::text[] read as text
	kindUUID                  // uuid                      -> []pgtype.UUID, ::uuid[]
	kindOther                 // everything else           -> []string,      ::text[] and a cast back
)

// The type OIDs the first kinds cover. They are stable catalog OIDs, not
// numbers looked up at run time; citext is an extension type whose OID is
// assigned at CREATE EXTENSION, so it is recognised by name below.
const (
	oidInt2    uint32 = 21
	oidInt8    uint32 = 20
	oidInt4    uint32 = 23
	oidText    uint32 = 25
	oidBpchar  uint32 = 1042
	oidVarchar uint32 = 1043
	oidUUID    uint32 = 2950
)

// keyType is one identity column's encoding.
type keyType struct {
	kind keyKind
	// arrayCast is the cast the unnest argument carries, "::int8[]" and so on.
	arrayCast string
	// typeName is pg_catalog.format_type. kindOther travels as text and is cast
	// back to this in the join condition, which is the "unnest($i::text[]) cast
	// to <format_type>" form in §2.
	typeName string
}

// typeOf maps a column to its key encoding.
func typeOf(col pipeline.Column) keyType {
	switch col.TypeOID {
	case oidInt2, oidInt4, oidInt8:
		return keyType{kind: kindInt, arrayCast: "::int8[]", typeName: col.TypeName}
	case oidText, oidVarchar:
		return keyType{kind: kindText, arrayCast: "::text[]", typeName: col.TypeName}
	case oidBpchar:
		// character(n) is blank-padded on disk and its comparison against text
		// is not: `t.c = k.k1` resolves to the text equality, which strips the
		// padding from the column. A key read back padded therefore matches no
		// row at all (verified on postgres:16: the padded form joins 0 rows and
		// the trimmed form 1). The value is read as text — the same trimming the
		// comparison does — so the stored key is the one the join compares.
		return keyType{kind: kindBpchar, arrayCast: "::text[]", typeName: col.TypeName}
	case oidUUID:
		return keyType{kind: kindUUID, arrayCast: "::uuid[]", typeName: col.TypeName}
	}
	// citext has no fixed OID. It is a text type in every way that matters here
	// and §2 lists it with text, varchar and bpchar.
	if base := strings.TrimPrefix(col.TypeName, "public."); base == "citext" {
		return keyType{kind: kindText, arrayCast: "::text[]", typeName: col.TypeName}
	}
	return keyType{kind: kindOther, arrayCast: "::text[]", typeName: col.TypeName}
}

// textOut says whether the select list reads this column as text. kindOther
// travels as its text form and is cast back in the join; kindBpchar travels as
// text because that is what its comparison against a text array compares.
func (t keyType) textOut() bool { return t.kind == kindOther || t.kind == kindBpchar }

// keyValue is one column of one key tuple. It is a struct rather than an `any`
// so that no branch of this package reads a key through an unchecked type
// assertion: the kind says which field is live.
//
// It is the shape a tuple takes on the way in and on the way out, never the
// shape the set stores: nothing in this package holds a []keyValue for longer
// than one call.
type keyValue struct {
	i int64
	s string
	u pgtype.UUID
}

// keys is the mutable set the walk accumulates per table. It is sealed into a
// pipeline.KeySet at construction, so Bytes on the sealed value reads the live
// set and the budget check during the walk and the estimate at the end are the
// same number (ARCHITECTURE.md §3 "checkBudgets").
//
// Tuples are appended unsorted and normalised lazily: normalize sorts the tail
// and merges it into the sorted head, dropping duplicates. Every read (Len,
// has, Chunks, Bytes, forEach) normalises first, so the set a caller sees is
// always sorted and deduplicated.
type keys struct {
	types []keyType

	// ints is the single-int8-column form.
	ints []int64
	// slab and spans are every other form: the encoded tuples, and one
	// [start,end) span per tuple into them. A span is a pair of int32, so the
	// index costs 8 bytes a tuple and §2's "slab size x 2" covers it for every
	// tuple of 8 encoded bytes or more — which is every composite key, every
	// uuid and every text key of six characters or more. Below that the figure
	// trails the heap by at most half again, on a set whose keys are two or
	// three bytes each and which is therefore kilobytes; the budget is a
	// control against gigabytes.
	//
	// A slab past 2 GiB would not fit an int32 span, and cannot be reached:
	// Bytes reports twice the slab and the budget is checked after every batch,
	// so it would take a --memory-budget above 4 GiB to get there.
	slab  []byte
	spans [][2]int32

	// sortedN is how many leading tuples are sorted and free of duplicates.
	sortedN int
	// scratch is the encode buffer has reuses, so a membership test allocates
	// nothing.
	scratch []byte

	set pipeline.KeySet
}

func newKeys(types []keyType) *keys {
	k := &keys{types: types}
	// §2: a single int8 key set is a []int64; everything else is a slab of
	// encoded tuples. The two Bytes implementations follow that split.
	if k.isInt() {
		k.set = intKeys{k}
	} else {
		k.set = slabKeys{k}
	}
	return k
}

// isInt says whether the set takes the []int64 form.
func (k *keys) isInt() bool { return len(k.types) == 1 && k.types[0].kind == kindInt }

// count is the number of tuples held, normalised or not.
func (k *keys) count() int {
	if k.isInt() {
		return len(k.ints)
	}
	return len(k.spans)
}

// add records one tuple. The tuple is copied: the caller may reuse its slice.
func (k *keys) add(t []keyValue) {
	if k.isInt() {
		k.ints = append(k.ints, t[0].i)
		return
	}
	start := len(k.slab)
	k.slab = k.encodeInto(k.slab, t)
	//nolint:gosec // G115: a span is bounded by the slab, and a slab past 2 GiB
	// is unreachable under any memory budget below 4 GiB (see the field).
	k.spans = append(k.spans, [2]int32{int32(start), int32(len(k.slab))})
}

// has says whether the tuple is already in the set.
func (k *keys) has(t []keyValue) bool {
	k.normalize()
	if k.isInt() {
		i := sort.Search(len(k.ints), func(j int) bool { return k.ints[j] >= t[0].i })
		return i < len(k.ints) && k.ints[i] == t[0].i
	}
	k.scratch = k.encodeInto(k.scratch[:0], t)
	i := sort.Search(len(k.spans), func(j int) bool { return bytes.Compare(k.at(j), k.scratch) >= 0 })
	return i < len(k.spans) && bytes.Equal(k.at(i), k.scratch)
}

// at is the encoded bytes of tuple i.
func (k *keys) at(i int) []byte { return k.slab[k.spans[i][0]:k.spans[i][1]] }

// Len is the number of distinct tuples.
func (k *keys) Len() int {
	k.normalize()
	return k.count()
}

// forEach calls f with every tuple, in key order. The slice f receives is
// reused between calls, so f must copy anything it keeps — add and has both do.
func (k *keys) forEach(f func([]keyValue)) {
	k.normalize()
	buf := make([]keyValue, len(k.types))
	for i := 0; i < k.count(); i++ {
		k.tupleAt(i, buf)
		f(buf)
	}
}

// tupleAt decodes tuple i into dst.
func (k *keys) tupleAt(i int, dst []keyValue) {
	if k.isInt() {
		dst[0].i = k.ints[i]
		return
	}
	k.decode(k.at(i), dst)
}

// normalize sorts the appended tail and merges it into the sorted head,
// dropping duplicates. Batches arrive already in key order (the walk subtracts
// a sorted set before it absorbs one), so the merge is linear in the common
// case and the whole set is never re-sorted.
func (k *keys) normalize() {
	if k.sortedN == k.count() {
		return
	}
	if k.isInt() {
		k.normalizeInts()
		return
	}
	k.normalizeSpans()
}

func (k *keys) normalizeInts() {
	tail := k.ints[k.sortedN:]
	sort.Slice(tail, func(a, b int) bool { return tail[a] < tail[b] })
	head := k.ints[:k.sortedN]
	out := make([]int64, 0, len(head)+len(tail))
	a, b := 0, 0
	appendUnique := func(v int64) {
		if n := len(out); n > 0 && out[n-1] == v {
			return
		}
		out = append(out, v)
	}
	for a < len(head) && b < len(tail) {
		if head[a] <= tail[b] {
			appendUnique(head[a])
			a++
			continue
		}
		appendUnique(tail[b])
		b++
	}
	for ; a < len(head); a++ {
		appendUnique(head[a])
	}
	for ; b < len(tail); b++ {
		appendUnique(tail[b])
	}
	k.ints = out
	k.sortedN = len(out)
}

func (k *keys) normalizeSpans() {
	tail := k.spans[k.sortedN:]
	sort.Slice(tail, func(a, b int) bool {
		return bytes.Compare(k.slab[tail[a][0]:tail[a][1]], k.slab[tail[b][0]:tail[b][1]]) < 0
	})
	head := k.spans[:k.sortedN]
	out := make([][2]int32, 0, len(head)+len(tail))
	dropped := false
	appendUnique := func(s [2]int32) {
		if n := len(out); n > 0 && bytes.Equal(k.slab[out[n-1][0]:out[n-1][1]], k.slab[s[0]:s[1]]) {
			dropped = true
			return
		}
		out = append(out, s)
	}
	a, b := 0, 0
	for a < len(head) && b < len(tail) {
		if bytes.Compare(k.slab[head[a][0]:head[a][1]], k.slab[tail[b][0]:tail[b][1]]) <= 0 {
			appendUnique(head[a])
			a++
			continue
		}
		appendUnique(tail[b])
		b++
	}
	for ; a < len(head); a++ {
		appendUnique(head[a])
	}
	for ; b < len(tail); b++ {
		appendUnique(tail[b])
	}
	k.spans = out
	k.sortedN = len(out)
	if dropped {
		// A dropped duplicate leaves its bytes in the slab, and Bytes reports the
		// slab. Rebuilding keeps the figure honest and keeps a set that is fed
		// the same key many times — the accumulators in referencedKeys and
		// identityKeys are — from growing without bound.
		k.compact()
	}
}

// compact rebuilds the slab from the live spans, in key order.
func (k *keys) compact() {
	slab := make([]byte, 0, len(k.slab))
	spans := make([][2]int32, 0, len(k.spans))
	for _, s := range k.spans {
		start := len(slab)
		slab = append(slab, k.slab[s[0]:s[1]]...)
		//nolint:gosec // G115: the rebuilt slab is no larger than the one it
		// replaces, whose spans already fit.
		spans = append(spans, [2]int32{int32(start), int32(len(slab))})
	}
	k.slab = slab
	k.spans = spans
}

// encodeInto appends the order-preserving encoding of one tuple to b.
func (k *keys) encodeInto(b []byte, t []keyValue) []byte {
	for i := range k.types {
		switch k.types[i].kind {
		case kindInt:
			var w [8]byte
			// The sign bit is flipped so that byte order is numeric order: -1
			// encodes below 0, which a plain big-endian two's complement does
			// not.
			//nolint:gosec // G115: a reinterpretation of the same 64 bits, not
			// a conversion; decode reverses it exactly.
			binary.BigEndian.PutUint64(w[:], uint64(t[i].i)^(1<<63))
			b = append(b, w[:]...)
		case kindUUID:
			b = append(b, t[i].u.Bytes[:]...)
		default:
			for j := 0; j < len(t[i].s); j++ {
				if c := t[i].s[j]; c == 0x00 {
					b = append(b, 0x00, 0xff)
				} else {
					b = append(b, c)
				}
			}
			b = append(b, 0x00, 0x00)
		}
	}
	return b
}

// decode reverses encodeInto into dst.
func (k *keys) decode(b []byte, dst []keyValue) {
	pos := 0
	for i := range k.types {
		switch k.types[i].kind {
		case kindInt:
			//nolint:gosec // G115: the inverse of the flip in encodeInto, over
			// the same 64 bits.
			dst[i].i = int64(binary.BigEndian.Uint64(b[pos:pos+8]) ^ (1 << 63))
			pos += 8
		case kindUUID:
			copy(dst[i].u.Bytes[:], b[pos:pos+16])
			dst[i].u.Valid = true
			pos += 16
		default:
			start := pos
			escaped := false
			for pos+1 < len(b) {
				if b[pos] != 0x00 {
					pos++
					continue
				}
				if b[pos+1] == 0x00 {
					break
				}
				escaped = true
				pos += 2
			}
			if !escaped {
				dst[i].s = string(b[start:pos])
			} else {
				out := make([]byte, 0, pos-start)
				for j := start; j < pos; {
					if b[j] == 0x00 {
						out = append(out, 0x00)
						j += 2
						continue
					}
					out = append(out, b[j])
					j++
				}
				dst[i].s = string(out)
			}
			pos += 2
		}
	}
}

// Chunks cuts the set into consecutive runs of at most n tuples, in key order.
func (k *keys) Chunks(n int) []pipeline.Chunk {
	if n <= 0 {
		n = 1
	}
	k.normalize()
	total := k.count()
	out := make([]pipeline.Chunk, 0, (total+n-1)/n)
	for start := 0; start < total; start += n {
		end := start + n
		if end > total {
			end = total
		}
		out = append(out, k.chunk(start, end))
	}
	return out
}

// chunk builds one unnest argument list: one typed array per identity column.
func (k *keys) chunk(start, end int) pipeline.Chunk {
	n := end - start
	ints := make([][]int64, len(k.types))
	strs := make([][]string, len(k.types))
	uuids := make([][]pgtype.UUID, len(k.types))
	for i, ty := range k.types {
		switch ty.kind {
		case kindInt:
			ints[i] = make([]int64, n)
		case kindUUID:
			uuids[i] = make([]pgtype.UUID, n)
		default:
			strs[i] = make([]string, n)
		}
	}
	buf := make([]keyValue, len(k.types))
	for j := 0; j < n; j++ {
		k.tupleAt(start+j, buf)
		for i, ty := range k.types {
			switch ty.kind {
			case kindInt:
				ints[i][j] = buf[i].i
			case kindUUID:
				uuids[i][j] = buf[i].u
			default:
				strs[i][j] = buf[i].s
			}
		}
	}
	cols := make([]any, len(k.types))
	for i, ty := range k.types {
		switch ty.kind {
		case kindInt:
			cols[i] = ints[i]
		case kindUUID:
			cols[i] = uuids[i]
		default:
			cols[i] = strs[i]
		}
	}
	return chunk{types: k.types, cols: cols, n: n}
}

// chunk is one unnest argument list (ARCHITECTURE.md §2 "Chunk").
type chunk struct {
	types []keyType
	cols  []any
	n     int
}

var _ pipeline.Chunk = chunk{}

// Len is the number of tuples in the chunk.
func (c chunk) Len() int { return c.n }

// Column returns the typed array for identity column i: []int64, []string or
// []pgtype.UUID. []any is never returned, because pgx cannot infer an array OID
// for it.
func (c chunk) Column(i int) any { return c.cols[i] }

// Cast is the SQL cast the unnest argument for column i carries.
func (c chunk) Cast(i int) string { return c.types[i].arrayCast }

// intKeys is the single-int8-column key set of ARCHITECTURE.md §2.
type intKeys struct{ *keys }

var _ pipeline.KeySet = intKeys{}

// Bytes is len x 8 x 2: the slice, doubled for overhead, which is the figure
// ADR-005 quotes. The doubling covers append's spare capacity and the set holds
// nothing else.
func (k intKeys) Bytes() int64 {
	k.normalize()
	return int64(len(k.ints)) * 8 * 2
}

// slabKeys is the composite or non-integer key set: a slab of encoded tuples.
type slabKeys struct{ *keys }

var _ pipeline.KeySet = slabKeys{}

// Bytes is the slab size, doubled for overhead. The doubling covers the span
// index (8 bytes a tuple) and append's spare capacity; the set holds nothing
// else.
func (k slabKeys) Bytes() int64 {
	k.normalize()
	return int64(len(k.slab)) * 2
}
