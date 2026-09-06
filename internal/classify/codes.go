// SPDX-License-Identifier: Apache-2.0

package classify

import "github.com/Liarea/lazyslice/internal/event"

// The event codes the classify stage renders. Classifier.Classify takes no
// event.Sink — it is pure, and core.Run is the only producer of events
// (ARCHITECTURE.md §7) — so these are declared here, beside the results that
// produce them, and emitted by core from a *pipeline.Classification. Every one
// of them has a row in internal/event/catalogue.yml, which is the source of
// docs/ERRORS.md.
const (
	// CodeColumnMasked and CodeColumnCopied render one line each of the
	// explanation in ARCHITECTURE.md §4: the column and the reason. The reason
	// is a template rendering from reasons.go, so the line carries identifiers
	// and counts only.
	//
	// They are two codes and not one because the code is the only place the
	// verdict can land: event.Args carries no category and no confidence key
	// (internal/event/event.go, ARCHITECTURE.md §7), so a single
	// "classify.column.decided" would leave a reader of the --json stream
	// unable to tell a masked column from a copied one. That reader exists:
	// internal/invariants/i2_masking_test.go sorts every column-scoped code
	// into flaggedCodePrefixes ("classify.masked", ...) or copiedCodePrefixes
	// ("classify.copied", ...) and calls t.Fatalf on a code in neither, because
	// treating an unknown code as "not flagged" is a test that disarms itself.
	// The two spellings below are the ones that list already anticipates.
	CodeColumnMasked event.Code = "classify.masked.column"
	CodeColumnCopied event.Code = "classify.copied.column"

	// CodeColumnDrift names a column the committed lazyslice.yml has never
	// seen. It is classified fresh and masked at or above possible
	// (ADR-004 "Drift", THREAT_MODEL.md T3); --strict-schema turns the set of
	// them into CodeRefusedStrictSchema.
	CodeColumnDrift event.Code = "classify.column.drift"

	// CodeColumnOptOutExpired names a column whose --unmask opt-out was
	// ignored because the column's type fingerprint changed. A column that
	// became something else is masked again rather than staying exempt.
	CodeColumnOptOutExpired event.Code = "classify.column.opt_out_expired"

	// CodeRefusedStrictSchema is exit 10: drift under --strict-schema
	// (ADR-005's exit table).
	CodeRefusedStrictSchema event.Code = "classify.refused.strict_schema"
)
