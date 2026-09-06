// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// enc is the length-prefixed encoding of ARCHITECTURE.md §5:
//
//	enc(f1, ..., fn) = u32be(len(f1)) ‖ f1 ‖ ... ‖ u32be(len(fn)) ‖ fn
//
// It is used here for the same reason it is used there: without it
// ("email", "a@b.com") and ("emai", "la@b.com") hash alike, and a fingerprint
// that collides on a schema change is worse than no fingerprint at all.
type enc struct{ buf bytes.Buffer }

func (e *enc) field(s string) {
	var n [4]byte
	//nolint:gosec // G115: len of an in-memory string is never negative, and no
	// catalog identifier, expression or definition is four gigabytes long.
	binary.BigEndian.PutUint32(n[:], uint32(len(s)))
	e.buf.Write(n[:])
	e.buf.WriteString(s)
}

func (e *enc) fields(ss ...string) {
	for _, s := range ss {
		e.field(s)
	}
}

func (e *enc) sum() []byte {
	h := sha256.Sum256(e.buf.Bytes())
	return h[:]
}

// columnFingerprint is sha256 over (TypeOID, TypMod, Nullable, Domain), the
// four fields ARCHITECTURE.md §2 names, truncated to eight hex characters. An
// --unmask opt-out expires when it changes, so a column that was retyped does
// not silently keep its exemption.
func columnFingerprint(c pipeline.Column) string {
	var e enc
	e.fields(
		strconv.FormatUint(uint64(c.TypeOID), 10),
		strconv.FormatInt(int64(c.TypMod), 10),
		strconv.FormatBool(c.Nullable),
		c.Domain,
	)
	return hex.EncodeToString(e.sum())[:8]
}

