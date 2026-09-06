// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"errors"

	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// SchemaFingerprint is ARCHITECTURE.md section 11.2's schema_fingerprint: sha256
// over the object classes section 11.1 recreates, as the DDL text section 11.1
// says is hashed, which is ddl.Fingerprint.
//
// It exists as a name in this package because section 11.2's binding has two
// ends — the value the loader writes into the marker and the value the gate
// recomputes over the target's catalog — and a marker written with one
// definition and checked with another can never bind, which per section 11.2
// leaves exit 4 on a target lazyslice itself wrote and no way forward. There is
// a second, disagreeing definition in the tree (internal/introspect fills
// Schema.Fingerprint by hashing the catalog fields this text is rendered from),
// so which one section 11.1 means is an ADR that has not been written. What this
// package can do without it, it does: Load computes the marker's value with this
// function rather than accepting one, and GateFingerprint below hands the gate
// the same function, so the two ends are one decision and not two.
//
// Do not pass Schema.Fingerprint to either end while both definitions live. When
// the ADR lands, one of these two disappears and this stays the single name.
func SchemaFingerprint(s *pipeline.Schema) (string, error) {
	return ddl.Fingerprint(s)
}

// GateFingerprint is the gate's end of section 11.2's binding: the
// pg.TargetOption that recomputes the target's schema fingerprint with
// SchemaFingerprint, over the catalog read back by in.
//
// It takes the introspector rather than importing internal/introspect because
// stage packages do not reach each other directly (internal/CLAUDE.md); the
// caller that has both is core, and this is the one call it makes so that it
// cannot wire the gate to a definition the marker was not written with.
//
// Owed, and this function does not paper over it: internal/pg hands the
// fingerprinter a pooled connection that is not in a transaction, and
// introspect.Introspect wraps its sampling in a SAVEPOINT, which outside a
// transaction block is 25P01. Until internal/pg opens one, in must be an
// introspector that puts its own reader in a transaction — the alternative, a
// BEGIN issued here, would roll back a transaction it did not open once
// internal/pg does open one. internal/load/CLAUDE.md records it.
func GateFingerprint(in pipeline.Introspector) pg.TargetOption {
	return pg.WithCatalogFingerprint(func(ctx context.Context, r pipeline.Reader) (string, error) {
		if in == nil {
			// Fail closed, as a Target with no fingerprinter does: an
			// unconfirmed binding authorises no truncation.
			return "", errors.New("load: no introspector to recompute the target schema fingerprint")
		}
		s, err := in.Introspect(ctx, r)
		if err != nil {
			return "", err
		}
		return SchemaFingerprint(s)
	})
}
