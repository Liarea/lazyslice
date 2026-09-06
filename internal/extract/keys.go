// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// How a key column travels in a chunk, from extract's side of the contract.
//
// internal/plan encodes the key sets (ARCHITECTURE.md §2 "Chunk") and this
// package joins against them, so the two have to agree on one thing and only
// one: whether the value arriving through unnest has to be cast back to the
// column's own type before it is compared. Everything else extract needs is on
// the Chunk itself — Cast(i) is the array cast the argument carries, and
// Column(i) is the typed array — so it is read from there rather than derived
// again here, and a divergence between the two packages cannot silently produce
// a join that matches nothing.
//
// The cast back is the kindOther case of internal/plan's typeOf: a column of a
// type with no array form in the chunk grammar travels as its text form, and
// `t."taken_at" = k.k1` would then be a text comparison against a
// timestamptz. It is written as `k.k1::timestamp with time zone`, which is the
// form ARCHITECTURE.md §2 "Chunk" gives ("unnest($i::text[]) cast to
// <format_type>"), so the comparison is the column type's own equality.

// The type OIDs that travel in a chunk as themselves. They are stable catalog
// OIDs, not numbers looked up at run time; citext is an extension type whose
// OID is assigned at CREATE EXTENSION, so it is recognised by name below.
const (
	oidInt2    uint32 = 21
	oidInt8    uint32 = 20
	oidInt4    uint32 = 23
	oidText    uint32 = 25
	oidBpchar  uint32 = 1042
	oidVarchar uint32 = 1043
	oidUUID    uint32 = 2950
)

// joinCast is the cast the chunk side of a key comparison carries, "" when the
// value arrives already in the column's own type.
//
// bpchar is deliberately not cast back. character(n) is blank-padded on disk
// and the planner reads such a key as text; casting the chunk value back to
// character(n) would pad it and the comparison would be padded-to-padded
// against a key that was trimmed, which matches no row (internal/plan's typeOf
// records the same finding on postgres:16).
func joinCast(col pipeline.Column) string {
	switch col.TypeOID {
	case oidInt2, oidInt4, oidInt8, oidText, oidVarchar, oidBpchar, oidUUID:
		return ""
	}
	if strings.TrimPrefix(col.TypeName, "public.") == "citext" {
		return ""
	}
	return "::" + col.TypeName
}