// schemaFingerprint is sha256 over the recreated object classes only —
// ARCHITECTURE.md §11.1 items 1 to 7, in that order — so that it is the same
// hash whether it is computed on the source or on a target lazyslice wrote,
// which is what the marker's binding needs (§11.2). Views, functions, triggers
// and everything else under NotRecreated do not enter it.
//
// Three consequences of "the same on a target we wrote" are visible below: a
// leaf partition contributes nothing, because §11.1 recreates a partitioned
// source table as one plain table and the target has no partitions; the
// partition key itself contributes nothing, for the same reason; and a
// partitioned root's index enters as plainIndexDef of its definition, because
// pg_get_indexdef prints ON ONLY for one and the target's plain table prints
// the same index without it.
//
// PROVISIONAL, and it diverges from the spec. §11.1 hashes those object classes
// "as their DDL text"; this hashes the catalog fields the DDL is rendered from,
// because internal/load/ddl — the package that renders that text (§12) — is
// still the scaffold whose every function returns ErrNotImplemented, and a hash
// cannot be taken over text nothing produces. The
// two agree on what is hashed and not on how, so this is a second definition of
// "the recreated schema" living outside the package that owns the first. When
// internal/load/ddl lands, either this moves there and takes the DDL text (and
// Introspect leaves Schema.Fingerprint for its caller to fill), or §11.1 is
// amended to say "the catalog fields the DDL is rendered from" — an ADR either
// way, not a decision this package may keep making by itself. Until then the
// divergence is an owed item on T-INTROSPECT, not a settled design.
func schemaFingerprint(s *pipeline.Schema) string {
	var e enc

	recreated := make([]pipeline.Table, 0, len(s.Tables))
	for _, t := range s.Tables {
		if t.Parent != nil {
			continue
		}
		recreated = append(recreated, t)
	}

	// 1. Schemas, for every schema a recreated table lives in.
	seen := map[string]bool{}
	var schemas []string
	for _, t := range recreated {
		if !seen[t.Ref.Schema] {
			seen[t.Ref.Schema] = true
			schemas = append(schemas, t.Ref.Schema)
		}
	}
	sort.Strings(schemas)
	for _, name := range schemas {
		e.fields("schema", name)
	}

	// 2. Extensions.
	exts := append([]pipeline.Extension(nil), s.Extensions...)
	sort.Slice(exts, func(i, j int) bool { return exts[i].Name < exts[j].Name })
	for _, x := range exts {
		e.fields("extension", x.Name, x.Schema)
	}

	// 3. Enum types, domains and composite types.
	names := make([]string, 0, len(s.Enums))
	for name := range s.Enums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		e.fields("enum", name)
		e.fields(s.Enums[name]...)
	}
	for _, d := range sortedDefs(s.Domains) {
		e.fields("domain", d.Name, d.Def)
	}
	for _, d := range sortedDefs(s.Composites) {
		e.fields("composite", d.Name, d.Def)
	}

	// 4 and 5. Sequences not owned by an identity column, then tables.
	for _, t := range recreated {
		identity := map[string]bool{}
		for _, col := range t.Columns {
			if col.Identity != "" {
				identity[col.Name] = true
			}
		}
		seqs := append([]pipeline.SequenceDef(nil), t.Sequences...)
		sort.Slice(seqs, func(i, j int) bool { return seqs[i].Name < seqs[j].Name })
		for _, q := range seqs {
			if identity[q.Column] {
				continue
			}
			e.fields("sequence", q.Name, q.Column,
				strconv.FormatInt(q.Start, 10), strconv.FormatInt(q.Increment, 10),
				strconv.FormatInt(q.Min, 10), strconv.FormatInt(q.Max, 10),
				strconv.FormatInt(q.Cache, 10), strconv.FormatBool(q.Cycle))
		}
	}
	for _, t := range recreated {
		e.fields("table", t.Ref.Schema, t.Ref.Name)
		for _, col := range t.Columns {
			e.fields("column", col.Name, col.TypeName, col.Collation,
				strconv.FormatBool(col.Nullable), col.Default, col.Generated, col.Identity)
			if col.IdentitySeq != nil {
				q := col.IdentitySeq
				e.fields("identity_sequence",
					strconv.FormatInt(q.Start, 10), strconv.FormatInt(q.Increment, 10),
					strconv.FormatInt(q.Min, 10), strconv.FormatInt(q.Max, 10),
					strconv.FormatInt(q.Cache, 10), strconv.FormatBool(q.Cycle))
			}
		}
		cons := append([]pipeline.Constraint(nil), t.Constraints...)
		sort.Slice(cons, func(i, j int) bool { return cons[i].Name < cons[j].Name })
		for _, con := range cons {
			// Foreign keys are class 6, below, so that the order here is the
			// order §11.1 creates the objects in.
			if con.Kind == 'f' {
				continue
			}
			e.fields("constraint", con.Name, string(con.Kind), con.Def)
		}
	}

	// 6. After data: indexes, then foreign keys.
	for _, t := range recreated {
		idxs := append([]pipeline.Index(nil), t.Indexes...)
		sort.Slice(idxs, func(i, j int) bool { return idxs[i].Name < idxs[j].Name })
		for _, idx := range idxs {
			def := idx.Def
			if t.Partitioned {
				def = plainIndexDef(def)
			}
			e.fields("index", idx.Name, def)
		}
	}
	fks := append([]pipeline.ForeignKey(nil), s.FKs...)
	sort.Slice(fks, func(i, j int) bool { return fks[i].Name < fks[j].Name })
	for _, fk := range fks {
		if fk.Virtual {
			continue
		}
		e.fields("foreign_key", fk.Name, fk.Child.String(), fk.Parent.String(),
			strconv.FormatBool(fk.MatchFull))
		e.fields(fk.ChildCols...)
		e.fields(fk.ParentCols...)
	}

	return hex.EncodeToString(e.sum())
}

// plainIndexDef is an index definition as §11.1 recreates it on a plain table:
// pg_get_indexdef's ON ONLY, which it prints for an index on a partitioned
// table's own relation, becomes ON.
//
// Without it the hash could never bind a marker for any source holding a
// partitioned table with an index or a primary key. Verified on postgres:16: a
// partitioned table prints
//
//	CREATE UNIQUE INDEX ev_pkey ON ONLY public.ev USING btree (id, at)
//
// while the same index on a plain table prints without ONLY, and ON ONLY is
// accepted at creation but is not round-tripped — so a target lazyslice wrote,
// where §11.1 has recreated the root as one plain table, reads its own index
// back without it. Two hashes that can never be equal make §11.2's marker fall
// through to the emptiness rule, and lazyslice then refuses a non-empty target
// it wrote itself (exit 4) with no way forward; testdata/nasty.sql's
// public.events has PRIMARY KEY (event_id, occurred_at), so this is the
// project's own fixture on its second run.
//
// The scan skips quoted identifiers, so an index actually named `x ON ONLY y`
// is not rewritten inside its own name; only the keyword between the index name
// and the table is.
func plainIndexDef(def string) string {
	const onOnly = " ON ONLY "
	quoted := false
	for i := 0; i+len(onOnly) <= len(def); i++ {
		if def[i] == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if def[i:i+len(onOnly)] == onOnly {
			return def[:i] + " ON " + def[i+len(onOnly):]
		}
	}
	return def
}

func sortedDefs(in []pipeline.NamedDef) []pipeline.NamedDef {
	out := append([]pipeline.NamedDef(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
