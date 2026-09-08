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
// Schema.Fingerprint is this value. Introspect leaves the field empty and
// internal/core fills it from this function after introspection, being the
// caller that has both halves (ADR-009); both ends here compute their own value
// through this function rather than reading that field, so a Schema.Fingerprint
// that came from anywhere else cannot reach either end — which is the failure
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
// around this call and ends it (Target.catalogFingerprint). This function issues
// no BEGIN of its own and must not: it would be ending a transaction it did not
// start. That transaction is REPEATABLE READ, so the catalog read below sees one
// version of the target however many statements it takes, and it is what makes a
// SAVEPOINT legal on this connection — outside a transaction block a SAVEPOINT
// is 25P01, which is what the full read's sampler takes and how the binding
// could never be confirmed before T-FPR.
//
// The read is the schema-only one when the introspector offers it
// (pipeline.SchemaOnlyIntrospector). This end hashes generated DDL, which reads
// no sample, so a full read would take a TABLESAMPLE per table whose rows are
// discarded — production values held in memory (THREAT_MODEL.md T4), from a
// database the gate has not yet agreed to touch, with internal/pg's transaction
// open for the length of it. The fall-back to the full read is correct and only
// slower: the extra fields it fills are fields SchemaFingerprint does not read,
// so both spellings hash the same catalog to the same value, which is what the
// binding requires of the marker's end and this one.
func GateFingerprint(in pipeline.Introspector) pg.TargetOption {
	return pg.WithCatalogFingerprint(gateFingerprint(in))
}

// gateFingerprint is the function GateFingerprint hands the gate. It is named
// rather than written inline so that the choice of read below is a unit test
// and not an assertion nothing runs: an introspector that reaches the gate
// wrapped — a decorator, a fake, a core refactor — satisfies pipeline.Introspector
// and not pipeline.SchemaOnlyIntrospector, the assertion misses silently, and
// the gate is back to sampling a database it has not yet agreed to touch with
// every other test in the tree still green
// (TestGateFingerprintAsksForTheSchemaOnlyRead,
// TestGateFingerprintFallsBackToTheFullRead).
func gateFingerprint(in pipeline.Introspector) pg.CatalogFingerprinter {
	return func(ctx context.Context, r pipeline.Reader) (string, error) {
		if in == nil {
			// Fail closed, as a Target with no fingerprinter does: an
			// unconfirmed binding authorises no truncation.
			return "", errors.New("load: no introspector to recompute the target schema fingerprint")
		}
		catalog := in.Introspect
		if schemaOnly, ok := in.(pipeline.SchemaOnlyIntrospector); ok {
			catalog = schemaOnly.IntrospectSchema
		}
		s, err := catalog(ctx, r)
		if err != nil {
			return "", err
		}
		return SchemaFingerprint(s)
	}
}
