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
// leaves exit 4 on a target lazyslice itself wrote and no way forward. There was
// a second, disagreeing definition in the tree, in internal/introspect; ADR-009
// removed it, and this is now the only one. Load computes the marker's value
// with this function rather than accepting one, and GateFingerprint below hands
// the gate the same function, so the two ends are one decision and not two.
//
// Schema.Fingerprint is this value, but nothing fills it: introspect leaves the
// field empty and both ends here compute their own value through this function,
// so the field is dead in the tree. Filling it from here is owed to
// internal/core, the caller that has both halves (ADR-009). Passing a
// Schema.Fingerprint that came from anywhere else to either end is the failure
// this function exists to make impossible.
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
// The reader it is handed is already in a transaction: internal/pg opens one
// around this call and ends it (Target.catalogFingerprint), which is what lets
// introspect take the SAVEPOINT its sampler needs — outside a transaction block
// that is 25P01, and the binding could never be confirmed. This function issues
// no BEGIN of its own and must not: it would be ending a transaction it did not
// start.
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
